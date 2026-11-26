// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"

	"github.com/go-steer/core-agent/v3/pkg/agent"
	"github.com/go-steer/core-agent/v3/pkg/permissions"
	"github.com/go-steer/core-agent/v3/pkg/pricing"
	"github.com/go-steer/core-agent/v3/pkg/usage"
)

// PullRequest is the change under review.
type PullRequest struct {
	Repo    string // owner/name
	Number  int
	Title   string
	Body    string
	BaseSHA string
	HeadSHA string
	Diff    string // unified diff, base...head
}

// Input is everything one review needs.
type Input struct {
	PR PullRequest
	// Workdir is a checkout of the repository at PR.HeadSHA. The review
	// can read only inside it (ReadOnlyTools).
	Workdir string
	// Rules is the repository's own guidance (AGENTS.md and the like),
	// read from the base branch: a PR cannot rewrite the rules it is
	// reviewed against.
	Rules string
	// ReadOnlyRoots are further directories the review may read, such as
	// a module cache with the change's dependencies.
	ReadOnlyRoots []string
}

// Options tune a review. The zero value works with Finder and Verifier
// set.
type Options struct {
	// Finder and Verifier are the models for the two passes; they may be
	// the same model.
	Finder, Verifier adkmodel.LLM
	// FinderLevel and VerifierLevel are thinking levels (minimal..max);
	// empty leaves the provider default.
	FinderLevel, VerifierLevel string
	// FinderMaxSteps and VerifierMaxSteps cap model calls per agent.
	// Defaults 60 and 25.
	FinderMaxSteps, VerifierMaxSteps int
	// VerifyConcurrency bounds verifiers running at once. Default 4.
	VerifyConcurrency int
	// MaxDiffBytes caps the diff placed in the finder's prompt; beyond
	// it the finder gets the file list and reads the files. Default
	// 200,000.
	MaxDiffBytes int
	// Pricing and Backend price the usage; both optional.
	Pricing *pricing.Catalog
	Backend string
}

// Result is a finished review.
type Result struct {
	// Findings are the confirmed and plausible findings, in report order.
	Findings []Finding `json:"findings"`
	// Refuted are findings verification rejected, kept for measurement.
	Refuted []Finding `json:"refuted"`
	// Summary is the finder's closing description of the change.
	Summary  string        `json:"summary"`
	Usage    usage.Totals  `json:"usage"`
	Duration time.Duration `json:"duration"`
}

// Review runs the finder over the change, then verifies each finding
// independently.
func Review(ctx context.Context, in Input, opt Options) (*Result, error) {
	if opt.Finder == nil || opt.Verifier == nil {
		return nil, errors.New("review: Finder and Verifier models are required")
	}
	if in.Workdir == "" {
		return nil, errors.New("review: Workdir is required")
	}
	opt = withDefaults(opt)
	start := time.Now()
	tracker := usage.NewTracker()
	ctx = permissions.WithWorkDir(ctx, in.Workdir)

	sink := &findingSink{}
	summary, err := runFinder(ctx, in, opt, tracker, sink)
	if err != nil {
		return nil, err
	}
	verified, refuted, err := verifyAll(ctx, in, opt, tracker, sink.all())
	if err != nil {
		return nil, err
	}
	return &Result{
		Findings: verified,
		Refuted:  refuted,
		Summary:  summary,
		Usage:    tracker.Totals(),
		Duration: time.Since(start),
	}, nil
}

func withDefaults(opt Options) Options {
	if opt.FinderMaxSteps <= 0 {
		opt.FinderMaxSteps = 60
	}
	if opt.VerifierMaxSteps <= 0 {
		opt.VerifierMaxSteps = 25
	}
	if opt.VerifyConcurrency <= 0 {
		opt.VerifyConcurrency = 4
	}
	if opt.MaxDiffBytes <= 0 {
		opt.MaxDiffBytes = 200_000
	}
	return opt
}

