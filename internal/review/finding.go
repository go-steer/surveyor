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
	"errors"
	"fmt"
	"strings"
	"sync"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// Severity says whether a finding should block approval.
type Severity string

const (
	// SeverityImportant is a defect this change introduces. Only these
	// block approval.
	SeverityImportant Severity = "important"
	// SeverityPreExisting is a real defect the change did not introduce:
	// reported, never blocking.
	SeverityPreExisting Severity = "pre_existing"
	// SeverityNit is minor.
	SeverityNit Severity = "nit"
)

// Verdict is the verifier's judgement of a finding.
type Verdict string

const (
	// VerdictConfirmed: the verifier traced the failure in the code.
	VerdictConfirmed Verdict = "confirmed"
	// VerdictPlausible: likely, but it rests on something the code alone
	// cannot settle (a caller outside the repo, runtime configuration).
	VerdictPlausible Verdict = "plausible"
	// VerdictRefuted: the failure cannot happen as described.
	VerdictRefuted Verdict = "refuted"
)

// Finding is one problem the review found.
type Finding struct {
	File            string   `json:"file"`
	Line            int      `json:"line"`
	Severity        Severity `json:"severity"`
	Category        string   `json:"category"`
	Summary         string   `json:"summary"`
	FailureScenario string   `json:"failure_scenario"`

	// Set by verification.
	Verdict   Verdict `json:"verdict,omitempty"`
	Reasoning string  `json:"reasoning,omitempty"`
}

func (f Finding) key() string {
	return fmt.Sprintf("%s:%d:%s", f.File, f.Line, strings.ToLower(strings.TrimSpace(f.Summary)))
}

// reportArgs is what the finder sends for one finding. The jsonschema
// descriptions are prompt text: they say what each field must contain.
type reportArgs struct {
	File            string `json:"file" jsonschema:"Path of the file, relative to the repository root, as it is after the change."`
	Line            int    `json:"line" jsonschema:"Line number in the changed file where the problem is, as it is after the change."`
	Severity        string `json:"severity" jsonschema:"important: a defect this change introduces; pre_existing: a real defect the change did not introduce; nit: minor."`
	Category        string `json:"category" jsonschema:"One short kebab-case word for the kind of problem, e.g. correctness, concurrency, security, error-handling, test-coverage, docs."`
	Summary         string `json:"summary" jsonschema:"The defect in one sentence."`
	FailureScenario string `json:"failure_scenario" jsonschema:"The concrete inputs or state and the wrong result they produce, traced through the code you read."`
}

type reportResult struct {
	Recorded bool `json:"recorded"`
}

// findingSink collects reported findings, dropping exact duplicates.
type findingSink struct {
	mu       sync.Mutex
	seen     map[string]bool
	findings []Finding
}

func (s *findingSink) add(f Finding) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if s.seen[f.key()] {
		return false
	}
	s.seen[f.key()] = true
	s.findings = append(s.findings, f)
	return true
}

func (s *findingSink) all() []Finding {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Finding(nil), s.findings...)
}

func newReportTool(sink *findingSink) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "report_finding",
		Description: "Record one problem in the change. Call it once per distinct problem, " +
			"only after reading the code that shows the failure.",
	}, func(_ adkagent.Context, in reportArgs) (reportResult, error) {
		f := Finding{
			File:            strings.TrimPrefix(strings.TrimSpace(in.File), "./"),
			Line:            in.Line,
			Severity:        Severity(strings.TrimSpace(in.Severity)),
			Category:        strings.TrimSpace(in.Category),
			Summary:         strings.TrimSpace(in.Summary),
			FailureScenario: strings.TrimSpace(in.FailureScenario),
		}
		switch f.Severity {
		case SeverityImportant, SeverityPreExisting, SeverityNit:
		default:
			return reportResult{}, fmt.Errorf("severity must be important, pre_existing or nit, not %q", in.Severity)
		}
		if f.File == "" || f.Summary == "" || f.FailureScenario == "" {
			return reportResult{}, errors.New("file, summary and failure_scenario are required")
		}
		return reportResult{Recorded: sink.add(f)}, nil
	})
}

type verdictArgs struct {
	Verdict   string `json:"verdict" jsonschema:"confirmed: you traced the failure in the code; plausible: likely, but it depends on something the code cannot settle; refuted: it cannot happen as described."`
	Reasoning string `json:"reasoning" jsonschema:"What you read and why it supports the verdict, citing files and lines."`
}

type verdictResult struct {
	Recorded bool `json:"recorded"`
}

// verdictBox holds a verifier's single verdict.
type verdictBox struct {
	mu        sync.Mutex
	verdict   Verdict
	reasoning string
}

func (b *verdictBox) get() (Verdict, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.verdict, b.reasoning
}

func newVerdictTool(box *verdictBox) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "submit_verdict",
		Description: "Record your verdict on the claimed problem. Call it exactly once, after reading the code.",
	}, func(_ adkagent.Context, in verdictArgs) (verdictResult, error) {
		v := Verdict(strings.TrimSpace(in.Verdict))
		switch v {
		case VerdictConfirmed, VerdictPlausible, VerdictRefuted:
		default:
			return verdictResult{}, fmt.Errorf("verdict must be confirmed, plausible or refuted, not %q", in.Verdict)
		}
		box.mu.Lock()
		defer box.mu.Unlock()
		box.verdict, box.reasoning = v, strings.TrimSpace(in.Reasoning)
		return verdictResult{Recorded: true}, nil
	})
}
