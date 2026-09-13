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
     -d '{"name":"laptop","project":"agenda","expires_days":30,"scopes":["runs:read","runs:write"]}'
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
trilha-runner worker --cloud http://localhost:3000 --token tc_… --project agenda --once
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

## Reproduce the homologated flow

The Cloud repository carries an executable acceptance of the whole ecosystem,
not a prebuilt fixture. Every run creates a fresh application and protocol:

| Component | What the acceptance exercises |
|---|---|
| `trilha` | `new`, `openapi`, all six `check` gates, `ctx`, `build`, the page and generated API |
| `trilha-spec` | `init`, `spec new`, `task add`, `task move`, `doctor`, `context` and `evidence` |
| `trilha-runner` | Cloud worker, isolated worktree, `echo` driver, checks, branch and commit |
| `trilha-cloud` | project, key, queue, claim, result, review, approval, audit, portal and persistence |

Run the complete acceptance from the `trilha-cloud` checkout:

```bash
make homologate
# or include the Cloud gate and headless-browser portal acceptance
make check-all
```

The protocol is generated through its public CLI rather than copied from the
repository:

```bash
trilha-spec init "$APP_DIR"
cd "$APP_DIR"
trilha-spec spec new "Homologate the Trilha ecosystem"
trilha-spec task add "Execute the end-to-end ecosystem flow" \
  --spec 001-homologate-the-trilha-ecosystem \
  --agent coder \
  --accept "trilha-runner creates TRILHA_RUN.md inside an isolated worktree" \
  --accept "the generated Trilha application remains green" \
  --check "test -f TRILHA_RUN.md" \
  --check "go test ./..."
trilha-spec task move TASK-001 ready
trilha-spec doctor
trilha-spec context TASK-001 --json
```

The Cloud then queues the generated task and invokes a real worker:

```bash
trilha-runner worker \
  --cloud http://127.0.0.1:3901 \
  --token tc_… \
  --project homologation-app \
  --name homologation-worker \
  --once \
  --driver echo
```

Acceptance requires `TASK-001` to reach `review` with branch
`trilha/task-001`, a commit and passed evidence for `test -f TRILHA_RUN.md`
and `go test ./...`. The same evidence is read through `trilha-spec evidence
TASK-001 --json` and the Cloud API. The script then approves the run, verifies
the six audit events from project creation through `run.closed`, restarts the
control plane and confirms that the final `done` status persisted.

To exercise the UI, start the Cloud with `make dev`, open
`http://localhost:3000`, use **Configure access** to store the admin token and
the generated key in the browser session, then register the application,
queue `TASK-001`, inspect its evidence after the worker exits and select
**Approve**. The application source remains in the worker checkout
throughout the procedure.

The deterministic `echo` driver keeps this acceptance independent from a
model provider. AI drivers, remote sandboxes, billing and production deployment
belong to separate acceptance environments.

The browser part is also available as `make homologate-ui`. It drives the real
portal through Chrome DevTools: configures access, registers a project, issues
a key, queues a run, opens protected evidence and approves the review.

## Product Studio: create the product without a CLI

In Trilha Cloud 0.2.0, the operator can enable a managed workspace and the user
completes the whole cycle through the UI. The CLIs still run behind the Cloud,
but they are no longer the product creator's responsibility.

The operator starts the Cloud once, defining where projects may be created and
which homologated binaries it may invoke:

```bash
cd trilha-cloud
mkdir -p data workspaces
export TRILHA_SECRET="$(trilha secret)"
export TRILHA_CLOUD_ADMIN_TOKEN='replace-with-an-admin-token'
export TRILHA_CLOUD_DATA="$PWD/data/cloud.json"
export TRILHA_CLOUD_WORKSPACE_ROOT="$PWD/workspaces"
export TRILHA_BIN="$(command -v trilha)"
export TRILHA_RUNNER_BIN="$(command -v trilha-runner)"
make dev
```

Open `http://localhost:3000`, select **Configure access** and enter the admin
token. Then select **New product** and enter:

- name `cadastro-usuarios` and organization `trilha`;
- description `User registration with login and password change`;
- module `example.com/trilha/cadastro-usuarios`;
- template `app`, language `Português`, and the `Echo` driver for deterministic
  homologation — use `AI` only when its provider is configured in the runner
  environment.

![Product Studio form for creating the application](/docs/agentic-cloud/cloud-product-studio-create.png "The user describes the product in the UI and types no generation command.")

When **Create product** is selected, the Cloud performs fixed, auditable
operations:

1. **Trilha** generates the `app` application, which already includes login,
   invitation, registration, profile and password change;
2. **Trilha Spec** initializes `.trilha/`, writes the approved
   `001-product-foundation` spec and creates ready task `TASK-001`;
3. the Cloud initializes Git and records the starting point;
4. when **Start build** is selected, **Trilha Runner** opens the worktree, runs
   the task and returns branch, commit, log and evidence to **Trilha Cloud**.

For review, use **Issue key** with `runs:read` and `runs:write`, select **Use in
this session**, open **Details**, and inspect every check. **Approve** completes
`review → done` and records `run.closed` in the audit trail.

![Approved execution evidence in Trilha Cloud](/docs/agentic-cloud/cloud-product-studio-evidence.png "The UI shows the task, worker, branch, commit, checks and log before the human decision.")

Managed mode is opt-in. Names are validated, every workspace must be a direct
child of `TRILHA_CLOUD_WORKSPACE_ROOT`, user input is never evaluated by a
shell, and Cloud secrets are removed from child-process environments. Only the
provider credentials required by the `AI` driver reach the runner.

Operators can reproduce the same flow with:

```bash
make homologate-studio  # Trilha → Spec → Runner → Cloud → approval
make homologate-ui      # real portal flow in Chrome
make check-all          # security + ecosystem + Product Studio + UI
```

## Tutorial: user registration from end to end

This walkthrough creates a real Trilha application with its own login, user
invitations and password changes; describes the work with `trilha-spec`; runs
the task with `trilha-runner`; and uses `trilha-cloud` for the queue, evidence
and approval. The Cloud receives run metadata, never the application's source
code.

Use two sibling directories and three terminals: one for the Cloud, one for
the app and one for the worker. The examples reserve `localhost:3000` for the
Cloud and `localhost:3100` for the application.

### 1. Start and configure Trilha Cloud

From an authorised checkout of the `trilha-cloud` repository:

```bash
cd trilha-cloud
mkdir -p data
export TRILHA_SECRET="$(trilha secret)"
export TRILHA_CLOUD_ADMIN_TOKEN='replace-with-an-admin-token'
export TRILHA_CLOUD_DATA="$PWD/data/cloud.json"
make dev
```

Open `http://localhost:3000`. Under **Configure access**, first enter the same
`TRILHA_CLOUD_ADMIN_TOKEN`. The API key may stay empty until one is issued.
Credentials remain in that browser's `sessionStorage` and are sent as Bearer
tokens to the Cloud APIs.

![Trilha Cloud Configure access dialog](/docs/agentic-cloud/cloud-configure-access.png "Configure the admin token and, after issuing it, the worker API key.")

### 2. Generate the user registration application

In another terminal, from the directory that will hold the project:

```bash
trilha new cadastro-usuarios \
  --module example.com/cadastro-usuarios \
  --template app \
  --lang pt \
  --agents
cd cadastro-usuarios
```

The `app` template already provides the flow being accepted:

| Route | Responsibility |
|---|---|
| `/entrar` | checks e-mail and password and opens the session |
| `/admin/usuarios` | lists people, assigns roles and issues invitations |
| `/convite/{token}` | lets an invited person define the first password |
| `/perfil` | changes the name, requests an e-mail change, changes the password and lists sessions |
| `/sair` | closes the current session |

The central files are `app/entrar/page.go`, `app/admin/usuarios/page.go`,
`app/convite/token_/page.go` and `app/perfil/page.go`. The first administrator
is seeded from the environment:

```bash
export TRILHA_SECRET='use-a-secret-with-at-least-32-bytes'
export ADMIN_EMAIL='admin@example.com'
export ADMIN_PASSWORD='senha-segura-123'
```

### 3. Configure the `make check` gate

`trilha check` runs the framework gates (`gen`, `gofmt`, `vet`, `test`,
`audit` and, when a versioned document exists, `openapi`). A small `Makefile`
keeps the CLI replaceable in CI without duplicating the policy:

```make
TRILHA ?= trilha

.PHONY: check
check:
	$(TRILHA) check
```

Run the gate with the same variables used by the application:

```bash
TRILHA_SECRET="$TRILHA_SECRET" \
ADMIN_EMAIL="$ADMIN_EMAIL" \
ADMIN_PASSWORD="$ADMIN_PASSWORD" \
make check
```

To test another CLI version without editing the file:

```bash
make check TRILHA='go run github.com/emersonjoe/trilha/cmd/trilha@v0.123.0'
```

