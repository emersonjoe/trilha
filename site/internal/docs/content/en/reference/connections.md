---
title: connections
description: Connections, Connection, ConnectionKind, ConnectionStore, ValidateExternalURL and TestHTTP — the external services an application talks to, with the secret sealed and a Test button.
---

A management application has the same screen three times under three names — integrations,
MCP servers, the LLM provider — because "a thing outside that this app talks to" is a pattern
nobody wrote once: a name, a URL, how to authenticate, a masked secret that "empty on update
keeps", a Test button and the last result.

`trilha.Connections` is that list. It is in the runtime because it needs what the runtime has —
the request, the tenant on the actor, the environment, the audit trail — and because the
[`Secret`](/reference/security) it stores is already there.

## Declaring the kinds

```go
var Conexoes = trilha.NewConnections(trilha.ConnectionsOpts{Kinds: []trilha.ConnectionKind{
	{Key: "api", Label: "APIs", Auth: []string{"none", "bearer", "header", "basic"},
		Test: trilha.TestHTTP("GET", "/")},
	{Key: "mcp", Label: "MCP servers", Auth: []string{"none", "bearer"}, Test: testarMCP},
}})
```

A kind says which authentications it accepts and how it is tested. `Auth` is a subset of
`ConnectionAuths` — `none`, `bearer`, `header` (a named header with the secret as its value),
`basic` (a user and the secret as the password) — and a `Save` with an authentication the kind
did not declare is a `FieldErrors` on `auth`. A kind without `Test` has no Test button.

`Store` is memory by default; `Timeout` (30 s) bounds a test and every client.

## The record

```go
type Connection struct {
	ID, Tenant, Kind, Name, URL string
	Auth     string            // none | bearer | header | basic
	Username string            // basic
	Header   string            // header: the header's name
	Secret   Secret            // token, header value or password
	Headers  map[string]string // fixed headers, never a secret
	LastTest *ConnectionTest   // At, OK, Message
	CreatedAt, UpdatedAt time.Time
}
```

| Method | Role |
|---|---|
| `List(c)`, `Get(c, id)` | the tenant's, sorted by kind then name; `ErrNotFound` |
| `Save(c, conn) (Connection, error)` | validates, creates or updates, audits `connection.save` |
| `Delete(c, id)` | audits `connection.delete` |
| `Test(c, id) (ConnectionTest, error)` | runs the kind's test under `Timeout`, stores `LastTest`, audits `connection.test` |
| `Client(c, id) (*http.Client, error)` | the authenticated client, pinned to the URL's host |
| `Kinds()`, `Kind(key)` | what was declared |

Every method takes the `*Ctx` because the tenant comes from `c.Actor()`, the environment from
`c.Env()` and the audit record from `c.Audit` — the same three things the rest of the runtime
reads, and none of them an argument somebody can forget.

## What Save checks

- **The URL is external.** `ValidateExternalURL(url, env)` wants `http` or `https`, a host, no
  credentials in the address, and — in `Prod` — nothing loopback, private, link-local or ending in
  `.local`, `.internal`, `.lan`. In `Dev` the private address passes with a warning in the log,
  because that is where the service under test lives.
- **An empty secret on update keeps the previous one.** The form renders the field empty
  ([`ui.SecretField`](/reference/ui)) and the person editing the name is not asked to retype the
  token. A new value replaces it; the old one is not kept anywhere.
- **`Authorization` is not a fixed header.** It is the auth, and a header that carried a
  credential in the clear would be the secret stored outside the sealed field.
- **Changing the URL, the auth or the secret clears `LastTest`.** A green badge on a connection
  whose token was replaced is a badge about another connection.

The audit record carries the kind, the name, the URL and the auth. Never the secret.

## The client

```go
cli, err := Conexoes.Client(c, id)
resp, err := cli.Do(req)   // req.URL on another host → error, before any bytes leave
```

The transport is the only place the secret is read: it puts on the `Authorization` (or the named
header, or the basic pair) and the fixed headers, and **refuses any request whose host is not the
connection's**. A redirect to another domain therefore does not take the token with it — the
follow is refused by the same rule. `Connection.Client(timeout)` is the same client for whoever
writes a `Test`.

`TestHTTP(method, path)` is the ready-made test for APIs: any answer below 400 passes, the rest
becomes the message (`HTTP 401 Unauthorized`). For an MCP server, dial with
[`mcp.HTTPWith`](/reference/mcp#client) over the connection's client and list the tools.

## The secret

`Secret` masks itself in `%v`, in JSON and in `slog`; the screen renders the field empty; the
audit record does not carry it. What is left is the store, and there the rule is the same as
everywhere else in Trilha: `Secret.Value()` seals with the app's key on the way into
`database/sql`, `Scan` opens on the way out. A memory store holds it in the clear, as memory does.

## The store

```go
type ConnectionStore interface {
	List(ctx context.Context, tenant string) ([]Connection, error)
	Get(ctx context.Context, tenant, id string) (Connection, error)
	Save(ctx context.Context, conn Connection) error
	Delete(ctx context.Context, tenant, id string) error
}
```

`trilha.ConnectionMemory()` is the default. A table is one row per connection with `headers` as JSON and `secret` as
the sealed blob — and, as with [Search](/reference/search), the SQL belongs in your project: the
framework has no driver.

## The screen and the recipe

[`ui.ConnectionsPanel`](/reference/ui#connectionspanel) is the list grouped by kind, the badge of
the last test, and the form — [demo there](/reference/ui#connectionspanel). Every button is a
`POST` to one path with a hidden `_action` (`save`, `test`, `delete`), so it works with no script.

```
trilha add connections
```

writes `internal/conexoes` with the kinds `api` and `mcp`, the `/conexoes` page with the three
actions, and the tests — see [`trilha add`](/reference/cli#trilha-add) and
[Talking to a third-party API](/cookbook/third-party-api).
