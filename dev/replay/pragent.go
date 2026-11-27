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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-steer/core-agent/v3/pkg/usage"

	"github.com/go-steer/surveyor/internal/review"
	"github.com/go-steer/surveyor/internal/source"
)

// PR-Agent (github.com/The-PR-Agent/pr-agent) runs in its plain-diff mode:
// it reads the PR's base...head diff from a file and never talks to a
// forge, so it cannot post to the pull request. Its findings come from
// the --json-output file.

func isPRAgent(name string) bool {
	return name == "pr-agent" || strings.HasPrefix(name, "pr-agent-")
}

type prAgentOutput struct {
	Review struct {
		KeyIssues []struct {
			File      string `json:"relevant_file"`
			Header    string `json:"issue_header"`
			Content   string `json:"issue_content"`
			StartLine int    `json:"start_line"`
		} `json:"key_issues_to_review"`
		Security string `json:"security_concerns"`
	} `json:"review"`
	Usage struct {
		Prompt     int `json:"prompt_tokens"`
		Completion int `json:"completion_tokens"`
	} `json:"usage"`
	Coverage struct {
		NotReviewed       []string `json:"files_not_reviewed"`
		PartiallyReviewed []string `json:"files_partially_reviewed"`
		FailedChunks      int      `json:"failed_chunks"`
	} `json:"coverage"`
}

func (rc *runConfig) runPRAgent(ctx context.Context, c Case, name string) (*Run, error) {
	dir := caseDir(rc.dir, c.Number)
	src, err := checkout(ctx, rc.dir, rc.token, c)
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	pr, err := (&source.Client{Token: rc.token}).FetchPR(ctx, c.Repo, c.Number)
	if err != nil {
		return nil, err
	}
	diffPath := filepath.Join(dir, name+".diff")
	if err := os.WriteFile(diffPath, []byte(pr.Diff), 0o644); err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(dir, name+".out.json")
	_ = os.Remove(jsonPath)
	logFile, err := os.Create(filepath.Join(dir, name+".log"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.CommandContext(ctx, rc.prAgentPython, "-m", "pr_agent.cli",
		"--diff-file", diffPath, "--json-output", jsonPath, "--output", filepath.Join(dir, name+".md"), "review")
	cmd.Dir, cmd.Stdout, cmd.Stderr = src, logFile, logFile
	cmd.Env = append(os.Environ(),
		"CONFIG__MODEL="+rc.prAgentModel,
		`CONFIG__FALLBACK_MODELS=["`+rc.prAgentModel+`"]`,
		"VERTEXAI__VERTEX_PROJECT="+rc.project,
		"VERTEXAI__VERTEX_LOCATION="+rc.region,
		"PLAIN_DIFF__TITLE="+pr.Title,
		"PLAIN_DIFF__DESCRIPTION="+pr.Body,
	)
	cmd.Env = append(cmd.Env, rc.prAgentEnv...)
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pr-agent: %w (see %s.log)", err, name)
	}
	var out prAgentOutput
	if err := readJSON(jsonPath, &out); err != nil {
		return nil, fmt.Errorf("pr-agent output: %w", err)
	}
	r := &Run{Reviewer: name, Model: rc.prAgentModel, Args: rc.prAgentEnv, PR: c.Number, RanAt: start.UTC(),
		Seconds: time.Since(start).Seconds()}
	r.Findings = out.findings()
	cov := &review.Coverage{DiffBytes: len(pr.Diff),
		DiffTruncated: len(out.Coverage.NotReviewed)+len(out.Coverage.PartiallyReviewed)+out.Coverage.FailedChunks > 0}
	r.Coverage = cov
	if cov.DiffTruncated {
		r.Text = fmt.Sprintf("files not reviewed: %v; partially reviewed: %v; failed chunks: %d",
			out.Coverage.NotReviewed, out.Coverage.PartiallyReviewed, out.Coverage.FailedChunks)
	}
	r.CostUSD = rc.price(strings.TrimPrefix(rc.prAgentModel, "vertex_ai/"), out.Usage.Prompt, out.Usage.Completion)
	return r, nil
}

// findings converts PR-Agent's key issues, and a security concern other
// than "No", into findings. PR-Agent has no severity scale; its key issues
// are what it would hold a merge for, so they count as important.
func (o prAgentOutput) findings() []review.Finding {
	var fs []review.Finding
	for _, k := range o.Review.KeyIssues {
		fs = append(fs, review.Finding{File: k.File, Line: k.StartLine, Severity: review.SeverityImportant,
			Category: strings.ToLower(strings.ReplaceAll(strings.TrimSpace(k.Header), " ", "-")),
			Summary:  strings.TrimSpace(k.Header) + ": " + strings.TrimSpace(k.Content), FailureScenario: strings.TrimSpace(k.Content)})
	}
	if s := strings.TrimSpace(o.Review.Security); s != "" && !strings.EqualFold(strings.TrimRight(s, ". \n"), "no") {
		fs = append(fs, review.Finding{Severity: review.SeverityImportant, Category: "security", Summary: s, FailureScenario: s})
	}
	return fs
}

// price prices one model's tokens on surveyor's Vertex catalog; 0 when the
// model has no rate.
func (rc *runConfig) price(model string, in, out int) float64 {
	if rc.claude == nil {
		return 0
	}
	p := usage.PriceOnCatalog(rc.claude.Pricing, rc.claude.Backend, model)
	return (float64(in)*p.InputPerMTok + float64(out)*p.OutputPerMTok) / 1e6
}
