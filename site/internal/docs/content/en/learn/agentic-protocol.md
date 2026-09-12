---
title: Agentic development — the protocol
description: Describe the agenda's next feature as tasks an agent can execute, with acceptance criteria, checks and evidence, using trilha-spec.
---

The [previous chapter](/learn/ai-and-agents) put agents *inside* the agenda: a chat, tools, an
MCP server. This one and the next two turn the question around: how does an agent work *on*
the agenda — pick a task, do it in its own branch, prove it, hand it to a reviewer?

Trilha answers with three tools, and this chapter is about the first:

| Tool | What it is | Where |
|---|---|---|
| **trilha-spec** | the open protocol: what to do, in files any agent reads | [github.com/emersonjoe/trilha-spec](https://github.com/emersonjoe/trilha-spec) |
| **trilha-runner** | how it runs on your machine: worktree, agent, checks, evidence | [github.com/emersonjoe/trilha-runner](https://github.com/emersonjoe/trilha-runner) |
| **trilha-cloud** | the control plane for a team and a fleet of workers | private |

The protocol depends on nothing — not even on this framework — so the `.trilha/` directory it
writes is readable by Claude Code, Codex, a script, or a person with an editor.

## Install and initialise

```bash
go install github.com/emersonjoe/trilha-spec/cmd/trilha-spec@latest
cd agenda
trilha-spec init --name agenda --description "Events agenda from the Learn trail"
```

From Trilha 0.124 on, `trilha spec …` is the same call as `trilha-spec …`: the framework's CLI
hands any command it does not know to `trilha-<name>` on your `PATH`, the way `git` does. Both
spellings appear below; use whichever you like.

```text
.trilha/
├── .gitignore        ignores runs/ and cache/; everything else is committed
├── project.md        what this project is, for an agent that just arrived
├── constitution.md   the rules every task obeys
├── specs/            what to build and why
├── tasks/            TASK-001.md … the executable units
├── agents/           coder.md, reviewer.md — who may execute, with what
├── context/          extra documents every agent receives
└── evidence/         the proof each task produced
```

:::note
`trilha dev` keeps its build cache in `.trilha/cache/`, which the framework ignores on its
own. If you initialised the protocol with a Trilha older than 0.124, run `trilha-spec doctor`
after the first `trilha dev`: it tells you when the cache's `.gitignore` is hiding the
protocol from git.
:::

Open `.trilha/project.md` and fill in the two things an agent cannot guess — how to run and
how to test:

```markdown
---
name: agenda
description: Events agenda from the Learn trail
default_agent: coder
verify:
  - go vet ./...
  - go test ./...
---

# agenda

A Trilha app: routes live in `app/`, `trilha gen` regenerates `trilha_gen.go`, `trilha check`
is the gate. Run with `trilha dev`.
```

`verify` is the list of commands every task runs on top of its own checks.

## A specification and its tasks

The feature: **reminders** — an event can carry a reminder, and the API lists the events
whose reminder is due.

```bash
trilha spec new "Event reminders"
# created 001-event-reminders (.trilha/specs/001-event-reminders.md)
```

Write the *why* and the *what* in that file; the tasks point at it. Then cut it into work an
agent can pick up on its own:

```bash
trilha spec task add "Reminder field on Event" --spec 001-event-reminders --status ready \
  --accept "Event has ReminderAt (time.Time) and the form accepts it" \
  --check "go test ./internal/events/..."

trilha spec task add "GET /api/events/due" --spec 001-event-reminders --status ready \
  --depends TASK-001 \
  --accept "the route answers the events whose ReminderAt is before now" \
  --accept "trilha openapi --check passes" \
  --check "go test ./..." --check "trilha openapi --check"
```

Each task is one Markdown file with a small front matter — readable, diffable, promptable:

```markdown
---
id: TASK-002
title: GET /api/events/due
status: ready
spec: 001-event-reminders
depends_on:
  - TASK-001
acceptance:
  - the route answers the events whose ReminderAt is before now
  - trilha openapi --check passes
checks:
  - go test ./...
  - trilha openapi --check
created: "2026-09-12T14:03:11Z"
---
```

`checks` are programs and arguments, never a shell — a check that needs a pipe says so with
`sh -c "…"`. That is what makes the evidence trustworthy: what ran is exactly what is written.

## The graph decides what runs

```bash
trilha spec task next
# TASK-001  Reminder field on Event
trilha spec task move TASK-002 running
# error: task TASK-002: cannot run, waiting on TASK-001
trilha spec task graph
```

`next` answers the tasks that are `ready` with every dependency `done`, in dependency order.
A task moves through a strict life — `idea → spec → ready → running → verify → review → done`,
with `blocked` and `failed` as the two ways out — and a transition that skips a step is
refused. The graph is Mermaid by default (`--dot` for Graphviz), so it drops into a README or
a pull request.

## What the agent receives

```bash
trilha spec context TASK-001
```

The *context pack* is one Markdown document, in the order a reader needs it: the project,
the constitution, the agent's own manifest (role, tools allowed, constraints), the
specification, the task with its acceptance criteria and checks, the dependencies and their
status, the evidence so far, and every file in `.trilha/context/`. `--json` is the same for a
tool. It ends by telling the agent not to mark the task done — that is the reviewer's decision.

Do the first task yourself now, by hand, the way the next chapter will let an agent do it:

```bash
trilha spec task move TASK-001 running
# … add ReminderAt to internal/events and to the form, with a test …
trilha spec task move TASK-001 verify
trilha spec verify TASK-001
# ✓ go test ./internal/events/... (exit 0)
# ✓ go vet ./... (exit 0)
# ✓ go test ./... (exit 0)
# evidence: 3 record(s) in .trilha/evidence/TASK-001
# TASK-001 is now review
```

## Evidence

Each check left a JSON record — the command, where it ran, the exit code, the output and its
SHA-256, who ran it and when:

```bash
trilha spec evidence TASK-001
# #1   ✓ check    trilha-spec verify   go test ./internal/events/... (exit 0)
# #2   ✓ check    trilha-spec verify   go vet ./... (exit 0)
# #3   ✓ check    trilha-spec verify   go test ./... (exit 0)
trilha spec evidence TASK-001 add --note "Reviewed the diff; the form validates the date." --by ana
trilha spec task move TASK-001 done
trilha spec task next
# TASK-002  GET /api/events/due
```

A record is never edited; a correction is a new record. That is the whole idea of the
protocol: a task is done when its acceptance criteria have evidence, not when a chat says so.

## The same thing over MCP

Everything above is available to any MCP host — Claude Code, Cursor, the `ai.Agent` of the
previous chapter — through a stdio server:

```json
{ "mcpServers": { "trilha": { "command": "trilha-spec", "args": ["mcp", "--write"] } } }
```

Read-only by default (`trilha_list_tasks`, `trilha_get_task`, `trilha_next`, `trilha_context`,
`trilha_graph`); `--write` adds `trilha_move`, `trilha_evidence` and `trilha_verify`. A tool
that is not offered cannot be called — the same posture as [`trilha mcp`](/reference/cli#trilha-mcp).

The full protocol — file formats, the transition table, the evidence schema — is in
[docs/protocol.md](https://github.com/emersonjoe/trilha-spec/blob/main/docs/protocol.md).

## Challenge

Add a third task, "Reminder e-mail", that depends on **both** TASK-001 and TASK-002, with a
check that cannot pass yet. Show that `task next` does not list it while TASK-002 is open,
move TASK-002 through to `done`, and then verify the new task and watch it land in `failed`
with the evidence that says why.

:::solution
```bash
trilha spec task add "Reminder e-mail" --spec 001-event-reminders --status ready \
  --depends TASK-001,TASK-002 \
  --accept "an e-mail goes out when a reminder is due" \
  --check "go test ./internal/reminders/..."
trilha spec task next            # only TASK-002: TASK-003 waits on it
trilha spec task move TASK-002 running
trilha spec task move TASK-002 verify
trilha spec verify TASK-002 && trilha spec task move TASK-002 done
trilha spec task next            # TASK-003
trilha spec task move TASK-003 running
trilha spec task move TASK-003 verify
trilha spec verify TASK-003
# ✗ go test ./internal/reminders/... (exit 1)
# TASK-003 is now failed
trilha spec evidence TASK-003    # the record carries the compiler's output and its hash
trilha spec task move TASK-003 ready   # failed → ready is the only way back
```
`failed` is not a dead end: the evidence says what to fix, and `ready` puts the task back in
the queue for the next attempt — by you or, in the next chapter, by an agent.
:::
