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

package gsecret

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestVersionName(t *testing.T) {
	cases := map[string]string{
		"projects/p/secrets/s":              "projects/p/secrets/s/versions/latest",
		"projects/p/secrets/s/versions/3":   "projects/p/secrets/s/versions/3",
		"projects/p/secrets/s/versions/new": "projects/p/secrets/s/versions/new",
	}
	for in, want := range cases {
		got, err := VersionName(in)
		if err != nil || got != want {
			t.Errorf("VersionName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "s", "projects/p", "projects/p/secrets/", "projects/p/secrets/s/versions", "projects/p/secrets/s/../x/y"} {
		if _, err := VersionName(bad); err == nil {
			t.Errorf("VersionName(%q): want error", bad)
		}
	}
}

func crc(b []byte) string {
	return fmt.Sprint(crc32.Checksum(b, crc32.MakeTable(crc32.Castagnoli)))
}

type fakeSM struct {
	t       *testing.T
	payload []byte
	crc     string
	status  int
	path    string
}

func (f *fakeSM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.path = r.URL.Path
	if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
		f.t.Errorf("Authorization = %q", got)
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
		fmt.Fprint(w, `{"error":{"code":403,"message":"Permission 'secretmanager.versions.access' denied","status":"PERMISSION_DENIED"}}`)
		return
	}
	fmt.Fprintf(w, `{"name":"x","payload":{"data":%q,"dataCrc32c":%q}}`, base64.StdEncoding.EncodeToString(f.payload), f.crc)
}

func newClient(t *testing.T, f *fakeSM) *Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return &Client{
		Tokens:  oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token", TokenType: "Bearer"}),
		BaseURL: srv.URL,
		HTTP:    srv.Client(),
	}
}

func TestAccess(t *testing.T) {
	payload := []byte("-----BEGIN RSA PRIVATE KEY-----\n...")
	f := &fakeSM{t: t, payload: payload, crc: crc(payload)}
	got, err := newClient(t, f).Access(context.Background(), "projects/go-steer-drydock/secrets/deckhand-key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload = %q", got)
	}
	if f.path != "/v1/projects/go-steer-drydock/secrets/deckhand-key/versions/latest:access" {
		t.Errorf("path = %q", f.path)
	}
}

func TestAccessRejectsChecksumMismatch(t *testing.T) {
	f := &fakeSM{t: t, payload: []byte("key"), crc: crc([]byte("other"))}
	_, err := newClient(t, f).Access(context.Background(), "projects/p/secrets/s")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v, want checksum mismatch", err)
	}
}

func TestAccessRejectsEmptyPayload(t *testing.T) {
	f := &fakeSM{t: t, payload: nil, crc: ""}
	if _, err := newClient(t, f).Access(context.Background(), "projects/p/secrets/s"); err == nil {
		t.Fatal("empty payload: want error")
	}
}

func TestAccessReportsGoogleError(t *testing.T) {
	f := &fakeSM{t: t, status: http.StatusForbidden}
	_, err := newClient(t, f).Access(context.Background(), "projects/p/secrets/s")
	if err == nil || !strings.Contains(err.Error(), "secretmanager.versions.access") || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want the 403 and Google's message", err)
	}
}

func TestAccessRejectsBadRefBeforeNetwork(t *testing.T) {
	// No server: a bad reference must fail before any request or token fetch.
	c := &Client{Tokens: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "x"}), BaseURL: "http://127.0.0.1:1"}
	if _, err := c.Access(context.Background(), "deckhand-key"); err == nil || !strings.Contains(err.Error(), "projects/") {
		t.Fatalf("err = %v, want a format error", err)
	}
}
