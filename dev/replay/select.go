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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ghPR struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	MergedAt  time.Time `json:"mergedAt"`
	BaseRef   string    `json:"baseRefOid"`
	HeadRef   string    `json:"headRefOid"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
	Files []struct {
		Path string `json:"path"`
	} `json:"files"`
}

func cmdSelect(ctx context.Context, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("select", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "replay directory")
	repo := fs.String("repo", "", "repository, owner/name")
	since := fs.String("since", "", "earliest merge date, YYYY-MM-DD")
	until := fs.String("until", "", "latest merge date, YYYY-MM-DD (default: today)")
	limit := fs.Int("limit", 30, "number of cases")
	include := fs.String("include", "", "comma-separated PR numbers to include whatever the filters say (e.g. PRs with known bugs)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" || *repo == "" || *since == "" {
		return fmt.Errorf("--dir, --repo and --since are required")
	}
	search := "merged:>=" + *since
	if *until != "" {
		search += " merged:<=" + *until
	}
	prs, err := listMerged(ctx, *repo, search)
	if err != nil {
		return err
	}
	forced := map[int]bool{}
	for _, s := range strings.Split(*include, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("--include: %w", err)
		}
		forced[n] = true
	}
	// Included PRs may predate --since: fetch any that are missing.
	have := map[int]bool{}
	for _, p := range prs {
		have[p.Number] = true
	}
	for n := range forced {
		if !have[n] {
			p, err := viewPR(ctx, *repo, n)
			if err != nil {
				return err
			}
			prs = append(prs, p)
		}
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].MergedAt.After(prs[j].MergedAt) })

	var cases []Case
	for _, p := range prs {
		if forced[p.Number] {
			cases = append(cases, toCase(*repo, p))
		}
	}
	for _, p := range prs {
		if len(cases) >= *limit {
			break
		}
		if forced[p.Number] {
			continue
		}
		if why := skip(p); why != "" {
			_, _ = fmt.Fprintf(stderr, "skip #%d (%s): %s\n", p.Number, why, p.Title)
			continue
		}
		cases = append(cases, toCase(*repo, p))
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].Number < cases[j].Number })
	_, _ = fmt.Fprintf(stderr, "selected %d cases (%d included)\n", len(cases), len(forced))
	return writeJSON(filepath.Join(*dir, "cases.json"), cases)
}

// skip says why a PR is not worth a review, or "".
func skip(p ghPR) string {
	if strings.HasPrefix(p.Author.Login, "app/") || strings.HasSuffix(p.Author.Login, "[bot]") {
		return "bot"
	}
	t := strings.ToLower(p.Title)
	for _, pre := range []string{"chore(changelog)", "chore(release)", "chore: bump version"} {
		if strings.HasPrefix(t, pre) {
			return "release chore"
		}
	}
	code := false
	for _, f := range p.Files {
		if strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, ".sh") ||
			strings.HasSuffix(f.Path, ".yml") || strings.HasSuffix(f.Path, ".yaml") {
			code = true
			break
		}
	}
	if !code {
		return "no code"
	}
	return ""
}

func toCase(repo string, p ghPR) Case {
	return Case{Repo: repo, Number: p.Number, Title: p.Title, BaseSHA: p.BaseRef, HeadSHA: p.HeadRef,
		Additions: p.Additions, Deletions: p.Deletions, MergedAt: p.MergedAt}
}

const prFields = "number,title,mergedAt,baseRefOid,headRefOid,additions,deletions,author,files"

func listMerged(ctx context.Context, repo, search string) ([]ghPR, error) {
	out, err := exec.CommandContext(ctx, "gh", "pr", "list", "--repo", repo, "--state", "merged",
		"--search", search, "--limit", "500", "--json", prFields).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", exitDetail(err))
	}
	var prs []ghPR
	return prs, json.Unmarshal(out, &prs)
}

func viewPR(ctx context.Context, repo string, n int) (ghPR, error) {
	var p ghPR
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", strconv.Itoa(n), "--repo", repo, "--json", prFields).Output()
	if err != nil {
		return p, fmt.Errorf("gh pr view %d: %w", n, exitDetail(err))
	}
	return p, json.Unmarshal(out, &p)
}

// exitDetail adds a failed command's stderr to its error.
func exitDetail(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}
