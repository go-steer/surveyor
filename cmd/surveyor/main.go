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

// Command surveyor reviews a pull request.
//
//	surveyor review --repo go-steer/core-agent --pr 1603
//
// It checks out the pull request's head commit in a temporary directory,
// runs the finder and the verifiers (internal/review) with Claude on
// Vertex AI, and prints the verified findings. Nothing is posted.
//
// Credentials: Google Application Default Credentials for Vertex AI, and
// a GitHub token from GH_TOKEN, GITHUB_TOKEN or `gh auth token` (public
// repositories work without one, within GitHub's anonymous rate limit).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/go-steer/surveyor/internal/review"
	"github.com/go-steer/surveyor/internal/source"
	"github.com/go-steer/surveyor/internal/vertex"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "review" {
		_, _ = fmt.Fprintln(stderr, "usage: surveyor review --repo owner/name --pr N [flags]")
		return 2
	}
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", "", "repository, owner/name")
	pr := fs.Int("pr", 0, "pull request number")
	model := fs.String("model", "claude-sonnet-5-5", "finder model")
	verifyModel := fs.String("verify-model", "", "verifier model (default: --model)")
	level := fs.String("level", "high", "finder thinking level (minimal..max)")
	verifyLevel := fs.String("verify-level", "medium", "verifier thinking level")
	finderStyle := fs.String("finder-style", review.FinderStrict, "finder prompt: strict (report only traced failures) or broad (report every suspected defect; the verifiers filter)")
	finderSteps := fs.Int("finder-steps", 60, "finder model-call cap")
	verifySteps := fs.Int("verify-steps", 25, "verifier model-call cap")
	project := fs.String("project", envOr("ANTHROPIC_VERTEX_PROJECT_ID", "go-steer-drydock"), "GCP project for Claude on Vertex AI")
	region := fs.String("region", envOr("CLOUD_ML_REGION", "global"), "Vertex AI region")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	keep := fs.Bool("keep", false, "keep the checkout and print its path")
	deps := fs.Bool("deps", true, "download Go modules into surveyor's own cache so the review can read dependency source")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *repo == "" || *pr <= 0 {
		_, _ = fmt.Fprintln(stderr, "surveyor: --repo and --pr are required")
		return 2
	}
	if *verifyModel == "" {
		*verifyModel = *model
	}

	src := &source.Client{Token: githubToken(ctx)}
	p, err := src.FetchPR(ctx, *repo, *pr)
	if err != nil {
		return fail(stderr, err)
	}
	dir, err := source.TempDir()
	if err != nil {
		return fail(stderr, err)
	}
	if *keep {
		_, _ = fmt.Fprintf(stderr, "surveyor: checkout at %s\n", dir)
	} else {
		defer func() { _ = source.Clean(dir) }()
	}
	rules, err := src.Checkout(ctx, *repo, p.BaseSHA, p.HeadSHA, dir)
	if err != nil {
		return fail(stderr, err)
	}
	var readOnly []string
	if *deps {
		cache, err := modCacheDir()
		if err != nil {
			return fail(stderr, err)
		}
		_, _ = fmt.Fprintf(stderr, "surveyor: downloading Go modules into %s\n", cache)
		if ok, err := source.DownloadGoModules(ctx, dir, cache); err != nil {
			// Not fatal: the review still runs, it just cannot read
			// dependency source.
			_, _ = fmt.Fprintf(stderr, "surveyor: %v (continuing without dependency source)\n", err)
		} else if ok {
			readOnly = append(readOnly, cache)
		}
	}

	claude, err := vertex.Open(*project, *region)
	if err != nil {
		return fail(stderr, err)
	}
	finder, err := claude.Model(ctx, *model)
	if err != nil {
		return fail(stderr, err)
	}
	verifier := finder
	if *verifyModel != *model {
		if verifier, err = claude.Model(ctx, *verifyModel); err != nil {
			return fail(stderr, err)
		}
	}

	_, _ = fmt.Fprintf(stderr, "surveyor: reviewing %s#%d (%s) with %s\n", *repo, *pr, short(p.HeadSHA), *model)
	res, err := review.Review(ctx, review.Input{PR: p, Workdir: dir, Rules: rules, ReadOnlyRoots: readOnly}, review.Options{
		Finder: finder, Verifier: verifier,
		FinderLevel: *level, VerifierLevel: *verifyLevel,
		FinderStyle: *finderStyle, FinderMaxSteps: *finderSteps, VerifierMaxSteps: *verifySteps,
		Pricing: claude.Pricing, Backend: claude.Backend,
	})
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	printMarkdown(stdout, p, res)
	return 0
}

func printMarkdown(w io.Writer, p review.PullRequest, res *review.Result) {
	_, _ = fmt.Fprintf(w, "# Review of %s#%d: %s\n\n", p.Repo, p.Number, p.Title)
	if res.Summary != "" {
		_, _ = fmt.Fprintf(w, "%s\n\n", res.Summary)
	}
	if len(res.Findings) == 0 {
		_, _ = fmt.Fprintln(w, "No findings.")
	}
	for i, f := range res.Findings {
		_, _ = fmt.Fprintf(w, "## %d. [%s, %s] %s:%d\n\n**%s**\n\n- Failure: %s\n- Verifier: %s\n\n",
			i+1, f.Severity, f.Verdict, f.File, f.Line, f.Summary, f.FailureScenario, f.Reasoning)
	}
	u := res.Usage
	cost := fmt.Sprintf("$%.2f", u.CostUSD)
	if u.UnpricedTurns > 0 {
		cost += "+ (some turns unpriced)"
	}
	if c := res.Coverage; !c.Complete() {
		_, _ = fmt.Fprintln(w, "**Incomplete review:** findings may be missing.")
		if c.DiffTruncated {
			_, _ = fmt.Fprintf(w, "- The diff (%d bytes) was over the prompt cap; the finder got the file list.\n", c.DiffBytes)
		}
		if c.FinderHitCap {
			_, _ = fmt.Fprintf(w, "- The finder reached its %d-call cap.\n", c.FinderMaxSteps)
		}
		if c.VerifiersHitCap > 0 {
			_, _ = fmt.Fprintf(w, "- %d verifiers reached their call cap.\n", c.VerifiersHitCap)
		}
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintf(w, "---\n%d findings kept, %d refuted. %d model calls, %d input (%d cached) and %d output tokens, %s, %s.\n",
		len(res.Findings), len(res.Refuted), u.Turns, u.InputTokens, u.CachedInputTokens, u.OutputTokens, cost, res.Duration.Round(1e9))
}

// githubToken finds a token without asking: GH_TOKEN, GITHUB_TOKEN, then
// the gh CLI's stored login. Empty is fine for public repositories.
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

// modCacheDir is surveyor's own Go module cache, never the user's.
func modCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "surveyor", "gomod"), nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func fail(w io.Writer, err error) int {
	if errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintln(w, "surveyor: interrupted")
		return 130
	}
	_, _ = fmt.Fprintf(w, "surveyor: %v\n", err)
	return 1
}
