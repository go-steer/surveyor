# Replay: 30 pull requests

Every finding was judged blind (reviewer hidden, order shuffled) by claude-opus-5-5 reading the code at the PR's head. A **defect** is a group of findings the judge labelled real and about the same problem. Known bugs are defects a later PR fixed. Judge cost: $41.66.

| | claude-code | coderabbit | pr-agent | pr-agent-20 | surveyor | surveyor-broad | surveyor-opus | surveyor-rerun | surveyor-steps150 |
|---|---|---|---|---|---|---|---|---|---|
| model | claude-opus-5-5 | coderabbit-cli | vertex_ai/claude-sonnet-5-5 | vertex_ai/claude-sonnet-5-5 | claude-sonnet-5-5 | claude-sonnet-5-5 | claude-opus-5-5 | claude-sonnet-5-5 | claude-sonnet-5-5 |
| settings | defaults | defaults | defaults | `PR_REVIEWER__NUM_MAX_FINDINGS=20` | defaults | `--project go-steer-drydock --region global --finder-style broad` | `--project go-steer-drydock --region global --model claude-opus-5-5 --verify-model claude-sonnet-5-5` | `--project go-steer-drydock --region global` | `--project go-steer-drydock --region global --finder-steps 150` |
| PRs reviewed | 30 | 14 | 30 | 30 | 30 | 30 | 30 | 30 | 30 |
| findings reported | 212 | 103 | 52 | 49 | 47 | 78 | 61 | 43 | 38 |
| real / nit / false alarm | 74 / 131 / 7 | 30 / 70 / 3 | 20 / 19 / 13 | 22 / 20 / 7 | 29 / 18 / 0 | 43 / 33 / 2 | 44 / 17 / 0 | 29 / 14 / 0 | 24 / 13 / 1 |
| share of findings that are real | 35% | 29% | 38% | 45% | 62% | 55% | 72% | 67% | 63% |
| false alarms | 3% | 3% | 25% | 14% | 0% | 3% | 0% | 0% | 3% |
| defects found (of those found by anyone in the PRs it reviewed) | 69 of 106 (65%) | 26 of 95 (27%) | 19 of 106 (18%) | 19 of 106 (18%) | 29 of 106 (27%) | 43 of 106 (41%) | 44 of 106 (42%) | 29 of 106 (27%) | 24 of 106 (23%) |
| defects only this reviewer found | 12 | 6 | 1 | 1 | 1 | 3 | 4 | 2 | 2 |
| known bugs caught | 14 of 25 | 7 of 25 | 9 of 25 | 8 of 25 | 10 of 25 | 10 of 25 | 14 of 25 | 8 of 25 | 8 of 25 |
| cost, total (mean per PR) | $36.63 ($1.22) | $0.00 ($0.00) | $3.88 ($0.13) | $3.92 ($0.13) | $16.48 ($0.55) | $21.47 ($0.72) | $40.03 ($1.33) | $17.04 ($0.57) | $14.70 ($0.49) |
| time, mean (max) | 219s (482s) | 630s (1526s) | 31s (72s) | 32s (83s) | 92s (385s) | 122s (273s) | 153s (385s) | 97s (371s) | 94s (323s) |
| incomplete reviews (cap hit or diff truncated) | not recorded | 0 of 14 (finder: 0 calls mean) | 0 of 30 (finder: 0 calls mean) | 0 of 30 (finder: 0 calls mean) | not recorded | 2 of 30 (finder: 15 calls mean) | 2 of 30 (finder: 24 calls mean) | 2 of 30 (finder: 14 calls mean) | 2 of 30 (finder: 13 calls mean) |

surveyor's verifier refuted 0 findings: 0 false alarms and 0 nits correctly dropped, 0 real findings lost.
surveyor-broad's verifier refuted 4 findings: 2 false alarms and 1 nits correctly dropped, 1 real findings lost.
surveyor-opus's verifier refuted 1 findings: 0 false alarms and 0 nits correctly dropped, 1 real findings lost.
surveyor-rerun's verifier refuted 0 findings: 0 false alarms and 0 nits correctly dropped, 0 real findings lost.
surveyor-steps150's verifier refuted 2 findings: 0 false alarms and 2 nits correctly dropped, 0 real findings lost.

## Known bugs

