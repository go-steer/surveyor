# Replay: 30 pull requests

Every finding was judged blind (reviewer hidden, order shuffled) by claude-opus-5-5 reading the code at the PR's head. A **defect** is a group of findings the judge labelled real and about the same problem. Known bugs are defects a later PR fixed. Judge cost: $32.56.

| | claude-code | surveyor |
|---|---|---|
| model | claude-opus-5-5 | claude-sonnet-5-5 |
| settings | defaults | defaults |
| PRs reviewed | 30 | 30 |
| findings reported | 212 | 47 |
| real / nit / false alarm | 73 / 129 / 10 | 31 / 16 / 0 |
| share of findings that are real | 34% | 66% |
| false alarms | 5% | 0% |
| defects found (of those found by anyone in the PRs it reviewed) | 70 of 75 (93%) | 31 of 75 (41%) |
| defects only this reviewer found | 44 | 5 |
| known bugs caught | 14 of 25 | 10 of 25 |
| cost, total (mean per PR) | $36.63 ($1.22) | $16.48 ($0.55) |
| time, mean (max) | 219s (482s) | 92s (385s) |
| incomplete reviews (cap hit or diff truncated) | not recorded | not recorded |

surveyor's verifier refuted 0 findings: 0 false alarms and 0 nits correctly dropped, 0 real findings lost.

## Known bugs

| PR | fixed in | bug | claude-code | surveyor |
|---|---|---|---|---|
| #1197 | #1233 | The auto-mode approver's task must contain only words the operator wrote. #1197 cut core-tui's @-inlined file content from the local TUI's prompt, but on the... | missed | missed |
| #1429 | #1452 | On any AppendEvent error the drain re-queues pending[i:] at the front of the queue and stops, with no attempt cap and no distinction between transient and de... | caught | caught |
| #1429 | #1480 | RestoreGuardrails folds the pre-upgrade agent_eventlog guardrail history only when no agent_guardrail_state row exists, but RecordGuardrailReset does not res... | caught | missed |
| #1430 | #1481 | The resume rebuild prices each root-session turn under resolveRebuildModel, which prefers the provider's served ev.ModelVersion. The compose pricing callback... | caught | missed |
| #1431 | #1443 | endpointURL takes the JSON-RPC endpoint verbatim from the fetched agent card (card.url or additionalInterfaces[].url) with no scheme/host validation, while v... | missed | missed |
| #1431 | #1457 | The message/stream SSE parser treats each 'data:' line as a complete event and JSON-decodes it immediately, instead of accumulating consecutive data: lines a... | caught | missed |
| #1431 | #1457 | Streaming artifact-update events are appended as separate artifacts regardless of the event's append flag / artifactId, and the final result joins all artifa... | caught | caught |
| #1431 | #1479 | #1431 added a status guard to the shared claimSync primitive, which the in-process awaitResult also uses. Between a handle's status turning terminal and take... | caught | caught |
| #1441 | #1449 | The new unknown-key walker matches JSON keys against struct json tags case-sensitively, but encoding/json matches field names case-insensitively. A mixed-cas... | caught | caught |
| #1445 | #1477 | On an operator verdict of "edit", beforeTool persists the decision and then calls applyArgs(args, edited), which writes the edited keys into the caller's arg... | missed | missed |
| #1448 | #1460 | The new capability: read_only check calls tools.IsReadOnlyTool on built-in tools after cmd/core-agent has wrapped them with the MCP digester. digestingTool.R... | missed | missed |
| #1448 | #1460 | Markdown subagent authoring (.agents/subagents/*.md) and the baseDir stamping used for output_schema resolution are wired only into config.Load. The -c/--con... | missed | missed |
| #1451 | #1476 | RunOneShot/compose.Run select a subagent's MCP toolsets with `ts.Name() == name \|\| strings.HasPrefix(ts.Name(), name+"_")`, ignore spec.MCPTools entirely (... | caught | caught |
| #1454 | #1470 | When converting tool_catalog entries into permissions.allow rules, any non-mutating tool whose name contains a hyphen is classified as a subagent and emitted... | caught | missed |
| #1454 | #1501 | The workload importer maps mast's per-turn/per-session call budgets (budget.max_turns, budget.max_model_calls_per_turn) onto cfg.Agent.MaxSteps, but nothing ... | caught | caught |
| #1463 | #1474 | persist writes the new fire count to in-memory t.fires before SaveSchedule succeeds, yet recomputes the count from the stored prev.Fires on the next call. Wh... | missed | missed |
| #1465 | #1473 | Notifier.sendTimeline falls back to Replace/Post only when Client.Append returns ErrSendFullText or ErrNoSuchMessage, but Append is a POST and doAttempt maps... | missed | caught |
| #1465 | #1473 | loadAckedSubjects returns every subject that has any AckRecord, ignoring AckRecord.Forwarded and AckRecord.At, and filterTransitions drops every transition f... | caught | caught |
| #1465 | #1473 | The scheduled-turn waiter has no turn correlation. runScheduledTurn sets w.out, calls a.InjectAs (which only queues the prompt), and accepts the result of wh... | caught | missed |
| #1468 | #1472 | The A2A server's turn driver iterates the agent's event sequence directly without a usage.TurnTap and never calls ag.Tracker().AppendUsage, unlike the other ... | missed | missed |
| #1468 | #1472 | tasks/get and tasks/cancel authorize only against the top-level a2a.auth default scopes, ignoring the skill-level Auth.Required/Auth.Scopes that message/send... | missed | missed |
| #1468 | #1472 | RunContent passes an empty prompt plus a prebuilt message into runTurn, which still runs the pre-turn steps that drain pending background alerts, the inject ... | caught | caught |
| #1512 | #1572 | VCSEventSink appends the vcs.published / vcs.proposed audit event to the session inline, from inside the vcs_publish / vcs_propose tool handler, while ADK's ... | caught | caught |
| #1512 | #1572 | The VCS audit event is written with user-role Content (Author "system") instead of as a content-less audit row with the payload only in CustomMetadata (as me... | missed | missed |
| #1614 | #1616 | #1614 added ModelConfig.ThinkingLevel but did not update writeSubagentMarkdownFile, which writes a subagent's model block as a bare `model: <name>` when ever... | missed | missed |

