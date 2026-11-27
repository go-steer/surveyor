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
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// defect is one underlying problem: a judge group with at least one
// finding labelled real.
type defect struct {
	pr      int
	summary string
	file    string
	line    int
	known   int             // index into the case's known bugs, or -1
	foundBy map[string]bool // reviewer -> reported it (not refuted)
	lostBy  map[string]bool // reviewer -> found it, then its verifier refuted it
}

type tally struct {
	cases, findings, real, nit, fp, unjudged int
	refutedReal, refutedNit, refutedFP       int
	cost, seconds, maxSeconds                float64
	noReport                                 int
	model                                    string
}

func cmdReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "replay directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" {
		return errors.New("--dir is required")
	}
	cases, err := loadCases(*dir)
	if err != nil {
		return err
	}

	var (
		reviewers []string
		tallies   = map[string]*tally{}
		defects   []*defect
		judged    []Judged
		judgeCost float64
		titles    = map[int]string{}
		knownAll  []struct {
			pr  int
			bug KnownBug
			d   *defect
		}
		fps = map[string][]string{}
	)
	for _, c := range cases {
		titles[c.Number] = c.Title
		var j Judged
		if err := readJSON(caseDir(*dir, c.Number)+"/judge.json", &j); err != nil {
			continue
		}
		judged = append(judged, j)
		judgeCost += j.CostUSD
		runs, _ := loadRuns(*dir, c.Number, j.Reviewers)
		for _, r := range runs {
			t := tallies[r.Reviewer]
			if t == nil {
				t = &tally{}
				tallies[r.Reviewer] = t
				reviewers = append(reviewers, r.Reviewer)
			}
			t.cases++
			t.cost += r.CostUSD
			t.seconds += r.Seconds
			t.maxSeconds = max(t.maxSeconds, r.Seconds)
			t.model = r.Model
			if r.NoReportTool && len(r.Findings) == 0 && !noFindingsText(r.Text) {
				t.noReport++
			}
		}
		groups := map[string]*defect{}
		var order []string
		for _, x := range j.Judgements {
			t := tallies[x.Reviewer]
			if t == nil {
				continue
			}
			if x.Refuted {
				switch x.Label {
				case "real":
					t.refutedReal++
				case "nit":
					t.refutedNit++
				case "false_positive":
					t.refutedFP++
				}
			} else {
				t.findings++
				switch x.Label {
				case "real":
					t.real++
				case "nit":
					t.nit++
				case "false_positive":
					t.fp++
					fps[x.Reviewer] = append(fps[x.Reviewer], fmt.Sprintf("#%d `%s:%d` %s", c.Number, x.Finding.File, x.Finding.Line, oneLine(x.Finding.Summary)))
				default:
					t.unjudged++
				}
			}
			if x.Label != "real" {
				continue
			}
			d := groups[x.Group]
			if d == nil {
				d = &defect{pr: c.Number, summary: x.Finding.Summary, file: x.Finding.File, line: x.Finding.Line,
					known: -1, foundBy: map[string]bool{}, lostBy: map[string]bool{}}
				groups[x.Group] = d
				order = append(order, x.Group)
			}
			if x.KnownBug >= 0 {
				d.known = x.KnownBug
			}
			if x.Refuted {
				d.lostBy[x.Reviewer] = true
			} else {
				d.foundBy[x.Reviewer] = true
			}
		}
		for _, g := range order {
			defects = append(defects, groups[g])
		}
		for i, kb := range j.KnownBugs {
			var match *defect
			for _, g := range order {
				if groups[g].known == i {
					match = groups[g]
				}
			}
			knownAll = append(knownAll, struct {
				pr  int
				bug KnownBug
				d   *defect
			}{c.Number, kb, match})
		}
	}
	if len(judged) == 0 {
		return errors.New("no judged cases yet (run judge)")
	}
	sort.Strings(reviewers)
	w := func(format string, a ...any) { _, _ = fmt.Fprintf(stdout, format, a...) }

	w("# Replay: %d pull requests\n\n", len(judged))
	w("Every finding was judged blind (reviewer hidden, order shuffled) by %s reading the code at the PR's head. ", judged[0].Model)
	w("A **defect** is a group of findings the judge labelled real and about the same problem. ")
	w("Known bugs are defects a later PR fixed. Judge cost: $%.2f.\n\n", judgeCost)

	totalDefects := 0
	for _, d := range defects {
		if len(d.foundBy) > 0 {
			totalDefects++
		}
	}
	w("| | %s |\n|---|%s\n", strings.Join(reviewers, " | "), strings.Repeat("---|", len(reviewers)))
	row := func(label string, f func(r string, t *tally) string) {
		cells := make([]string, len(reviewers))
		for i, r := range reviewers {
			cells[i] = f(r, tallies[r])
		}
		w("| %s | %s |\n", label, strings.Join(cells, " | "))
	}
	row("model", func(_ string, t *tally) string { return t.model })
	row("PRs reviewed", func(_ string, t *tally) string { return fmt.Sprint(t.cases) })
	row("findings reported", func(_ string, t *tally) string { return fmt.Sprint(t.findings) })
	row("real / nit / false alarm", func(_ string, t *tally) string {
		s := fmt.Sprintf("%d / %d / %d", t.real, t.nit, t.fp)
		if t.unjudged > 0 {
			s += fmt.Sprintf(" (+%d unjudged)", t.unjudged)
		}
		return s
	})
	row("share of findings that are real", func(_ string, t *tally) string { return pct(t.real, t.findings) })
	row("false alarms", func(_ string, t *tally) string { return pct(t.fp, t.findings) })
	row(fmt.Sprintf("defects found (of %d found by anyone)", totalDefects), func(r string, _ *tally) string {
		n := 0
		for _, d := range defects {
			if d.foundBy[r] {
				n++
			}
		}
		return fmt.Sprintf("%d (%s)", n, pct(n, totalDefects))
	})
	row("defects only this reviewer found", func(r string, _ *tally) string {
		n := 0
		for _, d := range defects {
			if d.foundBy[r] && len(d.foundBy) == 1 {
				n++
			}
		}
		return fmt.Sprint(n)
	})
	row(fmt.Sprintf("known bugs caught (of %d)", len(knownAll)), func(r string, _ *tally) string {
		n := 0
		for _, k := range knownAll {
			if k.d != nil && k.d.foundBy[r] {
				n++
			}
		}
		return fmt.Sprint(n)
	})
	row("cost, total (mean per PR)", func(_ string, t *tally) string {
		return fmt.Sprintf("$%.2f ($%.2f)", t.cost, t.cost/float64(max(1, t.cases)))
	})
	row("time, mean (max)", func(_ string, t *tally) string {
		return fmt.Sprintf("%.0fs (%.0fs)", t.seconds/float64(max(1, t.cases)), t.maxSeconds)
	})
	if t := tallies["surveyor"]; t != nil {
		w("\nsurveyor's verifier refuted %d findings: %d false alarms and %d nits correctly dropped, %d real findings lost.\n",
			t.refutedReal+t.refutedNit+t.refutedFP, t.refutedFP, t.refutedNit, t.refutedReal)
	}
	for _, r := range reviewers {
		if n := tallies[r].noReport; n > 0 {
			w("\n%s did not report structured findings on %d PRs and its closing text did not say \"no findings\": check those transcripts.\n", r, n)
		}
	}

	if len(knownAll) > 0 {
		w("\n## Known bugs\n\n| PR | fixed in | bug | %s |\n|---|---|---|%s\n", strings.Join(reviewers, " | "), strings.Repeat("---|", len(reviewers)))
		for _, k := range knownAll {
			cells := make([]string, len(reviewers))
			for i, r := range reviewers {
				switch {
				case k.d != nil && k.d.foundBy[r]:
					cells[i] = "caught"
				case k.d != nil && k.d.lostBy[r]:
					cells[i] = "found, then refuted"
				default:
					cells[i] = "missed"
				}
			}
			w("| #%d | #%d | %s | %s |\n", k.pr, k.bug.FixPR, oneLine(k.bug.Description), strings.Join(cells, " | "))
		}
	}

	w("\n## Per pull request\n\n| PR | title | %s |\n|---|---|%s\n", strings.Join(reviewers, " | "), strings.Repeat("---|", len(reviewers)))
	for _, j := range judged {
		cells := make([]string, len(reviewers))
		for i, r := range reviewers {
			real, all := 0, 0
			for _, x := range j.Judgements {
				if x.Reviewer == r && !x.Refuted {
					all++
					if x.Label == "real" {
						real++
					}
				}
			}
			var run Run
			cost := ""
			if readJSON(resultPath(*dir, j.PR, r), &run) == nil {
				cost = fmt.Sprintf(", $%.2f, %.0fs", run.CostUSD, run.Seconds)
			}
			cells[i] = fmt.Sprintf("%d real of %d%s", real, all, cost)
		}
		w("| #%d | %s | %s |\n", j.PR, oneLine(titles[j.PR]), strings.Join(cells, " | "))
	}

	w("\n## Defects\n\n| PR | where | defect | %s |\n|---|---|---|%s\n", strings.Join(reviewers, " | "), strings.Repeat("---|", len(reviewers)))
	for _, d := range defects {
		cells := make([]string, len(reviewers))
		for i, r := range reviewers {
			switch {
			case d.foundBy[r]:
				cells[i] = "yes"
			case d.lostBy[r]:
				cells[i] = "refuted"
			default:
				cells[i] = ""
			}
		}
		k := ""
		if d.known >= 0 {
			k = " (known bug)"
		}
		w("| #%d | `%s:%d` | %s%s | %s |\n", d.pr, d.file, d.line, oneLine(d.summary), k, strings.Join(cells, " | "))
	}

	for _, r := range reviewers {
		if len(fps[r]) == 0 {
			continue
		}
		w("\n## False alarms: %s\n\n", r)
		for _, s := range fps[r] {
			w("- %s\n", s)
		}
	}
	return nil
}

func pct(n, d int) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(d))
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	if len(s) > 160 {
		s = s[:157] + "..."
	}
	return s
}

// noFindingsText reports whether a reviewer's closing text says it found
// nothing, which explains a missing ReportFindings call.
func noFindingsText(s string) bool {
	s = strings.ToLower(s)
	for _, p := range []string{"no problems", "no findings", "no issues", "found nothing", "no bugs", "nothing to report"} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
