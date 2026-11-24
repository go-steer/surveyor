# Agent identity: the go-steer-agent GitHub App

Status: decided 2026-11-24, not yet set up. Background is in
[DESIGN.md §6.4](./DESIGN.md#64-how-bots-are-kept-from-editing-codeowners).

## Why

Coding agents have been opening pull requests with the maintainer's own
GitHub token (308 of 360 go-steer PRs in the 30 days to 2026-11-24 were
opened as `mastersingh24`). With surveyor's approval enough to merge, that
breaks the one place a human must sign off:

- An agent holding the maintainer's token can approve another author's PR to
  a code-owned path, as the maintainer.
- It can merge with `--admin` or push straight to `main` wherever admins are
  not subject to the rules.
- GitHub doesn't let anyone approve their own PR, so an agent PR opened as the
  maintainer that touches a code-owned path can never get the code-owner
  approval it needs.

The likely failure is not an attacker. It is an agent told to get CI green
that raises a lint threshold or edits a workflow, then merges. Prompt
injection through issues and PR comments on public repos is the other route.

## The model

| | Who |
|---|---|
| Commit author | The maintainer, unchanged (`git config user.name/user.email`) |
| Pushes branches, opens PRs, enables auto-merge | `go-steer-agent[bot]`, via `agent-token` |
| Approves PRs | surveyor; the maintainer on code-owned paths |
| Ruleset bypass | The maintainer only (repository admin role, "pull requests only") |

A commit's author is metadata in git, so the history and contributor graph
still credit the maintainer. The App appears only as the PR's opener. The
no-agent-attribution scanner checks commits and the PR title and body, not
the opener.

**To confirm once the App exists:** open one PR with the App, squash-merge it,
and check who GitHub records as the squash commit's author. If it is the
App, merge agent PRs with rebase instead, or adjust the attribution rule.

## Create the App (maintainer, about 15 minutes)

GitHub org settings → Developer settings → GitHub Apps → New GitHub App.

- **Name:** `go-steer-agent`
- **Homepage URL:** `https://github.com/go-steer/surveyor`
- **Webhook:** uncheck "Active". The App only acts; it doesn't listen.
- **Where can this App be installed:** Only on this account.

**Repository permissions:**

| Permission | Access | Why |
|---|---|---|
| Contents | Read and write | Push branches. `v*` tags are blocked by each repo's tag ruleset |
| Pull requests | Read and write | Open PRs, reply to review threads, enable auto-merge |
| Issues | Read and write | File and comment on issues |
| Actions | Read | Read CI logs to fix failures |
| Checks | Read | See check results |
| Commit statuses | Read | See status results |
| Metadata | Read | Required by GitHub |
| Workflows | **No access** | See below |
| Administration | **No access** | Cannot touch rulesets, branch protection or settings |
| Secrets, Environments, Deployments | **No access** | |

**Organization and account permissions:** none.

After creating it:

1. Note the App ID (or client ID).
2. Generate a private key. Store it where agents run with owner-only
   permissions, e.g. `~/.config/go-steer-agent/key.pem` (`chmod 600`). For
   agents on GKE, keep it in Secret Manager and mount it.
3. Install the App on go-steer with **Only select repositories**: start with
   `surveyor` and `core-agent`, then add repos as their baseline issues land.

### Why no Workflows permission (for now)

Without it, the App cannot push any commit that changes `.github/workflows/`.
That matters because workflows on a same-repo branch run with the repo's
secrets on `pull_request`, so pushing a modified workflow is enough to read
secrets, before any review or merge.

The cost is real: 19 of 127 commits on core-agent's `main` in the 30 days to
2026-11-24 touched workflows, and those changes would need the maintainer to
push them. To lift this, first move each repo's secrets into GitHub
Environments restricted to `main` and release tags, so branch runs cannot
read them. Then grant Workflows: Read and write. Workflow files stay
code-owned either way.

## Point agents at it

Build the helper:

```sh
go install github.com/go-steer/surveyor/cmd/agent-token@latest
```

In the environment agents run in:

```sh
export AGENT_TOKEN_APP_ID=<App ID or client ID>
export AGENT_TOKEN_KEY_FILE=$HOME/.config/go-steer-agent/key.pem
# AGENT_TOKEN_OWNER defaults to go-steer

# git: fetch and push as the App.
git config --global credential.https://github.com.helper ''
git config --global --add credential.https://github.com.helper '!agent-token git-credential'

# gh: a fresh token per call (cached; re-minted when under 10 minutes remain).
gh() { GH_TOKEN="$(agent-token)" command gh "$@"; }
```

Then remove the maintainer's token from that environment: `gh auth logout`
there, or give agents a separate `GH_CONFIG_DIR`, and unset any `GH_TOKEN` or
`GITHUB_TOKEN` that holds a personal token. The maintainer keeps their own
token for their own use, including approving and merging code-owned PRs.

Commits keep the maintainer as author because git's `user.name` and
`user.email` are unchanged.

Some `gh` commands don't work with installation tokens (for example
`gh api user`, which needs a user). Pull requests, issues, checks and
`gh pr merge --auto` do.

## Repo changes that go with it

Each repo's baseline issue covers these:

- **Rulesets:** pull request required; code-owner review; conversation
  resolution; no force pushes or deletions; the repository admin role as the
  only bypass, in "pull requests only" mode. A tag ruleset so only admins can
  create, move or delete `v*` tags, because `contents: write` would otherwise
  let the App push a release tag.
- **Do not** add `go-steer-agent[bot]` to `review-gate`'s bot exemption. Its
  PRs are agent-written code and get reviewed like any other.
- **Do not** add it to any bypass list.
