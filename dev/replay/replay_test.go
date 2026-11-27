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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-steer/surveyor/internal/review"
)

func writeLines(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseClaudeCode(t *testing.T) {
	init := `{"type":"system","subtype":"init","model":"claude-opus-5-5"}`
	report := func(file string) string {
		return `{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"tool_use","name":"ReportFindings","input":{"findings":[{"file":"` + file + `","line":7,"severity":"important","summary":"s","failure_scenario":"f","verdict":"CONFIRMED"}]}}]}}`
	}
	// A subagent's tool call must not count as the reviewer's report.
	sub := `{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"content":[{"type":"tool_use","name":"ReportFindings","input":{"findings":[{"file":"sub.go","line":1,"severity":"nit","summary":"x","failure_scenario":"y"}]}}]}}`
	userText := `{"type":"user","message":{"content":"plain string content"}}`
	result := `{"type":"result","subtype":"success","total_cost_usd":1.5,"duration_ms":2000,"result":"done"}`

	r, err := parseClaudeCode(writeLines(t, init, userText, report("a.go"), sub, report("b.go"), result))
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "claude-opus-5-5" || r.CostUSD != 1.5 || r.Seconds != 2 || r.Text != "done" || r.NoReportTool {
		t.Errorf("run = %+v", r)
	}
	if len(r.Findings) != 1 || r.Findings[0].File != "b.go" || r.Findings[0].Verdict != review.VerdictConfirmed {
		t.Errorf("findings = %+v, want only the last report's b.go, verdict lower-cased", r.Findings)
	}

	r, err = parseClaudeCode(writeLines(t, init, result))
	if err != nil || !r.NoReportTool || len(r.Findings) != 0 {
		t.Errorf("no report: run %+v, err %v; want NoReportTool and no findings", r, err)
	}

	if _, err := parseClaudeCode(writeLines(t, init, `{"type":"result","subtype":"error_max_turns","is_error":true,"result":"x"}`)); err == nil {
		t.Error("error result: want an error")
	}
	if _, err := parseClaudeCode(writeLines(t, init, report("a.go"))); err == nil {
		t.Error("truncated transcript: want an error")
	}
}

func TestSkip(t *testing.T) {
	pr := func(login, title string, files ...string) ghPR {
		p := ghPR{Title: title}
		p.Author.Login = login
		for _, f := range files {
			p.Files = append(p.Files, struct {
				Path string `json:"path"`
			}{f})
		}
		return p
	}
	for _, tc := range []struct {
		p    ghPR
		want string
	}{
		{pr("app/dependabot", "build(deps): bump x", "go.mod"), "bot"},
		{pr("me", "chore(changelog): promote", "CHANGELOG.md"), "release chore"},
		{pr("me", "docs: x", "README.md", "docs/a.md"), "no code"},
		{pr("me", "fix: y", "README.md", "pkg/a.go"), ""},
		{pr("me", "ci: z", ".github/workflows/ci.yml"), ""},
	} {
		if got := skip(tc.p); got != tc.want {
			t.Errorf("skip(%q) = %q, want %q", tc.p.Title, got, tc.want)
		}
	}
}

// TestReport checks the arithmetic on a hand-built judged case: grouping,
// known bugs, verifier losses and false alarms.
func TestReport(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(writeJSON(filepath.Join(dir, "cases.json"), []Case{{Repo: "o/r", Number: 7, Title: "feat: x"}}))
	f := func(s string) review.Finding { return review.Finding{File: "a.go", Line: 1, Summary: s} }
	must(writeJSON(resultPath(dir, 7, "surveyor"), Run{Reviewer: "surveyor", Model: "sonnet", CostUSD: 0.5, Seconds: 60,
		Findings: []review.Finding{f("bug A"), f("wrong B")}, Refuted: []review.Finding{f("bug C")}}))
	must(writeJSON(resultPath(dir, 7, "claude-code"), Run{Reviewer: "claude-code", Model: "opus", CostUSD: 2, Seconds: 200,
		Findings: []review.Finding{f("bug A again"), f("bug C"), f("tiny")}}))
	must(writeJSON(filepath.Join(caseDir(dir, 7), "judge.json"), Judged{PR: 7, Model: "judge", Reviewers: []string{"surveyor", "claude-code"},
		KnownBugs: []KnownBug{{IntroducingPR: 7, FixPR: 9, Description: "bug A"}},
		Judgements: []Judgement{
			{ID: "F1", Reviewer: "claude-code", Finding: f("bug A again"), Label: "real", Group: "F1", KnownBug: 0},
			{ID: "F2", Reviewer: "surveyor", Finding: f("bug A"), Label: "real", Group: "F1", KnownBug: 0},
			{ID: "F3", Reviewer: "surveyor", Finding: f("wrong B"), Label: "false_positive", Group: "F3", KnownBug: -1},
			{ID: "F4", Reviewer: "surveyor", Refuted: true, Finding: f("bug C"), Label: "real", Group: "F4", KnownBug: -1},
			{ID: "F5", Reviewer: "claude-code", Finding: f("bug C"), Label: "real", Group: "F4", KnownBug: -1},
			{ID: "F6", Reviewer: "claude-code", Finding: f("tiny"), Label: "nit", Group: "F6", KnownBug: -1},
		}}))
	var out, errOut bytes.Buffer
	if err := cmdReport([]string{"--dir", dir}, &out, &errOut); err != nil {
		t.Fatalf("report: %v\n%s", err, errOut.String())
	}
	got := out.String()
	for _, want := range []string{
		"| | claude-code | surveyor |",
		"| findings reported | 3 | 2 |",
		"| real / nit / false alarm | 2 / 1 / 0 | 1 / 0 / 1 |",
		"| defects found (of 2 found by anyone) | 2 (100%) | 1 (50%) |",
		"| defects only this reviewer found | 1 | 0 |",
		"| known bugs caught (of 1) | 1 | 1 |",
		"0 false alarms and 0 nits correctly dropped, 1 real findings lost",
		"| #7 | `a.go:1` | bug C | yes | refuted |",
		"#7 `a.go:1` wrong B",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q\n%s", want, got)
		}
	}
}