## Per pull request

| PR | title | claude-code | surveyor |
|---|---|---|---|
| #1197 | feat(agent): stamp the auto-mode approver's context on every turn (#1175) | 1 real of 9, $2.90, 482s | 0 real of 1, $0.60, 109s |
| #1429 | fix(guardrails): bound out-of-band drain reads, re-queue errors, and add GuardrailStore | 4 real of 8, $0.99, 210s | 1 real of 1, $0.50, 103s |
| #1430 | fix(usage,eventlog,compose): rebuild session-tree spend with per-model pricing and add SpendStore (#1359) | 3 real of 10, $1.42, 254s | 0 real of 1, $0.41, 96s |
| #1431 | feat(background): ship A2A v0.3 RemoteAgentSpawner and subagents[].remote routing | 6 real of 10, $1.43, 249s | 3 real of 4, $0.77, 106s |
| #1441 | fix(config): walk unknown nested keys recursively and fail on safety/cost typos | 2 real of 10, $0.79, 227s | 3 real of 4, $0.60, 116s |
| #1445 | feat(approval): port durable write gate, effect outbox, and /parks endpoints (#1400) | 4 real of 10, $1.89, 287s | 2 real of 2, $0.88, 138s |
| #1448 | feat(subagents): add mode, output_schema, tier, capability, mcp_tools, and Markdown authoring (#1401) | 2 real of 10, $2.70, 412s | 1 real of 2, $0.62, 85s |
| #1451 | feat(agent,compose): add one-call RunOneShot / RunRecipe / RunWorkload facade (#1404) | 4 real of 9, $1.93, 342s | 1 real of 2, $1.04, 183s |
| #1454 | feat(config): add workload.yaml converter and import-workload CLI (#1402) | 6 real of 10, $1.54, 249s | 1 real of 3, $0.68, 133s |
| #1463 | feat(compose,eventlog): port scheduled trigger, fleet lease, auto-resume, and pause rescan (#1405) | 7 real of 10, $1.91, 314s | 2 real of 3, $0.91, 385s |
| #1465 | feat(compose,attach): port deterministic monitoring cycle, notify, and /monitor-ack (#1406) | 9 real of 10, $1.28, 220s | 3 real of 4, $0.70, 72s |
| #1468 | feat(attach,compose,config): mount A2A v0.3 server on attach mux (#1407) | 7 real of 10, $1.66, 282s | 4 real of 5, $1.02, 134s |
| #1512 | feat: add forge-neutral vcs_* tool suite and config.vcs (#1490) | 6 real of 10, $2.25, 298s | 3 real of 3, $1.10, 127s |
| #1557 | feat(models): bump core-models to v0.15.3 and classify Gemini 4 Carbon/Hydrogen | 0 real of 2, $0.55, 102s | 0 real of 0, $0.26, 32s |
| #1570 | docs(models): point the ANTHROPIC_BASE_URL comment at the archived scenarios doc | 0 real of 3, $0.18, 34s | 0 real of 0, $0.04, 11s |
| #1572 | fix(vcs): defer VCSEventSink audit writes past in-flight turns (#1571) | 0 real of 5, $1.17, 240s | 0 real of 0, $0.41, 102s |
| #1576 | feat(webui): pin mast-web v0.7.0 and wire fetch-mast-web into release builds (#348) | 0 real of 6, $0.40, 99s | 0 real of 0, $0.11, 23s |
| #1577 | fix(examples): document secrets exclusion, disable web_search, and add sufficiency rules in GKE recipes (#695, #696) | 2 real of 8, $0.66, 150s | 2 real of 2, $0.54, 62s |
| #1578 | fix(transcript): record assistant reply in headless one-shot and report save errors (#1553) | 1 real of 7, $0.91, 182s | 0 real of 1, $0.46, 66s |
| #1579 | fix(permissions): reject .. traversal and escaping symlinks on path-prefix rules (#1274) | 5 real of 8, $0.80, 204s | 1 real of 2, $0.30, 83s |
| #1580 | fix(test): fix synchronous subagent alert race and CI test flakes (#1551) | 2 real of 8, $0.75, 156s | 1 real of 2, $0.40, 77s |
| #1581 | feat(vcs,models): bump core-vcs to v1.2.0 and core-models to v0.15.4 (#1490) | 2 real of 7, $0.65, 144s | 2 real of 2, $0.71, 63s |
| #1592 | chore: remove three functions unreachable from any main or test | 0 real of 1, $0.25, 46s | 0 real of 0, $0.09, 14s |
| #1603 | fix(modelretry): classify Claude errors by HTTPStatus, not the SDK type | 0 real of 3, $0.49, 102s | 0 real of 0, $0.09, 20s |
| #1609 | fix(agent): gate Gemini parallelism quirk on enabled tools | 0 real of 4, $1.92, 373s | 0 real of 0, $0.52, 107s |
| #1610 | fix(subagents): keep router descriptions out of child ADK identity prompts | 0 real of 3, $0.96, 190s | 0 real of 0, $1.24, 65s |
| #1611 | fix(subagents): skip digest wrap when subagent lacks retrieve_raw | 0 real of 10, $1.41, 246s | 0 real of 2, $0.60, 116s |
| #1612 | fix(mcp): omit MCP elicitation in headless and non-TUI runs (#1606) | 0 real of 4, $1.00, 160s | 0 real of 0, $0.27, 37s |
| #1614 | feat(config): model.thinking_level, one thinking setting across providers | 0 real of 5, $1.38, 232s | 1 real of 1, $0.57, 78s |
| #1616 | fix(config): keep a subagent's thinking_level in the Markdown write | 0 real of 2, $0.44, 82s | 0 real of 0, $0.05, 15s |

