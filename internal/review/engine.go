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
	"google.golang.org/genai"

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
	// FinderStyle is FinderStrict (the default) or FinderBroad; see
	// prompts.go.
	FinderStyle string
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
	Summary string `json:"summary"`
	// Coverage says whether the review saw the whole change.
	Coverage Coverage      `json:"coverage"`
	Usage    usage.Totals  `json:"usage"`
	Duration time.Duration `json:"duration"`
}

// Coverage records how much of the change the review could have read. A
// review that hit a cap may have missed defects, so "no findings" from it
// is not evidence the change is clean.
type Coverage struct {
	// DiffBytes is the size of the diff; DiffTruncated is set when it was
	// over MaxDiffBytes, so the finder got only the file list.
	DiffBytes     int  `json:"diff_bytes"`
	DiffTruncated bool `json:"diff_truncated"`
	// FinderCalls is the finder's model calls; FinderHitCap is set when it
	// reached FinderMaxSteps and had to stop.
	FinderCalls    int  `json:"finder_calls"`
	FinderMaxSteps int  `json:"finder_max_steps"`
	FinderHitCap   bool `json:"finder_hit_cap"`
	// VerifiersHitCap counts verifiers that reached VerifierMaxSteps.
	VerifiersHitCap int `json:"verifiers_hit_cap"`
}

