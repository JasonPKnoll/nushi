# 0127. Claude's workflows show in the chat

Status: accepted (the owner, 2026-11-16: "Yeah start on the workflow").

## Context
Claude Code's Workflow tool launches a script that runs many subagents in phases, in the background. The call answers at once, so the turn ends and Nushi showed the chat idle while dozens of agents worked; nothing showed the run, and the owner had to ask Claude what was running. Claude sends almost nothing of it over ACP: claude-acp-go drops the run's task frames, its agents write nothing to stdout, and the end comes only as a turn of Claude's own after a `<task-notification>` (`docs/facts/claude.md`, the `workflow` and `workflow-stop` parity scenarios). Claude's own files say it all: a journal as agents start and end, each agent's transcript, and a snapshot written as the run ends. Their format is undocumented.

## Decision
- **The run is followed from Claude's files**, which the daemon already reads for history: the call's `toolResponse` (or, replayed, its text) names the run and task; `claudehistory` reads `subagents/workflows/<run>/journal.jsonl` incrementally and `workflows/<run>.json` once there, inside the session's folder, capped and tolerant (unknown lines skipped). Nothing new is asked of Claude, and no credentials are read. The reader is Nushi's, not claude-acp-go's, so Nushi needs no library release for it.
- **A small summary rides on the call's item** (`model.Workflow`, about 2 KB at most: name, state, phases with counts, up to 8 running labels, times); the agent list and an agent's steps are fetched on demand (`GetWorkflow`, `GetWorkflowAgent`, Mac only). Nothing per agent is kept.
- **How a run ends**, first match wins: Claude's end for it, its `<task-notification>` naming the run's task in the session's transcript (the queue's enqueue line, or the user line of Claude's own turn: completed is done, failed failed, killed/stopped/cancelled stopped; another status is no end), with the snapshot (checked against the task id) for the detail and as the end when no notification comes (a run stopped by `TaskStop` gets none). Claude sends the notification without waiting for the snapshot's write, so an end the notification told waits 3 s for it; and a snapshot alone ends a run too (anything but those words "ended" in Claude's word). Then the claude that ran it gone (a run lives in its process): stopped, after a 2 s grace and a last read; after a reopen, with no claude of this daemon's running it, stopped once its files are still for 2 minutes. Every agent having a result is no end (it is true between phases). Claude's `session_state_changed: idle`, which comes only once a run ended, is on claude's stdout, which claude-acp-go doesn't pass on, so it isn't used.
- **A run that writes nothing for an hour, its claude alive, stalls**: shown so, no longer the chat's work (auto-sleep, a restart and the working dot no longer wait on it), not ended; a write clears it. No spinner forever, and no false verdict.
- **A running workflow is the chat's work**: it counts in `ChatStatus.background` and in `workflows`, the app shows the chat working (dot, status bar, phone), and auto-sleep, restarts and the other quiet checks wait for it. A turn of Claude's own (after a background task) counts too (`own_turn`). The composer still goes by `busy`: Claude takes prompts while a run goes.
- **Stop** asks Claude to stop the run by its task id (`TaskStop`, which takes it, as recorded).
- **The app**: a card where the call is (name, description, phase progress, who runs, time; done, failed or stopped at the end), a Workflow pane like the subagent pane (the phase tree, and an agent's steps). The phone shows the card's progress line and the working state, no pane.
- **Other agents**: only Claude has workflows; `model.Workflow` and `chats.WorkflowReader` name nothing of Claude's, so another harness could feed them.

## Consequences
- Long runs hold auto-sleep and an update's restart, as subagents do, until they end or stall.
- An undocumented format can change under Nushi: a call whose files never appear (30 s) shows as the plain call it was; a read that fails keeps the last state.
- Background Agent-tool subagents are unchanged: they count while the daemon saw them start, and read done after a reopen.
- A workflow agent's steps are read for the pane by a plain reading of its transcript in Nushi, not the library's replay.

## Alternatives considered
- Claude's task frames over ACP (`task_progress` carries the progress rows): claude-acp-go drops them, and a change there needs a release; files also work for a run Nushi didn't see start.
- Every agent on the item: up to a thousand rows rewritten each second into Nushi's transcript copy.
- "Every started agent has a result" as the end: wrong between phases.
- A fixed timeout as an end: a long run would be called over while it works.