| PR | fixed in | bug | claude-code | coderabbit | pr-agent | pr-agent-20 | surveyor | surveyor-broad | surveyor-opus | surveyor-rerun | surveyor-steps150 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| #1197 | #1233 | The auto-mode approver's task must contain only words the operator wrote. #1197 cut core-tui's @-inlined file content from the local TUI's prompt, but on the... | missed | missed | missed | missed | missed | missed | missed | missed | missed |
| #1429 | #1452 | On any AppendEvent error the drain re-queues pending[i:] at the front of the queue and stops, with no attempt cap and no distinction between transient and de... | caught | missed | caught | caught | caught | caught | caught | caught | caught |
| #1429 | #1480 | RestoreGuardrails folds the pre-upgrade agent_eventlog guardrail history only when no agent_guardrail_state row exists, but RecordGuardrailReset does not res... | caught | caught | caught | caught | missed | caught | caught | caught | caught |
| #1430 | #1481 | The resume rebuild prices each root-session turn under resolveRebuildModel, which prefers the provider's served ev.ModelVersion. The compose pricing callback... | caught | caught | missed | missed | missed | found, then refuted | caught | caught | missed |
| #1431 | #1443 | endpointURL takes the JSON-RPC endpoint verbatim from the fetched agent card (card.url or additionalInterfaces[].url) with no scheme/host validation, while v... | missed | caught | caught | caught | missed | caught | missed | missed | missed |
| #1431 | #1457 | The message/stream SSE parser treats each 'data:' line as a complete event and JSON-decodes it immediately, instead of accumulating consecutive data: lines a... | caught | caught | missed | missed | missed | missed | missed | missed | missed |
| #1431 | #1457 | Streaming artifact-update events are appended as separate artifacts regardless of the event's append flag / artifactId, and the final result joins all artifa... | caught | caught | caught | caught | caught | caught | caught | missed | caught |
| #1431 | #1479 | #1431 added a status guard to the shared claimSync primitive, which the in-process awaitResult also uses. Between a handle's status turning terminal and take... | caught | missed | missed | missed | caught | missed | caught | caught | missed |
| #1441 | #1449 | The new unknown-key walker matches JSON keys against struct json tags case-sensitively, but encoding/json matches field names case-insensitively. A mixed-cas... | caught | caught | caught | caught | caught | missed | caught | caught | caught |
| #1445 | #1477 | On an operator verdict of "edit", beforeTool persists the decision and then calls applyArgs(args, edited), which writes the edited keys into the caller's arg... | missed | missed | missed | missed | missed | missed | missed | missed | missed |
| #1448 | #1460 | The new capability: read_only check calls tools.IsReadOnlyTool on built-in tools after cmd/core-agent has wrapped them with the MCP digester. digestingTool.R... | missed | missed | missed | missed | missed | missed | missed | missed | missed |
| #1448 | #1460 | Markdown subagent authoring (.agents/subagents/*.md) and the baseDir stamping used for output_schema resolution are wired only into config.Load. The -c/--con... | missed | missed | missed | missed | missed | missed | caught | missed | missed |
| #1451 | #1476 | RunOneShot/compose.Run select a subagent's MCP toolsets with `ts.Name() == name \|\| strings.HasPrefix(ts.Name(), name+"_")`, ignore spec.MCPTools entirely (... | caught | missed | missed | missed | caught | caught | caught | caught | missed |
| #1454 | #1470 | When converting tool_catalog entries into permissions.allow rules, any non-mutating tool whose name contains a hyphen is classified as a subagent and emitted... | caught | missed | missed | missed | missed | missed | missed | missed | caught |
| #1454 | #1501 | The workload importer maps mast's per-turn/per-session call budgets (budget.max_turns, budget.max_model_calls_per_turn) onto cfg.Agent.MaxSteps, but nothing ... | caught | missed | missed | missed | caught | caught | caught | missed | caught |
| #1463 | #1474 | persist writes the new fire count to in-memory t.fires before SaveSchedule succeeds, yet recomputes the count from the stored prev.Fires on the next call. Wh... | missed | missed | missed | missed | missed | missed | missed | missed | missed |
| #1465 | #1473 | Notifier.sendTimeline falls back to Replace/Post only when Client.Append returns ErrSendFullText or ErrNoSuchMessage, but Append is a POST and doAttempt maps... | missed | missed | caught | caught | caught | missed | caught | missed | missed |
| #1465 | #1473 | loadAckedSubjects returns every subject that has any AckRecord, ignoring AckRecord.Forwarded and AckRecord.At, and filterTransitions drops every transition f... | caught | missed | caught | caught | caught | caught | caught | caught | caught |
| #1465 | #1473 | The scheduled-turn waiter has no turn correlation. runScheduledTurn sets w.out, calls a.InjectAs (which only queues the prompt), and accepts the result of wh... | caught | caught | caught | missed | missed | caught | caught | caught | missed |
| #1468 | #1472 | The A2A server's turn driver iterates the agent's event sequence directly without a usage.TurnTap and never calls ag.Tracker().AppendUsage, unlike the other ... | missed | missed | missed | missed | missed | missed | missed | missed | missed |
| #1468 | #1472 | tasks/get and tasks/cancel authorize only against the top-level a2a.auth default scopes, ignoring the skill-level Auth.Required/Auth.Scopes that message/send... | missed | missed | missed | missed | missed | missed | missed | missed | missed |
| #1468 | #1472 | RunContent passes an empty prompt plus a prebuilt message into runTurn, which still runs the pre-turn steps that drain pending background alerts, the inject ... | caught | missed | caught | caught | caught | caught | caught | missed | caught |
| #1512 | #1572 | VCSEventSink appends the vcs.published / vcs.proposed audit event to the session inline, from inside the vcs_publish / vcs_propose tool handler, while ADK's ... | caught | missed | missed | missed | caught | missed | missed | missed | missed |
| #1512 | #1572 | The VCS audit event is written with user-role Content (Author "system") instead of as a content-less audit row with the payload only in CustomMetadata (as me... | missed | missed | missed | missed | missed | missed | missed | missed | missed |
| #1614 | #1616 | #1614 added ModelConfig.ThinkingLevel but did not update writeSubagentMarkdownFile, which writes a subagent's model block as a bare `model: <name>` when ever... | missed | missed | missed | missed | missed | caught | caught | missed | missed |

## Per pull request