### 4. Describe the delivery with `trilha-spec`

Initialise the protocol and generate the spec:

```bash
trilha-spec init .
trilha-spec spec new "User registration and authentication"
```

Complete `.trilha/project.md` with the project commands and edit
`.trilha/specs/001-user-registration-and-authentication.md` to record the
problem, the change, what is out of scope and the acceptance criteria. Then
generate an executable task:

```bash
trilha-spec task add "Validate registration login and password change" \
  --spec 001-user-registration-and-authentication \
  --agent coder \
  --accept "administrator can sign in" \
  --accept "administrator can invite a user" \
  --accept "invited user defines the first password and can sign in" \
  --accept "user can change their own password" \
  --check "go test ./..." \
  --check "make check"

trilha-spec task move TASK-001 ready
trilha-spec doctor
trilha-spec context TASK-001 --json
```

`doctor` validates the protocol structure. `context` shows the exact package
the runner gives the agent: project, constitution, spec, task and agent
profile.

### 5. Version the starting point

The runner creates one worktree and branch per task, so it needs a Git
repository with a clean commit:

```bash
git init
git add .
git commit -m "start user registration application"
```

### 6. Register the project in the portal

In Trilha Cloud, select **Register project** and enter:

- **Name:** `cadastro-usuarios`;
- **Organisation:** your team's identifier, for example `trilha`;
- **Repository:** the Git URL or a local reference meaningful to the operator,
  for example `local:///workspace/cadastro-usuarios`.

![Register project dialog](/docs/agentic-cloud/cloud-register-project.png "The Cloud identifies the project; the checkout stays on the worker machine.")

The same step can be automated:

```bash
curl -X POST http://localhost:3000/api/admin/projects \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"cadastro-usuarios","org":"trilha","repo":"local:///workspace/cadastro-usuarios"}'
```

### 7. Issue the worker key

Select **Issue key**, use the name `cadastro-usuarios-worker` and keep the
`runs:read` and `runs:write` scopes. The `tc_…` secret is shown only once;
store it in a secret manager. In the portal, **Use this key** also fills it in
for the current browser session.

![Issue API key dialog](/docs/agentic-cloud/cloud-issue-key.png "The worker key must read the queue and publish the result.")

The equivalent API operation is:

```bash
curl -X POST http://localhost:3000/api/admin/keys \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"cadastro-usuarios-worker","project":"cadastro-usuarios","expires_days":30,"scopes":["runs:read","runs:write","deployments:write","secrets:read"]}'
# copy the response key field to TRILHA_CLOUD_API_KEY
```

### 8. Queue the task and connect the runner

In the portal, select **New run**, choose `cadastro-usuarios` and enter
`TASK-001`. Through the API:

```bash
export TRILHA_CLOUD_API_KEY='tc_…'
curl -X POST http://localhost:3000/api/runs \
  -H "Authorization: Bearer $TRILHA_CLOUD_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"project":"cadastro-usuarios","task_id":"TASK-001"}'
```

Run a worker from the application checkout. The `echo` driver makes this
acceptance deterministic: it proves the protocol, worktree, commit and checks
without depending on an AI provider.

```bash
cd cadastro-usuarios
TRILHA_SECRET="$TRILHA_SECRET" \
ADMIN_EMAIL="$ADMIN_EMAIL" \
ADMIN_PASSWORD="$ADMIN_PASSWORD" \
trilha-runner worker \
  --cloud http://localhost:3000 \
  --token "$TRILHA_CLOUD_API_KEY" \
  --project cadastro-usuarios \
  --workspace-root /var/lib/trilha-runner/workspaces \
  --repo git@github.com:trilha/cadastro-usuarios.git \
  --default-branch main \
  --push \
  --name cadastro-usuarios-worker \
  --once \
  --driver echo
```

The expected result is `TASK-001 → review`, a `trilha/task-001` branch, a
commit and passed evidence for `go test ./...` and `make check`. Open
**Details**, inspect the commands, exit codes, branch, commit and log; only
then select **Approve**.

![Run evidence under review](/docs/agentic-cloud/cloud-run-review.png "The review shows the task, worker, branch, commit, every check and the log before approval.")

![Completed run in the portal](/docs/agentic-cloud/cloud-run-done.png "After approval, the run is done and the worker returns to idle.")

### 9. Test login, invitation registration and password change

Start the application on a port different from the Cloud port:

