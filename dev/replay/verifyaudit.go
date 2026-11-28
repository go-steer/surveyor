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
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-steer/core-agent/v3/pkg/usage"

	"github.com/go-steer/surveyor/internal/review"
	"github.com/go-steer/surveyor/internal/source"
	"github.com/go-steer/surveyor/internal/vertex"
)

// verify-audit tests surveyor's verifier on findings a judge has already
// labelled: it should refute the false alarms and keep the real findings.
// A verifier that keeps everything adds cost and filters nothing.

// AuditItem is one labelled finding and the verifier's verdict on it.
type AuditItem struct {
	PR       int            `json:"pr"`
	Reviewer string         `json:"reviewer"` // who reported it
	Label    string         `json:"label"`    // the judge's: real, nit, false_positive
	Finding  review.Finding `json:"finding"`
	Verdict  string         `json:"verdict"`
	Reason   string         `json:"reason"`
	CostUSD  float64        `json:"cost_usd"`
	Error    string         `json:"error,omitempty"`
}

func cmdVerifyAudit(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("verify-audit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dirs := fs.String("dirs", "", "comma-separated judged replay directories to draw findings from")
	out := fs.String("out", "", "output JSON file (resumable: items already in it are skipped)")
	nReal := fs.Int("real", 40, "real findings to sample (all false alarms are used)")
	nNit := fs.Int("nit", 10, "nits to sample")
	model := fs.String("model", "claude-sonnet-5-5", "verifier model")
	level := fs.String("level", "medium", "verifier thinking level")
	project := fs.String("project", "go-steer-drydock", "GCP project for Vertex AI")
	region := fs.String("region", "global", "Vertex AI region")
	parallel := fs.Int("parallel", 4, "verifiers at once")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dirs == "" || *out == "" {
		return errors.New("--dirs and --out are required")
	}
	items, err := auditSample(strings.Split(*dirs, ","), *nReal, *nNit)
	if err != nil {
		return err
	}
	var done []AuditItem
	_ = readJSON(*out, &done)
	seen := map[string]bool{}
	for _, d := range done {
		if d.Error == "" {
			seen[auditKey(d)] = true
		}
	}
	models, err := vertex.Open(*project, *region)
	if err != nil {
		return err
	}
	m, err := models.Model(ctx, *model)
	if err != nil {
		return err
	}
	opt := review.Options{Verifier: m, VerifierLevel: *level, Pricing: models.Pricing, Backends: models.Backends()}
	cache, _ := os.UserCacheDir()
	cache = filepath.Join(cache, "surveyor", "gomod")
	token := githubToken(ctx)

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, max(1, *parallel))
		prs = map[int]*review.Input{}
		res []AuditItem
	)
	// Keep only successful earlier results; failed ones are retried.
	for _, d := range done {
		if d.Error == "" {
			res = append(res, d)
		}
	}
	input := func(it auditSource) (*review.Input, error) {
		mu.Lock()
		defer mu.Unlock()
		if in, ok := prs[it.pr]; ok {
			return in, nil
		}
		src, err := checkout(ctx, it.dir, token, it.c)
		if err != nil {
			return nil, err
		}
		pr, err := (&source.Client{Token: token}).FetchPR(ctx, it.c.Repo, it.c.Number)
		if err != nil {
			return nil, err
		}
		in := &review.Input{PR: pr, Workdir: src}
		if ok, err := source.DownloadGoModules(ctx, src, cache); err == nil && ok {
			in.ReadOnlyRoots = []string{cache}
		}
		prs[it.pr] = in
		return in, nil
	}
	todo := 0
	for _, it := range items {
		if seen[auditKey(it.item)] {
			continue
		}
		todo++
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			item := it.item
			in, err := input(it)
			if err == nil {
				tracker := usage.NewTracker()
				f := item.Finding
				f.Verdict, f.Reasoning = "", ""
				f, err = review.Verify(ctx, *in, opt, tracker, f)
				item.Verdict, item.Reason, item.CostUSD = string(f.Verdict), f.Reasoning, tracker.Totals().CostUSD
			}
			if err != nil {
				item.Error = err.Error()
			}
			mu.Lock()
			res = append(res, item)
			if werr := writeJSON(*out, res); werr != nil {
				_, _ = fmt.Fprintf(stderr, "write: %v\n", werr)
			}
			_, _ = fmt.Fprintf(stderr, "%s #%d %s -> %s %s\n", time.Now().Format("15:04:05"), item.PR, item.Label, item.Verdict, item.Error)
			mu.Unlock()
		}()
	}
	_, _ = fmt.Fprintf(stderr, "verify-audit: %d to verify (%d sampled, %d done before)\n", todo, len(items), len(res))
	wg.Wait()
	printAudit(stdout, res, *model, *level)
	return ctx.Err()
}

