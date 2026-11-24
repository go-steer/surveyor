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

// Package gsecret reads secret payloads from Google Cloud Secret Manager
// over its REST API, authenticating with Application Default Credentials:
// a gcloud login on a workstation, Workload Identity on GKE.
//
// It is deliberately small: one call, no client library. agent-token uses
// it to load the go-steer-deckhand private key, and the surveyor service
// will use it to load its own App key.
package gsecret

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// DefaultBaseURL is the Secret Manager REST API root.
const DefaultBaseURL = "https://secretmanager.googleapis.com"

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// maxErrorBody bounds how much of an error response is quoted in an error.
const maxErrorBody = 4 << 10

var (
	secretName  = regexp.MustCompile(`^projects/[^/]+/secrets/[^/]+$`)
	versionName = regexp.MustCompile(`^projects/[^/]+/secrets/[^/]+/versions/[^/]+$`)
)

// VersionName normalizes a secret reference to a version resource name.
// "projects/P/secrets/S" becomes "projects/P/secrets/S/versions/latest";
// a name that already names a version is returned unchanged.
func VersionName(ref string) (string, error) {
	switch {
	case versionName.MatchString(ref):
		return ref, nil
	case secretName.MatchString(ref):
		return ref + "/versions/latest", nil
	default:
		return "", fmt.Errorf("gsecret: %q is not projects/<project>/secrets/<secret>[/versions/<version>]", ref)
	}
}

// Client accesses secret versions.
type Client struct {
	// Tokens supplies OAuth2 tokens. Nil means Application Default
	// Credentials with the cloud-platform scope.
	Tokens oauth2.TokenSource
	// BaseURL defaults to DefaultBaseURL.
	BaseURL string
	// HTTP defaults to a client with a 30-second timeout.
	HTTP *http.Client
}

// Access returns the payload of a secret version. ref is
// "projects/P/secrets/S" (latest version) or a full version name. The
// payload's CRC32C checksum is verified when Secret Manager supplies one.
func (c *Client) Access(ctx context.Context, ref string) ([]byte, error) {
	name, err := VersionName(ref)
	if err != nil {
		return nil, err
	}
	ts := c.Tokens
	if ts == nil {
		ts, err = google.DefaultTokenSource(ctx, cloudPlatformScope)
		if err != nil {
			return nil, fmt.Errorf("gsecret: find Application Default Credentials: %w", err)
		}
	}
	tok, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("gsecret: get access token: %w", err)
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/v1/"+name+":access", nil)
	if err != nil {
		return nil, fmt.Errorf("gsecret: build request: %w", err)
	}
	tok.SetAuthHeader(req)
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gsecret: access %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gsecret: access %s: HTTP %d: %s", name, resp.StatusCode, errorMessage(resp.Body))
	}
	var out struct {
		Payload struct {
			Data       string `json:"data"`
			DataCrc32c string `json:"dataCrc32c"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gsecret: decode %s response: %w", name, err)
	}
	data, err := base64.StdEncoding.DecodeString(out.Payload.Data)
	if err != nil {
		return nil, fmt.Errorf("gsecret: decode %s payload: %w", name, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("gsecret: %s has an empty payload", name)
	}
	if out.Payload.DataCrc32c != "" {
		want, err := strconv.ParseUint(out.Payload.DataCrc32c, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("gsecret: parse %s checksum: %w", name, err)
		}
		if got := crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)); uint64(got) != want {
			return nil, errors.New("gsecret: payload checksum mismatch for " + name)
		}
	}
	return data, nil
}

// errorMessage extracts Google's error.message, falling back to the raw
// (bounded) body.
func errorMessage(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, maxErrorBody))
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return strings.TrimSpace(string(b))
}
