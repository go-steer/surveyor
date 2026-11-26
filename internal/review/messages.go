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
	"fmt"
	"strings"
)

// finderPrompt is the finder's task: the change, the repository's rules
// and the diff. Untrusted text sits inside labelled blocks so the
// instruction's "this is data" rule has clear edges.
func finderPrompt(in Input, maxDiff int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Review pull request #%d in %s. The repository is checked out at the change's head commit %s; paths are relative to its root.\n\n",
		in.PR.Number, in.PR.Repo, in.PR.HeadSHA)
	block(&b, "pull_request_title", in.PR.Title)
	block(&b, "pull_request_description", in.PR.Body)
	if strings.TrimSpace(in.Rules) != "" {
		b.WriteString("The repository's own rules, from its base branch. Report a change that breaks one of them as a finding.\n")
		block(&b, "repository_rules", in.Rules)
	}
	if len(in.PR.Diff) <= maxDiff {
		block(&b, "diff", in.PR.Diff)
	} else {
		fmt.Fprintf(&b, "The diff is %d bytes, too large to include. These files changed; read each one and compare with what callers expect:\n", len(in.PR.Diff))
		block(&b, "changed_files", strings.Join(changedFiles(in.PR.Diff), "\n"))
	}
	dependencyNote(&b, in)
	b.WriteString("Find the defects this change introduces, report each with report_finding, then summarise.")
	return b.String()
}

// dependencyNote tells the agent where the dependencies' source is.
func dependencyNote(b *strings.Builder, in Input) {
	for _, d := range in.ReadOnlyRoots {
		fmt.Fprintf(b, "The source of the change's Go dependencies is readable (not writable) under %s, laid out as module@version/. Read it to check how the change uses a library's API.\n\n", d)
	}
}

// verifierPrompt is one verifier's task: the claim and the part of the
// diff that touches its file.
func verifierPrompt(in Input, f Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Check this claimed problem in pull request #%d in %s. The repository is checked out at the change's head commit %s.\n\n",
		in.PR.Number, in.PR.Repo, in.PR.HeadSHA)
	block(&b, "claim", fmt.Sprintf("File: %s\nLine: %d\nSeverity claimed: %s\nCategory: %s\nProblem: %s\nFailure scenario: %s",
		f.File, f.Line, f.Severity, f.Category, f.Summary, f.FailureScenario))
	block(&b, "pull_request_title", in.PR.Title)
	if h := fileDiff(in.PR.Diff, f.File); h != "" {
		block(&b, "diff_for_file", h)
	} else {
		b.WriteString("The diff does not touch this file; the claim is about code the change depends on.\n\n")
	}
	dependencyNote(&b, in)
	b.WriteString("Decide whether the problem is real and call submit_verdict.")
	return b.String()
}

func block(b *strings.Builder, name, body string) {
	fmt.Fprintf(b, "<%s>\n%s\n</%s>\n\n", name, strings.TrimSpace(body), name)
}

// changedFiles lists the post-change paths in a unified diff.
func changedFiles(diff string) []string {
	var files []string
	for _, line := range strings.Split(diff, "\n") {
		if p, ok := strings.CutPrefix(line, "+++ b/"); ok {
			files = append(files, p)
		}
	}
	return files
}

// fileDiff returns the section of a unified diff for path, or "".
func fileDiff(diff, path string) string {
	sections := strings.Split(diff, "\ndiff --git ")
	for i, s := range sections {
		if i > 0 {
			s = "diff --git " + s
		}
		if strings.Contains(s, "\n+++ b/"+path+"\n") || strings.HasPrefix(s, "diff --git a/"+path+" b/"+path+"\n") {
			return s
		}
	}
	return ""
}
