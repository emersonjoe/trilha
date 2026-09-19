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
# claude-code
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

## A check that measures: metrics as evidence

Some acceptance is not a yes or a no. Triage accuracy on a labelled set, a translation score, a
p95 latency, accessibility violations — each is a number against a threshold, and a number
buried inside a command's output is invisible to whoever reviews the task. So a harness prints
one line of JSON per number and the runner turns each into an `eval` record:

```bash
cat > eval/triage.sh <<'SH'
#!/bin/sh
python eval/triage.py            # whatever measures it
echo '{"metric":"triage_top1","value":0.87,"threshold":0.85,"comparator":">=","dataset":{"id":"triage-v3","manifest":"eval/golden/manifest.json"}}'
SH
trilha spec task add "Triage stays above 0.85" --status ready \
  --accept "top-1 accuracy is at least 0.85" --check "sh eval/triage.sh"
trilha runner run TASK-005 --driver echo
# · eval triage_top1=0.87 >= 0.85 (passed=true)
# TASK-005 → review (driver echo, 1.2s)
```

The comparators are `>=`, `<=`, `>`, `<`, `==` and `!=`. **A metric that misses its threshold
fails the task even when the script exits 0** — the exit code says the harness ran, the metric
says whether the result is good enough. The optional `dataset` names a manifest; the runner
hashes the manifest, never the data, so a golden set that is large or private still gets
pinned. Any other line the script prints is ordinary output.

## The sandbox: when the checks need services

The worktree fences what the agent may *touch*. It does not provide what the checks *need*: an
API suite that wants Postgres fails in it, and correctly so. The agent manifest declares what
to bring up, and `--sandbox docker` brings it:

```markdown
---
name: coder
role: Implements a task inside its own worktree and produces evidence.
driver: exec
command: claude -p -
tools: [read, write, run]
sandbox: {"image":"golang:1.22","services":[{"name":"postgres","image":"pgvector/pgvector:pg16","env":{"POSTGRES_PASSWORD":"trilha","POSTGRES_DB":"acervo"},"ready":["pg_isready","-U","postgres"]}]}
---
```

```bash
trilha runner run TASK-006 --sandbox docker
# · sandbox: service postgres started
# · sandbox: service postgres ready
# · sandbox: trilha-task-006 running golang:1.22
# · sandbox docker: commands run in /workspace
```

The services come up on a network of their own and answer by name — the check connects to
`postgres`, not to a port on your machine. The agent and the checks run in a container on that
network, and the evidence records the wrapped command, so the record says where it ran.

Four things are the runner's and not the manifest's, on purpose: the worktree is the only
writable path that survives (the root filesystem is read-only, `/tmp` dies with the container),
the resource limits are fixed by the runner, the agent runs as the user that owns the worktree
rather than as root, and nothing mounts the Docker socket. A sandbox that can talk to the
daemon is not a sandbox. Whatever was created is removed afterwards, even
when the run failed halfway, so `docker ps` is empty when it is over.

A machine without Docker does not lose anything it had: `--sandbox` defaults to `none` and the
worktree is still the sandbox.

## Waiting on another repository

A product task can depend on a framework task that lives in a different repository. Say so with
`depends_on_remote`, and tell `next` where the other checkout is:

```bash
trilha runner next --repo trilha=../trilha
# · TASK-005: waiting:trilha:TASK-004 (running)
# error: runner: no task is ready with every dependency done
```

The task is not offered, it is reported as `blocked` with the reason as evidence, and the queue
puts it back to `ready` by itself once the other repository's task is `done`. A dependency the
runner cannot resolve at all blocks too — it never reads what it cannot see as done. A
Cloud-connected worker resolves the same dependency against the control plane instead of a path.

## Persistent worker and delivery

A Cloud-connected worker can keep dedicated checkouts under `--workspace-root`, materialize
versioned Trilha Spec bundles and publish specification and implementation branches with
`--push`.

A fleet is rarely one kind of machine, so a worker says what it is:

```bash
trilha runner worker --cloud https://cloud.example --token "$TOKEN" --project acervo \
  --label docker --label region:br --capacity 2
```

The labels and the capacity ride both the heartbeat and the claim, with the runs in flight, so
the control plane can send a run whose checks need Postgres to the host that has Docker, and
keep a project whose data must not leave the country on a worker in that region. A run this
host cannot honour is refused and reported, never executed quietly. With `--capacity 2` the
worker keeps two runs going at once, each in its own worktree.

The same claim can carry the project's own model access — provider, endpoint, credential and
the hosts that credential may be spent on. It reaches the agent as process environment and
nowhere else, and it is redacted from the output. When the project lists allowed hosts, a
model endpoint outside them is refused *before the first request*: the evidence is a `run`
record with `stage: policy` and the task goes to `failed`. Residency is enforced by the runner,
not by trusting each worker's configuration.

Deploy and rollback use `--delivery-config`: only commands configured locally may run; Cloud
never supplies a shell line. A profile that is a compose stack with a database declares its own
sequence — `steps[]` in order, a `migrate` that runs immediately before the step marked
`switch`, and `health[]` per service — so a migration that fails aborts the delivery with the
previous revision still serving, and a service that never becomes healthy triggers the
rollback. The rollback reverses the schema only when the migration declared itself reversible;
otherwise it is image-only and the log says the schema keeps the new shape.

The worker advertises its execution envelope on every heartbeat and claim. Labels are repeatable
and capacity defaults to one:

```bash
trilha runner worker --cloud https://cloud.example --token "$TOKEN" --project my-app \
  --label docker --label region:br --capacity 2
```

At capacity two, two runs execute concurrently in separate worktrees. The payload also includes
the active-run count and runner/driver versions, allowing the control plane to reject incompatible
claims before source code is touched.

## Project-scoped AI and residency

A claimed run may carry a transient AI configuration (`provider`, `base_url`, `model`,
`credential`, `allowed_hosts`). It overrides the worker's global environment for that run only.
The `claude-code` preset always executes `claude -p -`; credentials are injected through
`CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY` and redacted from captured output. The `ai`
driver refuses a base URL whose hostname is outside `allowed_hosts`, records a `run` evidence
entry with `stage: policy`, and moves the task to `failed` without retrying.

## Metric gates

A check can print one JSON object per metric:

```json
{"metric":"triage_top1","value":0.87,"threshold":0.85,"comparator":">=","dataset":{"id":"triage-v3","manifest":"eval/golden/manifest.json"}}
```

The runner keeps the ordinary `check` evidence and adds an `eval` record. A failed comparator
fails verification even when the process exits zero. Only the named dataset manifest is hashed;
the underlying dataset is never copied into evidence.

## Cross-repository dependencies

Use `depends_on: trilha:TASK-007` for a dependency owned by another repository and map its alias
when selecting local work:

```bash
trilha runner next --repo trilha=../trilha --repo cloud=../trilha-cloud
```

Missing or incomplete dependencies are reported as `waiting:<alias>:TASK-NNN`. Remote workers use
the control-plane task-status endpoint and apply the same rule.

## Compose delivery and Docker sandbox

Delivery profiles can declare ordered `steps`, a `migrate` gate before the `switch` step, several
`health` endpoints and a fixed rollback. Health failure triggers rollback; a non-reversible
migration is logged as image-only rollback.

For checks that need services, an agent manifest can select Docker:

```yaml
sandbox:
  image: ghcr.io/acme/app-ci:latest
  services: '[{"name":"db","image":"pgvector/pgvector:pg16","env":{"POSTGRES_PASSWORD":"test"},"ready":["pg_isready","-U","postgres"]}]'
```

The runner owns the limits: read-only roots, one CPU, 1 GiB memory, 256 PIDs and
`no-new-privileges`. Only the worktree is mounted writable. The Docker socket is never mounted,
and containers plus their private network are removed even after failure.

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
