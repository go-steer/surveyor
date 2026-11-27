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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-steer/surveyor/internal/review"
)

// The CodeRabbit CLI reviews a local checkout: `coderabbit review --agent
// --deep --committed --base-commit <merge base>` in a worktree at the PR's
// head, with the repository's AGENTS.md passed as instructions. Its
// results differ from CodeRabbit's hosted PR bot (no PR description, for
// one), so they are reported as the CLI's. The CLI needs a full-history
// clone to find the merge base; one clone under D/coderabbit-repo serves
// every case, with a worktree per case.

func isCodeRabbit(name string) bool {
	return name == "coderabbit" || strings.HasPrefix(name, "coderabbit-")
}

// crRepoMu serializes git operations on the shared clone.
var crRepoMu sync.Mutex

func (rc *runConfig) crWorktree(ctx context.Context, c Case) (dir, mergeBase string, err error) {
	crRepoMu.Lock()
	defer crRepoMu.Unlock()
	repo := filepath.Join(rc.dir, "coderabbit-repo")
	git := func(args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...).Output()
		return strings.TrimSpace(string(out)), exitDetail(err)
	}
	if !exists(filepath.Join(repo, ".git")) {
		if out, err := exec.CommandContext(ctx, "git", "clone", "-q", "--filter=blob:none",
			"https://github.com/"+c.Repo, repo).CombinedOutput(); err != nil {
			return "", "", fmt.Errorf("clone: %w: %s", err, out)
		}
	}
	if _, err := git("fetch", "-q", "origin", fmt.Sprintf("+refs/pull/%d/head:refs/remotes/pr/%d", c.Number, c.Number)); err != nil {
		return "", "", fmt.Errorf("fetch PR head: %w", err)
	}
	if mergeBase, err = git("merge-base", c.BaseSHA, c.HeadSHA); err != nil {
		return "", "", fmt.Errorf("merge-base: %w", err)
	}
	dir = filepath.Join(caseDir(rc.dir, c.Number), "coderabbit-src")
	if head, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output(); err == nil &&
		strings.TrimSpace(string(head)) == c.HeadSHA {
		return dir, mergeBase, nil
	}
	_, _ = git("worktree", "remove", "--force", dir)
	if _, err := git("worktree", "add", "-q", "--detach", dir, c.HeadSHA); err != nil {
		return "", "", fmt.Errorf("worktree: %w", err)
	}
	return dir, mergeBase, nil
}

type crEvent struct {
	Type          string   `json:"type"`
	Severity      string   `json:"severity"`
	FileName      string   `json:"fileName"`
	StartLine     int      `json:"startLine"`
	Title         string   `json:"title"`
	Comment       string   `json:"comment"`
	Status        string   `json:"status"`
	Outcome       string   `json:"outcome"`
	Message       string   `json:"message"`
	ReviewedFiles []string `json:"reviewedFiles"`
}

func (rc *runConfig) runCodeRabbit(ctx context.Context, c Case, name string) (*Run, error) {
	src, mergeBase, err := rc.crWorktree(ctx, c)
	if err != nil {
		return nil, err
	}
	args := []string{"review", "--agent", "--committed", "--base-commit", mergeBase}
	if rc.crDeep {
		args = append(args, "--deep")
	}
	if exists(filepath.Join(src, "AGENTS.md")) {
		args = append(args, "-c", "AGENTS.md")
	}
	out := filepath.Join(caseDir(rc.dir, c.Number), name+".ndjson")
	start := time.Now()
	// The free tiers allow a few CLI reviews an hour: wait out a rate
	// limit rather than fail the case.
	for attempt := 0; ; attempt++ {
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "coderabbit", args...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = src, &stdout, &stderr
		runErr := cmd.Run()
		if err := os.WriteFile(out, stdout.Bytes(), 0o644); err != nil {
			return nil, err
		}
		r, perr := parseCodeRabbit(stdout.Bytes())
		if perr == nil && runErr == nil {
			r.Reviewer, r.Model, r.PR, r.RanAt = name, "coderabbit-cli", c.Number, start.UTC()
			r.Seconds = time.Since(start).Seconds()
			changed, _ := exec.CommandContext(ctx, "git", "-C", src, "diff", "--name-only", mergeBase, c.HeadSHA).Output()
			r.Coverage, r.Text = crCoverage(strings.Fields(string(changed)), r.Text)
			return r, nil
		}
		detail := strings.TrimSpace(stderr.String())
		if perr != nil {
			detail = perr.Error() + "; " + detail
		}
		low := strings.ToLower(detail)
		if attempt < rc.crRetries && (strings.Contains(low, "rate") || strings.Contains(low, "limit") || strings.Contains(low, "quota")) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(rc.crWait):
			}
			continue
		}
		return nil, fmt.Errorf("coderabbit: %v: %s", runErr, detail)
	}
}

// parseCodeRabbit reads `coderabbit review --agent` NDJSON. Severities
// critical and major count as important, the rest as nits.
func parseCodeRabbit(b []byte) (*Run, error) {
	r := &Run{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	done := false
	var reviewed []string
	for sc.Scan() {
		var ev crEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "finding":
			sev := review.SeverityNit
			if ev.Severity == "critical" || ev.Severity == "major" {
				sev = review.SeverityImportant
			}
			r.Findings = append(r.Findings, review.Finding{File: ev.FileName, Line: ev.StartLine, Severity: sev,
				Category: ev.Severity, Summary: strings.TrimSpace(ev.Title), FailureScenario: strings.TrimSpace(ev.Comment)})
		case "complete":
			if ev.Status != "review_completed" {
				return nil, fmt.Errorf("review ended %s (%s): %s", ev.Status, ev.Outcome, ev.Message)
			}
			done, reviewed = true, ev.ReviewedFiles
		}
	}
	if !done {
		return nil, fmt.Errorf("no complete event in coderabbit output")
	}
	r.Text = strings.Join(reviewed, "\n")
	return r, sc.Err()
}

// crCoverage compares the files CodeRabbit reviewed with the files the PR
// changed. Lockfiles and generated sums are expected to be skipped.
func crCoverage(changed []string, reviewedText string) (*review.Coverage, string) {
	reviewed := map[string]bool{}
	for _, f := range strings.Split(reviewedText, "\n") {
		if f != "" {
			reviewed[f] = true
		}
	}
	var missing []string
	for _, f := range changed {
		if !reviewed[f] && !strings.HasSuffix(f, ".sum") {
			missing = append(missing, f)
		}
	}
	cov := &review.Coverage{DiffTruncated: len(missing) > 0}
	text := strconv.Itoa(len(reviewed)) + " files reviewed"
	if len(missing) > 0 {
		text += "; not reviewed: " + strings.Join(missing, ", ")
	}
	return cov, text
}