type auditSource struct {
	dir  string
	c    Case
	pr   int
	item AuditItem
}

func auditKey(a AuditItem) string {
	return fmt.Sprintf("%d|%s|%s|%d|%s", a.PR, a.Reviewer, a.Finding.File, a.Finding.Line, a.Finding.Summary)
}

// auditSample collects every judged false alarm and a seeded sample of real
// findings and nits, deduplicated across directories. Findings a verifier
// already refuted are left out: the point is what reaches the verifier.
func auditSample(dirs []string, nReal, nNit int) ([]auditSource, error) {
	by := map[string][]auditSource{}
	seen := map[string]bool{}
	for _, dir := range dirs {
		cases, err := loadCases(dir)
		if err != nil {
			return nil, err
		}
		for _, c := range cases {
			var j Judged
			if readJSON(filepath.Join(caseDir(dir, c.Number), "judge.json"), &j) != nil {
				continue
			}
			for _, x := range j.Judgements {
				if x.Refuted || x.Label == "" {
					continue
				}
				it := AuditItem{PR: c.Number, Reviewer: x.Reviewer, Label: x.Label, Finding: x.Finding}
				k := fmt.Sprintf("%d|%s|%d|%s", it.PR, it.Finding.File, it.Finding.Line, it.Finding.Summary)
				if seen[k] {
					continue
				}
				seen[k] = true
				by[x.Label] = append(by[x.Label], auditSource{dir: dir, c: c, pr: c.Number, item: it})
			}
		}
	}
	rng := rand.New(rand.NewPCG(7, 7))
	pick := func(xs []auditSource, n int) []auditSource {
		sort.Slice(xs, func(i, j int) bool { return auditKey(xs[i].item) < auditKey(xs[j].item) })
		rng.Shuffle(len(xs), func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
		if n < len(xs) {
			xs = xs[:n]
		}
		return xs
	}
	out := pick(by["false_positive"], len(by["false_positive"]))
	out = append(out, pick(by["real"], nReal)...)
	out = append(out, pick(by["nit"], nNit)...)
	return out, nil
}

func printAudit(w io.Writer, items []AuditItem, model, level string) {
	type row struct{ n, refuted, plausible, confirmed, failed int }
	rows := map[string]*row{}
	cost := 0.0
	for _, it := range items {
		r := rows[it.Label]
		if r == nil {
			r = &row{}
			rows[it.Label] = r
		}
		r.n++
		cost += it.CostUSD
		switch {
		case it.Error != "":
			r.failed++
		case it.Verdict == string(review.VerdictRefuted):
			r.refuted++
		case it.Verdict == string(review.VerdictPlausible):
			r.plausible++
		default:
			r.confirmed++
		}
	}
	_, _ = fmt.Fprintf(w, "Verifier %s at %s on judged findings ($%.2f):\n\n| judge's label | findings | refuted | plausible | confirmed | failed |\n|---|---|---|---|---|---|\n", model, level, cost)
	for _, l := range []string{"false_positive", "nit", "real"} {
		if r := rows[l]; r != nil {
			_, _ = fmt.Fprintf(w, "| %s | %d | %d (%s) | %d | %d | %d |\n", l, r.n, r.refuted, pct(r.refuted, r.n), r.plausible, r.confirmed, r.failed)
		}
	}
}