| PR | title | claude-code | coderabbit | pr-agent | pr-agent-20 | surveyor | surveyor-broad | surveyor-opus | surveyor-rerun | surveyor-steps150 |
|---|---|---|---|---|---|---|---|---|---|---|
| #1197 | feat(agent): stamp the auto-mode approver's context on every turn (#1175) | 0 real of 9, $2.90, 482s | 0 real of 2, $0.00, 416s | 0 real of 2, $0.18, 43s | 0 real of 3, $0.18, 41s | 0 real of 1, $0.60, 109s | 1 real of 2, $0.75, 124s | 0 real of 1, $1.99, 193s | 0 real of 1, $0.40, 73s | 0 real of 1, $0.52, 93s |
| #1429 | fix(guardrails): bound out-of-band drain reads, re-queue errors, and add GuardrailStore | 4 real of 8, $0.99, 210s | 1 real of 3, $0.00, 333s | 2 real of 2, $0.10, 32s | 2 real of 2, $0.10, 36s | 1 real of 1, $0.50, 103s | 2 real of 3, $0.56, 117s | 2 real of 3, $1.23, 184s | 2 real of 2, $0.60, 131s | 2 real of 2, $0.49, 96s |
| #1430 | fix(usage,eventlog,compose): rebuild session-tree spend with per-model pricing and add SpendStore (#1359) | 3 real of 10, $1.42, 254s | 1 real of 4, $0.00, 1526s | 0 real of 0, $0.11, 32s | 0 real of 1, $0.10, 25s | 0 real of 1, $0.41, 96s | 1 real of 1, $0.65, 140s | 1 real of 2, $1.74, 194s | 1 real of 1, $0.64, 125s | 1 real of 1, $0.46, 98s |
| #1431 | feat(background): ship A2A v0.3 RemoteAgentSpawner and subagents[].remote routing | 6 real of 10, $1.43, 249s | 7 real of 17, $0.00, 668s | 3 real of 4, $0.19, 44s | 3 real of 4, $0.18, 39s | 3 real of 4, $0.77, 106s | 4 real of 7, $1.25, 192s | 3 real of 4, $2.11, 200s | 2 real of 3, $0.70, 114s | 3 real of 4, $0.88, 125s |
| #1441 | fix(config): walk unknown nested keys recursively and fail on safety/cost typos | 3 real of 10, $0.79, 227s | 1 real of 2, $0.00, 377s | 2 real of 2, $0.06, 30s | 2 real of 2, $0.06, 29s | 3 real of 4, $0.60, 116s | 2 real of 2, $0.38, 95s | 3 real of 3, $0.76, 124s | 2 real of 3, $0.64, 110s | 1 real of 1, $0.43, 87s |
| #1445 | feat(approval): port durable write gate, effect outbox, and /parks endpoints (#1400) | 7 real of 10, $1.89, 287s | 3 real of 16, $0.00, 546s | 2 real of 2, $0.57, 61s | 4 real of 4, $0.57, 64s | 2 real of 2, $0.88, 138s | 4 real of 7, $1.95, 244s | 3 real of 3, $2.31, 219s | 1 real of 1, $1.42, 174s | 2 real of 2, $0.95, 144s |
| #1448 | feat(subagents): add mode, output_schema, tier, capability, mcp_tools, and Markdown authoring (#1401) | 2 real of 10, $2.70, 412s | 4 real of 10, $0.00, 537s | 0 real of 2, $0.31, 72s | 0 real of 2, $0.32, 83s | 1 real of 2, $0.62, 85s | 2 real of 4, $1.22, 175s | 2 real of 3, $3.93, 357s | 2 real of 6, $1.24, 161s | 1 real of 2, $0.71, 119s |
| #1451 | feat(agent,compose): add one-call RunOneShot / RunRecipe / RunWorkload facade (#1404) | 5 real of 9, $1.93, 342s | 0 real of 0, $0.00, 390s | 0 real of 2, $0.19, 58s | 0 real of 1, $0.16, 42s | 2 real of 2, $1.04, 183s | 4 real of 4, $1.08, 273s | 2 real of 3, $2.82, 385s | 2 real of 2, $1.11, 371s | 1 real of 1, $1.24, 323s |
| #1454 | feat(config): add workload.yaml converter and import-workload CLI (#1402) | 5 real of 10, $1.54, 249s | 0 real of 3, $0.00, 745s | 1 real of 3, $0.17, 48s | 1 real of 3, $0.16, 43s | 1 real of 3, $0.68, 133s | 4 real of 7, $1.15, 173s | 3 real of 4, $1.70, 172s | 2 real of 3, $0.86, 119s | 2 real of 2, $0.47, 111s |
| #1463 | feat(compose,eventlog): port scheduled trigger, fleet lease, auto-resume, and pause rescan (#1405) | 7 real of 10, $1.91, 314s | 3 real of 15, $0.00, 574s | 2 real of 2, $0.32, 65s | 2 real of 2, $0.32, 68s | 2 real of 3, $0.91, 385s | 4 real of 5, $1.05, 177s | 3 real of 6, $2.46, 292s | 2 real of 2, $1.17, 172s | 2 real of 2, $0.77, 261s |
| #1465 | feat(compose,attach): port deterministic monitoring cycle, notify, and /monitor-ack (#1406) | 9 real of 10, $1.28, 220s | 2 real of 11, $0.00, 646s | 3 real of 4, $0.27, 49s | 2 real of 3, $0.27, 48s | 3 real of 4, $0.70, 72s | 4 real of 6, $1.39, 120s | 6 real of 6, $2.07, 192s | 4 real of 4, $0.61, 67s | 3 real of 4, $0.69, 74s |
| #1468 | feat(attach,compose,config): mount A2A v0.3 server on attach mux (#1407) | 7 real of 10, $1.66, 282s | 7 real of 15, $0.00, 1459s | 3 real of 5, $0.30, 71s | 3 real of 5, $0.28, 56s | 4 real of 5, $1.02, 134s | 5 real of 5, $1.33, 207s | 5 real of 5, $2.14, 206s | 4 real of 4, $1.14, 174s | 3 real of 3, $0.55, 102s |
| #1512 | feat: add forge-neutral vcs_* tool suite and config.vcs (#1490) | 6 real of 10, $2.25, 298s | 1 real of 5, $0.00, 397s | 1 real of 2, $0.25, 41s | 1 real of 3, $0.27, 50s | 3 real of 3, $1.10, 127s | 1 real of 5, $2.12, 242s | 4 real of 6, $2.31, 185s | 2 real of 3, $1.20, 153s | 1 real of 4, $1.88, 237s |
| #1557 | feat(models): bump core-models to v0.15.3 and classify Gemini 4 Carbon/Hydrogen | 0 real of 2, $0.55, 102s | 0 real of 0 | 0 real of 0, $0.03, 11s | 0 real of 0, $0.03, 11s | 0 real of 0, $0.26, 32s | 0 real of 1, $0.30, 53s | 0 real of 1, $0.49, 70s | 0 real of 0, $0.19, 32s | 0 real of 0, $0.28, 34s |
| #1570 | docs(models): point the ANTHROPIC_BASE_URL comment at the archived scenarios doc | 0 real of 3, $0.18, 34s | 0 real of 0 | 0 real of 0, $0.01, 7s | 0 real of 0, $0.01, 8s | 0 real of 0, $0.04, 11s | 0 real of 0, $0.05, 12s | 0 real of 0, $0.07, 11s | 0 real of 0, $0.04, 11s | 0 real of 0, $0.04, 10s |
| #1572 | fix(vcs): defer VCSEventSink audit writes past in-flight turns (#1571) | 0 real of 5, $1.17, 240s | 0 real of 0 | 0 real of 2, $0.12, 38s | 0 real of 0, $0.13, 43s | 0 real of 0, $0.41, 102s | 0 real of 1, $0.96, 196s | 0 real of 0, $1.56, 241s | 0 real of 0, $0.49, 113s | 0 real of 0, $0.31, 75s |
| #1576 | feat(webui): pin mast-web v0.7.0 and wire fetch-mast-web into release builds (#348) | 0 real of 6, $0.40, 99s | 0 real of 0 | 0 real of 2, $0.02, 10s | 0 real of 2, $0.02, 11s | 0 real of 0, $0.11, 23s | 0 real of 1, $0.19, 45s | 0 real of 0, $0.57, 46s | 0 real of 0, $0.32, 43s | 0 real of 0, $0.15, 30s |
| #1577 | fix(examples): document secrets exclusion, disable web_search, and add sufficiency rules in GKE recipes (#695, #696) | 1 real of 8, $0.66, 150s | 0 real of 0 | 0 real of 3, $0.04, 13s | 1 real of 3, $0.06, 26s | 1 real of 2, $0.54, 62s | 0 real of 3, $0.98, 204s | 0 real of 1, $0.70, 105s | 0 real of 1, $0.49, 65s | 0 real of 2, $0.40, 55s |
| #1578 | fix(transcript): record assistant reply in headless one-shot and report save errors (#1553) | 1 real of 7, $0.91, 182s | 0 real of 0 | 0 real of 0, $0.04, 17s | 0 real of 2, $0.05, 19s | 0 real of 1, $0.46, 66s | 0 real of 1, $0.27, 58s | 1 real of 1, $0.79, 121s | 0 real of 0, $0.61, 80s | 0 real of 2, $0.47, 99s |
| #1579 | fix(permissions): reject .. traversal and escaping symlinks on path-prefix rules (#1274) | 5 real of 8, $0.80, 204s | 0 real of 0 | 0 real of 4, $0.04, 16s | 0 real of 1, $0.07, 44s | 1 real of 2, $0.30, 83s | 2 real of 3, $0.33, 87s | 2 real of 3, $1.05, 185s | 1 real of 2, $0.29, 80s | 1 real of 2, $0.39, 119s |
| #1580 | fix(test): fix synchronous subagent alert race and CI test flakes (#1551) | 1 real of 8, $0.75, 156s | 0 real of 0 | 1 real of 3, $0.03, 12s | 1 real of 3, $0.03, 12s | 0 real of 2, $0.40, 77s | 0 real of 3, $0.35, 65s | 1 real of 2, $0.75, 122s | 1 real of 2, $0.42, 85s | 0 real of 1, $0.26, 65s |
| #1581 | feat(vcs,models): bump core-vcs to v1.2.0 and core-models to v0.15.4 (#1490) | 1 real of 7, $0.65, 144s | 0 real of 0 | 0 real of 3, $0.02, 10s | 0 real of 3, $0.02, 11s | 1 real of 2, $0.71, 63s | 1 real of 1, $0.21, 48s | 1 real of 1, $0.50, 82s | 0 real of 1, $0.19, 50s | 0 real of 0, $0.23, 58s |
| #1592 | chore: remove three functions unreachable from any main or test | 0 real of 1, $0.25, 46s | 0 real of 0 | 0 real of 0, $0.02, 10s | 0 real of 0, $0.01, 9s | 0 real of 0, $0.09, 14s | 0 real of 0, $0.05, 15s | 0 real of 0, $0.17, 18s | 0 real of 0, $0.08, 12s | 0 real of 0, $0.08, 11s |
| #1603 | fix(modelretry): classify Claude errors by HTTPStatus, not the SDK type | 0 real of 3, $0.49, 102s | 0 real of 0 | 0 real of 0, $0.03, 12s | 0 real of 0, $0.03, 14s | 0 real of 0, $0.09, 20s | 0 real of 1, $0.44, 85s | 0 real of 0, $0.28, 38s | 0 real of 0, $0.08, 18s | 0 real of 0, $0.09, 20s |
| #1609 | fix(agent): gate Gemini parallelism quirk on enabled tools | 0 real of 4, $1.92, 373s | 0 real of 0 | 0 real of 0, $0.11, 33s | 0 real of 0, $0.12, 37s | 0 real of 0, $0.52, 107s | 0 real of 1, $0.70, 176s | 0 real of 0, $1.56, 214s | 0 real of 0, $0.50, 126s | 0 real of 0, $0.52, 111s |
| #1610 | fix(subagents): keep router descriptions out of child ADK identity prompts | 0 real of 3, $0.96, 190s | 0 real of 0 | 0 real of 3, $0.06, 11s | 0 real of 0, $0.07, 15s | 0 real of 0, $1.24, 65s | 0 real of 1, $0.42, 75s | 0 real of 0, $0.51, 57s | 0 real of 0, $0.34, 59s | 0 real of 0, $0.23, 40s |
| #1611 | fix(subagents): skip digest wrap when subagent lacks retrieve_raw | 1 real of 10, $1.41, 246s | 0 real of 0 | 0 real of 0, $0.12, 27s | 0 real of 0, $0.12, 26s | 0 real of 2, $0.60, 116s | 1 real of 2, $0.58, 107s | 0 real of 1, $1.03, 127s | 0 real of 1, $0.40, 72s | 1 real of 2, $0.61, 113s |
| #1612 | fix(mcp): omit MCP elicitation in headless and non-TUI runs (#1606) | 0 real of 4, $1.00, 160s | 0 real of 0 | 0 real of 0, $0.09, 20s | 0 real of 0, $0.09, 17s | 0 real of 0, $0.27, 37s | 0 real of 0, $0.27, 53s | 0 real of 0, $0.63, 68s | 0 real of 0, $0.37, 46s | 0 real of 0, $0.19, 49s |
| #1614 | feat(config): model.thinking_level, one thinking setting across providers | 0 real of 5, $1.38, 232s | 0 real of 0, $0.00, 210s | 0 real of 0, $0.08, 20s | 0 real of 0, $0.09, 21s | 1 real of 1, $0.57, 78s | 1 real of 1, $0.45, 71s | 2 real of 2, $1.65, 169s | 1 real of 1, $0.42, 61s | 0 real of 0, $0.34, 55s |
| #1616 | fix(config): keep a subagent's thinking_level in the Markdown write | 0 real of 2, $0.44, 82s | 0 real of 0 | 0 real of 0, $0.01, 7s | 0 real of 0, $0.01, 7s | 0 real of 0, $0.05, 15s | 0 real of 0, $0.07, 14s | 0 real of 0, $0.15, 20s | 0 real of 0, $0.07, 18s | 0 real of 0, $0.06, 16s |