func newAgent(model adkmodel.LLM, instruction, level string, maxSteps int, extra tool.Tool, in Input, opt Options, tracker *usage.Tracker) (*agent.Agent, error) {
	ro, err := ReadOnlyTools(in.Workdir, in.ReadOnlyRoots...)
	if err != nil {
		return nil, err
	}
	opts := []agent.Option{
		agent.WithInstruction(instruction),
		agent.WithTools(append(ro, extra)),
		agent.WithMode(agent.ModeAutonomous),
		agent.WithMaxSteps(maxSteps),
		agent.WithFinalReport(true),
		agent.WithThinkingLevel(level),
		agent.WithUsageTracker(tracker),
		agent.WithoutSessionTitle(),
	}
	if opt.Pricing != nil {
		opts = append(opts, agent.WithPricingCatalog(opt.Pricing))
	}
	if opt.Backend != "" {
		opts = append(opts, agent.WithBillingBackend(opt.Backend))
	}
	return agent.New(model, opts...)
}

// run drives one agent turn, records each model call's usage on tracker,
// and returns the agent's last text. core-agent leaves usage recording to
// whoever drives Run (its facade does the same).
func run(ctx context.Context, a *agent.Agent, prompt string, tracker *usage.Tracker, opt Options) (string, error) {
	var (
		last string
		tap  usage.TurnTap
		rate = usage.PriceOnCatalog(opt.Pricing, opt.Backend, a.ModelName())
	)
	for ev, err := range a.Run(ctx, prompt) {
		if err != nil {
			return last, err
		}
		tap.Observe(ev)
		if u, ok := tap.Commit(ev); ok {
			tracker.AppendUsage(a.ModelName(), u, rate)
		}
		if ev == nil || ev.Content == nil || ev.Partial {
			continue
		}
		var b strings.Builder
		for _, p := range ev.Content.Parts {
			if p.Text != "" && !p.Thought {
				b.WriteString(p.Text)
			}
		}
		if t := strings.TrimSpace(b.String()); t != "" {
			last = t
		}
	}
	return last, nil
}

func runFinder(ctx context.Context, in Input, opt Options, tracker *usage.Tracker, sink *findingSink) (string, error) {
	report, err := newReportTool(sink)
	if err != nil {
		return "", err
	}
	a, err := newAgent(opt.Finder, finderInstruction, opt.FinderLevel, opt.FinderMaxSteps, report, in, opt, tracker)
	if err != nil {
		return "", fmt.Errorf("review: finder: %w", err)
	}
	summary, err := run(ctx, a, finderPrompt(in, opt.MaxDiffBytes), tracker, opt)
	if err != nil {
		return "", fmt.Errorf("review: finder: %w", err)
	}
	return summary, nil
}

func verifyAll(ctx context.Context, in Input, opt Options, tracker *usage.Tracker, findings []Finding) (verified, refuted []Finding, err error) {
	out := make([]Finding, len(findings))
	errs := make([]error, len(findings))
	sem := make(chan struct{}, opt.VerifyConcurrency)
	var wg sync.WaitGroup
	for i, f := range findings {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i], errs[i] = verifyOne(ctx, in, opt, tracker, f)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}
	for _, f := range out {
		if f.Verdict == VerdictRefuted {
			refuted = append(refuted, f)
		} else {
			verified = append(verified, f)
		}
	}
	return verified, refuted, nil
}

func verifyOne(ctx context.Context, in Input, opt Options, tracker *usage.Tracker, f Finding) (Finding, error) {
	box := &verdictBox{}
	vt, err := newVerdictTool(box)
	if err != nil {
		return f, err
	}
	a, err := newAgent(opt.Verifier, verifierInstruction, opt.VerifierLevel, opt.VerifierMaxSteps, vt, in, opt, tracker)
	if err != nil {
		return f, fmt.Errorf("review: verifier: %w", err)
	}
	if _, err := run(ctx, a, verifierPrompt(in, f), tracker, opt); err != nil {
		return f, fmt.Errorf("review: verifier for %s:%d: %w", f.File, f.Line, err)
	}
	f.Verdict, f.Reasoning = box.get()
	if f.Verdict == "" {
		// A verifier that never decided has not confirmed anything; the
		// finding is kept as plausible so it is not lost silently.
		f.Verdict, f.Reasoning = VerdictPlausible, "The verifier did not reach a verdict."
	}
	return f, nil
}
