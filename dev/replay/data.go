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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-steer/surveyor/internal/review"
)

// Case is one merged pull request to replay.
type Case struct {
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	BaseSHA   string    `json:"base_sha"`
	HeadSHA   string    `json:"head_sha"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	MergedAt  time.Time `json:"merged_at"`
}

// KnownBug is a defect a case introduced and a later pull request fixed.
type KnownBug struct {
	IntroducingPR int      `json:"introducing_pr"`
	FixPR         int      `json:"fix_pr"`
	Files         []string `json:"files"`
	LinesHint     string   `json:"lines_hint"`
	Description   string   `json:"description"`
	Failure       string   `json:"failure"`
	Confidence    string   `json:"confidence"`
}

// Run is one reviewer's result on one case.
type Run struct {
	Reviewer string `json:"reviewer"`
	Model    string `json:"model"`
	// Args are the extra `surveyor review` arguments for a surveyor
	// variant.
	Args []string `json:"args,omitempty"`
	PR   int      `json:"pr"`
	// Findings are what the reviewer reported.
	Findings []review.Finding `json:"findings"`
	// Refuted are findings the reviewer itself dropped (surveyor's
	// verifier), kept to measure what verification costs.
	Refuted []review.Finding `json:"refuted,omitempty"`
	// Coverage is surveyor's record of whether it read the whole change.
	Coverage *review.Coverage `json:"coverage,omitempty"`
	CostUSD  float64          `json:"cost_usd"`
	Seconds  float64          `json:"seconds"`
	// Text is the reviewer's closing message.
	Text string `json:"text,omitempty"`
	// NoReportTool is set when claude-code never called ReportFindings,
	// so Findings may be incomplete: check Text.
	NoReportTool bool `json:"no_report_tool,omitempty"`
	// Extracted is set when Findings were extracted from Text (extract).
	Extracted bool      `json:"extracted,omitempty"`
	RanAt     time.Time `json:"ran_at"`
}

// Judgement is the judge's label for one finding.
type Judgement struct {
	ID string `json:"id"`
	// Reviewer and Refuted say where the finding came from; the judge
	// never sees them.
	Reviewer string         `json:"reviewer"`
	Refuted  bool           `json:"refuted,omitempty"`
	Finding  review.Finding `json:"finding"`
	// Label is real, nit or false_positive; empty if the judge skipped it.
	Label string `json:"label"`
	// Group names the defect: the ID of the first finding about it.
	Group string `json:"group"`
	// KnownBug is the index into the case's known bugs, or -1.
	KnownBug int    `json:"known_bug"`
	Reason   string `json:"reason"`
}

// Judged is the judge's output for one case.
type Judged struct {
	PR         int         `json:"pr"`
	Model      string      `json:"model"`
	Judgements []Judgement `json:"judgements"`
	// KnownBugs are the known bugs for this case, as the judge saw them.
	KnownBugs []KnownBug `json:"known_bugs,omitempty"`
	Reviewers []string   `json:"reviewers"`
	CostUSD   float64    `json:"cost_usd"`
	Seconds   float64    `json:"seconds"`
}

func caseDir(dir string, n int) string { return filepath.Join(dir, fmt.Sprintf("pr-%d", n)) }

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// writeJSON writes atomically: a result file exists only when complete,
// which is what makes run and judge resumable.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadCases(dir string) ([]Case, error) {
	var cs []Case
	if err := readJSON(filepath.Join(dir, "cases.json"), &cs); err != nil {
		return nil, fmt.Errorf("cases (run select first): %w", err)
	}
	return cs, nil
}

// loadKnownBugs reads D/known-bugs.json; a missing file means none.
func loadKnownBugs(dir string) ([]KnownBug, error) {
	var kb []KnownBug
	err := readJSON(filepath.Join(dir, "known-bugs.json"), &kb)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return kb, err
}

func bugsFor(all []KnownBug, pr int) []KnownBug {
	var out []KnownBug
	for _, b := range all {
		if b.IntroducingPR == pr {
			out = append(out, b)
		}
	}
	return out
}