## Defects

| PR | where | defect | claude-code | surveyor |
|---|---|---|---|---|
| #1197 | `pkg/agent/approver_context.go:268` | The earlier calls shown to the approver are encoded with json.Marshal, which HTML-escapes <, > and &. So `go test && rm -rf x > /etc/foo` reaches the approve... | yes |  |
| #1429 | `pkg/eventlog/guardrail.go:892` | On a session from before the upgrade, a guardrail halt or granted budget can be silently lost. `LoadWithLegacyCheck` only looks at the event history when NO ... (known bug) | yes |  |
| #1429 | `pkg/eventlog/sql.go:167` | The published docs still describe the old database layout. `Open` now creates a new table, `agent_guardrail_state`, and exposes the new public API `Handle.Gu... | yes |  |
| #1429 | `pkg/agent/guardrail_persist.go:139` | Every Get/AppendEvent failure is re-queued unconditionally and with no bound, so one event that can never be written blocks all later rows and the queue grow... (known bug) | yes | yes |
| #1430 | `pkg/usage/rebuild.go:139` | A resume can bill spend that the live tracker never recorded, so the restored total jumps above the pre-eviction figure. Background subagents are built with ... | yes |  |
| #1430 | `pkg/usage/rebuild.go:169` | After a resume, per-model totals and cost can come out different from the live session. `resolveRebuildModel` prefers `ev.ModelVersion`, which `stampModelVer... (known bug) | yes |  |
| #1430 | `pkg/compose/multi_session.go:605` | Resuming a session that was written before this upgrade double-bills or misprices its subagent spend. The resume path now scans the whole `WithSessionTree`, ... | yes |  |
| #1431 | `pkg/agent/background/a2a_client.go:696` | artifact-update frames are appended as separate artifacts, so the `append` and `lastChunk` flags and the artifactId are ignored. (known bug) | yes | yes |
| #1431 | `pkg/agent/background/manager.go:241` | The new status check in claimSync applies to every subagent, not just remote ones, and it makes double delivery more likely for in-process subagents. spawn.g... (known bug) | yes | yes |
| #1431 | `pkg/agent/background/templates.go:419` | Daemon shutdown can stall for several seconds per remote subagent. Manager.Close calls each handle's cancel one after another. For remote handles that cancel... | yes |  |
| #1431 | `pkg/agent/background/a2a_client.go:634` | SSE frames whose data spans several lines are parsed one line at a time. The scanner treats each 'data:' line as a complete JSON frame, but SSE says consecut... (known bug) | yes |  |
| #1431 | `pkg/agent/background/a2a_client.go:779` | For a remote whose card does not advertise streaming, stop_agent and timeouts never send tasks/cancel, and no progress alerts arrive. runPoll (and Send at li... | yes |  |
| #1431 | `pkg/agent/background/remote.go:410` | Existing RemoteAgentSpawner implementations behave differently after this change. Before, an Events() channel that closed without a terminal event finalized ... | yes | yes |
| #1441 | `pkg/config/discovery.go:308` | The fatal check is a name heuristic, and it also runs inside unknown extension blocks. Any unknown leaf containing cost/budget/usd/mtok/rate_limit, or within... | yes | yes |
| #1441 | `pkg/config/discovery.go:205` | Configs that work today now fail to load or get false warnings because of key casing. `structFieldsByJSONKey` is looked up with an exact `known[p.key]`, but ... (known bug) | yes | yes |
| #1441 | `docs/site/src/content/docs/reference/configuration.md:191` | The published configuration reference is not updated. It still says an unknown key such as `permisions` is preserved and only logs a warning. After this chan... |  | yes |
| #1445 | `pkg/approval/plugin.go:422` | `honorVerdict` assumes `Config.Gate` is set, but Gate is only required under `require_approval`. Under `apply` or `dry_run` with a nil Gate, any call that ar... | yes | yes |
| #1445 | `pkg/approval/plugin.go:235` | If the gate's mutating mode is `apply` while the plugin policy is `require_approval`, the write gate panics on every mutating call. `Gate.CheckMutatingToolCa... | yes |  |
| #1445 | `pkg/eventlog/decisions.go:726` | enrichLongRunningPark reads the parked call's key from `Request["subject"]`, but the write gate's parked payload (approval.Request) serialises it as `"key"`,... | yes | yes |
| #1445 | `pkg/eventlog/decisions.go:505` | On a SQL-backed session, POST /ack-effects can make a running turn fail. `AckEffects` appends an event to the live session from outside the turn, through the... | yes |  |
| #1448 | `pkg/config/subagents_markdown.go:85` | The mapping form of `model:` in a Markdown subagent is decoded into `ModelConfig` with yaml.v3 under `KnownFields(true)`, but `ModelConfig` and its nested ty... | yes | yes |
| #1448 | `pkg/agent/modes.go:1847` | Chat-mode agents built with `agent.New` never check the call gate, although the `WithCallGate` doc says every agent built with New consults it. `buildInnerLL... | yes |  |
| #1451 | `pkg/agent/run_facade.go:883` | A subagent with a named `skills:` list gets every skill, not the ones named. `if spec.Skills == nil \|\| len(spec.Skills) > 0 { out = append(out, skillSets..... (known bug) | yes | yes |
| #1451 | `pkg/agent/run_facade.go:855` | Subagent skill scoping and the read_only check are broken whenever a recipe has skills. The facade looks for a toolset named "skill_toolset", but `skills.Loa... | yes |  |
| #1451 | `pkg/agent/run_facade.go:872` | Naming one MCP server in a subagent's `mcp:` list can also grant other servers. The match `strings.HasPrefix(ts.Name(), name+"_")` hits any server whose key ... (known bug) | yes |  |
| #1451 | `pkg/agent/run_facade.go:864` | A subagent scoped with `mcp_tools` gets every MCP server. `selectFacadeSubagentToolsets` never reads `spec.MCPTools`, so `spec.MCP == nil` falls through to i... (known bug) | yes |  |
| #1454 | `pkg/config/workload.go:650` | The importer writes settings that nothing in the runtime reads, and silently drops other workload.yaml blocks. A successful import therefore looks like a fai... (known bug) | yes | yes |
| #1454 | `pkg/config/workload.go:693` | A workload with `hitl.on_mutation: apply` imports as a recipe that can never apply a change. `mapWorkloadHITLMode` maps it to `permissions.mode: "allow"`, bu... | yes |  |
| #1454 | `pkg/config/workload.go:721` | Two allowlist heuristics can grant the wrong thing. Any read-only catalog tool with a hyphen in its name becomes `spawn_agent:<name>`, even when it's an MCP ... (known bug) | yes |  |
| #1454 | `pkg/config/workload.go:568` | The import-time capability-split check can miss a diagnoser that holds write tools, and nothing catches it later. The check is skipped when `tool_catalog.mcp... | yes |  |
| #1454 | `examples/gke-platform-agent/gated-apply/.agents/subagents/change-executor.md:29` | The change-executor persona describes safety machinery this recipe doesn't have. It says each mutating call still pauses at the write gate for a human, but l... | yes |  |
| #1463 | `cmd/core-agent/schedule.go:84` | A replica that loses the lease race at boot stays passive for its whole life and never tries to take the lease again. When the holder exits, no replica drive... | yes |  |
| #1463 | `pkg/compose/pausesched.go:305` | A timed pause that was later changed to indefinite is still resumed automatically at the old deadline. In `fireDue`, a zero `rec.ResumeAt` fails `rec.ResumeA... | yes |  |
| #1463 | `cmd/core-agent/main.go:2767` | `schedule` silently does nothing in REPL, TUI and headless runs, and in multi-session mode it fires into the bootstrap `default` session. The trigger is only... | yes | yes |
| #1463 | `pkg/compose/pausesched.go:115` | scheduleFromScan refuses to re-arm a token when the persisted ResumeAt moves earlier, so the rescan cannot shorten a pending timer. |  | yes |
| #1463 | `pkg/eventlog/schedule.go:139` | The schedule record session grows without limit, and every fire and rescan reads all of it back. Each `SaveSchedule` appends a new event to `mast-scheduler:m... | yes |  |
| #1463 | `cmd/core-agent/schedule.go:56` | Auto-continue now treats every MCP tool as mutating, so any turn interrupted during an MCP call stops being auto-continued until an operator acks it. This is... | yes |  |
| #1463 | `pkg/compose/schedtrigger.go:327` | A database blip can stop scheduled work on a single replica for good. If the lease heartbeat fails for the loss window, `Lost()` closes, and the trigger, the... | yes |  |
| #1463 | `cmd/core-agent/schedule.go:77` | Scheduled fires are queued even when the session is paused, halted by a guardrail, or still running the previous scheduled turn. These notes can push operato... | yes |  |
| #1465 | `pkg/compose/monitorack.go:145` | When the ack tool call fails, POST /monitor-ack returns 502 "the suppression did not take", but the ack has already been saved locally and takes effect. Ack ... (known bug) | yes | yes |
| #1465 | `internal/notify/client.go:334` | The client never maps 409 to ErrSendFullText, and a 404 on POST is not ErrNoSuchMessage, so the notifier's timeline fallbacks cannot run against the real cli... (known bug) |  | yes |
| #1465 | `pkg/config/config.go:2712` | schedule.max_fires and schedule.timeout are accepted but never used. The PR adds them to ScheduleConfig, WorkloadScheduledTrigger and the config-surface list... | yes | yes |
| #1465 | `pkg/compose/monitor.go:259` | remind_after does nothing unless wake_on also lists "remained". matchesWakeOn returns slices.Contains(WakeOn, edge) whenever wake_on is non-empty, so remaine... | yes |  |
| #1465 | `pkg/compose/notify.go:395` | Timeline mode moves its anchor to the latest reply instead of keeping the root message. Append returns the reply's Ref (parseRef of the POST response), and s... | yes |  |
| #1465 | `pkg/runner/wakeloop.go:197` | OnTurnDone gets only the last text part of the turn, not the final assistant message. The loop overwrites lastText for each non-thought text part, so a final... (known bug) | yes |  |
| #1465 | `pkg/config/monitor.go:257` | The check that keeps self-run monitor tools away from the model only looks at subagents. The main agent still gets every MCP toolset, including the collect a... | yes |  |
| #1465 | `cmd/core-agent/schedule.go:210` | A scheduled fire can report some other turn's result as its own. runScheduledTurn sets w.out before InjectAs, and AfterTurn runs after every wake-loop turn, ... (known bug) | yes |  |
| #1465 | `pkg/compose/monitorctx.go:156` | The code that runs collect and ack tools on the runtime's own behalf does not actually get the raw tool output. In pkg/mcp/lifecycle.go:wrapServerToolset, wh... | yes |  |
| #1468 | `pkg/compose/a2a.go:893` | Recovery from the store invents task results. `recoverTaskFromStore` reports `completed` with the session's latest assistant text for any ID that `findPersis... | yes |  |
| #1468 | `pkg/agent/agent.go:1683` | RunContent runs the whole pre-turn pipeline but discards its output (`tp.prompt`) whenever `msg != nil`, so drained alerts, inbox messages and watchdog feedb... (known bug) | yes | yes |
| #1468 | `pkg/compose/a2a.go:466` | Any pending park in the session is taken to be the answer to the current message, whichever task it belongs to. `startAgentTurn` does not check that `req.Tas... | yes | yes |
| #1468 | `pkg/compose/a2a.go:1031` | Every new task or context ID triggers a full scan of the event store. `findPersistedSession` and `findPersistedSessionByTaskID` list every session for the ap... | yes | yes |
| #1468 | `pkg/compose/a2a.go:416` | After a restart or registry eviction, `message/send` with only a `taskId` gives the task a new contextId. `normalizeTaskAndContextIDs` fills the contextId on... | yes | yes |
| #1468 | `pkg/attach/a2a.go:500` | When `a2a.input_schema` is configured, a parked task can never be resumed. `prepareTurnRequest` checks every `message/send` and `message/stream` payload agai... | yes |  |
| #1468 | `pkg/compose/a2a.go:220` | Memory grows without bound under A2A traffic. `sessionLocks` and `contextSessions` gain an entry for every new contextId and nothing ever deletes them. On a ... | yes |  |
| #1512 | `pkg/tools/vcs/vcs.go:196` | token_file with a `${VAR}` prefix is accepted by validation and credential protection (which expand it) but passed unexpanded to corevcs.TokenFile, so the cr... | yes | yes |
| #1512 | `pkg/tools/vcs/tools.go:113` | In multi-session, one session's vcs_clone deletes another session's working copy, including its unpublished commits. main.go builds a single vcs Suite into b... | yes |  |
| #1512 | `pkg/compose/vcs.go:172` | A successful vcs_publish or vcs_propose can make the rest of the turn fail with a stale-session error. VCSEventSink runs while the tool is executing, inside ... (known bug) | yes | yes |
| #1512 | `pkg/compose/run.go:117` | compose.Run prompts twice for each mutating vcs call, prompts for the read-only ones in ask mode, and gives the root agent the vcs tools when builtins are of... | yes |  |
| #1512 | `pkg/tools/vcs/tools.go:113` | vcs_clone, which is classed read-only and so never prompts, runs os.RemoveAll on the existing checkout before cloning. A repeat call destroys uncommitted edi... |  | yes |
| #1512 | `pkg/compose/run.go:183` | On the dirless compose.Run path, configuring VCS fails at startup for hosts that supply the model through RunConfig.Model. The block passes the raw cfg.Confi... | yes |  |
| #1512 | `cmd/core-agent/main.go:1347` | The PR adds VCS wiring inline in run(), which the funlen/gocognit ratchet exemption says must not grow. AGENTS.md says: "Don't grow it: reusable substrate go... | yes |  |
| #1577 | `examples/kube-platform-agent/cluster/AGENTS.md:9` | In the kube-platform-agent recipe, the cluster subagent is now told to "never substitute web search for a denied cluster read", but the recipe still leaves G... | yes | yes |
| #1577 | `examples/gke-platform-agent/cluster/skills/gke-workload-troubleshooting/SKILL.md:197` | The new stop condition tells the agent to stop and report after a single `Forbidden`/permission-denied read on any resource. The rule was written for Secrets... | yes | yes |
| #1578 | `pkg/runner/events.go:143` | When a streamed model response has text after a tool call, the transcript gets that text in the wrong place. Both text segments are merged and saved before t... | yes |  |
| #1579 | `pkg/permissions/gate.go:1102` | The symlink check can look at a different directory from the one the shell runs in. When the ctx has no WorkDir, `effectiveWorkDir` uses `scope.rwRoots[0]` (... | yes |  |
| #1579 | `pkg/permissions/policy.go:347` | The published permissions docs were not updated for this user-visible change. The open-prefix and safe-command sections in docs/site/src/content/docs/concept... | yes |  |
| #1579 | `pkg/permissions/policy.go:375` | Shell glob segments can still walk out of a path-prefix rule. The `..` check at line 375 only rejects a literal `..` segment, and `bashSafeForAutoAllow` trea... | yes |  |
| #1579 | `pkg/permissions/policy.go:365` | Path semantics now apply to every non-bash tool, not just file tools. For a non-bash tool the whole key becomes the path token whenever the prefix contains `... | yes |  |
| #1579 | `pkg/permissions/policy.go:405` | Path-prefix rules that start with `~/` are checked against the wrong directory. `resolvedPathStaysInside` joins the relative-looking `~/bin` onto baseDir bef... | yes | yes |
| #1580 | `pkg/agent/background/manager.go:1054` | reclaimTerminalAlert drains and refills the shared alerts channel without excluding concurrent producers or the drainer, so it can lose, reorder or delay oth... | yes | yes |
| #1580 | `pkg/agent/background/manager.go:1063` | Reclaiming can delete the final report of an earlier subagent that had the same name. reclaimTerminalAlert matches on `a.From == name` only, and names can be... | yes |  |
| #1581 | `pkg/tools/glob.go:51` | `glob`, `grep` and `read_many_files` now silently skip every directory named `.cache` at any depth, including project content that has nothing to do with the... | yes | yes |
| #1581 | `docs/site/src/content/docs/embed/api.md:215` | The published docs still list the skip set as `.git`, `.svn`, `.hg`, `node_modules`, `vendor`, but `.cache` is now skipped as well. The same list is in docs/... | yes | yes |
| #1614 | `pkg/agent/run_facade.go:339` | mergeFacadeModelConfig does not copy ModelConfig.ThinkingLevel, so a thinking level passed in the library facade's Config override is silently dropped. |  | yes |

