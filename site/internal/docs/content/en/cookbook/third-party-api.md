---
title: Talking to a third-party API
description: A connection somebody registers on a screen — URL, token, Test — and the client the code asks for, which carries the credential and only speaks to that host.
---

The token of the tax service is in an environment variable, the URL of the MCP server is in
another, and the day one of them changes somebody redeploys. The alternative is the screen every
management app ends up with: the list of things outside that this one talks to, with the secret
sealed and a button that says whether it still works.

```
trilha add connections
```

## 1. The kinds

The recipe writes `internal/conexoes` with two kinds. Each one says which authentications it
accepts and how it is tested:

```go
var Tipos = []trilha.ConnectionKind{
	{Key: "api", Label: "{{.T.conn_api}}", Auth: []string{"none", "bearer", "header", "basic"},
		// Any answer below 400 passes: the point is "does the credential
		// open the door", not what is behind it.
		Test: trilha.TestHTTP("GET", "/")},
	{Key: "mcp", Label: "{{.T.conn_mcp}}", Auth: []string{"none", "bearer", "header"},
		Test: testarMCP},
}
```

(`{{.T.…}}` is the recipe's word table; in your project it is already "APIs" and "MCP servers".)

The MCP test is the handshake and the list of tools, through the connection's own client, so the
token goes along without being copied into a headers map:

```go
func testarMCP(ctx context.Context, conn trilha.Connection) error {
	cli, err := mcp.Dial(ctx, mcp.HTTPWith(conn.URL, conn.Client(0), nil))
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.ListTools(ctx)
	return err
}
```

## 2. The screen

`/conexoes` is [`ui.ConnectionsPanel`](/reference/ui#connectionspanel): the list grouped by kind
with the badge of the last test, and the form. Save, Test and Delete are three `POST`s to the
same path with a hidden `_action`, and the page reads them like this:

```go
	case "test":
		res, err := conns.Test(c, c.Form("id"))
		if err != nil {
			return err
		}
		if res.OK {
			c.Flash(ui.FlashSuccess, "{{.T.conn_test_ok}}")
		} else {
			c.Flash(ui.FlashError, "{{.T.conn_test_failed}} " + res.Message)
		}
```

The secret is typed once. On edit the field comes back empty with "leave blank to keep", and the
audit trail (`connection.save`, `connection.test`) says who changed what without ever carrying the
value. Guard the folder: whoever reaches it can point your application at a server of their own.

## 3. Using it

Where the application calls the service, it asks for the client instead of reading a variable:

```go
func Chamar(c *trilha.Ctx, id, path string) (*http.Response, error) {
	conn, err := Conexoes.Get(c, id)
	if err != nil {
		return nil, err
	}
	cli, err := Conexoes.Client(c, id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, conn.URL+path, nil)
	if err != nil {
		return nil, err
	}
	return cli.Do(req)
}
```

The client already has the `Authorization` (or the named header, or the basic pair), the fixed
headers and a timeout — and it **only speaks to the connection's host**. A request built for
another domain, or a redirect that points there, is refused before any bytes leave: the token
cannot follow a `Location` somebody else controls.

## What to watch

- **In production the URL has to be public.** `http://127.0.0.1` and `.internal` names are
  refused by `Save` with the message on the field; in `dev` they pass with a warning, because that
  is where the service under test lives.
- **The store is memory** in the recipe: the connections last as long as the process. A table is
  one row each, `headers` as JSON and `secret` sealed by `Secret.Value()` — the
  [reference](/reference/connections#the-store) has the interface.
- **A kind without `Test` has no button.** Write the test the day you know what "it works" means
  for that service; a fake green is worse than no badge.
