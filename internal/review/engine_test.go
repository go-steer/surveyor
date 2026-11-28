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
	"iter"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// scriptLLM answers each request with step(req); a nil content ends the
// turn with "done".
type scriptLLM struct {
	step func(req *adkmodel.LLMRequest) *genai.Content
}

func (s *scriptLLM) Name() string { return "script" }

func (s *scriptLLM) GenerateContent(_ context.Context, req *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	c := s.step(req)
	if c == nil {
		c = &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "done"}}}
	}
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		yield(&adkmodel.LLMResponse{Content: c, TurnComplete: true}, nil)
	}
}

func calls(fcs ...*genai.FunctionCall) *genai.Content {
	c := &genai.Content{Role: genai.RoleModel}
	for _, fc := range fcs {
		c.Parts = append(c.Parts, &genai.Part{FunctionCall: fc})
	}
	return c
}

func hasToolResponse(req *adkmodel.LLMRequest) bool {
	for _, c := range req.Contents {
		for _, p := range c.Parts {
			if p.FunctionResponse != nil {
				return true
			}
		}
	}
	return false
}

func userText(req *adkmodel.LLMRequest) string {
	var b strings.Builder
	for _, c := range req.Contents {
		if c.Role != genai.RoleUser {
			continue
		}
		for _, p := range c.Parts {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func finding(id, file string, line int, sev, summary string) *genai.FunctionCall {
	return &genai.FunctionCall{ID: id, Name: "report_finding", Args: map[string]any{
		"file": file, "line": line, "severity": sev, "category": "correctness",
		"summary": summary, "failure_scenario": "an empty slice reaches index 0 and panics",
	}}
}

func TestReviewFindsThenVerifies(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/main.go", "package main\n")

	var finderPrompt string
	finder := &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
		if hasToolResponse(req) {
			return &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "Adds a helper; checked callers."}}}
		}
		finderPrompt = userText(req)
		return calls(
			finding("f1", "main.go", 3, "important", "Index without length check"),
			finding("f2", "main.go", 9, "nit", "Bogus claim"),
			finding("f3", "./main.go", 3, "important", "Index without length check"), // duplicate
		)
	}}

	var inFlight, maxInFlight atomic.Int32
	var mu sync.Mutex
	verifierPrompts := map[string]bool{}
	verifier := &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
		if hasToolResponse(req) {
			return nil
		}
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		p := userText(req)
		mu.Lock()
		verifierPrompts[p] = true
		mu.Unlock()
		v := "confirmed"
		if strings.Contains(p, "Bogus claim") {
			v = "refuted"
		}
		return calls(&genai.FunctionCall{ID: "v", Name: "submit_verdict", Args: map[string]any{"verdict": v, "reasoning": "read main.go:3"}})
	}}

	res, err := Review(context.Background(), Input{
		PR: PullRequest{
			Repo: "go-steer/example", Number: 7, Title: "Add helper", HeadSHA: "abc123",
			Diff: "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1,3 @@\n package main\n+func f(s []int) int { return s[0] }\n",
		},
		Workdir: root,
		Rules:   "Every exported function needs a test.",
	}, Options{Finder: finder, Verifier: verifier})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Summary != "Index without length check" || res.Findings[0].Verdict != VerdictConfirmed {
		t.Fatalf("findings = %+v; want the one confirmed finding", res.Findings)
	}
	if len(res.Refuted) != 1 || res.Refuted[0].Summary != "Bogus claim" {
		t.Errorf("refuted = %+v; want the bogus claim", res.Refuted)
	}
	if res.Summary != "Adds a helper; checked callers." {
		t.Errorf("summary = %q", res.Summary)
	}
	if len(verifierPrompts) != 2 {
		t.Errorf("verifier ran %d times; want 2 (the duplicate dropped)", len(verifierPrompts))
	}
	if maxInFlight.Load() < 2 {
		t.Errorf("verifiers ran one at a time; want them concurrent")
	}
	for _, want := range []string{"<diff>", "<repository_rules>", "Every exported function needs a test.", "<pull_request_title>"} {
		if !strings.Contains(finderPrompt, want) {
			t.Errorf("finder prompt lacks %q", want)
		}
	}
}

func TestReviewRequiresModels(t *testing.T) {
	if _, err := Review(context.Background(), Input{Workdir: t.TempDir()}, Options{}); err == nil {
		t.Fatal("want an error without models")
	}
}

