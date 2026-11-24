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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-steer/surveyor/internal/ghapp"
)

var testNow = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

// fakeGitHub serves one installation and counts token mints.
type fakeGitHub struct {
	installLookups, mints atomic.Int32
	expiresIn             time.Duration
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/orgs/go-steer/installation":
		f.installLookups.Add(1)
		fmt.Fprint(w, `{"id": 42}`)
	case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
		n := f.mints.Add(1)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"token":"ghs_%d","expires_at":%q}`, n, testNow.Add(f.expiresIn).Format(time.RFC3339))
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}
}

func newTestSource(t *testing.T, expiresIn time.Duration) (*source, *fakeGitHub) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeGitHub{expiresIn: expiresIn}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	now := func() time.Time { return testNow }
	return &source{
		client:    &ghapp.Client{AppID: "1", Key: key, BaseURL: srv.URL, HTTP: srv.Client(), Now: now},
		owner:     "go-steer",
		cachePath: filepath.Join(t.TempDir(), "cache", "1-go-steer.json"),
		now:       now,
	}, fake
}

func TestTokenIsCached(t *testing.T) {
	src, fake := newTestSource(t, time.Hour)
	for range 3 {
		tok, err := src.token(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if tok.Value != "ghs_1" {
			t.Fatalf("token = %q, want the first minted token reused", tok.Value)
		}
	}
	if fake.mints.Load() != 1 || fake.installLookups.Load() != 1 {
		t.Fatalf("mints=%d lookups=%d, want 1 and 1", fake.mints.Load(), fake.installLookups.Load())
	}
	info, err := os.Stat(src.cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache file mode = %o, want 600 (it holds a live credential)", perm)
	}
}

func TestTokenNearExpiryIsReminted(t *testing.T) {
	// Tokens that expire within minRemaining are never reused.
	src, fake := newTestSource(t, minRemaining-time.Minute)
	for range 2 {
		if _, err := src.token(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if fake.mints.Load() != 2 {
		t.Fatalf("mints = %d, want 2", fake.mints.Load())
	}
	// The installation ID is cached even when the token is not reusable.
	if fake.installLookups.Load() != 1 {
		t.Fatalf("installation lookups = %d, want 1", fake.installLookups.Load())
	}
}

func TestCorruptCacheIsIgnored(t *testing.T) {
	src, fake := newTestSource(t, time.Hour)
	if err := os.MkdirAll(filepath.Dir(src.cachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src.cachePath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	tok, err := src.token(context.Background())
	if err != nil || tok.Value != "ghs_1" {
		t.Fatalf("token = %q, %v", tok.Value, err)
	}
	if fake.mints.Load() != 1 {
		t.Fatalf("mints = %d, want 1", fake.mints.Load())
	}
}

func TestStaleInstallationIDIsRefreshed(t *testing.T) {
	src, fake := newTestSource(t, time.Hour)
	// A cache left by a previous installation (ID 7, now gone) with an
	// expired token.
	if err := src.writeCache(cacheEntry{InstallationID: 7, Token: ghapp.Token{Value: "old", ExpiresAt: testNow.Add(-time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	tok, err := src.token(context.Background())
	if err != nil || tok.Value != "ghs_1" {
		t.Fatalf("token = %q, %v; want a token from the current installation", tok.Value, err)
	}
	if fake.installLookups.Load() != 1 {
		t.Fatalf("installation lookups = %d, want 1", fake.installLookups.Load())
	}
	entry, err := src.readCache()
	if err != nil || entry.InstallationID != 42 {
		t.Fatalf("cached installation = %d, %v; want 42", entry.InstallationID, err)
	}
}

func TestWantsGitHub(t *testing.T) {
	cases := map[string]bool{
		"protocol=https\nhost=github.com\n\n":       true,
		"protocol=https\nhost=github.com\npath=x\n": true,
		"protocol=http\nhost=github.com\n":          false,
		"protocol=https\nhost=gitlab.com\n":         false,
		"protocol=https\nhost=github.com.evil.io\n": false,
		"": false,
	}
	for in, want := range cases {
		if got := wantsGitHub(strings.NewReader(in)); got != want {
			t.Errorf("wantsGitHub(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRunGitCredentialIgnoresOtherHostsAndOps(t *testing.T) {
	// Neither case may need configuration: git calls helpers for every host
	// and for store/erase, and those must succeed silently.
	noEnv := func(string) string { return "" }
	cases := []struct {
		args  []string
		stdin string
	}{
		{[]string{"git-credential", "get"}, "protocol=https\nhost=gitlab.com\n\n"},
		{[]string{"git-credential", "store"}, "protocol=https\nhost=github.com\npassword=x\n\n"},
		{[]string{"git-credential", "erase"}, "protocol=https\nhost=github.com\n\n"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		code := run(context.Background(), c.args, noEnv, strings.NewReader(c.stdin), &out, &errOut)
		if code != 0 || out.Len() != 0 {
			t.Errorf("%v: code=%d stdout=%q stderr=%q; want silent success", c.args, code, out.String(), errOut.String())
		}
	}
}

func TestRunRequiresConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(context.Background(), nil, func(string) string { return "" }, strings.NewReader(""), &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "AGENT_TOKEN_APP_ID") {
		t.Fatalf("code=%d stderr=%q; want a config error naming AGENT_TOKEN_APP_ID", code, errOut.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(context.Background(), []string{"nope"}, func(string) string { return "" }, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestCacheName(t *testing.T) {
	if got := cacheName("Iv1.abc", "../etc"); strings.ContainsAny(got, "/") {
		t.Fatalf("cacheName escaped its directory: %q", got)
	}
}
