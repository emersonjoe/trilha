---
title: Testing
description: A test client in the framework itself: one request, a whole session, CSRF that just works.
---

An app made with Trilha is an `http.Handler`, so it can always be tested with `httptest` and
nothing else. The problem is what comes before the first assertion: a client, a cookie jar,
and the CSRF token copied from the cookie into the form. That is fifty lines every project
writes again — and gets wrong the first time, because the double-submit only passes when the
cookie comes back in the request.

The framework already issues that cookie and already checks that token, so it ships the
client. No external test framework, no assertion library: `package trilha` never imports
`testing`.

## One request

```go
func TestListaPosts(t *testing.T) {
	res := trilha.TestRequest(t, newApp(), "GET", "/api/posts")
	res.WantStatus(200).WantContains(`"slug"`)
}
```

`newApp()` is the function the generator writes in `trilha_gen.go`: the same app that serves
in production. The request goes through the real path — mux, middlewares, layouts, CSRF,
error negotiation — and what comes back is the recorded response.

Assertions chain and never return an `error`. In a test the value of an error is stopping
with the right message, so a failure prints the status, the target and the body:

```text
GET /api/posts: status = 500, want 200
{"status":500,"title":"Internal Server Error","request_id":"…"}
```

## A whole session

When the test is a flow — open the form, submit it, follow the redirect — the client keeps
the cookies the app sets:

```go
func TestPublicar(t *testing.T) {
	c := trilha.NewTestClient(t, newApp())
	c.Get("/blog/novo").WantStatus(200)
	res := c.PostForm("/blog/novo", url.Values{"titulo": {"Hello"}})
	res.WantStatus(303).WantHeader("Location", "/blog/hello")
	c.Get("/blog/hello").WantContains("Hello")
}
```

`Get`, `PostForm` and `PostJSON` are shortcuts for `Request`, which takes any method. A
redirect is not followed on its own: the test that wants the destination asks for the
destination, because where a `303` lands is an assertion, not a detail.

## CSRF passes by default

Every request the helpers send carries the CSRF cookie, and every method with a body carries
the same value in the `X-CSRF-Token` header.

:::note
This is not a hole in the protection. Double submit asks the browser to prove it can read
its own cookie, and the test client proves exactly that: cookie and token come from the same
place. What the check rejects — a form posted from another site, which cannot read the
cookie — is still rejected.
:::

A test that wants to prove the rejection asks for it:

```go
c.PostForm("/blog/novo", form, trilha.WithoutCSRF()).WantStatus(403)
```

## A signed session without logging in

`WithSigned` writes a cookie signed with the app's own signer — the same one `c.SetSigned`
uses in a handler. The admin page stops requiring a `POST /login` before every case:

```go
res := trilha.TestRequest(t, newApp(), "GET", "/admin", trilha.WithSigned("sessao", "ana"))
res.WantStatus(200)
```

The signature is real: a session forged by hand still fails, which is what
`trilha.WithCookie("sessao", "ana|9999999999|fake")` is for when you want to test the
rejection.

## One `route.go`, one page

`TestRoute` mounts a throwaway app in `Dev` around a single route, so a handler can be tested
where it lives, before it is registered anywhere:

```go
res := trilha.TestRoute(t, trilha.Route{
	Pattern: "/api/itens/{id}",
	Methods: map[string]trilha.HandlerFunc{"GET": GET},
}, "GET", "/api/itens/7")
res.WantStatus(200).WantContains(`"id":7`)
```

The pattern is what resolves `{id}`, so `c.Param("id")` answers `7` — the router is doing the
work, not a mock.

`TestPage` does the same for a page and also hands back the rendered node, layouts already
applied:

```go
res := trilha.TestPage(t, trilha.Route{Page: Page, Layouts: []trilha.LayoutFunc{Layout}}, "/sobre")
res.WantStatus(200)
if h.Render(res.Node) == "" {
	t.Fatal("empty page")
}
```

`res.Body` holds the whole document, with the layout around it; `res.Node` is just what the
page returned. Asserting on the node survives a change of layout, which is usually what you
want.