func TestDiffHelpers(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\ndiff --git a/dir/b.go b/dir/b.go\n--- a/dir/b.go\n+++ b/dir/b.go\n@@ -1 +1 @@\n-p\n+q\n"
	if got := changedFiles(diff); len(got) != 2 || got[0] != "a.go" || got[1] != "dir/b.go" {
		t.Errorf("changedFiles = %v", got)
	}
	if h := fileDiff(diff, "dir/b.go"); !strings.Contains(h, "+q") || strings.Contains(h, "+y") {
		t.Errorf("fileDiff(dir/b.go) = %q", h)
	}
	if h := fileDiff(diff, "a.go"); !strings.Contains(h, "+y") || strings.Contains(h, "+q") {
		t.Errorf("fileDiff(a.go) = %q", h)
	}
	if fileDiff(diff, "missing.go") != "" {
		t.Error("fileDiff for an untouched file should be empty")
	}
}

// systemText is the request's system instruction.
func systemText(req *adkmodel.LLMRequest) string {
	var b strings.Builder
	if req.Config != nil && req.Config.SystemInstruction != nil {
		for _, p := range req.Config.SystemInstruction.Parts {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func TestReviewCoverage(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/main.go", "package main\n")
	diff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1,2 @@\n package main\n+var x = 1\n"
	in := Input{PR: PullRequest{Repo: "o/r", Number: 1, Diff: diff}, Workdir: root}

	// A finder that never stops calling tools while it has them: it runs
	// into its cap, and so does each verifier.
	var finderSystem string
	busy := func(call *genai.FunctionCall) *scriptLLM {
		return &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
			if req.Config == nil || len(req.Config.Tools) == 0 {
				return &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "out of calls"}}}
			}
			if strings.Contains(systemText(req), "reviewing a pull request") {
				finderSystem = systemText(req)
			}
			return calls(call)
		}}
	}
	finder := busy(finding("f", "main.go", 2, "important", "Same claim every time"))
	verifier := busy(&genai.FunctionCall{ID: "s", Name: "stat", Args: map[string]any{"path": "main.go"}})
	res, err := Review(context.Background(), in, Options{Finder: finder, Verifier: verifier,
		FinderStyle: FinderBroad, FinderMaxSteps: 3, VerifierMaxSteps: 2, MaxDiffBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Coverage
	if !c.FinderHitCap || c.FinderCalls != 4 || c.FinderMaxSteps != 3 {
		t.Errorf("finder coverage = %+v; want the cap hit after 3 calls plus the final report", c)
	}
	if c.VerifiersHitCap != 1 {
		t.Errorf("VerifiersHitCap = %d, want 1", c.VerifiersHitCap)
	}
	if !c.DiffTruncated || c.DiffBytes != len(diff) {
		t.Errorf("diff coverage = %+v; want truncated at 10 bytes", c)
	}
	if c.Complete() {
		t.Error("Complete() = true for a capped review")
	}
	if !strings.Contains(finderSystem, "a missed defect costs more here than a wrong one") {
		t.Errorf("FinderBroad did not select the broad prompt:\n%s", finderSystem)
	}

	// A finder that answers at once read the whole (small) change.
	quick := &scriptLLM{step: func(*adkmodel.LLMRequest) *genai.Content { return nil }}
	res, err = Review(context.Background(), in, Options{Finder: quick, Verifier: quick})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Coverage.Complete() || res.Coverage.FinderCalls != 1 {
		t.Errorf("coverage = %+v; want complete after one call", res.Coverage)
	}

	if _, err := Review(context.Background(), in, Options{Finder: quick, Verifier: quick, FinderStyle: "loose"}); err == nil {
		t.Error("unknown finder style: want an error")
	}
}

func TestReviewMoreFinders(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/main.go", "package main\n")
	in := Input{PR: PullRequest{Repo: "o/r", Number: 1, Diff: "diff --git a/main.go b/main.go\n"}, Workdir: root}
	finderReporting := func(fcs ...*genai.FunctionCall) *scriptLLM {
		return &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
			if hasToolResponse(req) {
				return nil
			}
			return calls(fcs...)
		}}
	}
	a := finderReporting(finding("a", "main.go", 1, "important", "Shared defect"), finding("b", "main.go", 2, "important", "Only A sees this"))
	b := finderReporting(finding("c", "main.go", 1, "important", "Shared defect"), finding("d", "main.go", 3, "important", "Only B sees this"))
	confirm := &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
		if hasToolResponse(req) {
			return nil
		}
		return calls(&genai.FunctionCall{ID: "v", Name: "submit_verdict", Args: map[string]any{"verdict": "confirmed", "reasoning": "read it"}})
	}}
	res, err := Review(context.Background(), in, Options{Finder: a, MoreFinders: []adkmodel.LLM{b}, Verifier: confirm})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range res.Findings {
		got[f.Summary] = true
	}
	if len(res.Findings) != 3 || !got["Shared defect"] || !got["Only A sees this"] || !got["Only B sees this"] {
		t.Errorf("findings = %+v; want the union of both passes with the shared one once", res.Findings)
	}
	if res.Coverage.FinderCalls != 4 {
		t.Errorf("FinderCalls = %d, want 4 (two calls per pass)", res.Coverage.FinderCalls)
	}
}

