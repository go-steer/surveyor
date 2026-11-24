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

package ghapp

import (
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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestParsePrivateKey(t *testing.T) {
	key := testKey(t)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"PKCS#1": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
		"PKCS#8": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParsePrivateKey(in)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(key) {
				t.Fatal("parsed key differs from original")
			}
		})
	}
}

func TestParsePrivateKeyRejects(t *testing.T) {
	cases := map[string][]byte{
		"not PEM":      []byte("hello"),
		"wrong type":   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1}}),
		"corrupt body": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte{1, 2, 3}}),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePrivateKey(in); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

// verifyJWT checks the signature and returns the claims.
func verifyJWT(t *testing.T, jwt string, pub *rsa.PublicKey) map[string]any {
	t.Helper()
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("bad signature: %v", err)
	}
	var header map[string]string
	b, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if err := json.Unmarshal(b, &header); err != nil {
		t.Fatal(err)
	}
	if header["alg"] != "RS256" {
		t.Fatalf("alg = %q, want RS256", header["alg"])
	}
	var claims map[string]any
	b, _ = base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(b, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func TestSignJWT(t *testing.T) {
	key := testKey(t)
	jwt, err := SignJWT("12345", key, testNow)
	if err != nil {
		t.Fatal(err)
	}
	claims := verifyJWT(t, jwt, &key.PublicKey)
	if claims["iss"] != "12345" {
		t.Errorf("iss = %v, want 12345", claims["iss"])
	}
	iat, exp := int64(claims["iat"].(float64)), int64(claims["exp"].(float64))
	if iat != testNow.Add(-time.Minute).Unix() {
		t.Errorf("iat not backdated by one minute: %d", iat)
	}
	// GitHub rejects JWTs whose lifetime exceeds 10 minutes.
	if exp-iat > 600 {
		t.Errorf("JWT lifetime %ds exceeds GitHub's 600s limit", exp-iat)
	}
}

func TestSignJWTRejectsMissingInputs(t *testing.T) {
	if _, err := SignJWT("", testKey(t), testNow); err == nil {
		t.Error("empty App ID: want error")
	}
	if _, err := SignJWT("1", nil, testNow); err == nil {
		t.Error("nil key: want error")
	}
}

func TestTokenValidFor(t *testing.T) {
	tok := Token{Value: "x", ExpiresAt: testNow.Add(10 * time.Minute)}
	if !tok.ValidFor(5*time.Minute, testNow) {
		t.Error("token with 10m left should be valid for 5m")
	}
	if tok.ValidFor(15*time.Minute, testNow) {
		t.Error("token with 10m left should not be valid for 15m")
	}
	if (Token{ExpiresAt: testNow.Add(time.Hour)}).ValidFor(0, testNow) {
		t.Error("empty token should never be valid")
	}
}

type fakeGitHub struct {
	t        *testing.T
	pub      *rsa.PublicKey
	orgFound bool
	lastBody string
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, `{"message":"no auth"}`, http.StatusUnauthorized)
		return
	}
	verifyJWT(f.t, strings.TrimPrefix(auth, "Bearer "), f.pub)
	b, _ := io.ReadAll(r.Body)
	f.lastBody = string(b)
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/orgs/go-steer/installation" && f.orgFound:
		_, _ = io.WriteString(w, `{"id": 42}`)
	case r.Method == http.MethodGet && r.URL.Path == "/users/someone/installation":
		_, _ = io.WriteString(w, `{"id": 7}`)
	case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"token":"ghs_test","expires_at":"2026-11-24T13:00:00Z"}`)
	case r.Method == http.MethodPost && r.URL.Path == "/app/installations/99/access_tokens":
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"Resource not accessible by integration"}`)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"Not Found"}`)
	}
}

func newTestClient(t *testing.T, orgFound bool) (*Client, *fakeGitHub) {
	t.Helper()
	key := testKey(t)
	fake := &fakeGitHub{t: t, pub: &key.PublicKey, orgFound: orgFound}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return &Client{AppID: "1", Key: key, BaseURL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return testNow }}, fake
}

func TestInstallationID(t *testing.T) {
	c, _ := newTestClient(t, true)
	id, err := c.InstallationID(context.Background(), "go-steer")
	if err != nil || id != 42 {
		t.Fatalf("org installation = %d, %v; want 42", id, err)
	}
	// Not an org: falls back to the user endpoint.
	id, err = c.InstallationID(context.Background(), "someone")
	if err != nil || id != 7 {
		t.Fatalf("user installation = %d, %v; want 7", id, err)
	}
	// Neither: the error says so.
	_, err = c.InstallationID(context.Background(), "nobody")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("missing installation: err = %v, want 404 APIError", err)
	}
}

func TestInstallationToken(t *testing.T) {
	c, fake := newTestClient(t, true)
	tok, err := c.InstallationToken(context.Background(), 42, TokenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if tok.Value != "ghs_test" || !tok.ExpiresAt.Equal(time.Date(2026, 11, 24, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("token = %+v", tok)
	}
	if fake.lastBody != "" {
		t.Errorf("zero options sent a body: %q", fake.lastBody)
	}

	_, err = c.InstallationToken(context.Background(), 42, TokenOptions{Repositories: []string{"surveyor"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastBody, `"repositories":["surveyor"]`) {
		t.Errorf("down-scoped request body = %q", fake.lastBody)
	}
}

func TestInstallationTokenError(t *testing.T) {
	c, _ := newTestClient(t, true)
	_, err := c.InstallationToken(context.Background(), 99, TokenOptions{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("err = %v, want 403 APIError", err)
	}
	if !strings.Contains(err.Error(), "Resource not accessible by integration") {
		t.Errorf("error lacks GitHub's message: %v", err)
	}
}
