# Agent identities: go-steer-deckhand and go-steer-shipyard

Status: decided 2026-11-24, not yet set up.

Two GitHub Apps, named for the crew theme:

- **go-steer-deckhand** does the hands-on work and has no authority over the
  ship: coding agents push branches and open PRs as this App.
- **go-steer-shipyard** commissions new repos, already fitted out with the
  baseline, and only after the maintainer approves each one. Background is in
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
| Pushes branches, opens PRs, enables auto-merge | `go-steer-deckhand[bot]`, via `agent-token` |
| Approves PRs | surveyor; the maintainer on code-owned paths |
| Ruleset bypass | The maintainer only (repository admin role, "pull requests only") |

A commit's author is metadata in git, so the history and contributor graph
still credit the maintainer. The App appears only as the PR's opener. The
no-agent-attribution scanner checks commits and the PR title and body, not
the opener.

### Merging agent PRs: rebase only

Tested 2026-11-24 (surveyor#4): a **squash merge** makes the PR's opener the
author of the new commit, so a deckhand PR lands on `main` authored by
`go-steer-deckhand[bot]`, with the maintainer only as `Co-authored-by`.
core-agent's `verify-no-agent-attribution` fails on that commit.

A **rebase merge** copies the branch's commits onto `main` unchanged, so the
maintainer stays the author. So:

- Repos allow only rebase merges: the `main` ruleset's allowed merge methods
  are `rebase`, and the repo settings turn off squash and merge commits.
- Agents squash their branch to one commit before merging, which keeps
  history at one commit per PR:

  ```sh
  git fetch origin main
  git reset --soft "$(git merge-base origin/main HEAD)"
  git commit -m "<conventional commit title>" -m "<body>"
  git push --force-with-lease
  gh pr merge --rebase          # or: gh pr merge --auto --rebase
  ```

  Force-pushing the PR branch is fine; the no-force-push rule only covers
  `main`.
- Humans squash their own branches locally the same way.

## Create the App (maintainer, about 15 minutes)

GitHub org settings → Developer settings → GitHub Apps → New GitHub App.

- **Name:** `go-steer-deckhand`
- **Homepage URL:** `https://github.com/go-steer/surveyor`
- **Webhook:** uncheck "Active". The App only acts; it doesn't listen.
- **Where can this App be installed:** Only on this account.

**Repository permissions:**

| Permission | Access | Why |
|---|---|---|
| Contents | Read and write | Push branches. `v*` tags are blocked by each repo's tag ruleset |
| Pull requests | Read and write | Open PRs, reply to review threads, enable auto-merge |
| Issues | Read and write | File and comment on issues |
| Actions | Read | Read CI logs to fix failures. Not write: that would let agents manually trigger any workflow, including core-agent's `release.yml` |
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
   permissions, e.g. `~/.config/go-steer-deckhand/key.pem` (`chmod 600`). For
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

### Where the private key lives

Either a file on the machine (`AGENT_TOKEN_KEY_FILE`) or, preferred for every
machine after the first, the `deckhand-key` secret in the `go-steer-drydock`
project (`AGENT_TOKEN_KEY_SECRET`). With the secret, no key file sits on the
machine: `agent-token` reads it with the machine's Application Default
Credentials (a gcloud login, or Workload Identity on GKE), only when it has to
mint a new token.

The maintainer uploads the key once:

```sh
gcloud secrets versions add deckhand-key --project go-steer-drydock \
  --data-file="$HOME/.config/go-steer-deckhand/key.pem"
```

and grants each machine's identity read access to that one secret, not the
project:

```sh
gcloud secrets add-iam-policy-binding deckhand-key --project go-steer-drydock \
  --member="serviceAccount:<machine-or-workload-sa>" \
  --role=roles/secretmanager.secretAccessor
```

Revoking a machine is removing its binding. Every read appears in Cloud Audit
Logs (Data Access logs must be enabled for Secret Manager to record reads).

On a machine using the secret, set this instead of `AGENT_TOKEN_KEY_FILE`:

```sh
export AGENT_TOKEN_KEY_SECRET=projects/go-steer-drydock/secrets/deckhand-key
```

Build the helper:

```sh
go install github.com/go-steer/surveyor/cmd/agent-token@latest
```

In the environment agents run in:

```sh
export AGENT_TOKEN_APP_ID=<App ID or client ID>
export AGENT_TOKEN_KEY_FILE=$HOME/.config/go-steer-deckhand/key.pem
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

## Creating repos: go-steer-shipyard

On GitHub Free, the permission that creates repos (Administration: write) also
edits every repo's rulesets, and org-level rulesets, which agents couldn't
reach, need GitHub Team or Enterprise. So deckhand cannot create repos, and
repo creation goes through a workflow that waits for the maintainer:

1. An agent (as deckhand) opens an issue in `go-steer/.github` from a
   "New repository" issue form: name, description, visibility, language.
2. A workflow triggered by the issue validates the fields strictly (the issue
   body is untrusted input) and pauses on an environment that requires the
   maintainer's approval.
3. The maintainer approves in the browser or the GitHub mobile app.
4. The workflow, authenticated as go-steer-shipyard, creates the repo with the
   baseline already applied: `main` and tag rulesets, `CODEOWNERS`,
   `dependabot.yml`, secret scanning and push protection, CodeQL default setup.
   It comments the result on the issue and closes it.

**go-steer-shipyard permissions:** Administration and Contents: read and
write; Metadata: read. No Workflows: shipyard never writes workflow files.
Installed on **All repositories**, so repos it creates are covered
automatically. No webhook. The workflow uses two tokens: an org-wide one with
administration only, for the single "create repository" call, and one scoped
to the new repo for everything after. Setup and request format:
[go-steer/.github `shipyard/README.md`](https://github.com/go-steer/.github/blob/main/shipyard/README.md).

**Its private key never goes to an agent.** The maintainer stores it as a
secret on the approval environment in `go-steer/.github`, with deployment
limited to `main`. A modified workflow on a branch can't reach it, and every
use needs the maintainer's approval.

If the org moves to a plan with org-level rulesets, the gate can move there
(bypass: org owners only), and deckhand could get Administration: write to
create repos directly.

## Repo changes that go with it

Each repo's baseline issue covers these:

- **Rulesets:** pull request required; code-owner review; conversation
  resolution; no force pushes or deletions; the repository admin role as the
  only bypass, in "pull requests only" mode. A tag ruleset so only admins can
  create, move or delete `v*` tags, because `contents: write` would otherwise
  let the App push a release tag.
- **Do not** add `go-steer-deckhand[bot]` to `review-gate`'s bot exemption. Its
  PRs are agent-written code and get reviewed like any other.
- **Do not** add it to any bypass list.