// Complete reports whether nothing was cut short.
func (c Coverage) Complete() bool {
	return !c.DiffTruncated && !c.FinderHitCap && c.VerifiersHitCap == 0
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

	sink := &findingSink{}
	finder, err := runFinder(ctx, in, opt, tracker, sink)
	if err != nil {
		return nil, err
	}
	verified, refuted, capped, err := verifyAll(ctx, in, opt, tracker, sink.all())
	if err != nil {
		return nil, err
	}
	return &Result{
		Findings: verified,
		Refuted:  refuted,
		Summary:  finder.Text,
		Coverage: Coverage{
			DiffBytes:       len(in.PR.Diff),
			DiffTruncated:   len(in.PR.Diff) > opt.MaxDiffBytes,
			FinderCalls:     finder.ModelCalls,
			FinderMaxSteps:  opt.FinderMaxSteps,
			FinderHitCap:    finder.HitStepCap,
			VerifiersHitCap: capped,
		},
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

// AgentSpec is one sandboxed agent: the finder, a verifier, or a
// caller's own (such as dev/replay's judge).
type AgentSpec struct {
	Model       adkmodel.LLM
	Instruction string
	// Level is the thinking level; empty leaves the provider default.
	Level    string
	MaxSteps int
	// Tools are added to the read-only tools; typically the one tool the
	// agent reports its result through.
	Tools []tool.Tool
}

// AgentRun is one finished agent turn.
type AgentRun struct {
	// Text is the agent's last text.
	Text string
	// ModelCalls counts the turn's model calls. HitStepCap is set when
	// the agent reached MaxSteps: its tools were taken away and it had
	// to report with what it had.
	ModelCalls int
	HitStepCap bool
}

// RunAgent runs one turn of a sandboxed agent that can read only
// in.Workdir and in.ReadOnlyRoots and records each model call's usage on
// tracker.
func RunAgent(ctx context.Context, in Input, opt Options, tracker *usage.Tracker, spec AgentSpec, prompt string) (AgentRun, error) {
	if in.Workdir == "" {
		return AgentRun{}, errors.New("review: Workdir is required")
	}
	a, err := newAgent(spec, in, opt, tracker)
	if err != nil {
		return AgentRun{}, err
	}
	r, err := run(permissions.WithWorkDir(ctx, in.Workdir), a, prompt, tracker, opt)
	// core-agent's step gate allows MaxSteps calls, then one tool-less
	// final-report call: more calls than MaxSteps means the cap was hit.
	r.HitStepCap = spec.MaxSteps > 0 && r.ModelCalls > spec.MaxSteps
	return r, err
}

func newAgent(spec AgentSpec, in Input, opt Options, tracker *usage.Tracker) (*agent.Agent, error) {
	ro, err := ReadOnlyTools(in.Workdir, in.ReadOnlyRoots...)
	if err != nil {
		return nil, err
	}
	opts := []agent.Option{
		agent.WithInstruction(spec.Instruction),
		agent.WithTools(append(ro, spec.Tools...)),
		agent.WithMode(agent.ModeAutonomous),
		agent.WithMaxSteps(spec.MaxSteps),
		agent.WithFinalReport(true),
		agent.WithThinkingLevel(spec.Level),
		agent.WithUsageTracker(tracker),
		agent.WithoutSessionTitle(),
	}
	if opt.Pricing != nil {
		opts = append(opts, agent.WithPricingCatalog(opt.Pricing))
	}
	if opt.Backend != "" {
		opts = append(opts, agent.WithBillingBackend(opt.Backend))
	}
	return agent.New(spec.Model, opts...)
}

// run drives one agent turn and records each model call's usage on
// tracker. core-agent leaves usage recording to whoever drives Run (its
// facade does the same).
func run(ctx context.Context, a *agent.Agent, prompt string, tracker *usage.Tracker, opt Options) (AgentRun, error) {
	var (
		r    AgentRun
		tap  usage.TurnTap
		rate = usage.PriceOnCatalog(opt.Pricing, opt.Backend, a.ModelName())
	)
	for ev, err := range a.Run(ctx, prompt) {
		if err != nil {
			return r, err
		}
		tap.Observe(ev)
		if u, ok := tap.Commit(ev); ok {
			tracker.AppendUsage(a.ModelName(), u, rate)
		}
		if ev == nil || ev.Content == nil || ev.Partial {
			continue
		}
		// One complete model-role event per model call (tool results come
		// back as user-role events).
		if ev.Content.Role == genai.RoleModel {
			r.ModelCalls++
		}
		var b strings.Builder
		for _, p := range ev.Content.Parts {
			if p.Text != "" && !p.Thought {
				b.WriteString(p.Text)
			}
		}
		if t := strings.TrimSpace(b.String()); t != "" {
			r.Text = t
		}
	}
	return r, nil
}

func runFinder(ctx context.Context, in Input, opt Options, tracker *usage.Tracker, sink *findingSink) (AgentRun, error) {
	report, err := newReportTool(sink)
	if err != nil {
		return AgentRun{}, err
	}
	instruction, err := finderInstruction(opt.FinderStyle)
	if err != nil {
		return AgentRun{}, err
	}
	spec := AgentSpec{Model: opt.Finder, Instruction: instruction, Level: opt.FinderLevel, MaxSteps: opt.FinderMaxSteps, Tools: []tool.Tool{report}}
	r, err := RunAgent(ctx, in, opt, tracker, spec, finderPrompt(in, opt.MaxDiffBytes))
	if err != nil {
		return r, fmt.Errorf("review: finder: %w", err)
	}
	return r, nil
}

func verifyAll(ctx context.Context, in Input, opt Options, tracker *usage.Tracker, findings []Finding) (verified, refuted []Finding, capped int, err error) {
	out := make([]Finding, len(findings))
	hit := make([]bool, len(findings))
	errs := make([]error, len(findings))
	sem := make(chan struct{}, opt.VerifyConcurrency)
	var wg sync.WaitGroup
	for i, f := range findings {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i], hit[i], errs[i] = verifyOne(ctx, in, opt, tracker, f)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, nil, 0, err
	}
	for i, f := range out {
		if hit[i] {
			capped++
		}
		if f.Verdict == VerdictRefuted {
			refuted = append(refuted, f)
		} else {
			verified = append(verified, f)
		}
	}
	return verified, refuted, capped, nil
}

func verifyOne(ctx context.Context, in Input, opt Options, tracker *usage.Tracker, f Finding) (Finding, bool, error) {
	box := &verdictBox{}
	vt, err := newVerdictTool(box)
	if err != nil {
		return f, false, err
	}
	spec := AgentSpec{Model: opt.Verifier, Instruction: verifierInstruction, Level: opt.VerifierLevel, MaxSteps: opt.VerifierMaxSteps, Tools: []tool.Tool{vt}}
	r, err := RunAgent(ctx, in, opt, tracker, spec, verifierPrompt(in, f))
	if err != nil {
		return f, false, fmt.Errorf("review: verifier for %s:%d: %w", f.File, f.Line, err)
	}
	f.Verdict, f.Reasoning = box.get()
	if f.Verdict == "" {
		// A verifier that never decided has not confirmed anything; the
		// finding is kept as plausible so it is not lost silently.
		f.Verdict, f.Reasoning = VerdictPlausible, "The verifier did not reach a verdict."
	}
	return f, r.HitStepCap, nil
}