Both build the app for you; `trilha.WithApp(a)` uses yours instead, when the route depends on
something `Setup` puts in `a.Values()`.

## The options

| Option | What it does |
|---|---|
| `WithApp(a)` | uses your app in `TestRoute`/`TestPage` instead of a throwaway one |
| `WithHeader(name, value)` | one header (`Accept`, `Trilha-Fragment`, `Authorization`) |
| `WithCookie(name, value)` | one raw cookie |
| `WithSigned(name, value)` | one cookie signed by the app, valid for an hour |
| `WithForm(values)` | body as `application/x-www-form-urlencoded` |
| `WithJSON(v)` | body as `application/json` |
| `WithBody(contentType, body)` | body exactly as written (multipart, CSV, a broken JSON) |
| `WithoutCSRF()` | sends nothing about CSRF, to test the refusal |

## The response

`TestResponse` embeds `*httptest.ResponseRecorder`, so `Code`, `Body` and `Header()` are
still there for whatever the ready-made assertions do not cover.

| Method | What it does |
|---|---|
| `WantStatus(code)` | fails with the body when the status differs |
| `WantContains(text)` | fails with the body when the text is missing |
| `WantHeader(name, value)` | fails when the header differs |
| `JSON(&v)` | decodes the body into `v`, failing with the body on invalid JSON |
| `Cookie(name)` | the cookie this response set, or `nil` |
| `Node` | the page's node, filled in by `TestPage` |

`Cookie` is how you assert on a logout: what proves the session is gone is the app deleting
the cookie, not the redirect that follows.

```go
if res.Cookie("sessao") == nil {
	t.Fatal("logging out should clear the session")
}
```

## Race and fuzzing

Two bugs never show up in a deterministic suite. One is the data race: two requests
touching the same field of the app at the same time — the asset cache, the metric
counters, the rate-limit buckets. The other is the input nobody wrote: a path with
`%2e%2e`, a cookie with the signature of another key, a form body that is a single `;`.

The framework's own suite covers both, and the two commands are one line each:

```bash
make race    # go test -race ./...
make fuzz    # 20s on each fuzz target, same as CI
```

`make race` is only worth what the suite gives it to look at, so there is a test
(`TestConcorrencia`) that hits the same app from 32 goroutines: it logs in, reads a signed
page, calls an API route, asks for a static file and reads `/metrics`. Without it the
detector would run over an app answering one request at a time and find nothing.

The fuzz targets state an invariant rather than an expected output:

| Target | What it holds |
| --- | --- |
| `FuzzRouteMatch` | no target crashes the app or serves a file from outside `public/` |
| `FuzzBindForm` / `FuzzBindJSON` | if `Bind` returns no error, every `validate` rule holds |
| `FuzzSignedVerify` | a cookie is only accepted if some key would have produced it, and it has not expired |
| `FuzzParseTraceparent` | the trace id is either empty or hex that came from the header |
| `FuzzRenderEscapes` | whatever goes into `h.Text` or an attribute comes back escaped |

Fuzzing in your own app is the same shape. Write the target next to the code it tests, seed
it with the cases you already know, and assert the property — not the output:

```go
func FuzzSlug(f *testing.F) {
	for _, s := range []string{"", "Olá mundo", "a//b", "---"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := Slug(s)
		if strings.ContainsAny(got, " /?#") {
			t.Fatalf("%q gerou %q", s, got)
		}
	})
}
```

:::note
When fuzzing finds a failure, Go writes the input to `testdata/fuzz/<Target>/`. Commit that
file with the fix: from then on `go test ./...` replays it, and the bug cannot come back
quietly.
:::

## Challenge

Write a test proving that the blog's form rejects a title longer than the limit and shows
the message on the page, without going through the API.

:::solution
```go
func TestTituloLongo(t *testing.T) {
	c := trilha.NewTestClient(t, newApp())
	res := c.PostForm("/blog/novo", url.Values{"titulo": {strings.Repeat("a", 200)}})
	res.WantStatus(422).WantContains("no máximo")
}
```
The form answers `422` with the page re-rendered — the same body a browser would show — so
one request covers the validation and the message. The CSRF token went along on its own.
:::
