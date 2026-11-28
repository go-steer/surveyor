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
| known bugs caught (of 25) | 14 | 10 |
| cost per PR, mean | $1.22 | $0.55 |
| time per PR, mean (max) | 219s (482s) | 92s (385s) |

Judging cost $32.56 in total ($1.09 per PR).

### What it says

- **surveyor is precise but finds too little.**
  - Nearly everything it reports is real, and it raised no false alarm in
    30 PRs.
  - It reports about 1.6 findings per PR and misses more than half of
    what Claude Code finds, including 5 known bugs Claude Code caught.
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

## Second run: surveyor variants, PR-Agent and CodeRabbit (2026-11-27)

The same 30 PRs. All nine reviewers were judged together in one blind
pass, so the labels differ slightly from the first run's: the judge
varies too. The full report is in
[`replay/2026-11-core-agent-experiments.md`](./replay/2026-11-core-agent-experiments.md).

The reviewers:
- **surveyor:** its defaults (Sonnet 5.5 finder at `high`, verifier at
  `medium`), run twice, and three variants:
  - an Opus 5.5 finder (Sonnet verifier);
  - `--finder-style broad`: report every suspected defect and let the
    verifier filter;
  - `--finder-steps 150`.
- **PR-Agent** (`/review`, Sonnet 5.5 on Vertex):
  - It ran in plain-diff mode on the same base...head diff, which cannot
    post to the PR.
  - It ran at its defaults (6 findings at most), and with 20.
  - In this mode it does not read `AGENTS.md`, so a miss that depends on
    the repo's rules is not a fair comparison.
- **The CodeRabbit CLI** (`review --agent --deep`, free open-source
  tier):
  - It ran on the 14 known-bug PRs only, because of its rate limit.
  - It ran on a local worktree against the merge base, with `AGENTS.md`
    passed in.
  - CodeRabbit says its CLI reviews differ from its hosted PR bot.

| | Claude Code | surveyor | surveyor, again | **surveyor Opus finder** | surveyor broad | surveyor 150 steps | PR-Agent | PR-Agent 20 | CodeRabbit CLI |
|---|---|---|---|---|---|---|---|---|---|
| PRs | 30 | 30 | 30 | 30 | 30 | 30 | 30 | 30 | 14 |
| real / nit / false alarm | 74 / 131 / 7 | 29 / 18 / 0 | 29 / 14 / 0 | 44 / 17 / 0 | 43 / 33 / 2 | 24 / 13 / 1 | 20 / 19 / 13 | 22 / 20 / 7 | 30 / 70 / 3 |
| findings that are real | 35% | 62% | 67% | **72%** | 55% | 63% | 38% | 45% | 29% |
| defects found | **69 of 106** | 29 | 29 | 44 | 43 | 24 | 19 | 19 | 26 of 95 |
| known bugs caught | **14 of 25** | 10 | 8 | **14** | 10 | 8 | 9 | 8 | 7 of 25 |
| cost per PR | $1.22 | $0.55 | $0.57 | $1.33 | $0.72 | $0.49 | $0.13 | $0.13 | free |
| time per PR, mean | 219s | 92s | 97s | 153s | 122s | 94s | 31s | 32s | 630s |

Judging cost $43.

### What it says

- **An Opus finder matches Claude Code on known bugs at the same cost.**
  - Known bugs: 14 of 25, the same count as Claude Code but a different
    set (10 shared, 3 only Opus, 4 only Claude Code).
  - No false alarms, and 72% of its findings are real.
  - Claude Code still finds more defects overall (69 vs 44), but 62% of
    its findings are nits.
- **Variance is the biggest lever.** Identical runs disagree:
  - surveyor and its rerun found 29 defects each but only 14 in common,
    44 together.
  - All five surveyor runs together found 81 of 106 defects and 16 of
    25 known bugs. Claude Code found 69 and 14.
  - So several independent finder passes, deduplicated before
    verification, should raise recall more than any single setting.
    Two Sonnet passes (about $1.10) found as many defects as one Opus
    pass.
- **The step cap is not the limit.** No finder hit it: Sonnet averaged
  13–15 model calls and Opus 24 (55 at most). Raising it to 150 changed
  nothing. The finder stops on its own; breadth has to come from the
  prompt or from more passes.
- **The broad prompt helps recall (43 defects) but costs precision**
  (2 false alarms). Its verifier refuted 4 findings: 2 false alarms
  correctly, and 1 real finding by mistake. It is the only variant where
  the verifier did meaningful work.
- **Coverage is now recorded.** The only incomplete surveyor reviews were
  #1445 (454KB) and #1463 (201KB), whose diffs were over the 200KB prompt
  cap.
- **PR-Agent is cheap and noisy.** At $0.13 a PR, 25% of its findings
  were false alarms (14% with the cap at 20). It still caught 9 known
  bugs.
- **The CodeRabbit CLI was weak on this set.** It caught 7 of 25 known
  bugs, and two thirds of its findings were nits, at about 10 minutes a review.
  - That contrasts with CodeRabbit's hosted review of surveyor#10: 5 of 5
    comments were valid, including a real bug in this report's known-bug
    credit (fixed) and a real security gap in the harness.
  - The hosted bot needs its own replay before it is judged.