// TestReviewSalvagesCappedFinder: a finder that reaches its cap and writes
// its finding as text still gets that finding verified.
func TestReviewSalvagesCappedFinder(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/main.go", "package main\n")
	in := Input{PR: PullRequest{Repo: "o/r", Number: 1, Diff: "diff --git a/main.go b/main.go\n"}, Workdir: root}
	finder := &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
		if req.Config == nil || len(req.Config.Tools) == 0 {
			return &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "Findings: main.go:1 (important) the slice index panics on empty input."}}}
		}
		return calls(&genai.FunctionCall{ID: "s", Name: "stat", Args: map[string]any{"path": "main.go"}})
	}}
	var salvagePrompt string
	verifier := &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
		if hasToolResponse(req) {
			return nil
		}
		if strings.Contains(userText(req), "<review_text>") {
			salvagePrompt = userText(req)
			return calls(finding("r", "main.go", 1, "important", "Slice index panics on empty input"))
		}
		return calls(&genai.FunctionCall{ID: "v", Name: "submit_verdict", Args: map[string]any{"verdict": "confirmed", "reasoning": "read it"}})
	}}
	res, err := Review(context.Background(), in, Options{Finder: finder, Verifier: verifier, FinderMaxSteps: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(salvagePrompt, "the slice index panics") {
		t.Errorf("salvage prompt lacks the finder's text: %q", salvagePrompt)
	}
	if len(res.Findings) != 1 || res.Findings[0].Verdict != VerdictConfirmed || res.Coverage.FinderSalvaged != 1 || !res.Coverage.FinderHitCap {
		t.Errorf("findings = %+v, coverage = %+v; want the salvaged finding verified", res.Findings, res.Coverage)
	}
}

func TestReviewFinderStyles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/main.go", "package main\n")
	in := Input{PR: PullRequest{Repo: "o/r", Number: 1, Diff: "diff --git a/main.go b/main.go\n"}, Workdir: root}
	var mu sync.Mutex
	var systems []string
	finder := &scriptLLM{step: func(req *adkmodel.LLMRequest) *genai.Content {
		mu.Lock()
		systems = append(systems, systemText(req))
		mu.Unlock()
		return nil
	}}
	quiet := &scriptLLM{step: func(*adkmodel.LLMRequest) *genai.Content { return nil }}

	if _, err := Review(context.Background(), in, Options{Finder: finder, Verifier: quiet, FinderStyle: FinderLenses}); err != nil {
		t.Fatal(err)
	}
	if len(systems) != 3 {
		t.Fatalf("lenses ran %d passes, want 3", len(systems))
	}
	for _, want := range []string{"logic and contracts", "completeness and consistency", "concurrency, resources, security"} {
		found := 0
		for _, s := range systems {
			if strings.Contains(s, want) {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%d lens passes focus on %q, want exactly 1", found, want)
		}
	}

	systems = nil
	if _, err := Review(context.Background(), in, Options{Finder: finder, Verifier: quiet, FinderStyle: FinderOmissions}); err != nil {
		t.Fatal(err)
	}
	if len(systems) != 1 || !strings.Contains(systems[0], "A missing update is a defect") || !strings.Contains(systems[0], "a missed defect costs more here") {
		t.Errorf("omissions should be one broad pass with the completeness check:\n%v", systems)
	}
}
