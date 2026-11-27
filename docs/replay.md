# Replay: measuring surveyor on merged pull requests

`dev/replay` re-runs reviewers on pull requests that have already merged,
then has a blind judge label every finding. It answers the questions that
matter before surveyor's approval gates a merge: which bugs it catches,
which it misses, how often it raises false alarms, and what each review
costs.

## Method

```sh
go build -o bin/surveyor ./cmd/surveyor && go build -o bin/replay ./dev/replay
D=~/replay/core-agent-2026-11
bin/replay select  --dir $D --repo go-steer/core-agent --since 2026-10-27 --limit 30 --include 1431,1614
bin/replay run     --dir $D --surveyor bin/surveyor --surveyor-args "--project go-steer-drydock"
bin/replay extract --dir $D
bin/replay judge   --dir $D
bin/replay report  --dir $D > $D/report.md
```

1. **Cases.** `select` takes merged PRs, skipping bots, release chores and
   PRs with no code. `--include` adds PRs that are known to have
   introduced a bug.
2. **Known bugs** (`known-bugs.json`, written by hand) are defects that a
   merged PR introduced and a later PR fixed. They are found SZZ-style:
   - Take the fix PRs, blame the lines each fix changed, and map the
     result to the introducing PR.
   - Keep a case only when the defect is visible in the introducing PR's
     own diff.
   - Many are omissions, such as a new field one function forgot, which
     plain blame mislabels. So each entry is checked by hand against the
     introducing PR's merge commit.
3. **Reviewers.**
   - **`surveyor`** runs `surveyor review --json` at its defaults.
   - **`claude-code`** runs `claude -p "/code-review high <PR URL>"`:
     - It runs in a checkout of the PR's head.
     - Edits, PR comments, reviews and pushes are disallowed.
     - Headless runs write their findings as text rather than calling
       `ReportFindings`, so `extract` copies them out with Sonnet, told to
       copy and not judge.
   - Results are written atomically per PR, and `run` skips finished ones,
     so an interrupted run resumes where it stopped.
4. **Judge.** Opus 5.5 at `xhigh`, sandboxed like surveyor, reads the code
   at the PR's head and gets every finding from every reviewer:
   - Findings are shuffled and the reviewer is hidden.
   - surveyor's refuted findings are included, to measure what
     verification throws away.
   - It labels each finding *real*, *nit* or *false positive*.
   - It groups findings about the same defect, and matches them to the
     known bugs.
   - A **defect** is a group with at least one finding labelled real.

## First run: 30 core-agent PRs (2026-11-27)

The set: 14 PRs that introduced the 25 known bugs (2026-10-02 to 11-25),
plus the 16 most recent PRs that change code. The full report is in
[`replay/2026-11-core-agent.md`](./replay/2026-11-core-agent.md).

| | Claude Code `/code-review high` | surveyor |
|---|---|---|
| model | Opus 5.5 (session default) | Sonnet 5.5, finder `high`, verifier `medium` |
| findings | 212 | 47 |
| real / nit / false alarm | 73 / 129 / 10 | 31 / 16 / 0 |
| findings marked important that are real | 66 of 107 (62%) | 29 of 30 (97%) |
| defects found (of 75 found by either) | 70 (93%) | 31 (41%) |
| defects only this reviewer found | 44 | 5 |
| known bugs caught (of 25) | 14 | 9 |
| cost per PR, mean | $1.22 | $0.55 |
| time per PR, mean (max) | 219s (482s) | 92s (385s) |

Judging cost $32.56 in total ($1.09 per PR).

### What it says

- **surveyor is precise but finds too little.**
  - Nearly everything it reports is real, and it raised no false alarm in
    30 PRs.
  - It reports about 1.6 findings per PR and misses more than half of
    what Claude Code finds, including 6 known bugs Claude Code caught.
  - It caught 1 known bug Claude Code missed: #1465, where `Append`'s 404
    was never mapped to a fallback.
- **The verifier did nothing.** It refuted 0 of 47 findings: the finder
  only reports what it has already traced. Either the finder should
  report more and let the verifier filter, or the verifier should go.
- **Claude Code is thorough but noisy.**
  - 61% of its findings are nits.
  - It hits the 10-finding default cap on most large PRs, so its true
    count is higher.
  - Its 10 false alarms would each have cost a maintainer a look.
  - It also runs commands (`go vet`, `go test`) that surveyor's sandbox
    does not allow.
- **Quiet PRs stay quiet with surveyor.** On 11 of the 17 PRs since
  11-16, neither reviewer found a real defect:
  - surveyor reported nothing on 10 of them, and 2 nits on the other.
  - Claude Code reported 1–10 findings on each, judged nits or false
    alarms.
  - On #1614, only surveyor found a real defect: a second real bug, since
    fixed in core-agent#1617.
- **Both missed 10 of the 25 known bugs.** These are mostly omissions,
  plus bugs that need context outside the diff: a server path that never
  records usage, or a scope check that skips skill-level auth.

### Limitations

- **One run per reviewer per PR.** Reviews vary from run to run. On
  #1614, surveyor found the `writeSubagentMarkdownFile` bug on two
  earlier runs and a different real bug on this one.
- **The judge is Opus, the same model as the Claude Code run.** The judge
  is blind to the reviewer, but it may favour Opus-style findings.
- **The judge sees the known bugs**, which it needs in order to match
  findings to them. That can make a finding resembling a known bug
  likelier to be labelled real. Those findings are real by the ground
  truth anyway, but it means the judge is not independent of
  `known-bugs.json`.
- **An independent audit of the judge.** An agent labelled a stratified
  random sample of 15 judgements from the code before seeing the judge's
  labels. It agreed on 12 of 15 (80%), and the 3 disagreements went in
  different directions:
  - one "real" it would call a nit (though `known-bugs.json` backs the
    judge);
  - one "nit" it would call a false alarm;
  - one "nit" it would call real, an `AGENTS.md` error-surfacing rule the
    judge missed.

  Read individual labels with that error rate in mind. The large gaps
  between the reviewers are well outside it.
- **Coverage is not measured.** The result does not record whether
  surveyor's finder hit its step cap (60) or stopped early. On #1445
  (454KB) and #1463 (201KB) the diff exceeded the 200KB prompt cap, so
  the finder got only the file list. So a "no findings" from surveyor
  cannot yet be told apart from an incomplete read.
- **Known bugs are a floor.** Defects nobody has fixed yet are counted
  only when a reviewer found them.

### Next experiments

Each is one `run` into the same directory with a different
`--surveyor-args`, then `judge --redo`:
1. Finder on Opus 5.5 at `high`.
2. A finder prompt that asks for breadth: report every suspected defect
   and let the verifier filter.
3. A higher step cap, plus instrumentation of whether the cap was hit.
