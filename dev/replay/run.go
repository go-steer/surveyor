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

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-steer/surveyor/internal/review"
	"github.com/go-steer/surveyor/internal/source"
	"github.com/go-steer/surveyor/internal/vertex"
)

type runConfig struct {
	dir          string
	surveyorBin  string
	surveyorArgs []string
	ccLevel      string
	ccModel      string
	timeout      time.Duration
	token        string

	prAgentPython string
	prAgentModel  string
	prAgentEnv    []string
	project       string
	region        string
	claude        *vertex.Models // prices PR-Agent's tokens

	crDeep    bool
	crRetries int
	crWait    time.Duration
}

func cmdRun(ctx context.Context, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var rc runConfig
	fs.StringVar(&rc.dir, "dir", "", "replay directory")
	reviewers := fs.String("reviewers", "surveyor,claude-code", "comma-separated reviewers")
	parallel := fs.Int("parallel", 3, "reviews running at once")
	only := fs.String("only", "", "comma-separated PR numbers to run (default: all cases)")
	fs.StringVar(&rc.surveyorBin, "surveyor", "surveyor", "surveyor binary")
	sargs := fs.String("surveyor-args", "", "extra arguments for `surveyor review`, space-separated")
	fs.StringVar(&rc.ccLevel, "cc-level", "high", "effort level for claude-code's /code-review")
	fs.StringVar(&rc.ccModel, "cc-model", "", "model for claude-code (default: its configured model)")
	fs.DurationVar(&rc.timeout, "timeout", 30*time.Minute, "per review")
	fs.StringVar(&rc.prAgentPython, "pr-agent-python", "python3", "Python with pr_agent installed")
	fs.StringVar(&rc.prAgentModel, "pr-agent-model", "vertex_ai/claude-sonnet-5-5", "LiteLLM model for pr-agent")
	penv := fs.String("pr-agent-env", "", "extra KEY=VALUE settings for pr-agent, space-separated (e.g. PR_REVIEWER__NUM_MAX_FINDINGS=20)")
	fs.StringVar(&rc.project, "project", "go-steer-drydock", "GCP project for pr-agent's Vertex AI calls and pricing")
	fs.StringVar(&rc.region, "region", "global", "Vertex AI region for pr-agent")
	fs.BoolVar(&rc.crDeep, "cr-deep", true, "run coderabbit with --deep (its full PR review policy)")
	fs.IntVar(&rc.crRetries, "cr-retries", 8, "times to wait out a coderabbit rate limit")
	fs.DurationVar(&rc.crWait, "cr-wait", 10*time.Minute, "wait before retrying a rate-limited coderabbit review (set --timeout to cover it)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if rc.dir == "" {
		return errors.New("--dir is required")
	}
	rc.surveyorArgs = strings.Fields(*sargs)
	rc.prAgentEnv = strings.Fields(*penv)
	if strings.Contains(*reviewers, "pr-agent") {
		cl, err := vertex.Open(rc.project, rc.region)
		if err != nil {
			return err
		}
		// Opening the model records its billing backend for pricing.
		if _, err := cl.Model(ctx, strings.TrimPrefix(rc.prAgentModel, "vertex_ai/")); err != nil {
			return err
		}
		rc.claude = cl
	}
	rc.token = githubToken(ctx)
	cases, err := loadCases(rc.dir)
	if err != nil {
		return err
	}
	cases, err = filterCases(cases, *only)
	if err != nil {
		return err
	}

	type job struct {
		c        Case
		reviewer string
	}
	var jobs []job
	for _, c := range cases {
		for _, r := range strings.Split(*reviewers, ",") {
			r = strings.TrimSpace(r)
			if !isSurveyor(r) && !isPRAgent(r) && !isCodeRabbit(r) && r != "claude-code" {
				return fmt.Errorf("unknown reviewer %q (want claude-code, surveyor[-<variant>], pr-agent[-<variant>] or coderabbit[-<variant>])", r)
			}
			if !exists(resultPath(rc.dir, c.Number, r)) {
				jobs = append(jobs, job{c, r})
			}
		}
	}
	_, _ = fmt.Fprintf(stderr, "replay: %d reviews to run, %d at a time\n", len(jobs), *parallel)

	var (
		mu     sync.Mutex
		failed int
		wg     sync.WaitGroup
		sem    = make(chan struct{}, max(1, *parallel))
		logf   = func(format string, a ...any) {
			mu.Lock()
			defer mu.Unlock()
			_, _ = fmt.Fprintf(stderr, "%s "+format+"\n", append([]any{time.Now().Format("15:04:05")}, a...)...)
		}
	)
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			logf("start #%d %s", j.c.Number, j.reviewer)
			r, err := rc.review(ctx, j.c, j.reviewer)
			errPath := resultPath(rc.dir, j.c.Number, j.reviewer) + ".err"
			if err == nil {
				err = writeJSON(resultPath(rc.dir, j.c.Number, j.reviewer), r)
			}
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				_ = os.WriteFile(errPath, []byte(err.Error()+"\n"), 0o644)
				logf("FAIL  #%d %s: %v", j.c.Number, j.reviewer, err)
				return
			}
			_ = os.Remove(errPath)
			logf("done  #%d %s: %d findings, $%.2f, %.0fs", j.c.Number, j.reviewer, len(r.Findings), r.CostUSD, r.Seconds)
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d reviews failed (see pr-*/*.err); run again to retry them", failed, len(jobs))
	}
	return nil
}

func resultPath(dir string, n int, reviewer string) string {
	return filepath.Join(caseDir(dir, n), reviewer+".json")
}

func filterCases(cases []Case, only string) ([]Case, error) {
	if only == "" {
		return cases, nil
	}
	want := map[int]bool{}
	for _, s := range strings.Split(only, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("--only: %w", err)
		}
		want[n] = true
	}
	var out []Case
	for _, c := range cases {
		if want[c.Number] {
			out = append(out, c)
		}
	}
	return out, nil
}

func (rc *runConfig) review(ctx context.Context, c Case, reviewer string) (*Run, error) {
	ctx, cancel := context.WithTimeout(ctx, rc.timeout)
	defer cancel()
	if err := os.MkdirAll(caseDir(rc.dir, c.Number), 0o755); err != nil {
		return nil, err
	}
	if isSurveyor(reviewer) {
		return rc.runSurveyor(ctx, c, reviewer)
	}
	if isPRAgent(reviewer) {
		return rc.runPRAgent(ctx, c, reviewer)
	}
	if isCodeRabbit(reviewer) {
		return rc.runCodeRabbit(ctx, c, reviewer)
	}
	return rc.runClaudeCode(ctx, c)
}

// isSurveyor reports whether a reviewer name is surveyor or a variant of
// it ("surveyor-opus"): each variant is a run with its own
// --surveyor-args, stored under its own name.
func isSurveyor(name string) bool {
	return name == "surveyor" || strings.HasPrefix(name, "surveyor-")
}

func (rc *runConfig) runSurveyor(ctx context.Context, c Case, name string) (*Run, error) {
	args := append([]string{"review", "--repo", c.Repo, "--pr", strconv.Itoa(c.Number), "--json"}, rc.surveyorArgs...)
	logFile, err := os.Create(filepath.Join(caseDir(rc.dir, c.Number), name+".log"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = logFile.Close() }()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, rc.surveyorBin, args...)
	cmd.Stdout, cmd.Stderr = &stdout, logFile
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("surveyor: %w (see %s.log)", err, name)
	}
	var res review.Result
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return nil, fmt.Errorf("surveyor output: %w", err)
	}
	return &Run{
		Reviewer: name,
		Model:    surveyorModel(rc.surveyorArgs),
		Args:     rc.surveyorArgs,
		Coverage: &res.Coverage,
		PR:       c.Number,
		Findings: res.Findings,
		Refuted:  res.Refuted,
		CostUSD:  res.Usage.CostUSD,
		Seconds:  time.Since(start).Seconds(),
		Text:     res.Summary,
		RanAt:    start.UTC(),
	}, nil
}

// surveyorModel finds --model in surveyor's arguments the way Go's flag
// package does: one or two dashes, "=" or a separate value, last wins.
func surveyorModel(args []string) string {
	model := "claude-sonnet-5-5"
	for i := 0; i < len(args); i++ {
		a := strings.TrimPrefix(strings.TrimPrefix(args[i], "-"), "-")
		if a == "model" && i+1 < len(args) {
			model = args[i+1]
			i++
		} else if v, ok := strings.CutPrefix(a, "model="); ok {
			model = v
		}
	}
	return model
}

// checkout makes (or reuses) D/pr-N/src, the repository at the case's
// head commit.
func checkout(ctx context.Context, dir, token string, c Case) (string, error) {
	src := filepath.Join(caseDir(dir, c.Number), "src")
	if out, err := exec.CommandContext(ctx, "git", "-C", src, "rev-parse", "HEAD").Output(); err == nil &&
		strings.TrimSpace(string(out)) == c.HeadSHA {
		return src, nil
	}
	if err := os.RemoveAll(src); err != nil {
		return "", err
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		return "", err
	}
	cl := &source.Client{Token: token}
	if _, err := cl.Checkout(ctx, c.Repo, c.BaseSHA, c.HeadSHA, src); err != nil {
		return "", err
	}
	return src, nil
}

// ccDisallowed keeps claude-code a reviewer: no edits, no PR comments or
// reviews, no pushes.
const ccDisallowed = "Edit Write NotebookEdit Bash(gh pr comment:*) Bash(gh pr review:*) Bash(gh pr merge:*) Bash(gh pr edit:*) Bash(gh api:*) Bash(git push:*)"

func (rc *runConfig) runClaudeCode(ctx context.Context, c Case) (*Run, error) {
	src, err := checkout(ctx, rc.dir, rc.token, c)
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	url := fmt.Sprintf("https://github.com/%s/pull/%d", c.Repo, c.Number)
	// The checkout is the PR author's: read no project settings (hooks
	// there run commands, which --disallowedTools does not stop) and no
	// project MCP servers.
	args := []string{"-p", fmt.Sprintf("/code-review %s %s", rc.ccLevel, url),
		"--setting-sources", "user", "--strict-mcp-config",
		"--output-format", "stream-json", "--verbose", "--disallowedTools", ccDisallowed}
	if rc.ccModel != "" {
		args = append(args, "--model", rc.ccModel)
	}
	transcript := filepath.Join(caseDir(rc.dir, c.Number), "claude-code.jsonl")
	out, err := os.Create(transcript)
	if err != nil {
		return nil, err
	}
	defer func() { _ = out.Close() }()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = src, out, &stderr
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if err := out.Sync(); err != nil {
		return nil, err
	}
	r, err := parseClaudeCode(transcript)
	if err != nil {
		return nil, err
	}
	r.PR, r.RanAt = c.Number, start.UTC()
	if r.Seconds == 0 {
		r.Seconds = time.Since(start).Seconds()
	}
	return r, nil
}

// ccEvent is the part of a stream-json line parseClaudeCode reads.
type ccEvent struct {
	Type            string  `json:"type"`
	Subtype         string  `json:"subtype"`
	Model           string  `json:"model"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
	Message         *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	DurationMS   float64 `json:"duration_ms"`
	Result       string  `json:"result"`
	IsError      bool    `json:"is_error"`
}

type ccBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type ccFinding struct {
	File            string `json:"file"`
	Line            int    `json:"line"`
	Severity        string `json:"severity"`
	Category        string `json:"category"`
	Summary         string `json:"summary"`
	FailureScenario string `json:"failure_scenario"`
	Verdict         string `json:"verdict"`
}

func parseClaudeCode(path string) (*Run, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	r := &Run{Reviewer: "claude-code", NoReportTool: true}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	sawResult := false
	for sc.Scan() {
		var ev ccEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		switch {
		case ev.Type == "system" && ev.Subtype == "init":
			r.Model = ev.Model
		case ev.Type == "assistant" && ev.ParentToolUseID == nil && ev.Message != nil:
			var blocks []ccBlock
			if json.Unmarshal(ev.Message.Content, &blocks) != nil {
				continue
			}
			for _, b := range blocks {
				if b.Type != "tool_use" || b.Name != "ReportFindings" {
					continue
				}
				var in struct {
					Findings []ccFinding `json:"findings"`
				}
				if err := json.Unmarshal(b.Input, &in); err != nil {
					return nil, fmt.Errorf("ReportFindings input: %w", err)
				}
				// The last call wins: a re-report replaces the first.
				r.NoReportTool = false
				r.Findings = r.Findings[:0]
				for _, cf := range in.Findings {
					r.Findings = append(r.Findings, review.Finding{
						File: cf.File, Line: cf.Line, Severity: review.Severity(cf.Severity),
						Category: cf.Category, Summary: cf.Summary, FailureScenario: cf.FailureScenario,
						Verdict: review.Verdict(strings.ToLower(cf.Verdict)),
					})
				}
			}
		case ev.Type == "result":
			sawResult = true
			if ev.IsError {
				return nil, fmt.Errorf("claude-code ended with an error (%s): %s", ev.Subtype, ev.Result)
			}
			r.CostUSD, r.Seconds, r.Text = ev.TotalCostUSD, ev.DurationMS/1000, ev.Result
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !sawResult {
		return nil, errors.New("claude-code transcript has no result line")
	}
	return r, nil
}

// githubToken finds a token without asking: GH_TOKEN, GITHUB_TOKEN, then
// the gh CLI's stored login.
func githubToken(ctx context.Context) string {
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
