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

// Package ghapp authenticates as a GitHub App: it signs the App's JWT and
// exchanges it for short-lived installation access tokens.
//
// It has no dependencies outside the standard library. The surveyor service
// uses it to act as the surveyor App, and cmd/agent-token uses it to give
// coding agents their own identity (see docs/agent-identity.md).
package ghapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the GitHub REST API root.
const DefaultBaseURL = "https://api.github.com"

// GitHub rejects App JWTs that live longer than 10 minutes, and clocks
// drift, so the token is backdated by a minute and lives for nine.
const (
	jwtBackdate = 60 * time.Second
	jwtLifetime = 9 * time.Minute
)

// maxErrorBody bounds how much of an error response is quoted in an error.
const maxErrorBody = 4 << 10

// ParsePrivateKey parses a PEM-encoded RSA private key. GitHub issues App
// keys as PKCS#1 ("RSA PRIVATE KEY"); PKCS#8 ("PRIVATE KEY") is accepted too
// so a key re-encoded by other tooling still works.
func ParsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("ghapp: no PEM block found in private key")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("ghapp: parse PKCS#1 key: %w", err)
		}
		return key, nil
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("ghapp: parse PKCS#8 key: %w", err)
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("ghapp: PKCS#8 key is %T, want RSA", parsed)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("ghapp: unsupported PEM block type %q", block.Type)
	}
}

// SignJWT returns an RS256 JWT that authenticates as the App with the given
// ID (the numeric App ID or the App's client ID) at time now.
func SignJWT(appID string, key *rsa.PrivateKey, now time.Time) (string, error) {
	if appID == "" {
		return "", errors.New("ghapp: empty App ID")
	}
	if key == nil {
		return "", errors.New("ghapp: nil private key")
	}
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", fmt.Errorf("ghapp: encode JWT header: %w", err)
	}
	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-jwtBackdate).Unix(),
		"exp": now.Add(jwtLifetime).Unix(),
		"iss": appID,
	})
	if err != nil {
		return "", fmt.Errorf("ghapp: encode JWT claims: %w", err)
	}
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("ghapp: sign JWT: %w", err)
	}
	return signingInput + "." + enc.EncodeToString(sig), nil
}

// Token is an installation access token.
type Token struct {
	Value     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ValidFor reports whether the token is still valid for at least d after now.
func (t Token) ValidFor(d time.Duration, now time.Time) bool {
	return t.Value != "" && now.Add(d).Before(t.ExpiresAt)
}

// TokenOptions narrows an installation token below the installation's
// grant. The zero value asks for everything the installation allows.
type TokenOptions struct {
	// Repositories limits the token to these repository names (not
	// owner/name). Empty means every repository the installation covers.
	Repositories []string `json:"repositories,omitempty"`
	// Permissions limits the token to these permissions, e.g.
	// {"contents": "write", "pull_requests": "write"}. Empty means the
	// installation's full set.
	Permissions map[string]string `json:"permissions,omitempty"`
}

// Client talks to the GitHub API as one App.
type Client struct {
	AppID string
	Key   *rsa.PrivateKey

	// BaseURL defaults to DefaultBaseURL.
	BaseURL string
	// HTTP defaults to a client with a 30-second timeout.
	HTTP *http.Client
	// Now defaults to time.Now; tests override it.
	Now func() time.Time
}

// APIError is a non-2xx response from GitHub.
type APIError struct {
	Method, Path string
	StatusCode   int
	Message      string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ghapp: %s %s: HTTP %d: %s", e.Method, e.Path, e.StatusCode, e.Message)
}

// InstallationID returns the ID of the App's installation on owner, an
// organization or user login. It tries the organization endpoint first and
// falls back to the user endpoint on 404.
func (c *Client) InstallationID(ctx context.Context, owner string) (int64, error) {
	if owner == "" {
		return 0, errors.New("ghapp: empty installation owner")
	}
	var out struct {
		ID int64 `json:"id"`
	}
	esc := url.PathEscape(owner)
	err := c.do(ctx, http.MethodGet, "/orgs/"+esc+"/installation", nil, &out)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		err = c.do(ctx, http.MethodGet, "/users/"+esc+"/installation", nil, &out)
	}
	if err != nil {
		return 0, err
	}
	if out.ID == 0 {
		return 0, fmt.Errorf("ghapp: no installation ID in response for %q", owner)
	}
	return out.ID, nil
}

// InstallationToken mints a new installation access token. GitHub tokens
// last one hour; callers should cache and reuse them (see Token.ValidFor).
func (c *Client) InstallationToken(ctx context.Context, installationID int64, opts TokenOptions) (Token, error) {
	var body any
	if len(opts.Repositories) > 0 || len(opts.Permissions) > 0 {
		body = opts
	}
	var tok Token
	path := "/app/installations/" + strconv.FormatInt(installationID, 10) + "/access_tokens"
	if err := c.do(ctx, http.MethodPost, path, body, &tok); err != nil {
		return Token{}, err
	}
	if tok.Value == "" || tok.ExpiresAt.IsZero() {
		return Token{}, errors.New("ghapp: access token response missing token or expires_at")
	}
	return tok, nil
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	jwt, err := SignJWT(c.AppID, c.Key, now())
	if err != nil {
		return err
	}
	var reqBody io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("ghapp: encode request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, reqBody)
	if err != nil {
		return fmt.Errorf("ghapp: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("ghapp: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Method: method, Path: path, StatusCode: resp.StatusCode, Message: errorMessage(resp.Body)}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("ghapp: decode %s %s response: %w", method, path, err)
	}
	return nil
}

// errorMessage extracts GitHub's "message" field, falling back to the raw
// (bounded) body.
func errorMessage(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, maxErrorBody))
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &e) == nil && e.Message != "" {
		return e.Message
	}
	return strings.TrimSpace(string(b))
}