## Defects

| PR | where | defect | claude-code | coderabbit | pr-agent | pr-agent-20 | surveyor | surveyor-broad | surveyor-opus | surveyor-rerun | surveyor-steps150 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| #1197 | `pkg/attachadapter/capabilities.go:252` | AttachUsage builds PerTurn from tracker.All(), which now contains approver side calls, so remote consumers read the approver's call as the conversation's las... |  |  |  |  |  | yes |  |  |  |
| #1429 | `pkg/agent/guardrail_persist.go:136` | Any AppendEvent error re-queues the failed event at the head of the queue, with no distinction between a retryable failure and a permanent one, so one bad ev... (known bug) | yes |  | yes | yes | yes | yes | yes | yes | yes |
| #1429 | `pkg/eventlog/sql.go:167` | The published docs still describe the old database layout. `Open` now creates a new table, `agent_guardrail_state`, and exposes the new public API `Handle.Gu... | yes |  |  |  |  |  |  |  |  |
| #1429 | `pkg/eventlog/guardrail.go:320` | The legacy-history check runs only when no agent_guardrail_state row exists. On an upgraded database, any guardrail event written before the first successful... (known bug) | yes | yes | yes | yes |  | yes | yes | yes | yes |
| #1430 | `pkg/compose/multi_session.go:620` | Keep configured pricing for root conversation events. (known bug) | yes | yes |  |  |  | refuted | yes | yes |  |
| #1430 | `pkg/compose/multi_session.go:605` | Resuming a session that was written before this upgrade double-bills or misprices its subagent spend. The resume path now scans the whole `WithSessionTree`, ... | yes |  |  |  |  | yes |  |  | yes |
| #1430 | `pkg/usage/rebuild.go:139` | A resume can bill spend that the live tracker never recorded, so the restored total jumps above the pre-eviction figure. Background subagents are built with ... | yes |  |  |  |  |  |  |  |  |
| #1431 | `pkg/agent/background/a2a_client.go
:172` | Bearer token sent to card-advertised URL: When only `a2a` is configured, the JSON-RPC endpoint is taken from the fetched card's `url` or `additionalInterface... (known bug) |  | yes | yes | yes |  | yes |  |  |  |
| #1431 | `pkg/agent/background/remote.go:410` | fanInRemote is shared with the existing spawn_remote_agent path, and a handle whose Events channel closes without a terminal event now counts as failed inste... | yes |  |  |  | yes | yes | yes | yes |  |
| #1431 | `pkg/agent/background/templates.go:386` | launchRemote calls m.runningCount() after m.mu.Unlock() while building the ErrTooManyConcurrent message, although runningCount documents "Caller holds m.mu". |  |  |  |  |  | yes |  |  |  |
| #1431 | `pkg/agent/background/a2a_client.go:779` | For a remote whose card does not advertise streaming, stop_agent and timeouts never send tasks/cancel, and no progress alerts arrive. runPoll (and Send at li... | yes | yes |  |  |  |  |  |  | yes |
| #1431 | `pkg/agent/background/a2a_client.go:696` | artifact-update frames are appended as separate artifacts, so the `append` and `lastChunk` flags and the artifactId are ignored. (known bug) | yes | yes | yes | yes | yes | yes | yes |  | yes |
| #1431 | `pkg/agent/background/a2a_client.go:634` | SSE frames whose data spans several lines are parsed one line at a time. The scanner treats each 'data:' line as a complete JSON frame, but SSE says consecut... (known bug) | yes | yes |  |  |  |  |  |  |  |
| #1431 | `pkg/agent/background/a2a_client.go:428` | Stop on an already-finished remote handle still sends a blocking tasks/cancel (up to 5s) for the completed task. Manager.Close invokes bh.cancel for every ha... |  |  |  |  |  |  |  |  | yes |
| #1431 | `pkg/agent/background/manager.go:241` | The new status check in claimSync applies to every subagent, not just remote ones, and it makes double delivery more likely for in-process subagents. spawn.g... (known bug) | yes |  |  |  | yes |  | yes | yes |  |
| #1431 | `pkg/agent/background/templates.go:419` | Daemon shutdown can stall for several seconds per remote subagent. Manager.Close calls each handle's cancel one after another. For remote handles that cancel... | yes |  |  |  |  |  |  |  |  |
| #1441 | `pkg/config/discovery.go:205` | Schema lookup is an exact-case map hit, but encoding/json matches keys case-insensitively, so keys that Unmarshal applies are reported as unknown and can now... (known bug) | yes | yes | yes | yes | yes |  | yes | yes | yes |
| #1441 | `docs/site/src/content/docs/reference/configuration.md:191` | The published configuration reference is not updated. It still says an unknown key such as `permisions` is preserved and only logs a warning. After this chan... |  |  |  |  | yes | yes | yes |  |  |
| #1441 | `pkg/config/discovery.go:100` | Load now refuses any config that contains an unknown key under `permissions.*`, or any unknown key whose name contains cost/usd/budget/mtok/rate_limit. Older... | yes |  | yes | yes | yes | yes | yes | yes |  |
| #1445 | `pkg/attach/parks.go:187` | This PR adds user-visible attach endpoints (GET /parks, GET /parks/{sid}, GET /sessions/{sid}/parks, POST /ack-effects, POST /sessions/{sid}/ack-effects), bu... |  |  |  |  |  |  | yes |  |  |
| #1445 | `pkg/eventlog/decisions.go
:725` | Park Key Always Empty: `enrichLongRunningPark` reads the park's `key` from `d.Request["subject"]`, but the write gate's `Request` payload (`pkg/approval/plug... | yes |  | yes |  | yes | yes |  |  |  |
| #1445 | `pkg/eventlog/decisions.go:151` | agent_approval_decisions has no app_name/user_id columns, so ListDecisions, SessionDecisions and ExportDecisions return another tenant's decisions when two (... |  | yes |  |  |  | yes | yes |  |  |
| #1445 | `pkg/approval/plugin.go:422` | `honorVerdict` assumes `Config.Gate` is set, but Gate is only required under `require_approval`. Under `apply` or `dry_run` with a nil Gate, any call that ar... | yes |  |  |  | yes |  |  |  |  |
| #1445 | `pkg/approval/plugin.go
:234` | Nil Pointer Panic: `CheckMutatingToolCall` returns `nil` when the gate's `MutatingMode` is `apply`. `Gate.SetMutatingMode` is public and independent of `Conf... | yes |  |  | yes |  |  | yes | yes | yes |
| #1445 | `:0` | Authorization bypass: a change-set grant meant to authorize exactly one call can be consumed by two concurrent identical calls in the same model response, be... |  |  |  | yes |  |  |  |  |  |
| #1445 | `pkg/approval/plugin.go:243` | Use exact call identity for refusal suppression. |  | yes |  |  |  |  |  |  |  |
| #1445 | `pkg/approval/plugin.go:215` | Under `OnMutationApply` the gate never consults the permission policy, which contradicts the documented contract that "Policy still applies — a configured ... | yes |  |  |  |  | yes |  |  |  |
| #1445 | `internal/effects/effects.go:316` | The recorded-effect outbox is announced as a feature, but nothing can turn it on. It lives under `internal/`, so library consumers cannot import `effects.New... | yes |  |  |  |  |  |  |  |  |
| #1445 | `pkg/approval/plugin.go:426` | An approved change set can authorize its sibling calls even though the call that was actually approved never ran. `commitGrants` writes the sibling grants in... | yes |  |  |  |  |  |  |  |  |
| #1445 | `pkg/attach/parks.go:304` | executeAckEffects falls back to the unverified `Caller.Identity` when `verifiedApprover` returns "", so an anonymous request is recorded as the placeholder i... |  |  |  |  |  | yes |  |  | yes |
| #1445 | `pkg/eventlog/decisions.go
:74` | MySQL Index Migration: `FunctionCallID` is a plain `string` that appears only in a `uniqueIndex` tag, with no `size`. GORM's MySQL dialector gives unsized in... |  |  | yes |  |  |  |  |  |  |
| #1445 | `pkg/eventlog/decisions.go:396` | Keep each session's owner attached to its export entry. |  | yes |  |  |  |  |  |  |  |
| #1445 | `pkg/eventlog/decisions.go:505` | On a SQL-backed session, POST /ack-effects can make a running turn fail. `AckEffects` appends an event to the live session from outside the turn, through the... | yes |  |  |  |  |  |  |  |  |
| #1448 | `pkg/agent/modes.go:576` | WithCallGate's doc says every agent built with New consults the CallGate before each model call. buildInnerLLMAgent only installs RefuseOnGate in task and si... | yes | yes |  |  |  | yes |  |  |  |
| #1448 | `pkg/config/subagents_markdown.go:85` | The mapping form of `model:` in a Markdown subagent is decoded into `ModelConfig` with yaml.v3 under `KnownFields(true)`, but `ModelConfig` and its nested ty... | yes | yes |  |  | yes | yes | yes | yes |  |
| #1448 | `pkg/agent/modes.go:443` | Keep a gate refusal out of structured reply validation. |  | yes |  |  |  |  |  |  |  |
| #1448 | `pkg/agent/subagent.go:662` | Let `finishtask` complete on the last allowed model turn. |  | yes |  |  |  |  |  |  |  |
| #1448 | `cmd/core-agent/subagents_test.go:1262` | The PR body names the tests that cover its fixes, but those tests are not in the tree, and several new behaviours are untested. |  |  |  |  |  |  |  | yes | yes |
| #1448 | `cmd/core-agent/main.go:2885` | Markdown subagents (`.agents/subagents/*.md`) are only discovered inside `config.Load`. The `-c <config.json>` path in `loadConfig` never calls `Load`, so th... (known bug) |  |  |  |  |  |  | yes |  |  |
| #1451 | `pkg/agent/run_facade.go:854` | The skill toolset is detected with `ts.Name() == "skill_toolset"`, but the real toolset from `skills.LoadAll` (built with `skilltoolset.New` and no Name, the... | yes |  |  |  |  |  | yes | yes | yes |
| #1451 | `pkg/agent/run_facade.go:880` | Subagent `skills: [names]` only toggles the whole skill toolset on or off. The facade never narrows it to the named skills, and it matches MCP servers by a l... (known bug) | yes |  |  |  | yes | yes |  |  |  |
| #1451 | `pkg/agent/run_facade.go:988` | `safety.watchdog` from the recipe config is merged but never wired into the root or subagent agents, so the runaway-loop backstop is silently absent. | yes |  |  |  |  | yes |  |  |  |
| #1451 | `pkg/tools/gate.go:91` | GateTool wraps every tool in `gatedTool`, which only works for function tools; ADK built-in server-side tools passed in `Tools` stop working. |  |  |  |  | yes |  |  |  |  |
| #1451 | `pkg/agent/run_facade.go:863` | The facade's subagent builder ignores `spec.MCPTools` (`mcp_tools`). A subagent that narrows itself to a few tools on one server instead inherits every tool ... (known bug) | yes |  |  |  |  | yes | yes | yes |  |
| #1451 | `pkg/agent/run_facade.go:551` | A recipe with `permissions.mode: auto` gets a gate with no approver, because the facade never wires one. |  |  |  |  |  | yes |  |  |  |
| #1454 | `pkg/config/workload.go:595` | The import drops parts of the workload without saying so. `LoadWorkload` parses and validates `hitl.change_set_ttl`, `edge_trigger.scheduled`, `monitor`, `a2... | yes |  |  | yes |  |  |  |  |  |
| #1454 | `pkg/config/workload.go:554` | The capability-split check matches specialist tool names against catalog names by exact string, but the allow-pattern builder explicitly supports server-pref... | yes |  |  |  |  | yes |  |  |  |
| #1454 | `pkg/config/workload.go:193` | `coordinator.md` is never read, and several parsed workload.yaml blocks are validated but dropped without a warning. The import is silently lossy despite the... |  |  | yes |  |  |  | refuted | yes |  |
| #1454 | `pkg/config/workload.go:721` | Two allowlist heuristics can grant the wrong thing. Any read-only catalog tool with a hyphen in its name becomes `spawn_agent:<name>`, even when it's an MCP ... (known bug) | yes |  |  |  |  |  |  |  | yes |
| #1454 | `pkg/config/workload.go:693` | A workload with `hitl.on_mutation: apply` imports as a recipe that can never apply a change. `mapWorkloadHITLMode` maps it to `permissions.mode: "allow"`, bu... | yes |  |  |  |  | yes | yes |  |  |
| #1454 | `examples/gke-platform-agent/gated-apply/.agents/subagents/change-executor.md:1` | The recipe is deployed with `-c .../config.d1.json`, and `-c` loading (cmd/core-agent loadConfig) never scans `subagents/*.md`, so change-executor is never r... |  |  |  |  |  | yes |  | yes |  |
| #1454 | `pkg/config/workload.go:650` | The converter emits `agent.max_wallclock_seconds`, `agent.final_report` and `tools.policies`, but nothing in the repo reads them, so the imported recipe sile... (known bug) | yes |  |  |  | yes | yes | yes |  | yes |
| #1454 | `pkg/config/workload.go:573` | The importer treats a specialist `tools.mcp: [{server: X}]` entry with no tool list as "all tools on X". It copies that entry straight into `mcp_tools`, but ... |  |  |  |  |  |  | yes |  |  |
| #1463 | `pkg/compose/pausesched.go:119` | Apply earlier durable deadlines during rescans. |  | yes |  | yes | yes | yes |  | yes | yes |
| #1463 | `cmd/core-agent/schedule.go:84` | A replica that loses the lease race at boot stays passive for its whole life and never tries to take the lease again. When the holder exits, no replica drive... | yes |  |  |  |  |  | yes |  |  |
| #1463 | `pkg/eventlog/pause.go:360` | consumeToken does a non-atomic read-check-write (FindToken, then ConsumedAt check, then an unconditional append). Two concurrent consumers can both succeed o... |  |  |  |  |  |  |  |  | yes |
| #1463 | `pkg/compose/autoresume.go
:375` | Possible Bug: `lastEventIsUnhandledCall` does not skip the operator effects-ack annotation event. The PR's own change to `classifyInterruptedTail` skips it b... |  |  | yes | yes |  | yes |  |  |  |
| #1463 | `cmd/core-agent/main.go:2767` | `schedule` silently does nothing in REPL, TUI and headless runs, and in multi-session mode it fires into the bootstrap `default` session. The trigger is only... | yes |  |  |  | yes | yes |  | yes |  |
| #1463 | `pkg/compose/schedtrigger.go:239` | Do not replace a durable anchor when its read fails. |  | yes | yes |  |  |  |  |  |  |
| #1463 | `pkg/eventlog/schedule.go:139` | The schedule record session grows without limit, and every fire and rescan reads all of it back. Each `SaveSchedule` appends a new event to `mast-scheduler:m... | yes |  |  |  |  |  | yes |  |  |
| #1463 | `pkg/compose/pausesched.go:305` | A timed pause that was later changed to indefinite is still resumed automatically at the old deadline. In `fireDue`, a zero `rec.ResumeAt` fails `rec.ResumeA... | yes | yes |  |  |  |  |  |  |  |
| #1463 | `cmd/core-agent/schedule.go:67` | The wired `Fire` only calls `a.InjectAs` and returns immediately, so the trigger's single-flight overlap skipping and per-tick accounting never reflect the r... | yes |  |  |  |  | yes |  |  |  |
| #1463 | `cmd/core-agent/schedule.go:56` | Auto-continue now treats every MCP tool as mutating, so any turn interrupted during an MCP call stops being auto-continued until an operator acks it. This is... | yes |  |  |  |  |  |  |  |  |
| #1463 | `pkg/compose/auto_continue.go:334` | The ambiguous-effect guard checks every unpaired mutating call in the 128-event window, not just the interrupted tail. One old interrupted `bash` call that a... |  |  |  |  |  |  | yes |  |  |
| #1465 | `pkg/config/config.go:2712` | The new `schedule.max_fires` and `schedule.timeout` fields are accepted in config and workload bundles but nothing reads them. | yes |  |  |  | yes | yes | yes | yes | yes |
| #1465 | `pkg/compose/monitor.go:259` | remind_after does nothing unless wake_on also lists "remained". matchesWakeOn returns slices.Contains(WakeOn, edge) whenever wake_on is non-empty, so remaine... | yes |  |  |  |  |  |  |  |  |
| #1465 | `pkg/compose/monitor.go:224` | An operator ack hides a subject forever, for every kind of edge. filterTransitions drops every transition whose SubjectKey has an AckRecord, including closed... (known bug) | yes |  | yes | yes | yes | yes | yes | yes | yes |
| #1465 | `docs/site/src/content/docs/reference/configuration.md:1560` | The `transitions_from` docs describe a `key=value` text stream that the code never parses. |  | yes |  |  |  | yes | yes | yes |  |
| #1465 | `pkg/compose/monitorctx.go:156` | The code that runs collect and ack tools on the runtime's own behalf does not actually get the raw tool output. In pkg/mcp/lifecycle.go:wrapServerToolset, wh... | yes |  |  |  |  |  | yes |  |  |
| #1465 | `internal/notify/client.go:338` | The real client never returns ErrSendFullText from Append: a 409 Conflict becomes a generic *Error. As a result, sendTimeline's 409 Replace/fresh-Post fallba... (known bug) |  |  | yes | yes | yes |  | yes |  |  |
| #1465 | `cmd/core-agent/schedule.go:250` | The scheduled-fire waiter takes the result of whichever wake-loop turn finishes next, not the turn its own injected prompt started. (known bug) | yes |  | yes |  |  | yes | yes | yes |  |
| #1465 | `pkg/compose/notify.go:395` | sendTimeline treats any Append result whose Ref differs from the current ref as a "continuation rollover" and resets the accumulated text. The real client's ... | yes |  |  |  |  |  |  |  | yes |
| #1465 | `pkg/config/monitor.go:257` | The check that keeps self-run monitor tools away from the model only looks at subagents. The main agent still gets every MCP toolset, including the collect a... | yes |  |  |  |  |  |  |  |  |
| #1465 | `pkg/runner/wakeloop.go:197` | OnTurnDone gets only the last text part of the turn, not the final assistant message. The loop overwrites lastText for each non-thought text part, so a final... (known bug) | yes | yes |  |  |  |  |  |  |  |
| #1468 | `pkg/compose/a2a.go:220` | Memory grows without bound under A2A traffic. `sessionLocks` and `contextSessions` gain an entry for every new contextId and nothing ever deletes them. On a ... | yes |  |  |  |  |  | yes |  |  |
| #1468 | `pkg/agent/agent.go
:1681` | Dropped Prompt Prepends: In `runTurn`, the pre-turn pipeline output `tp.prompt` (which carries the prepends and guardrail text) is only used when `msg == nil... (known bug) | yes |  | yes | yes | yes | yes | yes |  | yes |
| #1468 | `pkg/attach/a2a.go
:500` | Possible Bug: `validateA2AInput` runs on every `message/send` and `message/stream`, including follow-up messages that resume an `input-required` task by carr... | yes |  | yes |  |  |  |  | yes |  |
| #1468 | `pkg/attach/a2a.go:639` | Cancel A2A streams when the attach server closes. |  | yes |  |  |  |  |  |  |  |
| #1468 | `pkg/compose/a2a.go:893` | Recovery from the store invents task results. `recoverTaskFromStore` reports `completed` with the session's latest assistant text for any ID that `findPersis... | yes | yes |  |  |  | yes |  |  |  |
| #1468 | `pkg/compose/a2a.go:375` | Cancelling an input-required task only flips the in-memory state to canceled and leaves the durable park pending in the session eventlog. | yes | yes |  | yes | yes |  | yes |  | yes |
| #1468 | `pkg/compose/a2a.go:1015` | Each new A2A task makes several full scans of the eventlog, loading every persisted session with all its events, because unknown task and context IDs are res... | yes |  | yes | yes | yes | yes | yes | yes |  |
| #1468 | `pkg/compose/a2a.go:400` | normalizeTaskAndContextIDs mints a fresh random contextId when a parked task is resumed after a restart with only a taskId, because it only looks in the in-m... | yes | yes |  |  | yes | yes | yes | yes | yes |
| #1468 | `pkg/compose/a2a.go:296` | Leave a task parked when the follow-up verdict is invalid. |  | yes |  |  |  | yes |  |  |  |
| #1468 | `pkg/compose/a2a.go:1196` | When both a data part and a text part are sent, the schema validates only the data part but the model prompt is built from the text part, so the input schema... |  |  |  |  |  |  |  | yes |  |
| #1468 | `pkg/compose/a2a.go:906` | Do not replace a live task with an older store read. |  | yes |  |  |  |  |  |  |  |
| #1512 | `pkg/config/vcs.go:101` | `proposal_url_template` is documented with `{source}`/`{target}` placeholders, but core-vcs v1.1.0 only substitutes `{host}`, `{path}`, `{base}` and `{branch}`. |  |  |  |  |  |  |  | yes |  |
| #1512 | `pkg/compose/run.go:117` | compose.Run prompts twice for each mutating vcs call, prompts for the read-only ones in ask mode, and gives the root agent the vcs tools when builtins are of... | yes |  |  |  |  |  | yes |  |  |
| #1512 | `pkg/tools/vcs/vcs.go:196` | token_file values that use `${VAR}` are accepted by validation and expanded for the credential-refusal list, but buildForges hands the raw, unexpanded string... | yes |  |  |  | yes |  | yes |  |  |
| #1512 | `pkg/tools/vcs/tools.go:79` | `vcs_clone` is classed read-only and plan-exempt, but it runs `os.RemoveAll(dest)` on the existing working copy, so uncommitted edits are deleted without any... |  | yes | yes | yes | yes | yes | yes | yes | yes |
| #1512 | `cmd/core-agent/main.go:1347` | The PR adds VCS wiring inline in run(), which the funlen/gocognit ratchet exemption says must not grow. AGENTS.md says: "Don't grow it: reusable substrate go... | yes |  |  |  |  |  |  |  |  |
| #1512 | `pkg/tools/vcs/tools.go:113` | In multi-session, one session's vcs_clone deletes another session's working copy, including its unpublished commits. main.go builds a single vcs Suite into b... | yes |  |  |  |  |  | yes |  |  |
| #1512 | `pkg/compose/vcs.go:172` | A successful vcs_publish or vcs_propose can make the rest of the turn fail with a stale-session error. VCSEventSink runs while the tool is executing, inside ... (known bug) | yes |  |  |  | yes |  |  |  |  |
| #1512 | `pkg/compose/run.go:183` | On the dirless compose.Run path, configuring VCS fails at startup for hosts that supply the model through RunConfig.Model. The block passes the raw cfg.Confi... | yes |  |  |  |  |  |  |  |  |
| #1577 | `examples/gke-platform-agent/cluster/skills/gke-workload-troubleshooting/SKILL.md:197` | The new stop condition tells the agent to stop and report after a single `Forbidden`/permission-denied read on any resource. The rule was written for Secrets... | yes |  |  | yes | yes |  |  |  |  |
| #1578 | `pkg/runner/events.go:157` | In streaming mode, text the model writes after a function call in the same response is recorded before that call and run together with the text that came bef... | yes |  |  |  |  |  | yes |  |  |
| #1579 | `pkg/permissions/policy.go:375` | Shell glob segments can still walk out of a path-prefix rule. The `..` check at line 375 only rejects a literal `..` segment, and `bashSafeForAutoAllow` trea... | yes |  |  |  |  |  |  |  |  |
| #1579 | `pkg/permissions/policy.go:392` | For a non-slash-terminated prefix whose parent is `/`, rootDir becomes "/" and the check `HasPrefix(cleanCand, rootDir+"/")` requires "//", so every candidat... |  |  |  |  |  | yes |  | yes |  |
| #1579 | `pkg/permissions/policy.go:405` | Path-prefix rules that start with `~/` are checked against the wrong directory. `resolvedPathStaysInside` joins the relative-looking `~/bin` onto baseDir bef... | yes |  |  |  | yes | yes |  |  |  |
| #1579 | `pkg/permissions/gate.go:1102` | The symlink check can look at a different directory from the one the shell runs in. When the ctx has no WorkDir, `effectiveWorkDir` uses `scope.rwRoots[0]` (... | yes |  |  |  |  |  |  |  | yes |
| #1579 | `pkg/permissions/policy.go:365` | Path semantics now apply to every non-bash tool, not just file tools. For a non-bash tool the whole key becomes the path token whenever the prefix contains `... | yes |  |  |  |  |  | yes |  |  |
| #1579 | `docs/site/src/content/docs/concepts/permissions.md:86` | The PR changes what allow and auto-eligible rules match, a user-visible change, but does not update the published permissions page, which the repository rule... | yes |  |  |  |  |  | yes |  |  |
| #1580 | `pkg/agent/background/manager.go:1063` | Reclaiming can delete the final report of an earlier subagent that had the same name. reclaimTerminalAlert matches on `a.From == name` only, and names can be... | yes |  | yes | yes |  |  | yes | yes |  |
| #1581 | `pkg/tools/glob.go:51` | The skip set gained `.cache`, but the published docs still list the old set (`.git`, `.svn`, `.hg`, `node_modules`, `vendor`). The change adds no site update... | yes |  |  |  | yes | yes | yes |  |  |
| #1611 | `cmd/core-agent/subagents.go:874` | This is a user-visible behavior change with no update to the published site. docs/site/src/content/docs/concepts/tools.md:361 and concepts/mcp.md:322 still s... | yes |  |  |  |  | yes |  |  | yes |
| #1614 | `pkg/config/workload.go:1034` | writeSubagentMarkdownFile collapses a subagent's model block to a bare name when only the old fields are empty. The new `ThinkingLevel` field is missing from... (known bug) |  |  |  |  |  | yes | yes |  |  |
| #1614 | `pkg/agent/run_facade.go:339` | mergeFacadeModelConfig does not copy ModelConfig.ThinkingLevel, so a thinking level passed in the library facade's Config override is silently dropped. |  |  |  |  | yes |  | yes | yes |  |

## False alarms: claude-code

- #1430 `pkg/eventlog/service.go:115` A new session that reuses a deleted session's ID can inherit the deleted session's subagent spend. `Delete` removes overlay, guardrail and now spend rows onl...
- #1454 `pkg/config/workload.go:695` A workload with `hitl.on_mutation: dry_run` imports into a recipe where no tool runs at all, reads included. It maps to `permissions.mode: "plan"`, and `perm...
- #1576 `.github/workflows/release-images.yml:114` The condition `matrix.variant == 'core-agent'` also matches `core-agent-slim` (variant core-agent, build_tags no_tui). The headless 'slim' image now embeds t...
- #1577 `examples/gke-platform-agent/deploy/base/10-serviceaccount-daemon.yaml:100` The new opt-in instructions say to bind a Kubernetes Role to `serviceAccount:<PROJECT_ID>.svc.id.goog[gke-platform-agent/core-agent-daemon]`. That is IAM mem...
- #1577 `examples/gke-platform-agent/recipe_test.go:178` The new regression assertion only checks `.agents/config.json`, through `loadConfig(agentsDir)`. The PR also sets `web_search: false` in `config.hub.json`, `...
- #1609 `pkg/agent/quirks.go:279` `observedToolset` forwards only Name/Tools/UnwrapToolset, so it hides any toolset-level `ProcessRequest`. ADK's `toolsetPreprocess` only calls that method wh...
- #1611 `cmd/core-agent/subagents.go:188` The multi-session ad-hoc fix depends on each session tool being the exact same instance as the daemon's tool. If a SessionCustomizer wraps tools (instrumenta...

## False alarms: coderabbit

- #1431 `pkg/agent/background/a2a_test.go:239` Synchronize reads of HTTP fixture state.
- #1463 `pkg/compose/auto_continue.go:679` Refund sessions that the pass never reached.
- #1463 `pkg/compose/auto_continue.go:213` Stop an in-progress continuation when the scheduling lease is lost.

## False alarms: pr-agent

- #1197 `cmd/core-agent/coretui_enabled.go
:961` Stale pending flag: `DrainInbox` sets `autoTurnPending` whenever the drain is non-empty. The flag is only cleared by the next `Run`. If core-tui does not sta...
- #1451 `pkg/compose/run.go
:64` Divergent Config Merge: `compose.Run` builds its permission gate from `mergeComposeConfig`, a hand-rolled partial merge. `agent.RunOneShot` uses the fuller `...
- #1465 `cmd/core-agent/main.go
:2406` Surface Guard Skipped: `config.CheckMonitorCollectSurface` runs only inside `buildMonitorAckHandler`, and that is called only in the attach-server setup bloc...
- #1468 `:0` Unauthorized session access: when `EnforceACL` is false (A2A exposed without multi-session), `GetTask` and `SendMessage` can resolve arbitrary non-A2A sessio...
- #1468 `pkg/compose/a2a.go
:1050` Authorization Gap: `findPersistedSession` matches any persisted session by its raw session ID, not only A2A-created ones. `EnforceACL` is tied to `attach.mul...
- #1512 `pkg/config/vcs.go
:124` Missing Env-Ref Tag: `VCSForgeConfig.TokenEnv` carries only `json`/`yaml` tags. The PR updates `TestEnvRefTagInventory` to expect `vcs.forges[].token_env`, a...
- #1576 `.github/workflows/release-images.yml
:113` Missing Assets: The fetch step runs only when `matrix.variant == 'core-agent'`. Other matrix variants that build the same `core-agent` binary (if any) would ...
- #1579 `pkg/permissions/policy.go
:408` Incomplete Fix: With a `baseDir` of `""` (the `Policy.Match` and `matchPolicy` paths, which are still used by other callers), relative prefixes are resolved ...
- #1579 `pkg/permissions/policy.go
:416` Behavior Regression: `resolvedPathStaysInside` returns false when `ResolvePath` errors. A legitimate allowed target that does not exist yet (for example a `w...
- #1579 `pkg/permissions/policy.go
:394` Possible Bug: For a non-slash-terminated path prefix such as `bash:dev/tools/x*`, the check `strings.HasPrefix(cleanCand, cleanPrefix)` runs after `path.Clea...
- #1580 `pkg/agent/agent.go
:1888` Unconditional cancel: `cancel()` is now called whenever the consumer's `yield` returns false. This cancels the turn context on any early break, not only in t...
- #1581 `go.mod
:17` Version Skew: `core-models` is bumped to `v0.15.4`, but the sibling module `core-models/adkv2` stays at `v0.15.3`. If the two modules are meant to be release...
- #1610 `pkg/agent/subagent.go
:447` Possible Bug: `subagentToolDescription` dereferences `opts.Inner.subagentDescription` and `opts.Inner.inner.Description()` without a nil check on `opts.Inner...

## False alarms: pr-agent-20

- #1197 `cmd/core-agent/coretui_enabled.go
:1001` Stale arming: `DrainInbox` sets `autoTurnPending` whenever the drain is non-empty, on the assumption that core-tui always starts an auto-continue turn next. ...
- #1465 `cmd/core-agent/schedule.go
:49` Workload/App Mismatch: `buildMonitorAckHandler` hardcodes `AppName: "core-agent"` and falls back to `"core-agent"` for the workload name. The collector and n...
- #1576 `internal/webui/webui_test.go
:32` Test Brittleness: `TestPinnedVersionFileIsNonEmpty` reads `.mast-web-version` through a relative path (`../../.mast-web-version`) and fails if the file is mi...
- #1576 `.github/workflows/release-images.yml
:113` Missing Assets: The fetch step runs only when `matrix.variant == 'core-agent'`, so any other variant that builds the same binary or Dockerfile ships an empty...
- #1577 `examples/gke-platform-agent/deploy/base/10-serviceaccount-daemon.yaml
:97` Incorrect RBAC guidance: The opt-in instructions say to bind a namespace-scoped Kubernetes Role to `serviceAccount:<PROJECT_ID>.svc.id.goog[ns/core-agent-dae...
- #1580 `pkg/agent/agent.go
:1888` Unconditional cancel: `cancel()` is now called whenever the consumer's `yield` returns false. If `cancel` is the turn-level context cancel, this aborts in-fl...
- #1581 `go.mod
:17` Version Skew: `core-models` is bumped to `v0.15.4` but the `core-models/adkv2` submodule stays at `v0.15.3`. If the adkv2 submodule depends on APIs or the pr...

## False alarms: surveyor-broad

- #1576 `.github/workflows/release-images.yml:113` The new unconditional fetch step breaks manual image republish for older tags that lack dev/tools/fetch-mast-web.
- #1609 `cmd/core-agent/run_steps.go:1457` hasModelTools reads s.agentRef from the /memory provider goroutine with no synchronization, although onPrimaryAgentPostConstruct writes it later.

## False alarms: surveyor-steps150

- #1465 `cmd/core-agent/schedule.go:43` CheckMonitorCollectSurface is only called from buildMonitorAckHandler, which main.go calls only inside the attach-listener branch, so the guard is skipped fo...
