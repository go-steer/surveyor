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

// Command replay measures surveyor against another reviewer on pull
// requests that already merged.
//
//	replay select --dir D --repo go-steer/core-agent --since 2026-10-27 --limit 30 [--include 1614,1543]
//	replay run    --dir D [--reviewers surveyor,claude-code] [--parallel 3]
//	replay run    --dir D --reviewers surveyor-opus --surveyor-args "--model claude-opus-5-5"
//	replay extract --dir D
//	replay judge  --dir D
//	replay report --dir D > D/report.md
//
// select writes D/cases.json. run reviews every case with every reviewer
// and writes D/pr-N/<reviewer>.json; it skips results that exist, so an
// interrupted run resumes where it stopped. extract turns claude-code's
// findings, which headless runs write as text, into structured ones.
// judge gives every finding, from every reviewer, shuffled and
// unlabelled, to a blind judge (Opus on Vertex AI, reading the code at
// the PR's head) that labels it real, nit or false positive, groups
// findings about the same defect, and matches them to D/known-bugs.json:
// defects a later PR fixed. report prints the comparison.
//
// The reviewers: "surveyor" runs the surveyor binary (--surveyor) with
// --json, and "surveyor-<variant>" does the same with that run's
// --surveyor-args, stored under its own name; "claude-code" runs `claude -p "/code-review <level> <PR URL>"`
// in a checkout of the PR's head, with edits and PR comments disallowed,
// and reads the findings from its ReportFindings tool call, or, when it
// writes them as text instead, from extract.
//
// This is a measurement tool, not part of the shipped binary.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: replay select|run|extract|judge|report --dir DIR [flags]")
		return 2
	}
	var err error
	switch args[0] {
	case "select":
		err = cmdSelect(ctx, args[1:], stderr)
	case "run":
		err = cmdRun(ctx, args[1:], stderr)
	case "extract":
		err = cmdExtract(ctx, args[1:], stderr)
	case "judge":
		err = cmdJudge(ctx, args[1:], stderr)
	case "report":
		err = cmdReport(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "replay: unknown command %q\n", args[0])
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "replay: %v\n", err)
		return 1
	}
	return 0
}
