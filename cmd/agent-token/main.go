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

// Command agent-token gives coding agents their own GitHub identity: it
// mints installation tokens for the go-steer-agent GitHub App, so agents
// open pull requests as the App instead of as a human maintainer.
//
// Usage:
//
//	agent-token [token]              print a token for GH_TOKEN
//	agent-token git-credential get   act as a git credential helper
//
// Configuration (environment):
//
//	AGENT_TOKEN_APP_ID      App ID or client ID (required)
//	AGENT_TOKEN_KEY_FILE    path to the App's PEM private key (required)
//	AGENT_TOKEN_OWNER       installation owner (default "go-steer")
//	AGENT_TOKEN_CACHE_DIR   token cache (default <user cache dir>/agent-token)
//
// Tokens last an hour. agent-token caches one and mints a new one when less
// than ten minutes remain, so calling it before every gh or git command is
// cheap. See docs/agent-identity.md for setup.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-steer/surveyor/internal/ghapp"
)

const (
	defaultOwner = "go-steer"
	// minRemaining is how long a cached token must stay valid to be reused.
	// It is longer than any single git push or gh command.
	minRemaining = 10 * time.Minute
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := "token"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "token", "git-credential":
	case "-h", "--help", "help":
		_, _ = fmt.Fprintln(stdout, "usage: agent-token [token | git-credential get]")
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "agent-token: unknown command %q (want token or git-credential)\n", cmd)
		return 2
	}

	if cmd == "git-credential" {
		// git calls helpers with get, store or erase. Only get needs an
		// answer; store and erase must succeed silently because the token
		// is managed here, not by git.
		if len(args) < 2 || args[1] != "get" {
			return 0
		}
		if !wantsGitHub(stdin) {
			return 0 // not github.com over HTTPS: let other helpers answer
		}
	}

	src, err := newSource(getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "agent-token: %v\n", err)
		return 1
	}
	tok, err := src.token(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "agent-token: %v\n", err)
		return 1
	}
	if cmd == "git-credential" {
		_, _ = fmt.Fprintf(stdout, "username=x-access-token\npassword=%s\n", tok.Value)
		return 0
	}
	_, _ = fmt.Fprintln(stdout, tok.Value)
	return 0
}

// wantsGitHub reads git's credential request and reports whether it is for
// https://github.com.
func wantsGitHub(r io.Reader) bool {
	attrs := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			attrs[k] = v
		}
	}
	return attrs["protocol"] == "https" && attrs["host"] == "github.com"
}

// source mints tokens and caches them on disk.
type source struct {
	client    *ghapp.Client
	owner     string
	cachePath string
	now       func() time.Time
}

func newSource(getenv func(string) string) (*source, error) {
	appID := getenv("AGENT_TOKEN_APP_ID")
	keyFile := getenv("AGENT_TOKEN_KEY_FILE")
	if appID == "" || keyFile == "" {
		return nil, errors.New("AGENT_TOKEN_APP_ID and AGENT_TOKEN_KEY_FILE must be set")
	}
	pemBytes, err := os.ReadFile(keyFile) // #nosec G304 -- path is the operator's own config
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	key, err := ghapp.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	owner := getenv("AGENT_TOKEN_OWNER")
	if owner == "" {
		owner = defaultOwner
	}
	dir := getenv("AGENT_TOKEN_CACHE_DIR")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("find cache dir (set AGENT_TOKEN_CACHE_DIR): %w", err)
		}
		dir = filepath.Join(base, "agent-token")
	}
	return &source{
		client:    &ghapp.Client{AppID: appID, Key: key},
		owner:     owner,
		cachePath: filepath.Join(dir, cacheName(appID, owner)),
		now:       time.Now,
	}, nil
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func cacheName(appID, owner string) string {
	return unsafeChars.ReplaceAllString(appID+"-"+owner, "_") + ".json"
}

type cacheEntry struct {
	InstallationID int64       `json:"installation_id"`
	Token          ghapp.Token `json:"token"`
}

func (s *source) token(ctx context.Context) (ghapp.Token, error) {
	entry, _ := s.readCache() // a missing or corrupt cache just means minting
	if entry.Token.ValidFor(minRemaining, s.now()) {
		return entry.Token, nil
	}
	cachedID := entry.InstallationID != 0
	if !cachedID {
		if err := s.lookupInstallation(ctx, &entry); err != nil {
			return ghapp.Token{}, err
		}
	}
	tok, err := s.client.InstallationToken(ctx, entry.InstallationID, ghapp.TokenOptions{})
	var apiErr *ghapp.APIError
	if cachedID && errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		// The App was reinstalled and the cached installation ID is gone.
		if err := s.lookupInstallation(ctx, &entry); err != nil {
			return ghapp.Token{}, err
		}
		tok, err = s.client.InstallationToken(ctx, entry.InstallationID, ghapp.TokenOptions{})
	}
	if err != nil {
		return ghapp.Token{}, err
	}
	entry.Token = tok
	// The token is good either way; a cache write failure only costs a
	// re-mint on the next call.
	_ = s.writeCache(entry)
	return tok, nil
}

func (s *source) lookupInstallation(ctx context.Context, entry *cacheEntry) error {
	id, err := s.client.InstallationID(ctx, s.owner)
	if err != nil {
		return fmt.Errorf("find installation on %q (is the App installed there?): %w", s.owner, err)
	}
	entry.InstallationID = id
	return nil
}

func (s *source) readCache() (cacheEntry, error) {
	var e cacheEntry
	b, err := os.ReadFile(s.cachePath)
	if err != nil {
		return e, err
	}
	err = json.Unmarshal(b, &e)
	return e, err
}

// writeCache writes the entry atomically with owner-only permissions: the
// file holds a live credential.
func (s *source) writeCache(e cacheEntry) error {
	dir := filepath.Dir(s.cachePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".agent-token-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op after a successful rename
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.cachePath)
}
