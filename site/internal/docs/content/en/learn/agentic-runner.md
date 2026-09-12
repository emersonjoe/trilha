---
title: Agentic development — the runner
description: Let an agent execute a task of the agenda in its own worktree, verified, with evidence, using trilha-runner.
---

The [protocol](/learn/agentic-protocol) says what to do. **trilha-runner** is how it runs on
a machine you control:

```text
Task → Agent → Worktree → Execution → Verification → Evidence
```

It takes the task that is ready, checks out a branch for it, hands the agent the context pack,
commits what the agent changed, runs the checks *in the worktree*, records every result as
evidence and moves the task to `review` — or to `failed`. Your working copy is never touched.

## Install and dry-run

```bash
go install github.com/emersonjoe/trilha-runner/cmd/trilha-runner@latest
cd agenda
trilha runner drivers
# ai
# echo
# exec
```

Before spending a token, watch the pipeline with the driver that has no model. `echo` only
writes the first line of the prompt to `TRILHA_RUN.md`; everything else — branch, commit,
checks, evidence, status — is the real thing:

```bash
trilha spec task add "Smoke" --status ready --accept "the runner works" \
  --check "sh -c \"test -f TRILHA_RUN.md\""
trilha runner run TASK-004 --driver echo
# · worktree .trilha/runs/TASK-004/wt on trilha/task-004
# · driver echo starting
# · committed 3f9c1a2b7e01
# · TASK-004 is now review (3 checks, passed=true)
# TASK-004 → review (driver echo, 210ms)
# branch trilha/task-004 in .trilha/runs/TASK-004/wt
# ✓ sh -c "test -f TRILHA_RUN.md" (exit 0)
# ✓ go vet ./... (exit 0)
# ✓ go test ./... (exit 0)
```

Look at what it left behind:

```bash
git branch                       # trilha/task-004
ls                               # no TRILHA_RUN.md here: the work is on the branch
trilha runner worktree list
trilha spec evidence TASK-004    # three checks and one `run` record
```

The `run` record carries the driver, the branch, the commit, the diff stat and the tail of
the agent's output, with a pointer to `.trilha/runs/TASK-004/agent.log`. A run that fails to
even start — no command, agent exited non-zero — leaves the same kind of record, with the
stage that failed, and the task goes to `failed`. There is no execution without a trace.

```bash
trilha spec task move TASK-004 done      # or: ready, to run it again on the same branch
trilha runner worktree clean TASK-004    # drops the checkout, keeps the branch
```

## A real agent: the exec driver

The agent is whatever command reads a prompt on stdin and works in the current directory.
Its manifest, `.trilha/agents/coder.md`, says how to start it and what it may do:

```markdown
---
name: coder
role: Implements a task inside its own worktree and produces evidence.
driver: exec
command: claude -p -
tools:
  - read
  - write
  - run
constraints:
  - Stay inside the worktree of the task.
  - Run the checks listed in the task before reporting.
---
```

`codex exec -`, `aider --message-file -` or a script of your own work the same way. Now the
second task of the reminders spec, the one that adds `GET /api/events/due`:

```bash
trilha spec task next            # TASK-002  GET /api/events/due
trilha runner next               # runs it with the manifest's command
```

The agent receives the context pack — project, constitution, the specification, the task,
what TASK-001 already did — on stdin, works in `.trilha/runs/TASK-002/wt`, and when it
exits the runner commits, runs `go test ./...` and `trilha openapi --check` in that worktree,
and records the result. `TRILHA_TASK` and `TRILHA_WORKTREE` are in the agent's environment.

Review it like any branch:

```bash
git diff main..trilha/task-002
trilha spec evidence TASK-002
trilha spec task move TASK-002 done      # approve
git merge trilha/task-002                # or open a pull request from the branch
```

## The ai driver: a model with fenced tools

Without a command-line agent, the `ai` driver runs the agent loop of this framework's
[`ai` package](/learn/ai-and-agents) on any model that speaks the OpenAI protocol — Ollama
on your machine included:

```bash
export OPENAI_BASE_URL=http://localhost:11434/v1 OPENAI_API_KEY=ollama TRILHA_AI_MODEL=qwen2.5-coder
trilha runner run TASK-002 --driver ai
```

The model gets `read_file` and `list_files` always, `write_file` only when the manifest lists
`write`, `run` only when it lists `run` — every one of them refuses a path outside the
worktree — and, when `trilha-spec` is on the `PATH`, the protocol's read-only MCP tools, so it
can look at the task's dependencies and evidence by itself. The fence is the same whatever
the driver: the worktree, the manifest's tools, and the checks.

## What the runner does not do

It does not push, open pull requests, run several tasks at once or put the agent in a
container. The first two are the next spec of the runner; the last two are the
[control plane](/learn/agentic-cloud). The seam is already there: `sandbox.Sandbox` prepares
an environment for a job, and the local runner's only sandbox is the worktree.

## Challenge

Give the agenda a `reviewer` agent that may only read, and use the manifest's constraints so
that a coder who forgets to run the tests is caught by the checks, not by a person. Then break
a test on purpose and run the task: where does it stop, and what does the evidence say?

:::solution
`.trilha/agents/reviewer.md` already exists from `init` with `tools: [read]`; a coder cannot
skip the checks because the runner runs them, not the agent — that is the point of `checks:`
living in the task rather than in the prompt. To see it:

```bash
# make a test fail in the worktree the agent will start from
sed -i 's/want := 3/want := 4/' internal/events/events_test.go
trilha spec task move TASK-002 ready
trilha runner run TASK-002 --driver echo
# ✗ go test ./... (exit 1)
# TASK-002 is now failed
trilha spec evidence TASK-002 --json | jq '.[-2].output' | head
```
The agent's step "succeeded" (echo always does); the *verification* failed, in the worktree,
and the record holds the test's output. The task is `failed`, the branch keeps the attempt,
and `task move TASK-002 ready` queues the next one. Fix the test and run again: the runner
reuses the same worktree and branch.
:::