- **Robustness bugs found by the runs:**
  - In about 3% of runs, one read-tool result overflowed the 1M-token
    context. core-agent cut the turn, and surveyor reported it as
    "interrupted" (#1448, #1557, #1581). The read tools need a per-result
    size cap.
  - On one Opus run (#1445), Vertex rejected a resent thinking block "bound
    to a different conversation". core-agent or core-models resends
    thinking blocks after the history has changed.
  - Both passed on retry.

## Third run: Gemini finders and Opus variance (2026-11-27)

The same 30 PRs. Seven reviewers were judged together in one blind pass:
Claude Code, surveyor at its defaults, two identical Opus-finder runs, and
three Gemini finders. surveyor now opens Gemini on Vertex AI, choosing the
provider by model name. Each Gemini finder ran with a Sonnet 5.5 verifier.

- **Prices used** (core-models v0.16.0 catalog, rates of 2026-11-20):
  - Gemini 4 Neon: $3 in / $15 out per million tokens.
  - Gemini 4 Hydrogen: $1.50 / $7.50.
- **When:** runs started 2026-11-27 21:56 UTC.
- **Gemini 4 Argon** was started and then dropped:
  - At this task it read millions of tokens per PR, costing $6.85–11.30
    a large PR.
  - On #1448 it hit the step cap with its findings still unreported.
- **The full report** is in
  [`replay/2026-11-core-agent-batch3.md`](./replay/2026-11-core-agent-batch3.md).

| | Claude Code | surveyor | Opus finder | Opus finder, again | **Neon `high`** | Hydrogen `high` | Hydrogen `medium` |
|---|---|---|---|---|---|---|---|
| real / nit / false alarm | 82 / 122 / 8 | 30 / 17 / 0 | 46 / 15 / 0 | 47 / 16 / 0 | 77 / 36 / 0 | 52 / 13 / 1 | 14 / 5 / 0 |
| findings that are real | 39% | 64% | 75% | 75% | 68% | 79% | 74% |
| defects found (of 127) | **77** | 30 | 46 | 46 | **76** | 51 | 13 |
| known bugs caught (of 25) | **14** | 10 | **14** | 11 | **14** | 12 | 4 |
| cost per PR | $1.22 | $0.55 | $1.33 | $1.42 | $6.39 | $2.41 | $0.48 |
| time per PR, mean | 219s | 92s | 153s | 166s | 506s | 246s | 109s |

Judging cost about $55.

### What it says

- **Neon at `high` matches Claude Code's recall without its noise.**
  - It found 76 defects to Claude Code's 77, and the same 14 known bugs
    by count, though not the same set.
  - It raised no false alarms, and 68% of its findings were real
    (Claude Code: 39%).
  - It costs $6.39 a PR, about five times Claude Code, and takes over
    8 minutes.
- **Hydrogen `high` is the value pick.** 51 defects and 12 known bugs at
  $2.41, with the highest share of real findings (79%). At `medium` it
  barely looks (13 calls on average) and finds little.
- **Opus varies run to run.** Two identical runs found 46 defects each,
  but caught 14 and 11 known bugs. Together: 58 defects, 15 known bugs.
- **Combining different models pays more than repeating one.** Unions of
  two runs' findings, which is what the engine's new `--also-model`
  passes produce:

  | finders | defects | known bugs | cost per PR |
  |---|---|---|---|
  | Opus + Hydrogen `high` | 74 | 18 | $3.74 |
  | Opus + Neon | 93 | 19 | $7.72 |
  | Opus + Opus | 58 | 15 | $2.75 |
  | Claude Code alone | 77 | 14 | $1.22 |

  Five known bugs were caught by no reviewer here: #1197, #1431's
  endpoint URL, #1463, #1468's skill-level auth, and #1512's user-role
  audit event.
- **The verifier still rarely acts.** Across these runs it refuted 2
  findings, both real. It earns its cost only where the finder is
  broad.
- **Coverage:**
  - No finder hit its step cap on these runs.
  - The only incomplete reviews were the two over-cap diffs, #1445 and
    #1463.
  - A capped finder's findings are now salvaged from its final text, so
    a cap no longer loses them.

### Next experiments

Each experiment is a surveyor variant: a reviewer named `surveyor-<name>`
with its own `--surveyor-args`, so its results sit beside the others
instead of being skipped as finished. Run it in a copy of the replay
directory, then judge every reviewer together:

```sh
bin/replay run   --dir $D --reviewers surveyor-opus --surveyor-args "--model claude-opus-5-5 --verify-model claude-sonnet-5-5"
bin/replay judge --dir $D --reviewers surveyor,claude-code,surveyor-opus
```

1. **Several finder passes** with the engine's `--also-model`, measured
   end to end: Opus with Hydrogen `high`, deduplicated, then verified.
2. **The broad prompt with an Opus finder**, where the verifier has
   something to filter.
3. **CodeRabbit's hosted bot**, replayed through PRs on a scratch repo.
4. **Each configuration run several times**, to put error bars on
   these numbers.
