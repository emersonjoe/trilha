---
title: Agentic development — the control plane
description: Share the queue among a team and a fleet of workers with trilha-cloud, and what the contract looks like if you run your own.
---

One runner on one machine serves one person. A team with several machines, several projects
and agents working in parallel needs somewhere that answers *what is queued*, *who is
executing what*, *what evidence did that run leave* and *who asked for it* — without the
code of any project leaving the machine that holds it. That is **trilha-cloud**, and it is
private: the protocol and the runner are open so that any tool can speak them; the control
plane is where operating a fleet costs something.

```text
                    trilha-spec (protocol)
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
   trilha-runner      Claude Code      another runner
          │                │                │
          └────────────────┼────────────────┘
                           ▼
                     trilha-cloud
        projects · queue · fleet · evidence · audit
```

trilha-cloud is itself a Trilha application — file-based routes, `trilha gen`,
[`auth.APIKeys`](/learn/authentication), `c.Audit` — which makes this chapter also the largest
example of the framework in use.

## The flow

If you have access to the repository, one process is the whole control plane:

```bash
export TRILHA_SECRET=$(trilha secret)
export TRILHA_CLOUD_ADMIN_TOKEN=change-me
export TRILHA_CLOUD_DATA=./data/cloud.json
make dev                                  # http://localhost:3000
```

The operator registers a project and issues a key for the workers. Administration is behind
the token from the environment — the first key has to come from somewhere — and everything
else is behind keys with scopes:

```bash
curl -X POST localhost:3000/api/admin/projects -H "Authorization: Bearer change-me" \
     -d '{"name":"agenda","org":"learn","repo":"git@github.com:you/agenda"}'
curl -X POST localhost:3000/api/admin/keys -H "Authorization: Bearer change-me" \
     -d '{"name":"laptop","scopes":["runs:write"]}'
# {"key":"tc_…"}   shown once; the secret is peppered with TRILHA_SECRET and never stored
```

Anyone with a key queues a task — the same `TASK-002` from the previous chapters:

```bash
curl -X POST localhost:3000/api/runs -H "Authorization: Bearer tc_…" \
     -d '{"project":"agenda","task_id":"TASK-002"}'
# {"id":"run-000001","status":"queued",…}
```

And a worker — a `trilha-runner` on a machine that has a checkout of the agenda — takes it:

```bash
cd agenda
trilha runner worker --cloud http://localhost:3000 --token tc_… --project agenda --once
# worker laptop on http://localhost:3000, project agenda
# TASK-002 → review (driver exec, 48s)
# branch trilha/task-002 in .trilha/runs/TASK-002/wt
# ✓ go test ./... (exit 0)
```

The worker asked for the next run, executed it locally exactly as in the
[runner chapter](/learn/agentic-runner) and reported status, branch, commit and the evidence
records. Open `http://localhost:3000`: the dashboard shows the project, the run in `review`
with its branch, and the fleet — `laptop`, idle, seen seconds ago. The reviewer's approval is
`DELETE /api/runs/run-000001` (review → done): the same human decision the protocol keeps out
of the agent's hands.

Every step is in the audit trail, with the key that did it as the actor:

```bash
curl localhost:3000/api/admin/audit -H "Authorization: Bearer change-me" | jq '.[].action'
# "run.closed" "run.finished" "run.claimed" "run.enqueued" "apikey.emitiu" "project.created"
```

## The contract

The worker only needs three routes, so another control plane — yours — can implement them:

| Method | Path | Body | Answer |
|---|---|---|---|
| POST | `/api/runs/next` | `{worker, project}` | `200 {id, project, task_id}`, or `204` when nothing is queued |
| POST | `/api/runs/{id}/result` | `{passed, status, branch, commit, evidence[], log, error}` | `202` |
| POST | `/api/workers/heartbeat` | `{name, project, status}` | `200` |

All with `Authorization: Bearer <key>`. The body of the result is `trilha-runner`'s
`queue.Result`; `evidence[]` is the protocol's record, unchanged. The code never travels.

## What is in the MVP and what is not

| | Delivered | Later |
|---|---|---|
| Control plane | projects, queue with claim, results with evidence, reviewer close | organisations and teams, billing |
| Fleet | workers with heartbeat; who is doing what | scheduling across projects, remote sandboxes |
| Governance | admin token, scoped keys with a per-key rate limit, audit of every action | SSO, policies per project, signed evidence |
| Storage | in memory, JSON snapshot on every write | SQL |

## Challenge

Write the smallest control plane that a `trilha-runner worker --once` accepts: a Trilha app
with the three routes above, a queue that is a slice in memory, and no authentication yet.
Run the worker against it with the `echo` driver.

:::solution
Three files under `app/api/` of a new project (`trilha new fila`):

```go
// app/api/runs/next/route.go
package next

var queue = []string{"TASK-004"} // the task ids to hand out, in order

func POST(c *trilha.Ctx) error {
	var in struct{ Worker, Project string }
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	if len(queue) == 0 {
		c.Status(204)
		return nil
	}
	id := queue[0]
	queue = queue[1:]
	return c.JSON(200, map[string]string{"id": "run-1", "project": in.Project, "task_id": id})
}
```

```go
// app/api/runs/id_/result/route.go
package result

func POST(c *trilha.Ctx) error {
	var in map[string]any
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	c.Log().Info("result", "run", c.Param("id"), "status", in["status"], "commit", in["commit"])
	c.Status(202)
	return nil
}
```

```go
// app/api/workers/heartbeat/route.go
package heartbeat

func POST(c *trilha.Ctx) error { return c.JSON(200, map[string]string{"ok": "1"}) }
```

`trilha gen && trilha dev` in one terminal; in the agenda,
`trilha runner worker --cloud http://localhost:3000 --token x --project agenda --once --driver echo`.
The worker claims `TASK-004`, runs it and posts the result you see in the log. A slice is
not a queue two workers can share and a missing `Authorization` check is not a control plane
— which is exactly the list of what trilha-cloud adds.
:::