## False alarms: claude-code

- #1429 `pkg/eventlog/guardrail.go:618` This PR adds a new durable table, a 419-line public store, an in-transaction write hook and a legacy backfill migration, but there is no design doc for it. F...
- #1454 `pkg/config/workload.go:695` A workload with `hitl.on_mutation: dry_run` imports into a recipe where no tool runs at all, reads included. It maps to `permissions.mode: "plan"`, and `perm...
- #1463 `cmd/core-agent/schedule.go:86` When `schedule` is configured, a passive replica never auto-continues its own interrupted startup session, which is a regression from the pre-PR behavior. Th...
- #1512 `pkg/permissions/gate.go:1132` In 'plan' permission mode, vcs_capabilities, vcs_proposal_view and vcs_proposal_list are refused, even though they are read-only. CheckPolicyDeny returns an ...
- #1577 `examples/gke-platform-agent/deploy/base/10-serviceaccount-daemon.yaml:100` The new opt-in instructions say to bind a Kubernetes Role to `serviceAccount:<PROJECT_ID>.svc.id.goog[gke-platform-agent/core-agent-daemon]`. That is IAM mem...
- #1577 `examples/gke-platform-agent/recipe_test.go:178` The new regression assertion only checks `.agents/config.json`, through `loadConfig(agentsDir)`. The PR also sets `web_search: false` in `config.hub.json`, `...
- #1580 `pkg/agent/background/tools.go:244` The reclaim in abandonWait only runs when Done has already fired. If claimed=false and the timeout or cancellation arrives while the completion goroutine sit...
- #1581 `pkg/tools/vcs/vcs.go:177` Re-running `vcs_clone` no longer gives a fresh copy from the forge, because worktrees now read historical objects from a shared, persistent bare store. `vcs_...
- #1609 `pkg/agent/quirks.go:279` `observedToolset` forwards only Name/Tools/UnwrapToolset, so it hides any toolset-level `ProcessRequest`. ADK's `toolsetPreprocess` only calls that method wh...
- #1611 `cmd/core-agent/subagents.go:188` The multi-session ad-hoc fix depends on each session tool being the exact same instance as the daemon's tool. If a SessionCustomizer wraps tools (instrumenta...
