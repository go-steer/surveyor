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

// Package source fetches a pull request for review: its metadata and
// diff from the GitHub API, a checkout of its head commit, and the
// repository's rules as they stand on its base commit.
package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-steer/surveyor/internal/review"
)

// APIBase is the GitHub REST API root.
const APIBase = "https://api.github.com"

// maxRulesBytes caps each rules file placed in the prompt.
const maxRulesBytes = 64 << 10

// RuleFiles are read from the base commit, in order, as the repository's
// rules; the first that exists of each pair is used.
var RuleFiles = [][]string{{"AGENTS.md", "CLAUDE.md"}}

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// Client reads pull requests from GitHub.
type Client struct {
	// Token authenticates API calls and the clone; empty works for
	// public repositories within GitHub's anonymous rate limit.
	Token   string
	BaseURL string // default APIBase
	HTTP    *http.Client
	// CloneURL builds the clone URL for owner/name; default
	// https://github.com/<repo>.git. Tests point it at a local repo.
	CloneURL func(repo string) string
}

// FetchPR returns the pull request's metadata and its base...head diff.
func (c *Client) FetchPR(ctx context.Context, repo string, number int) (review.PullRequest, error) {
	if !repoName.MatchString(repo) {
		return review.PullRequest{}, fmt.Errorf("source: %q is not owner/name", repo)
	}
	var meta struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Base  struct {
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	path := fmt.Sprintf("/repos/%s/pulls/%d", repo, number)
	raw, err := c.get(ctx, path, "application/vnd.github+json")
	if err != nil {
		return review.PullRequest{}, err
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return review.PullRequest{}, fmt.Errorf("source: decode %s: %w", path, err)
	}
	diff, err := c.get(ctx, path, "application/vnd.github.diff")
	if err != nil {
		return review.PullRequest{}, err
	}
	return review.PullRequest{
		Repo: repo, Number: number, Title: meta.Title, Body: meta.Body,
		BaseSHA: meta.Base.SHA, HeadSHA: meta.Head.SHA, Diff: string(diff),
	}, nil
}

func (c *Client) get(ctx context.Context, path, accept string) ([]byte, error) {
	base := c.BaseURL
	if base == "" {
		base = APIBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("source: GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("source: GET %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("source: GET %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 300)])))
	}
	return body, nil
}

// Checkout clones repo into dir (which must not exist or be empty) and
// checks out head. Blobs are fetched on demand (a partial clone), so a
// large repository costs only the files the review reads. It returns
// the repository's rules as of base.
func (c *Client) Checkout(ctx context.Context, repo, base, head, dir string) (rules string, err error) {
	if !repoName.MatchString(repo) {
		return "", fmt.Errorf("source: %q is not owner/name", repo)
	}
	url := "https://github.com/" + repo + ".git"
	if c.CloneURL != nil {
		url = c.CloneURL(repo)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	steps := [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", url},
		{"fetch", "-q", "--filter=blob:none", "--depth=1", "origin", head},
		{"-c", "advice.detachedHead=false", "checkout", "-q", "--detach", head},
	}
	for _, args := range steps {
		if err := c.git(ctx, dir, args...); err != nil {
			return "", err
		}
	}
	if base == "" {
		return "", nil
	}
	if err := c.git(ctx, dir, "fetch", "-q", "--filter=blob:none", "--depth=1", "origin", base); err != nil {
		return "", err
	}
	return c.rulesAt(ctx, dir, base)
}

// rulesAt reads the rule files as they are at commit, never from the
// checkout: the pull request cannot rewrite the rules it is reviewed
// against.
func (c *Client) rulesAt(ctx context.Context, dir, commit string) (string, error) {
	var parts []string
	for _, group := range RuleFiles {
		for _, name := range group {
			out, err := c.gitOutput(ctx, dir, "show", commit+":"+name)
			if err != nil {
				continue // not present at commit
			}
			if len(out) > maxRulesBytes {
				out = append(out[:maxRulesBytes:maxRulesBytes], []byte("\n[truncated]")...)
			}
			parts = append(parts, fmt.Sprintf("# %s\n\n%s", name, out))
			break
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func (c *Client) git(ctx context.Context, dir string, args ...string) error {
	_, err := c.gitOutput(ctx, dir, args...)
	return err
}

// gitOutput runs git in dir. The token goes to git through its
// environment-variable config, never on the command line, so it is not
// visible in the process list.
func (c *Client) gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if c.Token != "" {
		auth := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + c.Token))
		cmd.Env = append(cmd.Env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
			"GIT_CONFIG_VALUE_0=Authorization: Basic "+auth,
		)
	}
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("source: git %s: %w: %s", args[0], err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("source: git %s: %w", args[0], err)
	}
	return out, nil
}

// DownloadGoModules fetches the Go modules the checkout's go.mod
// requires into cacheDir, so a review can read their source. It returns
// false when there is no go.mod. Downloading runs no module code, and
// checksums are verified against the checksum database. A go.mod that
// needs a newer Go than the one installed makes go fetch that release
// (GOTOOLCHAIN=auto): a go.mod can only name an official toolchain,
// which comes from the module proxy and is checksum-verified like any
// module, so nothing from the pull request runs. cacheDir should belong
// to surveyor, never a user's own module cache, and in a service it
// should not be shared between organizations with private modules.
func DownloadGoModules(ctx context.Context, checkout, cacheDir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(checkout, "go.mod")); err != nil {
		return false, nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return false, err
	}
	cmd := exec.CommandContext(ctx, "go", "mod", "download")
	cmd.Dir = checkout
	cmd.Env = append(os.Environ(), "GOMODCACHE="+cacheDir, "GOTOOLCHAIN=auto", "GOFLAGS=-mod=mod", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		return true, fmt.Errorf("source: go mod download: %w: %s", err, strings.TrimSpace(string(out[:min(len(out), 500)])))
	}
	return true, nil
}

// TempDir makes a fresh directory for a checkout under the system temp
// directory.
func TempDir() (string, error) {
	return os.MkdirTemp("", "surveyor-review-*")
}

// Clean removes a checkout made by TempDir.
func Clean(dir string) error {
	if dir == "" || !strings.HasPrefix(filepath.Base(dir), "surveyor-review-") {
		return fmt.Errorf("source: refusing to remove %q", dir)
	}
	return os.RemoveAll(dir)
}
