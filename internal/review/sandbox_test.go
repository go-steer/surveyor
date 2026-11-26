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

package review

import (
	"context"
	"encoding/json"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/go-steer/core-agent/v3/pkg/agent"
	"github.com/go-steer/core-agent/v3/pkg/permissions"
)

// escapeLLM asks for the calls in its first turn, then records every tool
// response it is shown and finishes.
type escapeLLM struct {
	calls []*genai.FunctionCall

	mu        sync.Mutex
	turn      int
	responses map[string]map[string]any // call ID -> response
}

func (m *escapeLLM) Name() string { return "escape" }

func (m *escapeLLM) GenerateContent(_ context.Context, req *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	m.mu.Lock()
	m.turn++
	turn := m.turn
	if turn > 1 {
		for _, c := range req.Contents {
			for _, p := range c.Parts {
				if p.FunctionResponse != nil {
					m.responses[p.FunctionResponse.ID] = p.FunctionResponse.Response
				}
			}
		}
	}
	m.mu.Unlock()
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		content := &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "done"}}}
		if turn == 1 {
			content = &genai.Content{Role: genai.RoleModel}
			for _, c := range m.calls {
				content.Parts = append(content.Parts, &genai.Part{FunctionCall: c})
			}
		}
		yield(&adkmodel.LLMResponse{Content: content, TurnComplete: true}, nil)
	}
}

// TestReadOnlyToolsStayInRoot drives the real tools through a real agent
// and tries every way out of the checkout a hostile PR could set up.
func TestReadOnlyToolsStayInRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "credentials")
	mustWrite(t, secret, "SECRET-TOKEN-VALUE")
	mustWrite(t, filepath.Join(root, "main.go"), "package main // inside\n")
	if err := os.Symlink(secret, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "dirlink")); err != nil {
		t.Fatal(err)
	}

	call := func(id, name string, args map[string]any) *genai.FunctionCall {
		return &genai.FunctionCall{ID: id, Name: name, Args: args}
	}
	llm := &escapeLLM{
		responses: map[string]map[string]any{},
		calls: []*genai.FunctionCall{
			call("inside", "read_file", map[string]any{"path": filepath.Join(root, "main.go")}),
			call("relative", "read_file", map[string]any{"path": "main.go"}),
			call("outside", "read_file", map[string]any{"path": secret}),
			call("dotdot", "read_file", map[string]any{"path": filepath.Join(root, "..", filepath.Base(outside), "credentials")}),
			call("symlink", "read_file", map[string]any{"path": filepath.Join(root, "link")}),
			call("dirlink", "read_file", map[string]any{"path": filepath.Join(root, "dirlink", "credentials")}),
			call("many", "read_many_files", map[string]any{"paths": []string{secret, filepath.Join(root, "link")}}),
			call("grep", "grep", map[string]any{"pattern": "SECRET", "path": root}),
			call("glob", "glob", map[string]any{"pattern": "credentials", "path": root}),
			call("listdir", "list_dir", map[string]any{"path": outside}),
			call("stat", "stat", map[string]any{"path": secret}),
		},
	}
	tools, err := ReadOnlyTools(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := agent.New(llm, agent.WithTools(tools), agent.WithMode(agent.ModeAutonomous))
	if err != nil {
		t.Fatal(err)
	}
	// Relative paths resolve against the checkout (how the engine runs
	// every review), not the process's working directory.
	for _, err := range a.Run(permissions.WithWorkDir(context.Background(), root), "review") {
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	got := func(id string) string {
		b, _ := json.Marshal(llm.responses[id])
		return string(b)
	}
	if r := got("inside"); !strings.Contains(r, "package main") {
		t.Errorf("in-root read failed: %s", r)
	}
	if r := got("relative"); !strings.Contains(r, "package main") {
		t.Errorf("relative in-root read failed: %s", r)
	}
	for _, id := range []string{"outside", "dotdot", "symlink", "dirlink", "many", "grep", "glob", "listdir", "stat"} {
		r := got(id)
		if r == "null" {
			t.Errorf("%s: no tool response recorded", id)
			continue
		}
		if strings.Contains(r, "SECRET-TOKEN-VALUE") {
			t.Errorf("%s: leaked the outside file: %s", id, r)
		}
	}
	for _, id := range []string{"outside", "dotdot", "symlink", "dirlink", "listdir", "stat"} {
		if r := got(id); !strings.Contains(strings.ToLower(r), "denied") && !strings.Contains(strings.ToLower(r), "error") {
			t.Errorf("%s: expected a denial, got %s", id, r)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A read-only root (the dependency module cache) is readable; the rest
// of the filesystem is not, including a directory beside it.
func TestReadOnlyRootsAreReadable(t *testing.T) {
	root := t.TempDir()
	parent := t.TempDir()
	deps := filepath.Join(parent, "gomod")
	sibling := filepath.Join(parent, "other")
	for _, d := range []string{deps, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(deps, "lib.go"), "package lib // dependency source\n")
	mustWrite(t, filepath.Join(sibling, "secret"), "SECRET-TOKEN-VALUE")

	llm := &escapeLLM{
		responses: map[string]map[string]any{},
		calls: []*genai.FunctionCall{
			{ID: "dep", Name: "read_file", Args: map[string]any{"path": filepath.Join(deps, "lib.go")}},
			{ID: "sibling", Name: "read_file", Args: map[string]any{"path": filepath.Join(sibling, "secret")}},
		},
	}
	tools, err := ReadOnlyTools(root, deps)
	if err != nil {
		t.Fatal(err)
	}
	a, err := agent.New(llm, agent.WithTools(tools), agent.WithMode(agent.ModeAutonomous))
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range a.Run(permissions.WithWorkDir(context.Background(), root), "review") {
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	}
	dep, _ := json.Marshal(llm.responses["dep"])
	if !strings.Contains(string(dep), "dependency source") {
		t.Errorf("read-only root not readable: %s", dep)
	}
	sib, _ := json.Marshal(llm.responses["sibling"])
	if strings.Contains(string(sib), "SECRET-TOKEN-VALUE") || !strings.Contains(string(sib), "denied") {
		t.Errorf("a directory beside the read-only root was readable: %s", sib)
	}
}
