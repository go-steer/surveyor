# surveyor

AI code review for [go-steer](https://github.com/go-steer) pull requests.

A marine surveyor inspects a vessel and certifies it seaworthy. surveyor
reviews every pull request in the organizations it is installed in, posts
verified findings inline, and approves the pull request when nothing blocking
remains. Its approval is enough to merge, except on paths that
[`CODEOWNERS`](./.github/CODEOWNERS) reserves for a human.

> **Status: the review engine runs as a command; the GitHub App service is
> not built yet.** See [`docs/DESIGN.md`](./docs/DESIGN.md) for the
> architecture, the merge and security model, the configuration format, and
> the rollout plan.

## Try it

```sh
go install github.com/go-steer/surveyor/cmd/surveyor@latest
surveyor review --repo go-steer/core-agent --pr 1614
```

It checks out the pull request's head commit in a temporary directory,
reads the repository's rules (`AGENTS.md`, else `CLAUDE.md`) from the base
commit, and runs a finder pass and one verifier per finding with Claude on
Vertex AI (default `claude-sonnet-5-5` at thinking level `high` in project
`go-steer-drydock`; `--model`, `--level`, `--project`, `--region` change
that). It prints the verified findings, or `--json`. Nothing is posted.

The reviewer can read only the checkout and, read-only, a surveyor-owned Go
module cache holding the change's dependencies (`--deps=false` skips it).
It cannot write files, run commands or reach the network.

Credentials: Google Application Default Credentials for Vertex AI, and a
GitHub token from `GH_TOKEN`, `GITHUB_TOKEN` or `gh auth token`.

## In brief

- **A GitHub App served from GKE.** Webhooks in, reviews out. The App's
  private key lives in Secret Manager, never in GitHub Actions secrets, and
  models are reached on Vertex AI through Workload Identity.
- **Claude reviews first.** Claude Sonnet handles most pull requests and
  Claude Opus the risky or complex ones. Most go-steer code is written by
  Gemini, and a reviewer from a different model family is less likely to
  share the author's blind spots. Gemini 4 Neon gives an optional second
  opinion, and Gemini can be the primary reviewer for orgs that prefer it.
- **Deterministic checks run first.** surveyor reviews after the repo's
  static analysis passes, and reads those results so it doesn't repeat them.
- **Code decides the verdict, not model text.** A finder pass proposes
  findings and an independent verifier pass checks each one. Only verified
  findings are posted, and approval is computed from their severities.
- **Runs everywhere, configurable everywhere.** Installed org-wide with
  defaults from `<org>/.github/surveyor.yaml`. A repo can opt out, tune
  settings, or add custom rules in `.github/surveyor.yaml`, which is always
  read from the base branch.
- **One binary, two front ends.** `surveyor review --pr N` runs the same
  review locally before a pull request is opened.

## How well it reviews

[`docs/replay.md`](./docs/replay.md) replays surveyor and Claude Code's
`/code-review` on merged pull requests, with a blind judge. On 30
core-agent PRs, surveyor raised no false alarms, but it found 41% of the
defects found by either reviewer to Claude Code's 93%, at less than half
the cost.

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