```bash
cd cadastro-usuarios
TRILHA_SECRET="$TRILHA_SECRET" \
ADMIN_EMAIL="$ADMIN_EMAIL" \
ADMIN_PASSWORD="$ADMIN_PASSWORD" \
PORT=3100 trilha dev
```

Open `http://localhost:3100/entrar` and sign in with `admin@example.com` and
`senha-segura-123`.

![Generated application login](/docs/agentic-cloud/users-login.png "The initial administrator comes from ADMIN_EMAIL and ADMIN_PASSWORD.")

Open `http://localhost:3100/admin/usuarios`, enter the person's e-mail and
name, then select **Convidar**. The person starts inactive and without a
password; the administrator receives a temporary link instead of choosing the
password for them.

![User registration by invitation](/docs/agentic-cloud/users-admin.png "The screen shows the one-time link and the person while still inactive.")

Open the `/convite/{token}` link in a private window. The invited person sets
a password with at least 12 characters; the token is consumed, the account is
activated and the browser returns to login.

![First password definition](/docs/agentic-cloud/users-invite.png "The password originates with the user and the link stops working after use.")

After signing in, open `http://localhost:3100/perfil`. In the **Senha** card,
enter the current password and a new one. The application closes the sessions
and requires a new login with the new password.

![Password change in the profile](/docs/agentic-cloud/users-password.png "The change asks for the current password and closes the session when complete.")

![Confirmation after changing the password](/docs/agentic-cloud/users-password-changed.png "The logout after the change confirms that the new credential must be used.")

### 10. Limits of this example

The template uses in-memory stores to keep the example small. Restarting the
application removes users, invitations, sessions and password changes; the
initial administrator is seeded again from the environment variables. Before
production, replace those stores with real persistence, configure e-mail
delivery for invitations and address changes, use HTTPS and keep secrets out
of the repository.

To repeat the Cloud's own automated acceptance, including the headless
browser, run this in the `trilha-cloud` repository:

```bash
make check-all
```

### 11. Operate environments, secrets, deploy and rollback from the UI

After the worker is connected, daily operation requires no terminal:

1. In **Specs**, select **Run** on a ready task. Cloud gives the runner a versioned bundle with
   the repository, round, stages, acceptance criteria and checks.
2. The runner refreshes a dedicated checkout, materializes `.trilha/` through Trilha Spec,
   executes the task in a worktree and publishes `trilha/spec-*` and `trilha/task-*` branches
   when `--push` is enabled.
3. In **Environments**, create `production`, enter its HTTPS URL and a runner-local allowed
   profile such as `production`.
4. In **Secrets**, enter one variable per line (`NAME=value`). Cloud stores AES-256-GCM
   ciphertext and subsequently displays names only.
5. Select **Deploy**, enter an immutable commit or tag and follow its status. After success,
   **Rollback** schedules the previous revision.

The delivery profile exists only on the VPS in `/etc/trilha-runner/delivery.json`:

```json
{"profiles":{"production":{"deploy":["/usr/local/libexec/trilha/deploy-product"],"rollback":["/usr/local/libexec/trilha/rollback-product"],"health_url":"https://cadastro-usuarios.eoslab.com.br/health/ready","timeout_seconds":600}}}
```

Cloud does not send commands, Git keys or Docker socket access. The service runs as a dedicated
user without sudo, receives a minimal environment and redacts secret values from returned logs.

![Environments and delivery in the portal](/docs/agentic-cloud/cloud-environments.png "The environment shows secret names only, current revision, allowed profile and auditable actions.")

![Responsive portal at 390 pixels](/docs/agentic-cloud/cloud-mobile.png "Specifications, Kanban, environments, runs and fleet remain operable on a mobile screen.")

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
| Control plane | projects, specifications, rounds, Kanban, execution and review | organisations and teams, billing |
| Fleet | persistent project workers, isolated Git checkout, `.trilha` synchronization and optional push | configurable parallelism and remote sandboxes |
| Delivery | environments, encrypted secrets, local profiles, deploy, health check and rollback | progressive delivery and multiple approvals |
| Governance | login, CSRF, persistent project-scoped expiring/revocable keys and audit | SSO and signed evidence |
| Storage | atomic JSON snapshot on every write | SQL and high availability |

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
`trilha-runner worker --cloud http://localhost:3000 --token x --project agenda --once --driver echo`.
The worker claims `TASK-004`, runs it and posts the result you see in the log. A slice is
not a queue two workers can share and a missing `Authorization` check is not a control plane
— which is exactly the list of what trilha-cloud adds.
:::
