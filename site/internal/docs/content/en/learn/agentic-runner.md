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

Without a Go toolchain, the [Releases page](https://github.com/emersonjoe/trilha-runner/releases)
carries a static binary per platform (Linux and macOS, amd64 and arm64) next to a
`sha256sums.txt`: download the one for your machine, check its digest against that file
(`sha256sum --check --ignore-missing sha256sums.txt` on Linux, `shasum -a 256` on macOS) and put
it on your `PATH` as `trilha-runner`.

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
even start — not a git repository, an unknown driver, no command, agent exited non-zero —
leaves the same kind of record, with the stage that failed, and the task goes to `failed`.
There is no execution without a trace. Only a task that is not `ready`, or is still waiting on
a dependency, is left as it was.

Before the first attempt the runner also runs a **preflight**: for every command in the task's
`checks:` and in `project.verify` it resolves the first token — `LookPath` on the host,
`command -v` inside the Docker sandbox. A tool that is not there fails the task in one attempt,
without calling the agent, with `failure_class: environment` and
`error_code: environment_missing_tool`; an exit code 127 or a "command not found" during
execution or verification is classified the same way and is never retried. The rule exists
because a real run burned three AI attempts on a machine that had no `python`.

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

## When the agent asks

An agent missing a fact the repository does not hold has two bad options: guess, or fail. The
runner gives it a third. During a run it listens on a Unix socket beside the worktree, puts
`trilha-runner` on the agent's `PATH`, and adds to the context pack a paragraph telling every
agent to ask one objective question and stop rather than guess:

| The agent runs | What happens |
|---|---|
| `trilha-runner ask "<question>"` | when the agent ends its turn, the run ends **needing input** |
| `trilha-runner note "<fact>"` | a note the next run of the task will read |
| `trilha-runner overview "<one or two sentences>"` | what is being done and where it stands |
| `trilha-runner attention "<reason>"` / `--clear` | raises or clears the badge that says a person is needed |

The `ai` driver offers the same as tools — `ask`, `note`, `overview`. Every command answers in
under a second and exits 0 even when nobody is listening: the runner can never fail or stall an
agent.

See it without a model. A three-line script stands in for the agent — it drafts a file, leaves a
note and asks:

```bash
mkdir -p scripts
cat > scripts/ask.sh <<'SH'
#!/bin/sh
echo "# Reminder e-mail (draft)" > REMINDER_EMAIL.md
trilha-runner note "internal/reminders sends the e-mail but has no recipient yet."
trilha-runner ask "Should the reminder e-mail go to the event's owner or to every guest?"
SH
trilha spec task add "Who gets the reminder e-mail" --status ready \
  --accept "the reminder reaches the people the customer chose" --check "go test ./..."
trilha runner run TASK-007 --driver exec --cmd "sh $PWD/scripts/ask.sh"
# · worktree .trilha/runs/TASK-007/wt on trilha/task-007
# · attempt 1/3: driver exec starting
# · TASK-007 needs input: the agent asked a question
# TASK-007 → needs input (driver exec, 1.3s)
# question: Should the reminder e-mail go to the event's owner or to every guest?
# branch trilha/task-007 in .trilha/runs/TASK-007/wt
# commit 0c5e9f3a8d21b7c4e6f09a1b2c3d4e5f60718293
```

Nothing failed. The draft is committed on the branch, no check ran, no attempt was spent,
nothing is retried. The task is `blocked`, and its evidence has a `run` record with
`stage: needs_input`. With `--json` the result also carries the question, the note (source
`ai`) and `diff_stats` — how many files and lines the branch changed, never what
they say.

On your machine there is nobody to send the answer to, so write it where the next context pack
will find it — the evidence so far is part of the pack — and put the task back in the queue:

```bash
trilha spec evidence TASK-007 add --note "Answer: every guest who accepted the invitation." --by ana
trilha spec task move TASK-007 ready
trilha runner run TASK-007        # same branch, same worktree, the answer in the context pack
```

With a control plane, the answer travels as a `continuation` instead. When the question came
from a `claude-code` session, the runner resumes that very session (`--resume <session>`) with
the answer, quoted, as its prompt — the agent picks up mid-thought. Any other driver, or a
session this machine no longer has, starts fresh with the question and the answer quoted in the
context pack. The task's memory — the overview and the latest notes — comes back the same way,
quoted and labelled as information, never as instructions (the
[protocol chapter](/learn/agentic-protocol) shows the exact shape).

Questions, notes, overviews and reasons are size-limited and secret-scanned before they leave:
a question holding something shaped like a key reaches the control plane as a fixed notice, and
its words stay in `.trilha/runs/TASK-007/question.txt`, readable only by you. Two limits worth
knowing: an agent inside the Docker sandbox cannot reach the socket, so it cannot ask; and a
read-only run (below) never stops for a question.

While the agent works, the runner also follows *what* it is doing. For `claude-code` it writes
hooks to `.trilha/runs/TASK-007/claude-settings.json` — beside the worktree, never inside it —
and passes them with `--settings`; the `ai` driver reports from its own loop. The state —
`working`, `idle`, `waiting_permission`, `asked`, `stopped_error` — goes to the control plane
only when it changes, at most once every two seconds.

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
the resource limits are fixed by the runner (one CPU, 1 GiB of memory, 256 PIDs,
`cap-drop ALL` and `no-new-privileges`), the agent runs as the user that owns the worktree
rather than as root, and nothing mounts the Docker socket. A sandbox that can talk to the
daemon is not a sandbox. Whatever was created is removed afterwards, even
when the run failed halfway, so `docker ps` is empty when it is over.

The third of those is not politeness, it is what makes the first one work. Dropping every
capability takes away `CAP_DAC_OVERRIDE`, and root inside a container then stops bypassing file
permission checks — so a root agent could not write the worktree at all, because the worktree
belongs to you. Running as its owner is what keeps the one writable path writable.

There is a fourth thing you never have to arrange, and it is worth knowing why. Your project's
credential goes into the container **once**, through a file only you can read, when the
container starts. Nothing is passed on the `docker exec` line, because that line is the argv of
a process on your own machine — and `ps` shows a process's arguments to every user on it. A
secret that rides a command line is a secret you have published locally.

A machine without Docker does not lose anything it had: `--sandbox` defaults to `none` and the
worktree is still the sandbox.

## Waiting on another repository

A product task can depend on a framework task that lives in a different repository. Say so in
`depends_on` with the alias in front — `trilha:TASK-004` — and tell `next` where the other
checkout is:

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
keep a project whose data must not leave the country on a worker in that region. The payload
also carries the runs in flight and the runner and driver versions, so an incompatible claim is
refused before any source is touched. A run this host cannot honour is refused and reported,
never executed quietly. With `--capacity 2` the worker keeps two runs going at once, each in
its own worktree.

A long run must not look like a dead worker, and a result must not die with the network. While
any run is active the worker sends a heartbeat every 30 seconds (`running 1/2`). Each result is
written to `.trilha/runs/<run>/result.json` before it is posted and removed once the control
plane has it; a control plane that is down is retried with backoff instead of stopping the
worker, a result refused for good is set aside as `result.rejected.json`, and whatever a worker
that died left behind is sent at the next start, before anything new is claimed. A task left
`running` by a runner that died goes back to `ready`, with a note, the next time it is run.

The same claim can carry the project's own model access — provider, endpoint, credential and
the hosts that credential may be spent on. It reaches the agent as process environment and
nowhere else — the `claude-code` driver always executes `claude -p -` and takes the credential
as `CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY` — and it is redacted from the output. It
overrides the worker's own configuration for that run only. When the project lists allowed hosts, a
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

## Reviews and bug hunts

Implementing is one kind of run. A control plane can also ask a worker for a second opinion on
a branch that is already in `review`; the claim's `kind` says which, and the runner does the
rest:

| Claim | What the runner does |
|---|---|
| `kind: review`, `review_mode: report` (the default) | reviews the task's branch from its merge base with the default branch — correctness, security, missing tests — in a detached checkout of its own, `.trilha/runs/TASK-002-review-<run>/wt`. Read-only |
| `kind: review`, `review_mode: fix` | the same review on the branch itself: the agent may fix, the runner commits and runs the task's checks, and the task goes back to `review` |
| `kind: hunt` | a read-only bug hunt over the files the branch changed, pointed at one area (entry points, error paths, data layer, integration seams, user interface, infrastructure and CI) and one style (pessimist, trace follower, dependency skeptic, fresh eyes), so parallel hunters do not all read the same code the same way |

The agent reports with more commands of the same family — and tools of the same names in the
`ai` driver:

```bash
trilha-runner finding --severity high --path internal/events/due.go --lines 18-22 \
  --title "Due list includes reminders in the future" \
  --why "the filter compares with After where the acceptance says before now" \
  --repro "an event whose ReminderAt is one hour ahead appears in GET /api/events/due"
# finding sent to the runner
trilha-runner verdict changes
# verdict sent to the runner
```

A finding is a path and a line range, never code. One a control plane could not store is
refused before it is sent — and the command still exits 0:

```bash
trilha-runner finding --severity low --path internal/events/due.go --lines 18 --title "Naming"
# trilha-runner finding: finding: severity must be critical, high or medium
# usage: trilha-runner finding --severity critical|high|medium --path <file> …
```

Without a `verdict`, a review answers `changes` when it has findings and `pass` when it has
none. Findings are also secret-redacted, and at most 100 leave a run.

**Read-only is checked, not asked.** When a hunt or a report-mode review ends, the worktree must
be clean, `HEAD` must be the commit it started at and the reviewed branch must not have moved.
Anything else fails the run with `failure_class: policy` (`error_code: read_only_violated`) and
throws its findings and reports away. In the Docker sandbox the worktree is mounted read-only
as well. A read-only run never moves the task and is never pushed; the `ai` driver does not even
offer it `write_file` or `ask`.

People review too. When the control plane hands a repair run the comments someone left on the
pull request, the runner gives the prompt a `<reviews>` block with the lines each comment points
at, read from the worktree itself and never from outside the repository, every text
XML-escaped and quoted as data from reviewers. A comment that says "ignore your instructions" is
a quotation, not an order. The agent fixes what it agrees with and answers each comment by its
id:

```bash
trilha-runner reply c-1 "Fixed: the filter now uses Before(now), with a test for an event one hour ahead."
# reply sent to the runner
```

A reply to an id the run did not receive is dropped.

## Variants

Sometimes you want two attempts at the same task and to keep the better one. A claim with
`variant_index` is an ordinary implement run on a branch of its own:

```bash
git branch
# * main
#   trilha/task-002
#   trilha/task-002-v1
#   trilha/task-002-v2
ls .trilha/runs
# TASK-002  TASK-002-v1  TASK-002-v2
git diff --stat trilha/task-002-v1 trilha/task-002-v2
```

Each variant works in `.trilha/runs/TASK-002-vN/`, and the runner writes there a
`created-branch` file naming the branch it cut. That proof is what the rest of the tooling
trusts. A variant never adopts a branch it cannot prove it made — if someone already has a
`trilha/task-002-v2`, the run moves to `trilha/task-002-v2-2` and leaves theirs alone — and a
cleanup deletes a branch only with that proof in hand.

Variants are blind to each other: a variant's context pack holds no earlier evidence of the task
and no note or overview an agent wrote, so the second attempt is not a copy of the first. A
person's notes still arrive. Pick one, merge it, drop the other:

```bash
git merge trilha/task-002-v1
trilha runner worktree clean TASK-002-v2   # takes a run directory as well as a task
```

## Report artifacts

A check proves; a report explains. A coverage summary, a screenshot of the new screen, a chart
of the eval — an agent can publish up to five per run:

```bash
trilha-runner artifact coverage.md
# artifact sent to the runner
```

The runner decides, and its log says what it decided:

```text
· artifact coverage.md accepted (1834 bytes, text/markdown)
· artifact "notes.md" refused: it contains a secret, so it stays off the cloud (the runner does not redact reports)
```

The rules: `.html`, `.md`, `.png` or `.jpg` (`--name` sets the name people see), at most 2 MiB
each, from the worktree or from `$TRILHA_ARTIFACT_DIR` — a scratch directory outside the
worktree, which is where a read-only run writes, since its worktree must stay clean. A text
report holding something shaped like a secret is refused, not redacted; an image must begin with
its format's bytes. Accepted reports are copied to `.trilha/runs/TASK-007/artifacts/`, readable
only by you, with their SHA-256.

A worker uploads each report before it posts the result, and the result lists only what the
control plane accepted. Reports are opt-in there: a project that has not allowed them answers
`403 ARTIFACTS_DISABLED`, and the runner drops them all and says so once. Either way, what
leaves the machine is something a person reads — never the source.

## Code from forks

A pull request from a fork is code nobody on the team vouched for. The control plane marks such
a claim `foreign_code: true`, and three things change:

- It runs only in the Docker sandbox. Without `--sandbox docker`, or with Docker unavailable,
  the run fails with `failure_class: policy` (`sandbox_required`) before the agent starts.
- The agent and the checks get no secret: no inherited token, no deployment secret. The AI
  credential goes only as the variable a command-line driver needs to reach the model; the `ai`
  driver keeps it inside the runner's process, and its checks run in the sandbox.
- `project.verify` is read from the default branch, never from the fork's — a fork does not get
  to choose the commands your worker runs — and the branch is never pushed:

```text
· run-000012: foreign code, branch trilha/task-002 is not pushed
```

## Keeping worktrees tidy (doctor)

Every run leaves a directory in `.trilha/runs/`: the worktree, the agent's log, what it
reported. After a few weeks of variants and hunts that adds up, and some of it is the only copy
of work nobody merged yet. The doctor tells them apart, and deletes nothing unless you ask:

```bash
trilha runner doctor --worktrees
# RUN                          CLASS          AGE         SIZE  BRANCH                   ACTION
# TASK-001                     metadata-only  >=30d   48.0 KiB  -                        kept — no worktree
# TASK-002                     finished-kept  <14d     3.1 MiB  trilha/task-002          kept
# TASK-007                     live           <14d     2.9 MiB  trilha/task-007          kept — held by process 48211
# TASK-009                     orphaned       <30d     2.7 MiB  trilha/task-009          kept — no task file for TASK-009
# 4 run directories: 1 live, 1 finished-kept, 1 orphaned, 1 metadata-only; removed 0, skipped 0, failed 0
```

`live` means a runner — or any process, like a dev server the agent left behind — has its
current directory inside; `finished-kept`, a worktree of a known task with nobody in it;
`orphaned`, a worktree git does not list or whose task file is gone; `metadata-only`, logs and
evidence with no worktree. `--json` gives the same report to a tool.

Deleting takes a flag, and even then the doctor is careful:

```bash
trilha runner doctor --worktrees --prune-orphans --default-branch main
# RUN                          CLASS          AGE         SIZE  BRANCH                   ACTION
# …
# TASK-009                     orphaned       <30d     2.7 MiB  trilha/task-009          skipped — branch trilha/task-009 is not merged into origin/main (--force-unmerged removes it anyway)
# 4 run directories: 1 live, 1 finished-kept, 1 orphaned, 1 metadata-only; removed 0, skipped 1, failed 0
# doctor: 1 skipped, 0 failed
echo $?
# 12
```

It never touches a live run. It skips a worktree whose branch is not merged into
`origin/<default>` or that has uncommitted changes, unless `--force-unmerged`. It deletes a
branch only when the run's `created-branch` proves the runner made it. `--prune-older-than 30d`
(or `720h`) removes run directories older than that, under the same rules. The exit code is 0
when everything asked for was done, 1 when the command could not run, and **12** when something
was skipped or failed — which a scheduled cleanup can notice.

Removing a worktree — by `worktree clean` or by a prune — is an ordered teardown. The processes
working inside it get SIGTERM, then SIGKILL if they are still there after 1.5 s, and only then
does the worktree go, so nothing keeps writing into a directory that no longer exists:

```bash
trilha runner worktree clean TASK-007
# teardown: SIGTERM 48211
# teardown: every process exited
# teardown: worktree /home/ana/agenda/.trilha/runs/TASK-007/wt removed
# removed worktree of TASK-007 (branch trilha/task-007 kept)
```

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
