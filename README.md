# surveyor

AI code review for [go-steer](https://github.com/go-steer) pull requests.

A marine surveyor inspects a vessel and certifies it seaworthy. surveyor
reviews every pull request in the organizations it is installed in, posts
verified findings inline, and approves the pull request when nothing blocking
remains. Its approval is enough to merge, except on paths that
[`CODEOWNERS`](./.github/CODEOWNERS) reserves for a human.

> **Status: design.** Nothing is built yet. See
> [`docs/DESIGN.md`](./docs/DESIGN.md) for the architecture, the merge and
> security model, the configuration format, and the rollout plan.

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

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
