name: trilha-quality
description: The quality checklist of {{.Name}} (a Trilha app) — performance (renders, bundle, requests and cache, queries and N+1, images, fonts, scripts), design patterns, clean code, reuse, review for size, and security by OWASP and NIST. Use it while writing any change and again before saying it is done.
---

# Quality checklist — {{.Name}}

Go through the sections your change touches, then **Before you finish**. Each item says what
to do and what to grep for; "grep" means `grep -rn` over `app/` and `internal/`.

## 1. Renders

- A value the layout and the page both need is computed once: `cache.Once(c, "user", …)`.
- A change to part of the screen answers that part: `c.Fragment()` on the server,
  `ui.Swap`/`ui.Navigate` on the client. A full reload for a sort, a filter or a save is a bug.
- A route whose data has a version declares it: `if c.ETag(rev) { return nil, nil }` answers
  304 before anything renders. A polled fragment (`ui.Poll`) that has nothing new calls
  `c.PollStop()` or keeps its HTML identical — the client leaves an identical answer alone.
- Grep for: work inside a layout that the page also does; `ui.Poll` with a short interval on a
  heavy route; `h.Raw` built by string concatenation.

## 2. Bundle and lazy loading

- `ui.Head(c)` loads the kit's base. A component's behavior is loaded by the page that has the
  component — `ui.TreeScript(c)`, `ui.ChatScript(c)`, `ui.LiveScript(c)`,
  `ui.NavigateScript(c)` — never by the layout for every page.
- A slow section of a page is a `ui.Defer(c, id, url)`: the page answers now, the part when it
  is ready.
- An island (`c.Island`) is for state the server cannot hold; a link, a form or a fragment is
  cheaper and works without JavaScript.
- Grep for: `Script(c)` in `layout.go`; a `<script src>` without `defer`; an npm package
  pulled in for what the kit already does (`trilha ui components`).

## 3. Requests and caching

- Every file of `public/` is linked through `c.Asset("/x.css")`: the URL carries the content
  hash and is cached for a year, and a deploy never leaves anyone with old CSS. Leave
  `StaticCacheControl` short.
- A read shared between requests goes through `cache.Do(ctx, cache, cache.Key{Name, TTL,
  Tags}, fn)`; the write that changes it calls `Invalidate(tag)`. Never cache per-user data
  under a shared name.
- Independent calls to other services run concurrently, each with the request's context.
- An outbound HTTP call uses a client with a timeout and a shared transport, never
  `http.DefaultClient`. A link the user is about to follow can be prefetched with
  `ui.Prefetch()`.
- Grep for: `http.DefaultClient`, `http.Get(`, a hand-written `/style.css` without `c.Asset`,
  `time.Sleep` in a handler.

## 4. Queries and N+1

- No query inside a loop. Ten rows that each need their author is one `WHERE id IN (…)` or one
  `JOIN`, not ten `SELECT`s. A test that counts the store's calls proves it.
- Every list is paginated by the database: `LIMIT`/`OFFSET` (the store recipe's
  `store.Paginate`) and a `count(*)` with the same `WHERE`. Never load a table to filter or
  page it in Go.
- Every column in a `WHERE`, `JOIN` or `ORDER BY` has an index; check with `EXPLAIN` (no
  sequential scan, no temporary sort on a big table). Indexes go in a **new** migration — an
  applied one is checked by its sum.
- Name the columns (no `SELECT *`); a search is `LIKE` with `store.Like` (escaped) and by
  parameter.
- Grep for: `for … range` with `QueryContext`/`QueryRowContext`/a store call inside; a
  `SELECT` without `LIMIT` behind a screen; `ORDER BY` columns missing from the migrations.

## 5. Images, fonts and scripts

- An image says its size: `h.Img(h.Src(…), h.Alt(…), h.Width(w), h.Height(ht),
  h.Attr("loading", "lazy"), h.Attr("decoding", "async"))`. The first image of a page (above
  the fold) is not lazy.
- Photos are WebP (or AVIF), icons are SVG (`ui.Icon`), and nothing is larger than the size
  it is shown at.
- Fonts: the system stack first; a web font is a self-hosted woff2 with `font-display: swap`
  and a `preload` of the one weight above the fold — no third-party font CSS on every page.
- Scripts are `defer` (or modules); an inline script carries `trilha.NonceAttr(c)`. No
  `@import` chains in CSS.

## 6. Design patterns (GoF), where they remove code

- **Strategy**: one interface, interchangeable backends — the recipes' `Store` with a memory
  and an SQL implementation, tested by the same contract.
- **Decorator**: cross-cutting behavior wraps the handler — `middleware.go`, not an `if` copied
  into every route.
- **Adapter**: an outside API is wrapped once, in `internal/`, behind the shape the app needs.
- **Factory**: `New…` returns a ready value; no half-built structs in callers.
- **Observer**: events and `ui.Live`/SSE instead of polling what can be told.
- A pattern that adds a type and saves nothing is not used. Go prefers composition and small
  interfaces, declared by the consumer.

## 7. Clean code

- A function does one thing and fits on a screen; a name says the intent (`pendingInvoices`,
  not `data2`).
- Errors are values: return them wrapped with context (`fmt.Errorf("invoice %s: %w", id,
  err)`), never swallowed, never `panic` for control flow.
- No dead code, no commented-out code, no flag that is always the same.
- A comment says why something is the way it is; the code already says what.
- `gofmt` and `go vet` clean (`trilha check` runs both).

## 8. Reuse

- Before writing, search: `trilha ui components --json` (screens), `trilha add` (recipes),
  `trilha ui describe <Name>`, and `grep -rn` over `internal/` for a function that already does
  it.
- The second copy of a block becomes one function, in the package both callers already import.
- Shared types and rules live in `internal/<domain>`; `app/` only wires a route to them.

## 9. Review for size and readability

- Reread the whole diff as its reviewer. For every line ask: can it go? Can it be shorter and
  still clear?
- Remove what the change did not need: helpers used once, options nobody sets, logs left from
  debugging.
- The smallest readable change wins; a clever line that needs a comment to be understood loses
  to two plain ones.

## 10. Security (OWASP ASVS 5.0 level 2, Top 10:2025, NIST SSDF 1.1)

- **Input**: `c.Bind` into a struct with `validate` tags; a failed check answers the form with
  `FieldErrors`, never a 500. Size limits stay on.
- **Output**: HTML goes through `h` (escaped); `h.Raw` never receives anything a user typed.
- **Access**: authorization lives in `middleware.go` (a policy, a role, the tenant) — hiding a
  link is cosmetics. Default deny.
- **Sessions and CSRF**: use the framework's; a write in a `route.go` called by the browser
  sends the CSRF token.
- **SQL**: parameters only, never concatenated input; `LIKE` escaped.
- **Secrets**: from the environment, never in code, logs or HTML.
- **Supply chain (SSDF)**: no new dependency without need; `trilha check` runs `trilha audit`.
- Record in the change what crosses a trust boundary and how it is checked.

## Before you finish

1. `trilha check` passes.
2. Every section above that the change touches was checked.
3. A test proves the change; a fixed bug has a test that failed before the fix.
4. The diff was reread for size (section 9).
