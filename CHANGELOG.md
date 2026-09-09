# Changelog

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); semantic
versioning. This file is written in English only.

## 0.64.0 — 2026-09-09

Spec 082.

### Added

- **`trilha/task` — the work that outlives the request** ([#111](https://github.com/emersonjoe/trilha/issues/111)).
  Six screens of a real application start something long and then ask "is it done yet?". What
  gets written for that is one line — `go func() { processa(docID) }()` — and it has four defects
  that all show up in production. The error goes nowhere. The request's context dies when the
  browser closes, so half the work stops halfway. A panic takes down the whole process, web
  server included, because a background goroutine has nobody to recover it. And the screen has
  nothing to show, because there is no state: there is a goroutine.

  `Handle` registers the work by name and `Run` starts it. They are separate because of `Retry`:
  a closure lives as long as the process, and the task somebody wants to retry is usually exactly
  the one that died in a deploy. **Two runs with the same name and key, while one is alive, are
  one run** — the double click on the button does not process the document twice — and the key
  reaches the function as `Progress.Key`.

  `Setup` marks everything the store still calls queued or running as **interrupted**: those
  belong to a process that no longer exists, and a task stuck on "running" forever is the classic
  bug. `Shutdown` hangs on the app, so a deploy mid-run waits instead of cutting, and cancels the
  context when it runs out of patience.

  `ui.TaskProgress` is a `ui.Poll` that stops itself the moment the task ends, and draws an
  indeterminate bar when there is no step count — a bar stuck at 0% reads as broken.
  `ui.TaskTable` is the administration screen, with the retry button only on what has finished.

  **It is not a queue, and the package doc says so first**: tasks live in one process, two
  replicas run everything twice, and nothing survives a crash but the record. `Store` is the seam
  for anything more. There is no `task.SQL(db)` for the reason no store in this framework ships
  one — it would have to pick a placeholder dialect and own a DDL — and the recipe carries the
  whole implementation instead.

  `examples/blog` gained the four-stage processing of a document, the progress screen and the
  administration table, with a test that asserts the POST came back before the work finished.

## 0.63.0 — 2026-09-09

Spec 081.

### Added

- **`trilha/mail` — the messages an app actually sends** ([#113](https://github.com/emersonjoe/trilha/issues/113)).
  Every internal application mails somebody: the invitation, the link, the notice that a flow
  finished. The framework said nothing about it, so what got written was `net/smtp` inside the
  handler — and the questions that stops on are never about the product. 587 or 465. STARTTLS or
  implicit TLS. PLAIN or LOGIN. How to write a body without going back to 2003 tables. How to
  test without mailing a real person.

  The worst one is invisible: **`smtp.SendMail` has no deadline**, so a slow server pins the
  handler until TCP gives up, which the visitor sees as a page that spins. Here the deadline is
  the context's, on the connection, and a handler that gave up hangs up.

  **The body is an `h.Node`** — the same nodes the pages are written with — and every message
  goes out `multipart/alternative` with **the plain text generated from that same node**, so a
  client with no HTML reads it whole and a link becomes `text <https://…>` instead of
  disappearing. `mail.Layout` is the transactional email nobody should write twice: a centred
  table, widths in pixels, every rule inline, because Outlook renders with Word and Gmail strips
  `<style>` out of the head.

  Authentication never happens over a clear channel unless somebody types
  `SMTP.AllowInsecureAuth`: a server offering `PLAIN` unencrypted is misconfigured, not an
  invitation. `Bcc` is a recipient of the envelope and of no header. The headers the package
  writes cannot be replaced through `Message.Headers`.

  With `TRILHA_MAIL_URL` unset, **dev writes `.eml` files into `./mail`** — the format a client
  opens by double-clicking — and **production answers `mail.ErrNotConfigured`**. That asymmetry
  is the point: an app that quietly files invitations into a directory is an app whose users are
  never invited, and nobody finds out for a week. `trilha audit` warns about exactly that.

  `mail.Outbox` is how an application tests it: in memory, already taken apart into text and
  HTML, no network and no container. It lives in the package and not in a `_test.go` because a
  non-test package cannot import `testing` — everything that imports it registers the test flags
  in every binary of the project.

  The SMTP client is exercised against a real server on a real socket with real TLS, because
  that is the only way to prove STARTTLS was negotiated and that the password never left before
  it. `examples/local-login` gained the whole invitation flow: `c.Link` for the capability, the
  e-mail to deliver it, and an accept page in a folder of its own — a middleware guards its
  folder and everything under it, so an accept page under the invite screen would have demanded
  the session the invited person does not have yet.

## 0.62.0 — 2026-09-09

Spec 080.

### Fixed

- **`auth`: Keycloak roles never arrived** — roles are now read from the access token too.
  A login against a stock Keycloak worked and left `User.Roles` empty, so every `RequireRole`
  answered 403: **Keycloak puts `realm_access` and `resource_access` in the access token, not
  in the `id_token`**, and the callback only read the `id_token`. The symptom looks like a
  permissions misconfiguration and is a claim read from the wrong token.

  `Callback` now completes the role claims from the access token when it is a JWT from the same
  issuer. Identity still comes from the `id_token`, always, and the `id_token` wins wherever
  both carry the same claim — only what is missing is filled in. An opaque access token, one
  that does not verify, or one from another issuer is not an error and grants nothing: the
  login was already proven.

### Added

- **Live test suites against real servers** — `blob/s3_live_test.go` (MinIO) and
  `auth/oidc_live_test.go` (Keycloak). The test servers written here recompute what this code
  does the way this code does it, which is internal consistency and nothing else; a real server
  is what tells you the protocol was understood. They are skipped unless `TRILHA_S3_TEST` /
  `TRILHA_OIDC_TEST` point at one, so `go test ./...` still needs no network and no Docker.

  The S3 one creates the bucket, puts, gets, stats, lists, **fetches the presigned URL with a
  client that signs nothing**, watches an expired URL be refused, and deletes. Run once with the
  wrong secret, MinIO answers `SignatureDoesNotMatch` — which is what says the passing run means
  something. The OIDC one provisions realm, client, user and role through the admin API and then
  logs in through a cookie-keeping browser, all the way to a guarded page.

  It was the second one that found the Keycloak bug above.

## 0.61.0 — 2026-09-09

Spec 079.

### Added

- **`trilha/blob` — where the file lives** ([#114](https://github.com/emersonjoe/trilha/issues/114)).
  The framework knew how to receive a file and how to hand one back; where to keep it was never
  said, and the example wrote into `./uploads` with the name the client sent — both classic
  mistakes on one line.

  **The key is the content, never the name**: SHA-256, two levels deep, with the extension of the
  sniffed type (`ab/cd/abcd…ef.pdf`). Path traversal is not prevented by a check, it is
  impossible — nothing from the request reaches the key. The same file uploaded twice is one
  object, and no directory ends up with a hundred thousand entries.

  Three stores: `Disk` (the default, with an atomic write, so a crash never leaves a key that
  exists and cannot be read), `Memory` (for tests, and its reader seeks, so a test that passes
  here does not fail on disk over `Range`), and `S3` — **the signature written here, about two
  hundred lines, no SDK**, which is what keeps this module optional instead of the framework
  growing a dependency tree. It puts, gets, heads, deletes, lists and presigns; more than that is
  a reason to use the SDK in your own code.

  `Serve` is `http.ServeContent` from disk — `Range`, `304`, `HEAD` — and a redirect from a store
  that can presign, with the bytes never touching the application. **A presigned URL is a
  capability**, and the docs say so: whoever holds it has the file until it expires, with no
  session and no log of yours; `ServeOpts{Proxy: true}` is the answer when that is not acceptable.

  `Orphans` is the sweep every application ends up needing — a row deleted while the object
  stayed, an upload that failed after the write.

  Deduplication is the module's; deciding what it means is not. `Ref.SHA256` is there to count
  references with; `Delete` removes the object, and whether that is right when two rows point at
  one key is a business rule, not something a storage package should answer for you.

  `TRILHA_BLOB_URL` picks the store, and a URL it cannot read is a **panic at boot** rather than a
  quiet fallback: an application that starts with the wrong storage loses files quietly.

### Verified, and not

The SigV4 signature is checked against a test server that **recomputes** it with the secret: a
badly signed request fails there for the same reason it would fail at AWS. It is **not** checked
against an official AWS test vector — there was no way to verify one offline, and a made-up
"known vector" would suggest a guarantee that does not exist. The failure mode is loud: a wrong
signature is a 403 on the first call, not a quiet leak. A run against a real MinIO is one
`docker run` away and worth doing once.

### Not here

`Orphans` takes a function and not an `iter.Seq` (Go 1.22; the shape already matches, so the
signature changes without breaking anyone when the minimum moves), and the SVG-with-script check
stays in `Ctx.File`/`Ctx.Inline` where it already is — `Put` receives what `Ctx.File` approved,
and a second copy of that list is the copy that falls behind.

## 0.60.0 — 2026-09-09

Spec 078.

### Added

- **Multi-tenant by column** ([#110](https://github.com/emersonjoe/trilha/issues/110)). One column
  is the most common shape of multi-tenant, and forgetting that column in one query is the most
  common bug of multi-tenant: the report that shows another organisation's rows, found by a
  customer. Trilha has no ORM and should not have one — but the tenant is session data, and the
  session is the framework's.

  `auth.User.Tenant` is a field of its own, next to `Roles`, because everything the framework does
  with it has to find it in the same place in every application: it goes into the audit trail as
  `actor.tenant` and onto the access record as `tenant`. `auth.Tenant(c)` reads it in one call, so
  the `WHERE` still reads like a `WHERE`. `RequireTenant` refuses a session with no organisation —
  redirecting a browser to `Options.ChooseTenantPath`, answering 403 to anything else, because a
  redirect to a screen is not an answer an API can use — and `SwitchTenant` moves the session and
  audits both sides.

  **The framework carries the value and points at the query that forgot it. The query is yours.**
  A `WHERE` generated by this package would be a `WHERE` nobody could read in a review.

- **`trilha audit` counts the tenant filter.** Per table, how many queries filter by tenant, and
  which ones do not — with the file and the line:

  ```
  warn  queries that may be missing the tenant filter
        internal/docs/repo.go:88: documents is filtered by tenant in 7 of 8 queries, and not in this one
  ```

  It is a text heuristic and says so in its own message: no SQL parser, no verdict, a place to
  look. Two filtered queries are the threshold (one is not a pattern), and `_test.go` does not
  count — a fixture full of queries would make the tool accuse what does not exist in production.

### Changed

- The tenant goes on the **access record that already existed**. The first version of this wrote a
  second `request.tenant` line, which would have traded one problem for another: whoever
  investigates would have had to join two lines.

### Not here

`--tenant` in `generate crud` and in the app template ([#115](https://github.com/emersonjoe/trilha/issues/115),
[#117](https://github.com/emersonjoe/trilha/issues/117)), and multi-tenant by schema or by
database — another shape with other decisions, and mixing the two into one API would make both
worse.

## 0.59.0 — 2026-09-09

Spec 077.

### Added

- **`c.Link` and `c.Claim` — the link that works with no login**
  ([#106](https://github.com/emersonjoe/trilha/issues/106)). Three flows in every internal
  application happen without a session: somebody outside fills in a form, somebody checks a
  document by a code, somebody answers a request from an e-mail. The framework had the primitive
  — the signer — and not the pattern, and what gets written without it is a random string in a
  table, in the clear, with no deadline.

  **With `Uses: 0` there is no state at all**: no row, no lookup, no cleanup — verifying is a
  signature check. That is the verification code printed on a document, and it is what makes the
  common flow need no table.

  **Reading is not spending.** `Claim` checks that a use is left; only `Consume` takes one, and
  after the work. Otherwise a reload would burn the link somebody is still filling in, and a
  validation error would cost them the invitation.

  **All four failures answer the same 404** — wrong signature, wrong purpose, expired, spent.
  Telling a stranger which one happened tells them how close they are. A wrong token also costs
  the address a point of a small budget, because guessing a token in a URL is brute force.

  The purpose is part of the token: a link to a form does not open a verification, even signed by
  the same application with the same key. And what travels inside is **signed, not secret** —
  said in the doc comment, the reference and the recipe, with a test so nobody discovers it the
  other way.

### Not here

`E_CLAIM_BEHIND_LOGIN` in `gen`: reading a route's middleware statically is the same debt as
[#124](https://github.com/emersonjoe/trilha/issues/124) and goes in with it. The attempt budget
is also **a refilling one and not an hour of blocking**, which is a deliberate difference from
the issue: blocking by address turns one clumsy person behind an office NAT into an outage for
everybody behind it, and guessing is just as infeasible either way.

### Changed

- A negative `TTL` used to become one hour, silently. A caller computing "until the end of the
  day" after midnight would have got a valid link out of a bug; it is now an error. Zero is still
  the one-hour default.

## 0.58.0 — 2026-09-08

Spec 076.

### Added

- **`auth.APIKeys` — the key this application issues**
  ([#105](https://github.com/emersonjoe/trilha/issues/105)). The framework had a session cookie
  and somebody else's bearer (OIDC). The third case was missing, and it is a pile of security
  rules a beginner does not know: store only the hash, show the secret once, keep a handle so a
  key can be found without opening the hash, a scope per route, a limit per key rather than per
  address, revocation that takes effect now, a record of use. The first version always stores the
  key in the clear.

  **Only the hash is stored**, peppered with the new `trilha.Pepper` — HMAC-SHA256 under a key
  derived from the app's secret, sister to `Seal` from 0.57.0. Without a secret, `Issue`
  **refuses**: an unkeyed hash would look like it worked and leave a table anybody can attack
  offline.

  The comparison is constant time, and revocation and expiry are checked **after** it: answering
  faster for a revoked key than for a wrong one tells whoever is guessing which of the two
  happened. All three are the same 401 with `WWW-Authenticate`.

  A key is an actor: `Require` puts an `auth.User` in the request with the scopes as roles and
  `via: "api_key"`, so `c.Audit`, the log and the policy work with no extra line. The limit is
  **per key**, use is recorded **once a minute**, and a scope the application never declared is a
  panic when the route is wired — the alternative is a route that guards nothing because of a
  typo.

  `ui.SecretOnce` is the card that shows the key once, with the sentence that has to be there;
  `ui.APIKeysTable` lists them by handle and never by key.

- **`trilha.Limiter`** — the token bucket `Config.RateLimit` already used, exported so a limit
  keyed by something other than an IP does not need a second implementation. **`trilha.Pepper`** —
  a keyed hash under the app's secret, for an API key and never for a password.

### Fixed

- **A bug that only showed up sometimes.** The secret was base64url, whose alphabet **includes
  `_`** — the separator in `ak_<handle>_<secret>`. The split broke in the wrong place whenever
  the random bytes happened to encode an underscore: three tests failing, two passing, a
  different set each run. The alphabet is now lower-case base32 — letters and digits and nothing
  else — and the split is `SplitN`.

- Two repeated attributes in the kit's own markup, which the example screen showed:
  `class="ui-input ui-input"` and two `type="button"` on one button.

### Not here

`trilha openapi` marking the key-guarded routes with `securitySchemes`: that is the generator
reading a route's middleware, which is the same static reading
[#124](https://github.com/emersonjoe/trilha/issues/124) records as scanner debt, and it goes in
with it.

## 0.57.0 — 2026-09-08

Spec 075.

### Added

- **`trilha.Seal`, `trilha.Open` and `trilha.Secret` — encrypt this to store it**
  ([#108](https://github.com/emersonjoe/trilha/issues/108)). The framework had a secret, a signer
  and signed cookies, and no way to encrypt a value at rest. What gets written instead is a token
  in the clear, then AES copied off the internet with a fixed IV, then the whole key coming back
  in a `GET` and showing up in the DevTools.

  AES-256-GCM, a random nonce per value, and a key derived from the app's secret with HKDF-SHA256
  under a fixed info string — **the key that encrypts is never the key that signs**. The first
  byte is the format version and travels as additional data, so changing it invalidates the tag
  instead of becoming another format. `Open` tries the current secret and then
  `Config.PreviousSecret`, which is what makes a rotation possible, and answers one error for
  both "not mine" and "cannot open": telling them apart tells whoever is guessing which of the
  two they got right.

  `trilha.Secret` is a string with every accidental exit closed: JSON, `String` and `slog` answer
  a mask, `Reveal()` is the only way to read it, and a form field that comes back **empty or
  masked leaves the stored value alone** — the "leave blank to keep" every settings screen writes
  by hand.

  **`Value()` is the driver method, not the reader.** The issue asked for it the other way round
  and it cannot be: a `Secret` is a string underneath, and `database/sql` converts a
  string-kinded value all by itself — so unless `Value()` is the `driver.Valuer`, passing a
  `Secret` to a query stores the plaintext, silently, which is the exact accident the type exists
  to prevent.

  `Schema` grew a `password` type, so a `Secret` inside a `trilha.Settings` section draws as a
  password field that is always empty, with the mask of what is stored in the help line.
  `ui.SecretField` is the same field for a hand-written form.

  `trilha audit` warns when there is a `trilha.Secret` in the project and no
  `TRILHA_PREVIOUS_SECRET`: rotating then is the moment every stored token stops opening.

### Not here

A key manager. The key comes from the app's secret, and the threat model now says so in both
languages: this is encryption against a database dump and a backup, **not** against the operator
or anybody holding the environment. Pretending otherwise would be worse than not encrypting.
Automatic re-sealing on rotation is also out — the application knows where its values are stored;
the framework has no database to sweep.

## 0.56.0 — 2026-09-08

Spec 074.

### Added

- **`trilha.Settings[T]` — the configuration an administrator changes without a deploy**
  ([#107](https://github.com/emersonjoe/trilha/issues/107)). Every application writes this by
  hand: a settings table, a GET that answers JSON, a PUT, a screen with one form per section —
  and validation in none of them. Here the struct is the configuration and the screen comes from
  it.

  The tags do three jobs and none of them is new: `json` stores, `form` names the input (the same
  name `Bind` already reads), `validate` is the rule. `label` and `help` are what a person reads.
  `ui.SettingsForm` draws one field per field, of the type the tags asked for, and
  `trilha.SchemaOf[T]()` is that reflection on its own — the form of any struct, from its own
  tags.

  **A 422 saves nothing**, which is the half every hand-written settings page gets wrong: it
  validates on the screen and saves anyway. **A section written by an older version of the struct
  does not bring the app down**: a renamed field keeps its default, and an unreadable value falls
  back to the defaults with a line in the log, because an empty configuration in production is
  worse than an outdated one. **The audit line names the fields that changed and never their
  values** — a settings page is where a token lives.

  `SettingsStore` is two methods over whatever the app already runs; nil keeps the section in
  memory and says so once.

### Not here

`trilha.Secret` and `ui.SecretField` (they are [#108](https://github.com/emersonjoe/trilha/issues/108),
still open — what would have leaked first, the audit copying values, is already closed);
`settings.SQL` (no DDL in a framework with no database dependency, same as `audit.SQL` in
0.49.0); and `[]string` with `oneof` as checkboxes — `Schema` has no multiple-choice type, and
inventing one here would be deciding for another issue in the corridor. That is why a slice is
the case that panics, with a message saying what a settings struct holds.

## 0.55.0 — 2026-09-08

Spec 073.

### Added

- **`ui.Tree` and `ui.TreePicker` — the hierarchy, and the field that picks one node of it**
  ([#101](https://github.com/emersonjoe/trilha/issues/101)). A tree with thousands of nodes is
  the component people go to npm for: expanding, searching and the keyboard are each easy and
  together are three hundred lines. The kit had `ui.Combobox` for a flat list and nothing for a
  hierarchy.

  **A node is `<details>`, and that is the whole no-JavaScript story.** The script draws nothing:
  it fetches the children the first time a branch opens, instead of asking for a whole page. A
  node whose children already came from the server asks for nothing — which is how the path down
  to the current node arrives open and complete on the first render, including after a 422
  brought the form back.

  **The picker posts a radio.** No hidden input to keep in sync, no text to resolve on the
  server: somebody with no script browses the same `<details>` and picks the same radio, and the
  form posts the same field.

  The roles are the real ones and the keyboard is the real one — arrows through what is visible,
  `Home`/`End`, `*` to expand everything — with only the first node in the tab order, because a
  tree is one stop and the arrows move inside it. `ui.tree.js` is optional, like `ui.nav.js`: a
  page with no tree does not download it.

### Fixed

- **`AddRule` panicked with "already registered" when `Setup` ran twice** — and its own doc
  comment says to register rules *in Setup*, which is exactly what a test suite does once per
  test. The same function registering again is now accepted (compared by pointer); two different
  functions under one name is still a panic, which is the bug the guard exists for. Same trip
  `RegisterEnum` took in spec 064, same way out.

- **Two `role` attributes on one element.** The picker's tree emitted `role="tree"` and
  `role="group"` together — not a stronger promise, an invalid one. A tree of radios is a field
  and announces itself as a group of choices; a tree of links is a navigation; the choice happens
  once. An empty `Label` no longer becomes `aria-label=""` either: naming a field with nothing is
  worse than not naming it, because it hides whatever the `<label>` around it would have said.

### Not here

Expanding everything without script (`*` is the keyboard, and the keyboard is script) and virtual
scrolling (it would be a second rendering path in JavaScript, which is the opposite of what this
component does).

## 0.54.0 — 2026-09-08

Spec 072.

### Added

- **`ui.AuditTable` — the screen that reads the trail**
  ([#128](https://github.com/emersonjoe/trilha/issues/128)). 0.49.0 gave the framework `c.Audit`
  and left this out with the reason written down: it depended on `c.CSV`, and without the export
  button it was a `DataTable` with five columns. `c.CSV` shipped in 0.50.0.

  It **is** a `DataTable` underneath, and that is the point: the filter form, the ordering links,
  the pagination and the fragment swap are the ones every other listing already has. A trail that
  behaved differently from the rest of the app would be a second thing to learn, with its own
  copy of four mechanisms that already exist.

  **Reading the trail is still the application's job.** `Config.Audit` is a write interface with
  one method and did not grow a `Read`: the framework has no database, and the query behind this
  screen — a period, an actor, a table this app chose — is not something it could write. The
  example's is thirty lines over a slice.

  `Fields` is a detail and not a column: each action carries its own keys, so a column per key is
  a table that grows a column every time somebody audits something new. The keys come out sorted,
  because a detail that shuffles between two loads of the same screen is a detail nobody trusts.

  The export button carries the query that is on screen — an export that ignores the filter in
  front of somebody is an export of the wrong thing, and they only find out in the spreadsheet.

### Fixed

- **The `local-login` example did not declare `Config.Locale`**, so a component carrying its own
  text spoke English inside an application written in Portuguese. Same finding as 0.50.0 in the
  blog example, in the other example.

### Not here

The period filter the issue asks for. A date range belongs to the application's query, and a
pair of fields here would only be worth it if the component also built that query — which is
exactly what it deliberately does not do. `q` and the action cover the common path; the example
shows where the rest goes.

## 0.53.0 — 2026-09-08

Spec 071.

### Added

- **`c.Draft` — where step one lives while somebody is on step two**
  ([#103](https://github.com/emersonjoe/trilha/issues/103)). A form in several screens asks one
  hard question, and it is not the HTML. What gets written instead is a page full of
  `<input type="hidden">` (which the first upload breaks), a half-filled row in the database
  (which every report then has to learn to ignore), or one enormous screen with everything on it.

  `c.Draft(name)` has three methods — `Load`, `Save(v, ttl)`, `Clear` — and keeps the draft in a
  **signed cookie** under 2 KB of JSON: nothing to configure, nothing to clean up, and it expires
  on its own. Above that it needs `Config.Drafts`, three methods over whatever the app already
  runs. **Without a store, `Save` returns an error naming that field** rather than setting a
  cookie the browser would drop without a word — a form that loses step one in silence is the
  worst outcome available here. (The limit is 2 KB and not the 3 KB the issue proposed: what goes
  in the cookie is base64 of the draft plus an expiry and a signature, and 3 KB of JSON crosses
  the browser's 4 KB.)

  `ErrNoDraft` is an answer, not a failure — never saved, finished, expired, tampered with, or
  another browser — and it is what sends somebody back to step one. A draft written by an older
  version of the struct answers the same way: the field was renamed between deploys, and starting
  over beats a 500 in the middle of somebody's form.

  A draft is signed, so it cannot be edited by hand, and it is **not secret**: what is in a cookie
  travels to the browser and can be read there.

- **`ui.Steps`** draws the indicator: steps already done are links, the current one carries
  `aria-current="step"`, and the ones ahead are plain text — a wizard where step three is one
  click away is a wizard whose steps did not have to happen in order.

- The **"A form in steps"** recipe, in both languages, and a three-screen wizard in
  `examples/cadastro`. The part of it worth copying is not the framework call: it is **one struct
  per step**, each with only its own rules.

### Not here

`Draft.Attach` and `Draft.Attachment`, which the issue asks for. They need what the framework
does not have — a temporary directory, a token, a janitor, and deleting the file when the draft
expires. The issue describes them as if that lifecycle already existed ("cleaned up along with
the temporary uploads"); it does not, and building a second private copy of it inside `Draft` is
the duplication that [#114](https://github.com/emersonjoe/trilha/issues/114) (`trilha/blob`)
exists to avoid. The recipe shows step one saving the file and keeping its path in the draft.

## 0.52.0 — 2026-09-08

Spec 070.

### Fixed

- **`Inline` could not be shown in place, which is the one thing it exists for**
  ([#102](https://github.com/emersonjoe/trilha/issues/102)). Every response goes out with
  `X-Frame-Options: DENY` and `frame-ancestors 'none'`, and a document carrying those cannot be
  framed by anything, including a page of the same app. `Inline` now relaxes that pair to
  same-origin **on that one response**. `Attachment` keeps `DENY`; an app that wrote its own
  `Security.CSP`, or marked `Security.Delegated`, is left exactly as it was.

  **The advice this repository gave for the blank iframe was wrong, in four places** — the
  `Inline` doc comment, the `Ctx` reference in both languages, and the "From Next.js" recipe.
  All of them said to add `frame-src 'self'` to `Security.CSPExtra` on the page doing the
  framing. The default policy already has `default-src 'self'`, which covers `frame-src`: the
  page was never the problem. Anybody who followed it changed a policy that was not blocking
  anything, kept the blank frame, and lost the trail. The browser had been saying so all along:
  *"Framing … violates the following Content Security Policy directive: frame-ancestors
  'none'"* — the framed answer's policy, not the page's.

### Added

- **`ui.Preview`** — the file beside its metadata, which is the screen every document
  application has. A bar with the title, download and "open in a new tab"; a frame for what a
  browser renders; an `<img>` for an image, where a click opens the full size (the zoom, with
  no script); and for a type `Inline` refuses — HTML, SVG, XML — a card saying it cannot be
  previewed, with the download button, instead of a blank frame that explains nothing.

  **The sandbox is chosen per type, and that was measured.** The browser's PDF viewer refuses to
  run inside a sandboxed frame: the request comes back blocked and the frame is blank with
  nothing in the console. The two flags that would bring it back — `allow-scripts` with
  `allow-same-origin` — are precisely the pair that lets a same-origin document take its own
  sandbox off, so the attribute would be a label and not a fence. A PDF is framed without one;
  text and images keep `allow-same-origin`, which was verified rendering.

- **`trilha.CanInline(ctype)`** answers, from outside, the question `Inline` answers inside: is
  this a type a browser shows rather than runs. The blog example had a hand-written second copy
  of that list, and the second copy is the one that falls behind.

- **`trilha audit` warns about a hand-written `<iframe>`**, with the advice pointing at the
  framed answer rather than at the page.

### Changed

- The Portuguese threat model was three rows shorter than the English one. The two missing rows
  about downloads are back, and both now carry the row about framing.

## 0.51.0 — 2026-09-08

Spec 069.

### Added

- **`ui.Defer` — serve the page now, fill the slow part a moment later**
  ([#98](https://github.com/emersonjoe/trilha/issues/98)). Server-side rendering makes the
  loading skeleton disappear, because the page arrives ready. It disappears for a reason the
  beginner meets again from the other side: a dashboard that needs seven queries to draw now
  waits for the slowest of the seven. `ui.Defer(c, id, src, opts)` renders a placeholder and
  asks for that fragment as soon as the page has loaded — once, with no clock, carrying the
  session and the headers of the page it sits in.

  It is `ui.Poll`'s machinery with the clock left out, which is why `Then: ui.Poll("30s", src)`
  loads now and watches from then on with no extra code: the attributes travel on the same
  element and the route's answer decides the rest.

  **A fragment that fails shows a message and a "try again" in the hole**, not a skeleton
  pulsing for ever — and that block is rendered on the server and hidden, so every sentence and
  every class stays on the Go side and the behaviour never has to know a language. **Without
  JavaScript the placeholder carries a `<noscript>` link** to the same route, which answers as a
  page: the slow part is one click away instead of missing.

  `Height` is not decoration: a placeholder shorter than what replaces it makes the page jump
  under the cursor of somebody who had already started reading.

### Fixed

- **The blog example's copy of the kit was stale.** `public/ui.css` had been sitting at the
  version from spec 064, without the enum tones. The `local-login` example guarded one file
  (`ui.live.js`); the blog guarded none, and the browser — not the suite — is what showed it:
  the attribute was in the HTML and the behaviour never came. The blog now checks every
  `public/ui*` against what the package embeds.

### Not here

`E_NESTED_DEFER` in `gen`, which the issue asks for. Nesting is dynamic — route A defers to B,
and B's own page has a defer of its own — so a scanner that cannot see across routes would only
catch the case nobody writes. What is here instead is the guarantee that matters: a set of ids
already asked for, so a fragment that answers with a defer of its own id cannot ask for ever.

## 0.50.0 — 2026-09-08

Spec 068.

### Added

- **`c.CSV` and `trilha.BindCSV` — the spreadsheet, both ways**
  ([#109](https://github.com/emersonjoe/trilha/issues/109)). Every internal application does
  this twice: a button that downloads the list, and a screen that takes it back. Both fail in
  the same few places, and none of them is interesting — which is exactly why nobody gets them
  right. On the way out: no BOM, so Excel opens every accent as mojibake; a comma where that
  person's Excel expects a semicolon, so the file opens as one long column. On the way in:
  "error in the file", which leaves somebody to find one bad date among four thousand rows by
  eye.

  `c.CSV(name, rows)` takes a slice or a receive-only channel — the channel is what a
  two-hundred-thousand-row export wants, written as it is produced with the write deadline
  lifted. `Config.Locale` decides the separator, the date format, the decimal mark and the word
  for a boolean. **The BOM is the one thing with no option**: there is no application for which
  mangled accents are the desired behaviour.

  `trilha.BindCSV(r, &rows)` detects the separator and the BOM, matches the header by tag in
  any order, and **validates each row with the same `validate` tags a form uses** — a rule
  written once holds on the screen and in the import, translation included. `res.Errors` is
  `{Line, Column, Message}`, with the line taken from the reader rather than from a counter of
  our own: one cell containing a newline would otherwise shift every message after it by one.

  A required column missing from the header is one message at line 1, not the same message on
  ten thousand rows. A heading no field claims is a warning, because a spreadsheet grows a
  column all the time and refusing the file for it only teaches people to delete columns before
  uploading. Only rows that pass are appended, and `MaxRows` (100,000) is an error rather than
  a truncation — half an import that reports success is worse than one that fails.

- **`ui.CSVErrors`** renders the rejected cells as a table of line, column and problem, twenty
  at a time, with the warnings under them.

- **`Upload` is an `io.Reader`.** One line, and it is what lets `BindCSV(up, &rows)` be the
  call without the caller reaching into the struct for `.File`.

### Changed

- **The blog example declares `Locale: "pt-BR"` and `TimeZone: "America/Sao_Paulo"`.** It is an
  application written for people who read Portuguese, and until now it said so everywhere
  except in the one place that decides what a spreadsheet, a date and a number look like.

### Not here

`iter.Seq` for the export, which the issue asks for: this module builds on Go 1.22, where it
does not exist, and the channel covers what that part of the issue actually wanted. The
issue's `c.Draft` comes from [#103](https://github.com/emersonjoe/trilha/issues/103), which
does not exist yet; the recipe shows the flow without it.

## 0.49.0 — 2026-09-08

Spec 067.

### Added

- **`c.Audit` — who did what, in one line**
  ([#104](https://github.com/emersonjoe/trilha/issues/104)). Every internal application ends up
  needing the trail, and the framework already held half of it: the request id, the client IP
  behind `TrustedProxies`, the session, the route pattern. What was missing was the sentence
  and a place for it. What the beginner does instead is `slog.Info("deleted", "id", id)`, which
  loses the actor, the address and the request id — or a table that half the handlers forget to
  write to. Both failures are the same one: the information was right there and nobody joined
  it up.

  `Config.Audit` is an interface with one method, because the decision an application actually
  makes is *which table*, not *which shape*. Leave it nil and the record goes to the logger
  with `kind=audit`, which is enough to grep and enough to ship a first version with.

  **A sink that fails does not take the response with it.** The error is logged, loudly, and
  the request carries on: the document was deleted either way, and refusing to answer now would
  lose the trail *and* confuse the person who did it — two failures instead of one.

  `auth` fills in the actor, so any route behind `Require`, `RequireRole` or `RequirePolicy` is
  attributed without the application writing a line; an application that authenticates its own
  way calls `c.SetActor` once. Nobody recognised is recorded as `anonymous` and **is recorded**:
  a trail that silently drops the anonymous action has a hole exactly where somebody would
  look. `trilha audit` warns when `c.Audit` is called in a project where no route requires a
  session.

  `Route` is the pattern and not the path — the concrete id is already in `Target`, and the
  pattern is what lets a query group a thousand deletions into one row.

### Changed

- **`auth` sets the session and the actor in one place.** There were five `c.Set(ctxKey, u)`
  scattered through the package; five places doing two things is where the fifth forgets one of
  them.

### Not here

`audit.SQL` and `ui.AuditTable`, both asked for by the issue. The first would put DDL for two
SQL dialects into a framework that has no database dependency at all — a bigger decision than
this change, and the one-method interface exists precisely so the application writes its own
`INSERT`, as the example does. The second depends on `c.CSV`
([#109](https://github.com/emersonjoe/trilha/issues/109)), which does not exist yet; without
that button it is a `DataTable` with five columns.

## 0.48.0 — 2026-09-08

Spec 066.

### Added

- **`ui.Date`, `ui.Bytes`, `ui.Duration`, `ui.Number`, with `Config.Locale` and
  `Config.TimeZone`** ([#99](https://github.com/emersonjoe/trilha/issues/99)). The framework
  says, rightly, that it has no locale and that money belongs to the application. But a date, a
  size, a duration and a count are not domain: they are the same everywhere, and every
  application writes them again — fifteen screens with a hand-rolled date formatter in the
  measured app, all of them with the same beginner's bug, which is no time zone, no
  `<time datetime>`, and no answer for null.

  A missing value renders `—` and never `01/01/0001`. The text is local and translated while
  the `datetime` attribute is always the instant in RFC 3339 UTC, so a sort, a copy-paste or a
  screen reader gets the fact and not the presentation. `Bytes` is base 10, which is what the
  reader's own file manager shows them, with the exact count in the title.

  `Relative()` writes "3min ago" and does not move: the kit has no clock and does not want one
  — a screen that needs the number to keep changing puts the piece in a `ui.Poll`, which is a
  decision the page makes and pays for once.

  An unknown `TimeZone` falls back to UTC **and says so in the log**. Falling back in silence
  would shift every timestamp on the screen with nothing looking broken. The zone is resolved
  once and remembered, failure included: loading one reads the filesystem, and a page with
  fifty dates would otherwise read it fifty times.

  Money stays out. The currency, where the symbol goes and how a negative reads are the
  application's to decide, and a framework that guessed would be wrong in somebody's country.

- **`trilha audit` warns about a `time.Format("02/01/2006")` inside `app/`** — a layout in the
  page ignores `Config.TimeZone`, which is how a date shown to somebody in another country ends
  up simply wrong.

### Note on the signature

The issue proposed `ui.Date(t)`; these take a `*Ctx`. A package-level language would be shared
by two applications in one process — which `trilha.Provide` and the embedded app exist to
support — and the second one to boot would silently change the first. Spec 046 moved the
reference app off package state for that exact reason.

## 0.47.0 — 2026-09-08

Spec 065.

### Added

- **`ui.Empty` and `ui.EmptyError` — the empty state, and the one that failed**
  ([#97](https://github.com/emersonjoe/trilha/issues/97)). Thirty-five screens of the measured
  application have an empty state written by hand and no two are alike; whoever starts writes
  a `<p>` and leaves it there, and a `<p>` says the screen is empty without saying what to do
  about it. `Hint` is the field that earns its place — "Send the first PDF and classification
  starts on its own" is the difference between a dead end and a next step.

  An icon the kit does not carry draws nothing instead of panicking: `ui.Icon` panics on an
  unknown name, which is right for a page somebody is writing and wrong for a component that
  draws whatever an option happens to hold — and the screen that is already empty is the worst
  place for a 500.

  `ui.EmptyError(c, title, err, action)` shows the real error **only in development**. A
  driver's sentence on a production page is an information leak with a friendly font.

### Changed

- **`ui.DataTable` tells an empty list from a list filtered down to empty.** Saying "nothing
  here" to somebody who just searched for *xyz* tells them the application is empty, when what
  happened is that their term matched nothing — nine screens of the measured app get this
  wrong. With no search it draws "Nothing here yet"; with `?q=xyz` it names the term and
  offers a link that clears it **and goes back to the first page**, because clearing the
  search while keeping `page=2` only leads to another empty screen. `ListState.Empty` still
  replaces both, which is what an application that does not speak English wants.

## 0.46.0 — 2026-09-08

Spec 064.

### Added

- **`trilha.Enum` — a domain list declared once**
  ([#96](https://github.com/emersonjoe/trilha/issues/96)). A status, a document type, a
  pipeline stage: written by hand it lives in four places — the badge on the table, the
  options of a select, the validation of the form and a comment on the tag — and the fourth
  one is where the label is wrong. `examples/cadastro` had exactly that before this: two
  radios with the values typed in, a badge that printed the raw value, so somebody who chose
  *Mensal* saw `mensal` in the list.

  One declaration now answers all four. `Tone` is one of six names from the theme and never a
  CSS class — whoever declares a status picks a meaning, and picking a colour is how two
  screens end up with two different greens.

- **`ui.Status(enum, value)`** — the badge: the label, in the tone the enum declared. A value
  the list does not know renders raw and muted, because a row written before somebody retired
  that value must not take the screen down.

- **`enum=<name>` in the validate tag**, with `trilha.RegisterEnum` naming the list in `Setup`.
  The message lists the labels and not the values: the person filling the form read labels.
  Registering the same list again is fine — `Setup` is where this belongs and a test suite
  boots the app once per test — while two *different* lists behind one name panic, because the
  form would validate against one and the select would draw the other.

### Note

The issue also asked for `trilha ctx` to list the registered enums. It is not here: the
registry is filled at run time and `ctx` reads code, so telling an agent which values exist is
scanner work. That, and the same pending item spec 063 left behind, are being tracked
separately.

## 0.45.0 — 2026-09-08

Spec 063.

### Added

- **`auth.Policy` — the permission matrix as data**
  ([#100](https://github.com/emersonjoe/trilha/issues/100)). A role list answers "is this
  person an admin"; what an application asks is "may this person edit documents", and that
  answer is a matrix: role × module × level. `RequireFunc` was the right hook and the wrong
  blank page — whoever gets `func(*User, *Ctx) bool` writes
  `u.HasRole("admin") || (u.HasRole("analyst") && module == "docs")` on the third screen and
  gets it wrong on the fourth.

  Declared once, it answers in the four places the application repeats itself: the middleware
  that guards (`RequirePolicy`), the button that hides (`Can`), the 403 that explains, and the
  screen that edits it (`ui.PolicyGrid` with `auth.BindPolicy` on the other side).

  The order of `Levels` is the whole meaning — `manage ⊇ edit ⊇ view` is what lets a cell hold
  one value instead of three booleans — and a module a role does not name is a module it cannot
  reach: the absence is a denial, never an inheritance, so a role that was deleted loses access
  rather than inheriting somebody else's.

  The 403 says what was missing (`needs edit on docs`), because that sentence is what the
  person repeats to whoever administers the application; a bare "forbidden" turns a two-minute
  fix into a support thread.

  For a matrix people edit, `auth.PolicyStore` (two methods) and `auth.PolicyFrom` read the
  roles from a table while the modules and levels stay in the code — one is the shape of the
  application, the other is what an administrator invents on a Tuesday. `PolicyFrom` returns a
  snapshot on purpose: a value that changed underneath a request would let one request answer
  twice, allowed at the middleware and denied at the button.

- **`ui.PolicyGrid`** — the administration screen, without JavaScript: one row per role, one
  column per module, a select of levels per cell, fields named `grant.<role>.<module>`. It
  takes a four-method interface and not `auth.Policy`, because the kit must not drag
  authentication into every application that draws a button.

- **`trilha audit` warns about a module no route requires.** The matrix says the area is
  protected; if nothing asks for it, the protection is a sentence in a file.

### Note for whoever read the issue

The proposed `var Middleware = auth.RequirePolicy(...)` does not work: `middleware.go` has to
export a *function* with the middleware signature, and a var of the right type is not one — the
scanner says so, in those words. The documented idiom is two lines, and the example uses it.

## 0.44.0 — 2026-09-08

Spec 062. Three things that only showed up when 0.41.0–0.43.0 was run against a real
application instead of the fixture.

### Fixed

- **`auth.CheckPBKDF2` reads the hash a Python app writes by hand**
  ([#92](https://github.com/emersonjoe/trilha/issues/92)). `auth.Sessions` promised that an
  existing users table stays valid without a password migration, and then read only Django's
  spelling. The table that asked for the feature is not Django's: `hashlib.pbkdf2_hmac`
  returns bytes and no format at all, so an app that uses neither Django nor passlib picks
  one, and it picks `pbkdf2$<iterations>$<salt hex>$<digest hex>`. Three differences, and each
  one alone was enough — the prefix, the salt that is hex decoded to bytes before the
  derivation, the digest in hex. A login that refuses the right password is the worst way to
  fail: it looks like the person typing got it wrong. `CheckPBKDF2` now reads both spellings,
  told apart by the prefix; `HashPBKDF2` keeps writing one. SHA-256 only, comparison still
  constant-time, a hash it cannot read still `false` and never a panic.
- **`trilha client` gives back a pointer for `Optional[T]`**
  ([#95](https://github.com/emersonjoe/trilha/issues/95)). Pydantic writes every optional
  field as `anyOf [T, null]`, which is not a union — it is "T or nothing" — and in a FastAPI
  document it is the most common shape there is: 39% of the fields measured. Carrying those
  as `json.RawMessage` made the caller marshal by hand exactly where the generated client was
  supposed to help. Two members with one of them `null` now become `*T`; a real union — two
  types, or three members — stays `json.RawMessage` and stays in the report.
- **`trilha migrate next` reads the component beside the page**
  ([#93](https://github.com/emersonjoe/trilha/issues/93)). Three faults, all of which sent the
  agent to the wrong screen first. The classification read only `page.tsx`, and in a real Next
  app the page is thin: a fifty-line page importing three hundred lines of pointer handling
  came back as the easiest class there is. It now follows relative and `@/` imports — three
  levels, thirty files — runs the signals over all of them, and the printed reason names the
  file that produced the signal, `C — pointer (fluxos/FlowCanvas.tsx)`. The size column adds
  them up, because a fifty-line page in front of a three-hundred-line component is not a
  fifty-line port. Dropping a file is now `upload` and not `pointer` — the two signals
  together are what tell a drop area from something being dragged — so the upload screen stops
  landing in class C. And `apiGet<DocsResponse>("/x")` is a call again: the generic made the
  Calls column of whole screens come back empty, which is the column that says which endpoint
  of the generated client to use.

## 0.43.0 — 2026-09-08

Spec 061, the first of the two pieces of
[#50](https://github.com/emersonjoe/trilha/issues/50).

### Added

- **`trilha mcp` — the project as tools, over stdio.** An agent with a shell does not need
  this; `trilha ctx --json` already is the answer, which is why the issue left it for last.
  This is for the agent that has no shell — a chat client, an editor that speaks MCP and
  nothing else — and it offers what the commands already answer: `describe_project`
  (`trilha ctx --json`), `routes`, `check` (`trilha check --json`), `ui_describe` (the
  catalogue the binary already carries, so it needs neither a process nor a project) and,
  with `--write`, `generate`.

  Every tool is a wrapper, and the end-to-end test holds it to that: the tool answers what
  the command answers, byte for byte. There is no second implementation to drift.

  Point a client at it with `{"command": "trilha", "args": ["mcp"], "cwd": "/path/to/project"}`.
  The `cwd` is what decides the project, and nothing the model sends can change it.

  **It is read-only unless you ask otherwise.** Without `--write`, the tool that writes files
  is not registered at all — it is absent from `tools/list`, so a call for it comes back
  `unknown tool` rather than as a refusal a model can argue with. It never uses a shell:
  every command is a program plus an argument slice. Every argument is checked against an
  allowlist before it can reach a command line, so `/../../etc/passwd`, `/x; rm -rf /` and
  `--force` stop with the reason and without running. One command at a time, each with a
  deadline (ten minutes for `check`, which runs your suite) and a 1 MiB cap on output. No
  network. Every call that does run is written to stderr first — stdout belongs to the
  protocol — so you can watch what the agent asked for.

  Reference in both languages, and `AGENTS.md` now says which of the two to reach for: if you
  have a shell, run the command.

### Not yet

The other half of #50, a hosted documentation server answering `search_docs` / `get_page` /
`get_recipe`, is still open. Its source is the same Markdown the site is built from — about a
megabyte — and putting that inside the CLI binary to answer questions about a project it is
not part of is the wrong trade; `internal/uidoc` already records that constraint by shipping a
generated catalogue instead of the source. Serving it needs somewhere to run, and the site is
static on GitHub Pages: that is a hosting decision, not code.

## 0.42.0 — 2026-09-08

Spec 060. One thing the 0.41.0 island channel shipped twice.

### Changed

- **The island runtime is a kit file, `public/ui.island.js`, and `Ctx.Island` links it with a
  `<script src>`** instead of writing an inline script. The channel handed to a mount function
  existed in two places — minified inline in `island.go` for the page without the kit, and
  readable in `ui.js` for the island arriving inside a swapped fragment — because neither one
  could reach both cases. They had already drifted: only the `ui.js` copy restored focus,
  returned the caret to the field in use, hydrated what came back and ran the transition, so
  what `island.swap` did depended on whether the page happened to load the kit. A file is
  reachable from both, so there is one implementation now, and `island.swap` chooses
  `window.ui.swap` when the kit is there and replaces directly when it is not.

  Two things follow. There is **no inline island script left**, so `script-src 'self'` is all
  the CSP needs for this and the nonce is one requirement lighter; and the runtime is cached
  with a content hash instead of travelling in every response that has an island. `ui.js` lost
  4.1 KB.

  **Migration**: a project that already uses `c.Island` needs `trilha ui` once, to write the
  new file. `trilha audit` — and so `trilha check` — fails with a critical when a project calls
  `c.Island` without it, because the failure is silent otherwise: the fallback shows and
  nothing happens.

### Added

- **`trilha.IslandRuntime`** — the kit file name the script tag points at.

### Removed

- **`window.ui.mountIslands`** from the kit's public JavaScript. Mounting belongs to the
  runtime; a test fails if the channel or the mounting comes back into `ui.js`. The test that
  used to compare the two copies string by string is gone with them — a test whose job is to
  hold two implementations in step is a symptom, and it did not hold.

### Fixed

- **The suite runs on Windows again** ([#88](https://github.com/emersonjoe/trilha/issues/88)).
  The `windows` job failed on the 0.41.0 release commit with the error 0.39.2 had fixed: four
  new e2e tests build the CLI and then execute it without the extension Windows requires. The
  product was never affected. Four tests forgetting the same thing at once is a sign the
  knowledge was in the wrong place, so `buildCLI` now owns it and the eighth test cannot get
  it wrong.
- **`trilha migrate next` and `trilha client` print a path the way the rest of the CLI does**.
  Both were writing the native separator, so a migration report on Windows listed routes as
  `app\page.go` while `trilha new` writes `app/api/hello/route.go` and `trilha vendor` already
  calls `filepath.ToSlash` on its own line.

## 0.41.0 — 2026-09-08

The release of the app that already exists somewhere else. An API in another language keeps
answering behind `Config.Upstreams` while Go takes it over one endpoint at a time; the users
table that is already there logs people in without OIDC; `trilha migrate next` writes the
`app/` skeleton of a Next.js project and `trilha client` turns the API that stayed where it
is into Go types. The other half is the screens an internal app is made of — a listing whose
whole state lives in the URL, a fragment that refreshes itself, a shell, a dashboard, forms
that grow — and the island, which can now talk back to the server.

### Added

- **The island talks back: `island.post`, typed props and vendored modules**
  ([#70](https://github.com/emersonjoe/trilha/issues/70)). An island could render and it could
  read its props, and that was the end of it: writing back meant rediscovering the CSRF token
  the framework already mints, the headers it already agreed on and the error shape it already
  answers with — in every island, by hand. The mount function now takes a third argument:
  `export default function (el, props, island)`. `island.post(url, data)` sends JSON with the
  token on it and gives back what the route answered, `island.get` reads, `island.send` covers
  the other methods, `island.swap(url, id)` replaces a fragment the way a link with a target
  does, `island.csrf()` is the token and `island.signal` is an `AbortSignal` aborted when the
  element leaves the page, so an island swapped out of a fragment stops writing to what is no
  longer there. A `422` arrives as an `IslandInvalid` whose `.fields` is the same object a form
  would have shown, anything else as an `IslandError` with `.status` and `.detail`, and a
  `Trilha-Location` is followed as a navigation — POST → redirect → GET from an island too. All
  of it lives in the loader that already ships with the first island of a response: no second
  module to download, and `ui.js` did not grow. The double-submit cookie is `HttpOnly` on
  purpose, so the token reaches the island written into the element as `data-trilha-csrf` — the
  same token `CSRFInput` puts in every form of that response.
- **`trilha gen` writes `public/islands.d.ts`**
  ([#70](https://github.com/emersonjoe/trilha/issues/70)). The generator reads the `c.Island`
  calls in `app/` and writes one TypeScript interface per props struct, the map from module
  path to props, and the `island` object itself, so an editor checks both sides of the boundary
  without anybody writing the types twice. Field names come from the `json` tags, a pointer or
  an `omitempty` is optional, an embedded struct is flattened the way `encoding/json` flattens
  it, a struct that points at itself stays a name. Props given as a map literal or a variable
  have no name to hang a type on: the island is still declared, typed `unknown`, and the
  command says so out loud. `gen --check` compares the file like it compares `trilha_gen.go`,
  and an app whose last island is gone loses the file.
- **`trilha vendor`: one JavaScript module, downloaded once and pinned**
  ([#70](https://github.com/emersonjoe/trilha/issues/70)). `trilha vendor preact@10.19.3`
  writes `public/vendor/preact.js` and records the name, the version, the sha256 and the URL in
  `vendor.lock`, which is committed. It resolves nothing — no dependency tree, no
  `node_modules`, no install step — because a module that needs a resolver is the wrong module
  for an island. `--check` re-hashes the files in CI, `--from` and `TRILHA_VENDOR_BASE` point
  the download somewhere other than the default `https://esm.sh`, and `trilha audit` warns
  about a file under `public/vendor/` that `vendor.lock` does not name. None of this is a
  dependency of the framework: the file is served like any other asset and the island imports
  it by path. New recipe, in both languages: *A React component as an island*.
- **`trilha.RequireCSRF`**, the middleware for the route that answers an island. A `route.go`
  is an API and an API does not check the token, because its client carries a bearer token
  rather than a cookie; an island is the exception, because its client is the page. Putting it
  in the route's `middleware.go` asks for the token without turning the route into a page — the
  errors stay `problem+json`, which is what the island can read — and `trilha audit` reads it
  as the answer to its open-writes warning instead of one more case of it.

- **The API that stayed where it is becomes Go types: `trilha client`**
  ([#61](https://github.com/emersonjoe/trilha/issues/61)). Trilha wrote the OpenAPI document
  of its own routes; it could not read anybody else's. A migration in phases — the front
  becomes Trilha, the API stays put — meant the page had `map[string]any` where the shape of
  the answer should be, and the only source of truth for that shape was somebody's Python.
  `trilha client openapi.json` (a URL works too) writes one deterministic file, committed like
  `trilha_gen.go` and checked in CI with `--check`: a struct per schema with `json` tags and
  the `validate` tags of spec 027, so the same type is the answer of the API and the `Bind` of
  a form; a method per operation grouped by tag, with path parameters in the signature, query
  parameters in a struct, an upload as `io.Reader` that streams instead of buffering, and a
  binary answer as the `*http.Response`, which is what `c.Pipe` wants. A status outside 2xx is
  an `*Error` carrying the `Detail` of `problem+json` or of the `{"detail": ...}` a FastAPI
  writes, and `New(base, WithHeader(...))` is the only place a credential appears — the client
  keeps none of its own. `allOf` is flattened, a `$ref` that closes a cycle becomes a pointer,
  and what the generator will not guess at — `oneOf`, a schema with no type, an operation with
  no `operationId` — comes through as `json.RawMessage` or an invented name, each one a line
  of the report rather than a silent decision. The generated file imports the standard library
  and nothing else, not even Trilha, so it also runs in a job and in a test. The recipe *An app
  in front of an existing API* now has both halves side by side: the page reading through the
  client, the islands reading through `Config.Upstreams`, one session token for both.

- **The first day of a migration is one command: `trilha migrate next`**
  ([#59](https://github.com/emersonjoe/trilha/issues/59)). Moving a Next.js app used to start
  with a week of folder archaeology — which routes exist, which pages are really client, what
  each one calls — before a single screen could be ported. The command reads the project and
  writes the two mechanical halves: the tree of `app/`, one Go file per screen with the names
  Trilha expects (`[id]` → `id_`, `[...path]` → `path__`, `(group)` → `group-`, `not-found` →
  `not_found.go`), and a `MIGRATION.md` with a row per screen — source file and its line
  count, where it landed, the URL, whether it was `'use client'` and with which hooks, the
  endpoints it called, and which of three shapes it probably is: **A** a form or list with no
  island, **B** a page with one island, **C** an app that really is a client. The suggestion is
  printed with the reason that produced it, and the same reason is a doc comment above the
  function, so whoever ports the screen reads it where the work happens. What has no
  equivalent here is a report line rather than a file that lies: loading states, templates,
  parallel and intercepting routes, `middleware.ts` and the rewrites in `next.config`, each
  with the sentence saying what takes its place. The skeleton compiles as it is — `trilha
  gen`, `go build` and `go vet` pass on it with no edits — nothing is overwritten without
  `--force`, and `--dry-run` prints the report without touching the disk. The screens
  themselves are not translated: the body of a page is business logic, and the cookbook page
  *From Next.js to Trilha* is the reference for porting it by hand.

- **The kit answers what it has: `trilha ui describe`, and the guide for whoever comes from
  Next.js** ([#73](https://github.com/emersonjoe/trilha/issues/73)). Writing a screen with the
  kit meant knowing 137 names by heart or opening `ui/` and reading. `trilha ui describe`
  prints the catalogue — every component grouped, one line each — and `trilha ui describe
  Field` prints the signature, what it is for, the fields of the options struct, an example
  and the symbols the documentation cites; the name may be typed `Field`, `field` or
  `ui.Field`, an unknown one exits non-zero with the closest matches, and `--json` hands the
  whole thing to a tool. The catalogue comes from the kit's own doc comments through
  `internal/uidoc`, is generated by `make golden` and committed, so it ships inside the binary
  and cannot describe a version of the code that no longer exists; two tests keep the source
  honest — every exported symbol has a doc comment, and everything whose signature is not
  self-explanatory has an example. The new cookbook page, *From Next.js to Trilha*, is the
  other half of the same problem: the React pattern you reach for without thinking —
  `useEffect` + `fetch`, `useSearchParams`, `setInterval`, `toast()`, `useState(open)`,
  `dangerouslySetInnerHTML` — beside the line of Go that replaces it, with the Go half
  compiled from `examples/cookbook/next.go`.
- **Markdown that cannot become markup, and a chat that is two calls: `ui.Markdown`,
  `ui.Chat` and `ai.Serve`** ([#72](https://github.com/emersonjoe/trilha/issues/72)). A model
  writes Markdown, and until now the app had two choices: `h.Pre`, which is ugly, or `h.Raw`
  around a converter, which hands the page to whoever wrote the text. `ui.Markdown` returns
  `h.Node`, not a string — paragraphs, emphasis, headings (demoted to `<h3>` so they do not
  compete with the page), lists, quotes, inline and fenced code, GFM tables, links and breaks
  — so the escaping is structural and not a rule somebody has to remember. There is no raw
  HTML and no way to turn it on; a link only stays a link for `http`, `https`, `mailto` and
  relative addresses, an external one carries `rel="noopener nofollow ugc"`, and images are
  opt-in with the URL validated either way. A golden over a fixed corpus and a fuzz target
  hold the line: no input produces a tag this package did not write.
  `ai.Serve(c, client, agent)` is the route half of a chat — it reads `{message, history}`,
  runs the agent and emits `text`, `tool_call`, `tool_result`, `handoff`, `done` and `error`
  with those names, and answers the whole thing at once when the client did not ask for
  `text/event-stream`, which is the request that arrives when JavaScript is not there. The
  history stays with the app: the framework keeps no chat session, and `MaxHistory` caps what
  the browser can send back. `ui.Chat` renders the bubbles, the field and the `aria-live`, and
  `ui.chat.js` (opt-in, like the other kit scripts) reads the stream; what the visitor typed
  is always text, and the assistant's answer is rendered when the message ends, which is what
  `ui.ChatHTML` is for. `examples/assistente` has no JavaScript of its own any more and its
  chat route is one call.

- **Sending a file back: `c.Attachment`, `c.Inline`, `c.AttachmentFile`, `c.InlineFile` and
  `c.Pipe`** ([#71](https://github.com/emersonjoe/trilha/issues/71)). `Ctx` could receive a
  file and had no way to send one, so every download was the same six lines of header written
  again, and one of them was always the one that matters. The name is sanitised like an
  upload's and goes out twice — percent-encoded `filename*=UTF-8''` and a quoted ASCII
  `filename` — so an accent survives and a quote or a newline cannot add a parameter of its
  own. An empty type is detected from the first 512 bytes, with the extension allowed to
  sharpen it inside the same family and never to overrule it, and `nosniff` always goes out.
  `Inline` accepts only what a viewer renders — PDF, image (not SVG), audio, video, plain text
  and CSV — and refuses HTML, SVG and XML as a programming error rather than serving a
  document with script from the app's own origin; the `<iframe>` still needs `frame-src
  'self'` in `Security.CSPExtra`, which `Inline` will not add from below. An `io.ReadSeeker`
  gets `Range`, `If-Range`, `304` and `HEAD` through `http.ServeContent`; anything else
  streams and promises nothing. Every send clears the write deadline. `c.Pipe(res)` hands
  another service's answer to the browser: the status, a closed list of headers and the body
  in stream — never `Set-Cookie`.

- **A field that searches, and an area that takes several files at once**
  ([#67](https://github.com/emersonjoe/trilha/issues/67)). `ui.Combobox` is a text field over
  a list: the person types, the server searches, and what the form sends is a hidden field
  with the chosen value, so the round trip after a 422 comes back with the label written and
  the value intact. A short list is filtered in the browser with no request; a long one names
  a `Source` route that answers `ui.ComboboxOptions` — only the `<li>`s — and `With` carries
  the other fields of the form into the query, which is how a city list narrows to the state
  that is selected. It is a real listbox: arrows, Enter, Escape, `aria-activedescendant`, and
  without JavaScript it is a text input the server resolves against the same list.
  `ui.Dropzone` puts a drop area over a file input and a queue under it, one line per file;
  with `ui.UploadTo` the queue sends one file per request, so each line gets its own progress
  and its own message. `c.Files(field, FileRules)` is the other half: it applies the rules of
  `c.File` to every file the field carries, names a failure by position — `files[2]` — and
  refuses the whole request over `FileRules.MaxFiles` before a byte is read. Without
  JavaScript the same form posts every file at once, into the very same handler.

- **A form that grows: lists of sub-records, key/value matrices and a schema that comes as
  data** ([#69](https://github.com/emersonjoe/trilha/issues/69)). `Bind` fills a `[]Row` from
  `items[0].name`, `items[1].name`… and a `map[string]int` from `perm[docs]=2`, and the name
  of the input is also the key of the message, so `ui.Errors(errs, "items[1].qty")` reaches
  the field the person is looking at — `BindJSON` produces the very same key. An index nobody
  sent is not a row: sparse indices are compacted in numeric order and the message names the
  position the form is about to draw again; nothing is allocated by index, so
  `items[9999999999]` costs one row, with `maxitems` as the ceiling when the tag has one and
  `trilha.MaxItems` (1000) when it does not. Three rules count a collection — `minitems`,
  `maxitems`, `lenitems` — and they are the exception to "an empty value skips the rule". For
  the form that is not in the code at all, `trilha.Schema` is a list of `SchemaField` that
  decodes straight from JSON, `trilha.BindSchema` reads it through the same validation and
  answers the same `FieldErrors`, and `ui.SchemaForm` draws `text`, `textarea`, `number`,
  `date`, `datetime`, `select`, `checkbox`, `file`, `signature` and `display` inside a
  `<form>` that stays the app's. A schema with an unknown type or a pattern that does not
  compile is a plain error, never a 422.

- **`Config.Upstreams`: an app in front of an API that already exists**
  ([#60](https://github.com/emersonjoe/trilha/issues/60)). A URL prefix is forwarded to
  another service — the `rewrites` of a Next.js app — plus the two things a rewrite has no
  place for: the credential of the session, injected by `Upstream.Headers` into the outbound
  request, and the CSRF token, required on writes as on any other write of the app. The
  target lives in the configuration and no part of the request can move it; the
  `Authorization` the browser sent is dropped, so nobody talks to the API with a `Bearer` of
  their own. The body streams both ways, past `MaxBodyBytes` and the write deadline; the
  upstream's `Content-Type`, `Content-Disposition`, `Cache-Control` and `ETag` come back
  intact, with Trilha's security headers standing where the upstream said nothing;
  `X-Request-ID` and `traceparent` cross so the two logs are about the same request. A
  `route.go` of the app answers before the prefix, which is what lets an API move to Go one
  endpoint at a time. A dead upstream is 502 and a slow one 504, both `problem+json`.
- **`auth.Sessions`: a session without OIDC**
  ([#62](https://github.com/emersonjoe/trilha/issues/62)). The same `*Auth`, without a
  provider, for an app whose users are a table of its own: `Login(c, *User)` opens the
  session after the app checked the password, and `Require`, `RequireRole`, `Optional`, the
  `Store`, the rotation of the identifier and the idle window are the code that was already
  there. `Start` and `Callback` answer a clear error instead of dying on a nil provider, and
  `Logout` clears the session and lands.
- **`User.Extra map[string]string`** carries what a claim cannot say — the token an upstream
  injects, the tenant — and travels where the rest of the session travels.
- **`auth.HashPBKDF2`, `auth.CheckPBKDF2` and `auth.PBKDF2`** in the format Django and
  `hashlib.pbkdf2_hmac` write (`pbkdf2_sha256$iterations$salt$hash`), so an existing users
  table stays valid without a password migration. Constant-time comparison; a hash the
  function cannot read is a false, never a panic.
- **`Options.OnLogin`** runs inside `Login` and `Callback` with the session not yet written,
  and its error stops the login.
- **`(*Auth).RequireFunc(pred)`** guards a subtree with a rule the app writes — a matrix of
  module and level, a tenant, the owner of a record — which is the shape `RequireRole` cannot
  express. Anonymous never reaches the predicate.
- **`examples/local-login`**: the two of them together, which is the shape of most
  migrations — own users, own login, and `/api/` forwarded with what that login stored.
- **`trilha.ListParams`: page, ordering, filter and search live in the URL**
  ([#63](https://github.com/emersonjoe/trilha/issues/63)). Embedded in the struct `c.Bind`
  fills, it comes out with `Page` and `PerPage` already clamped (`DefaultPerPage` 20,
  `MaxPerPage` 200), `Dir` normalized to `asc`/`desc`, and the rest of the query kept, so
  `Href("page", "3")` and `PageHref(3)` write the next address without losing the filter that
  was already there. `Offset()`, `Limit()` and `Asc()` are what the repository asks for, and
  `Restrict(cols...)` turns a `sort` from the address into a column name someone declared —
  a crooked URL answers unordered instead of reaching the query.
- **`ui.DataTable(c, cols, rows, state)`** draws the whole screen from that: sortable headers
  as real links (with `aria-sort` on the one in force), the filter as a `<form method=get>`,
  pagination, the count, an empty state, an optional bulk-action bar, and rows that are
  clickable in CSS alone. With `ListState.ID` set, every link and the form are already
  `ui.Swap` targets, so ordering a column swaps the table and nothing else. It ships no
  JavaScript of its own.
- **`ui.Poll`, `ui.Live` and `ui.On`, with `ui.live.js`**
  ([#64](https://github.com/emersonjoe/trilha/issues/64)). `ui.Poll("6s", src)` refreshes a
  fragment on a clock the server owns: `c.PollEvery(d)` changes the interval and
  `c.PollStop()` ends it, both in the answer's own header, and the script pauses on a hidden
  tab, refreshes at once when the tab comes back, refuses to replace a fragment holding the
  focus, honours `Retry-After` and backs off to a minute on errors. `ui.Live(src)` opens one
  `EventSource` per page and `ui.On(name, src)` refreshes the fragment when an event by that
  name arrives — `Stream.Notify(name)` sends it, carrying the name of what changed and never
  the HTML, so the fragment is fetched with the session and the permissions of whoever is
  watching. A page that polls or listens loads `ui.LiveScript(c)` once.
- **`Pages.Attrs`** puts the same attributes on every page link, which is how `ui.DataTable`
  paginates inside a fragment.
- **`ui.Shell`: the frame of an internal app**
  ([#65](https://github.com/emersonjoe/trilha/issues/65)). Sidebar with groups, header, user
  menu and the screen in the middle, composed out of `ui.Sidebar`, `ui.Nav`, `ui.Menu` and
  `ui.ThemeToggle` rather than out of a new primitive. The active item is the longest `Href`
  that prefixes the current path, so `/items/42/edit` lights up `/items`; `Hide` leaves an
  item out of the HTML and says so — hiding a link is cosmetics, the middleware at the root
  of the folder is the rule. Collapsing stamps a class on `<html>`, is remembered in
  `localStorage` and is read back by the script `ui.Head` already emits, so nothing flashes
  on the first paint and no asset was added; on a narrow screen the same class turns the
  sidebar into a drawer. `ui.PageHeader` is the title of the screen inside the shell, with
  `ui.Back` as the way back.
- **`ui.Stat`, `ui.Bars`, `ui.Sparkline` and `ui.Donut`: a dashboard with no charting
  library** ([#68](https://github.com/emersonjoe/trilha/issues/68)). Server-side SVG that
  arrives with the page, prints, and works with JavaScript off. `ui.Datum` carries the value
  the drawing measures and the `Text` a person reads, because the framework has no locale and
  money is formatted by the app. Colours come from the theme (`--chart-1` to `--chart-5`).
  Every chart also renders an invisible table with the same numbers, which is what a screen
  reader reads — the SVG is `aria-hidden` unless `ui.ChartTitle` gives it a name. An empty
  series, a series of zeros and a single point each have a defined drawing.
- **`trilha new --template app`**
  ([#65](https://github.com/emersonjoe/trilha/issues/65)). A second shape for a new project:
  login with `auth.Sessions`, a middleware at the root of `app/` that protects the whole
  tree, the shell, a dashboard with the charts, a listing with `ui.DataTable` and an entity
  with create, edit and delete — with its own tests. It comes green: it compiles, `trilha
  check` passes and `go test ./...` passes without an edit. `blog` remains the default.

### Changed

- **`trilha audit` looks at the proxy and the login**: a `Target` written as `http://` to a
  host that is not this machine is critical (the session credential crossing a network in
  the clear); an upstream with no `Headers` in an app that requires a login, and a `Login`
  with no rate limit, are warnings.
- **`trilha audit` looks at the open stream**: a `ui.Live` in an app with no `Require`,
  `RequireRole` or `RequireFunc` anywhere is a warning — a stream open to anonymous is a
  channel that says when something happened to whoever is listening.
- **`trilha ui` writes six files**: `ui.live.js` joins `ui.theme.css`, `ui.css`, `ui.js`,
  `ui.nav.js` and `ui.upload.js` in `public/`.
- **The scaffold templates are split into `base/` and one folder per shape**, so a new shape
  is a folder and not a fork of the generator. `scaffold.Data.Template` and
  `scaffold.Templates()` name them; an unknown one is a message, not a stack trace.
- **`ui.js` hands the island the same third argument the loader does.** Since 0.40.0 the kit
  also mounts an island — the one that arrives inside a swapped fragment — and it runs first,
  so on a page that uses the kit the kit is what a module meets. It was calling
  `mod.default(el, props)`, which would have left `island` undefined exactly where the
  fragment story is most useful. The two implementations are one protocol, and
  `TestIslandChannelIsTheSameOnBothSides` compares them piece by piece so they cannot drift
  apart quietly.
- **The asset budget goes to 28 KB for `ui.js` and 30 KB for `ui.css`** (FR-007). The island
  channel is 1.2 KB of the first; the combobox, the prose rules `ui.Markdown` needs and the
  bubbles `ui.Chat` needs are the rest. This is the one duplication the kit accepts: an
  island cannot know which of the two mounted it.

### Documentation

- The `ctx` reference gained **Sending a file** in both languages, and `SECURITY-MODEL.md`
  gained the three rows a download is: a type the browser may re-interpret, a name that writes
  a header, and another service's headers landing on this app's session.
- The `ui` reference gained a **Combobox** section and the dropzone next to the progress bar,
  the `ctx` reference gained `Files` and `FileRules.MaxFiles`, and the uploads recipe gained
  **Several at once** — all in both languages.
- New reference page **Upstreams** and the recipe **An app in front of an existing API**, in
  both languages; the auth reference gained the session-without-OIDC section, and
  `SECURITY-MODEL.md` says what changes when the credential lives inside the app.
- New reference pages **Listings** and **Live**, the recipe **A listing that filters, orders
  and paginates**, and the `ui` reference updated, all in both languages; `AGENTS.md` gained
  the two things not to hand-write — a listing screen and a `setInterval`.
- New reference pages **Shell** and **Charts**, and the `--template` section of the CLI
  reference, in both languages.
- The validation reference gained **Lists and matrices** and **A form that comes as data**,
  the forms chapter gained **When the form grows**, and `examples/cadastro` gained a list of
  dependants and a screen whose schema is JSON — all in both languages.

## 0.40.1 — 2026-09-08

### Fixed

- **The `fuzz` job no longer fails by accident**
  ([#86](https://github.com/emersonjoe/trilha/issues/86)). Twice, on different targets, it
  reported `context deadline exceeded`: the `-fuzztime` deadline landing inside an iteration,
  which `go test` reports as a failure of the target. Nothing had been found — the target
  changing between the two is what says the problem was the arrangement and not the code
  under test. `scripts/fuzz.sh` now tells the two apart, and they are tellable apart: a real
  finding writes a file into `testdata/fuzz/<Target>/` and says where it wrote it. With all
  three signals pointing at the deadline the target runs a second time; failing the same way
  twice is reported as what it is, because a genuine hang repeats. A job that fails at random
  teaches people to re-run without reading, and this one is the only barrier the repository
  has against malformed input.

Nothing in the framework changed: there is no reason to upgrade an application for this
release.

## 0.40.0 — 2026-09-08

Spec 057, from a field report: someone who came from years of React, ran Go + templ + htmx in
production for six months and wrote down what they learned. The compliments describe what
Trilha already is; the two complaints are this release.

### Added

- **`ui.Indicator(id)`, and a threshold that keeps it from blinking**
  ([#81](https://github.com/emersonjoe/trilha/issues/81)). An element marked as the indicator
  for a target stays hidden and appears only once the request for that target has been in
  flight past 120 ms — `ui.PendingAfter(ms)` on the trigger changes it. Showing a spinner for
  the 40 ms answer is the flicker people complain about, not a courtesy, so nothing is marked
  before the threshold: a fast answer leaves the page exactly as it was. Several indicators
  may watch the same target.
- **`ui.Spinner(attrs…)`** — a turning ring sized by the font it sits in, hidden from
  assistive technology because the waiting is announced by `aria-busy` on the target.
- **`ui.NoTransition()`** — turns off the crossfade on one trigger.
- **`trilha:pending` and `trilha:settled`** fire on `document` with `detail.target` and
  `detail.id`, for what CSS cannot do.
- **Two chapters, in both languages**: *The ceiling* / *O teto* — where the swap model stops,
  the signs that you are above it, and the rules that keep an island from quietly growing
  into a SPA ([#83](https://github.com/emersonjoe/trilha/issues/83)); and *From htmx and
  templ* / *Vindo do htmx e do templ*, a translation table for people who already made both
  decisions, including what has **no** equivalent — out-of-band swaps, polling triggers,
  event triggers ([#84](https://github.com/emersonjoe/trilha/issues/84)).
- **`/blog/ordem` in `examples/blog`** — drag to reorder, the order saved by the same
  `POST → redirect → GET` the rest of the app uses, and ↑ ↓ buttons that keep the screen
  usable from a keyboard and with the module blocked.

### Fixed

- **An island that arrives inside a swapped fragment now mounts**
  ([#82](https://github.com/emersonjoe/trilha/issues/82)). It only did when the page already
  had an island: the loader `Ctx.Island` writes travelled with the fragment as a `<script>`,
  and the DOM does not run a script inserted by `outerHTML`. So the island sat there with its
  fallback showing and nothing in the console — and the symptom disappeared when you reloaded
  to check. A swap only ever happens through `ui.js`, so that is what mounts what a swap
  brings in; both sides skip an element already marked `data-trilha-mounted`, so an island
  mounts exactly once.
- **A second click no longer sends the request twice.** A trigger whose target is already in
  flight is ignored, which also replaces disabling the submit button on the spot — itself an
  instant visual change on a fast answer.

### Changed

- **`aria-busy` on a swap target now obeys the threshold.** It used to be set the moment the
  request left, so the target dimmed on every answer, however fast. If you styled
  `[aria-busy]` yourself, the rule still applies — it just starts later.
- **A swap crossfades where the browser has `document.startViewTransition`**, and does not
  where it has none or where the system asks for less motion. `ui.swap(id, html, status)` now
  returns a promise, because the replacement may be running inside that transition.
- **The `ui.js` budget goes from 16 KB to 20 KB** (FR-007). The threshold, the transition and
  the island mounting are about 3.8 KB, and all three sit on the path a swap already takes —
  none of them could move into a file only the apps that use it download, the way `ui.nav.js`
  and `ui.upload.js` do. The kit copies in `site/` and `examples/blog/` were refreshed with
  `trilha ui --force`; the site had been serving a kit from before 0.39.0.

## 0.39.2 — 2026-09-08

### Fixed

- **The CLI works on Windows** ([#79](https://github.com/emersonjoe/trilha/issues/79)).
  `trilha dev` died on a fresh project with `exec: ".trilha\app": executable file not found
  in %PATH%` — pointing at a binary it had just built and that was sitting on disk. `go build
  -o <path>` writes the literal name it is given, so on Windows the output was `app` and not
  `app.exe`, and `exec.LookPath` only accepts a file whose extension is in `PATHEXT`. The
  three places that build a binary and then run it now ask for the extension the system needs:
  `trilha dev`, `trilha export`, and `trilha build` — for the default `bin/<project>` and for
  the name given to `-o`, so `trilha build -o bin/app` writes `bin\app.exe` instead of a file
  Windows will not execute. A name that already ends in `.exe` is left alone; Linux and macOS
  are untouched.
- **`TestFileSaveStaysInTheDirectory` no longer asserts Unix mode bits on Windows**, where
  `os.Stat` reports `0666` for any writable file.

### Changed

- **CI runs on `windows-latest` too.** Only `ubuntu-latest` ever ran, which is why a CLI that
  could not start on Windows shipped green. The suite could not have caught it either: the e2e
  harness builds `trilha-cli` and executes it, with the same missing extension.
- **`.gitattributes` pins the working tree to LF.** Git checks files out with CRLF on Windows
  by default — `actions/checkout` included — and the golden files are compared byte for byte,
  so without this every golden test fails on a stock Windows clone.

## 0.39.1 — 2026-09-06

### Fixed

- **`trilha audit` no longer reports metrics an app never exposes**. The item that guards
  the monitoring endpoint used to turn on for any `Metrics:` in the project source, and
  `cache.Options` has a field with that name that only picks the registry the counters go
  to. The reference app uses it, so `trilha audit` — and therefore `trilha check` — failed
  there with a critical that was never true. The item now looks at what actually opens the
  endpoint: `Config.Observability.Metrics`, as an assignment or in the literal, or
  `TRILHA_METRICS`.

## 0.39.0 — 2026-09-06

### Added

- **`trilha generate` writes the contract, not only the folder**
  ([#49](https://github.com/emersonjoe/trilha/issues/49)). `--methods GET,POST` writes one
  handler per method with `c.Param` already reading each parameter of the path; `--bind Type`
  makes the methods that carry a body call `c.BindJSON`, which is the 422 with the fields;
  `--form Type` writes a page's whole round trip (`CSRFInput`, a `ui.Field` per field, 422
  with the messages beside them, `POST → redirect → GET`); `--layout` writes the `layout.go`
  the folder above is missing. A type the project declares is imported from where it is; one
  it does not have is born in the route's package with example tags, and a name declared in
  two packages is refused with both paths.
- **`trilha generate test <url>`** writes the test beside the route, in its package, with one
  case per method the scanner finds and a body built from the `validate` tags when the type
  the handler binds can be read. Generating a route and its test leaves `trilha check` green
  with nothing edited by hand.
- **`--lang en|pt` for `generate`**, like `new`: it chooses the language of the comments in
  the skeleton. Identifiers, field names and error messages stay in English.
- **`func OPTIONS` is a handler like the others**
  ([#76](https://github.com/emersonjoe/trilha/issues/76),
  [#78](https://github.com/emersonjoe/trilha/issues/78)). The scanner knows the method,
  `MiddlewareOPTIONS` guards it, and a preflight stops falling into the 405 the fallback
  answers before any middleware runs. The router always routed it; only the scanner was
  short.
- **`var CORS = trilha.CORS{...}` in a `route.go`**: the cross-origin policy of that route
  alone, preflight included. `Config.CORS` is the whole app, and a discovery document under
  `/.well-known/` is three paths out of ninety — opening the other eighty-seven to reach
  them would trade a gap for a surface. A route that declares a policy decides alone, and a
  `func OPTIONS` written by hand still wins over it.

- **`c.Flash(kind, text)` and `ui.Flashes(c)`**
  ([#66](https://github.com/emersonjoe/trilha/issues/66)). The Trilha way of answering a
  form is `POST → 303 → GET`, and the redirect used to eat the news with it: `ui.Toast`
  existed, but only the *next* page could render it, and it had no way of knowing. `c.Flash`
  writes the message in a signed cookie of its own, read once and cleared, and the
  `ui.Flashes(c)` that replaces `ui.Toaster()` in the layout shows it. On a fragment answer
  there is no redirect to survive, so the messages travel in the `Trilha-Flash` header and
  `ui.js` shows them — the call in the handler does not change. Without `TRILHA_SECRET`
  nothing is written and the app says so once, like `SetSigned`.
- **`ui.Confirm(title, description)`** asks before a form is submitted, fragment forms
  included: `ui.js` holds the submit, builds the kit's `<dialog>` and lets it through only
  after the answer. The confirming button repeats the pressed button's label, and the other
  one says `Cancel` unless `h.Data("ui-confirm-cancel", "…")` says otherwise. It replaces the
  fifteen hand-written lines the `examples/blog` used to spend on "delete this post?" —
  `onclick="return confirm()"` was never an option, because the CSP forbids inline script.
- **`h.Maxlength`, `h.Minlength`, `h.Autocomplete` and `h.Inputmode`**
  ([#58](https://github.com/emersonjoe/trilha/issues/58)): four attributes that only came out
  of `h.Attr`, next to `Pattern` and `Required`, which are functions. In a form where one
  attribute is a loose string, that string is the one nobody proofreads. **`h.Attrs(...)`**
  groups attributes into one node, for a component that sets more than one on the element it
  is placed in.
- **`trilha.NonceFrom(r)` and `trilha.CSRFTokenFrom(r)`**
  ([#44](https://github.com/emersonjoe/trilha/issues/44)). The nonce and the CSRF token used
  to be reachable only through the `*Ctx`, which is to say only by whoever renders with `h`.
  A renderer that receives the `*http.Request` and nothing else — `html/template`, `templ`, a
  handler of your own — had to add a middleware of its own to see either. Both now answer from
  the request, and `""` outside a Trilha request.
- **`tmpl.Wrap`, `Shell.Node` and `tmpl.HTML`**
  ([#57](https://github.com/emersonjoe/trilha/issues/57)). `tmpl.Node` put a template inside
  a page; the way back did not exist, so an app migrating from `html/template` had to rewrite
  its shell before it could write one new screen in `h`. `Wrap(t, name, slot)` prepares the
  shell once, at package load, and `shell.Node(data, children)` renders it with the `h` node
  in the slot — no `template.HTML` written by the app, because what `h` rendered was already
  escaped and `tmpl` is the single place that says so. Preparing the shell up front instead
  of cloning it per request costs 8.3× less on the same 21 templates. A shell that never
  reaches the slot fails the render instead of quietly answering a page with no content.
  `examples/blog` keeps a working copy in `app/legado-`.
- **`c.Pattern()`** ([#42](https://github.com/emersonjoe/trilha/issues/42)): the template of
  the route that matched, `/blog/{slug}` where the path is `/blog/hello`. The router knew it
  all along and nobody could ask: a handler could read one parameter at a time and never the
  shape they came from. The access record now carries both — `path` for whoever is looking
  into a single case, `route` for whoever is counting, since an app with an id in the URL has
  one path per record and one route per screen. `LogRequest` already receives the `*Ctx`, so
  it gets the template without changing signature. Empty for what the fallback answered.

### Changed

- **`Kind` is inherited by the subtree**
  ([#43](https://github.com/emersonjoe/trilha/issues/43)). It was the one thing in the tree
  that was not: `layout.go` and `middleware.go` decide a whole branch, `var Kind` decided one
  leaf, so "this branch is pages, not an API" had to be repeated in every `route.go` — forty
  of them in one app that adopted the framework's CSRF. `var Kind` in the package of a
  directory now decides that directory and everything below it, deepest declaration wins, and
  `kind.go` is the file name for a branch root with no `route.go` of its own. This is not
  about how errors render: **`Kind` is what turns CSRF on**, and a write route born without
  the line was born without CSRF, in silence. A `page.go` route stays a page whatever the
  branch above says.
- **`trilha audit` reports the write that no `Kind` reaches**: a `route.go` with a body method
  in an app that also serves pages, with no `Config.CSRFForAPI`, accepts a form posted from
  another site. It is a warning, not a critical item — an app whose API client really is its
  own is a real thing, and failing its `trilha check` would repeat the mistake #77 fixed.

- **`trilha audit` asks whether the app signs anything**
  ([#77](https://github.com/emersonjoe/trilha/issues/77)). A missing `TRILHA_SECRET` is a
  warning, not a critical item, when nothing in the project calls `SetSigned`, `Signed`,
  `NewSigner`, sets `Config.Secret` or imports `trilha/auth`. `trilha check` stops at the
  first failure, so the gate the AGENTS file tells an agent to run was failing on a secret
  that would sign nothing — and `openapi`, the step that catches regressions, never ran. Set
  and too short stays critical: whoever set it meant to use it.
- **A method the router cannot take from a file is an error, not a silent discard**. `func
  HEAD`, `func TRACE` and `func CONNECT` in a `route.go` stop the generation with
  `E_UNROUTABLE_METHOD`, line and fix included (HEAD is answered by the `GET` handler since
  Go 1.22). `var CORS` in a `page.go` is `E_CORS_ON_PAGE`.

### Documentation

- **How to turn on the agent files in a project that already exists**. `--agents` is a flag
  of `trilha new`, so a project created before it needs `trilha agents` instead; the
  [migration recipe](https://emersonjoe.github.io/trilha/cookbook/migration) now carries the
  whole sequence, including the step that is easy to miss — running `trilha agents` again
  after every CLI upgrade, since `AGENTS.md` names the commands of the version that wrote it.
- **How to keep the old shell while the inside is rewritten**. The
  [migration recipe](https://emersonjoe.github.io/trilha/cookbook/migration) and the
  [tmpl reference](https://emersonjoe.github.io/trilha/reference/tmpl) now show the halfway
  state of a migration: the `layout.html` of the app that already exists, with the pages
  under it written in `h`.
- **What `Kind` decides, and where to say it**. The
  [file conventions](https://emersonjoe.github.io/trilha/reference/conventions) gained the
  section on inheritance, and the [security reference](https://emersonjoe.github.io/trilha/reference/security)
  the new audit warning. The `AGENTS.md` written by `trilha agents` says it too: it is the
  kind of rule an agent breaks by writing perfectly reasonable code.

## 0.38.0 — 2026-09-06

### Added

- **`/.well-known/` is a route** ([#75](https://github.com/emersonjoe/trilha/issues/75)).
  `app/.well-known/security.txt/route.go` answers `/.well-known/security.txt`. It is the
  single exception to the rule that a folder whose name starts with a dot is skipped — the
  place where RFC 8414, RFC 9728, RFC 8555, RFC 9116 and OpenID Discovery publish their
  documents. Inside it the conventions are the usual ones, and since `.well-known` is not a
  Go identifier the file declares another package name, as `app.css` already did. The
  exception is honored by the three scans of the project: the router, the type index behind
  `trilha openapi`, and the `trilha dev` watcher.

### Fixed

- **A route inside a dot folder no longer disappears in silence**
  ([#75](https://github.com/emersonjoe/trilha/issues/75)). A `page.go` or a `route.go` under
  a skipped folder used to leave `trilha gen` successful, the route absent and the app
  answering 404 with nothing to read. It is now `E_HIDDEN_ROUTE`, at generation time, naming
  the file and offering the fix: rename the folder, or start its name with `_` if it is
  parked on purpose. Only the dot is loud — `_x` and `testdata` are the documented way to
  keep a folder out of the routing.

## 0.37.0 — 2026-09-06

### Added

- **`trilha check`, the single gate**
  ([#48](https://github.com/emersonjoe/trilha/issues/48)). One command runs `gen`, `gofmt`,
  `vet`, `test`, `audit` (without the vulnerability scan, which needs the network) and
  `openapi`, in the order that fails cheapest first, and stops at the first failure: what comes
  after a broken build says nothing about the project, and the steps that never ran say so
  instead of pretending. `--fix` rewrites `trilha_gen.go` and the formatting before judging
  them; `--json` writes `{ok, steps, problems}` for a tool to read. The `openapi` step reads the
  title, version and server back from the existing `openapi.json`, so it never reports a
  staleness that is only a missing flag.
- **`trilha ctx`, the map of the project**
  ([#47](https://github.com/emersonjoe/trilha/issues/47)). Routes with their file, methods,
  parameters, layouts and middlewares; each API operation with its query, body and responses;
  the types those operations exchange; what `app/setup.go` provides; and whether
  `trilha_gen.go` is up to date — in one read instead of a dozen file openings. Compact
  Markdown by default, `--routes`, `--types` and `--all` to choose how much, `--json` for the
  same model as a document. The API section comes from the same inference behind
  `trilha openapi`, so the map and the document cannot disagree, and the output is
  deterministic: same tree, same bytes.
- Every scanner violation now carries its **line** and its **fix**: `app/page.go:3:
  E_NO_PAGE_FUNC: page.go must export func Page(...); found func Render`, with
  `→ rename the function to Page, ...` underneath. `trilha gen`, `trilha dev`, `trilha check`
  and `trilha ctx` all print it, and the sixteen codes are covered by a test that fails when a
  new one arrives without its conserto.
- `AGENTS.md` now opens with `trilha check` as the single gate and `trilha ctx` as the read
  that comes before opening files one by one, in both languages.

## 0.36.0 — 2026-09-06

### Added

- **`AGENTS.md` on request, never by default**
  ([#46](https://github.com/emersonjoe/trilha/issues/46)). `trilha agents` writes `AGENTS.md`
  and `CLAUDE.md` at the project root: the three conventions, every command and what each one
  checks, what not to do, and where the cookbook and the reference live. `trilha new --agents`
  does the same at scaffolding time. Without the flag a new project gets neither file — AI
  support is opt-in, as it should be for a framework that ships no dependency you did not ask
  for. `AGENTS.md` carries the same stamp the ui kit uses, so an untouched copy is refreshed
  by the next `trilha agents` and an edited one is kept until `--force`; `CLAUDE.md` is
  created once and never rewritten, because that file is where your own repository rules go.
- **`/llms.txt` and `/llms-full.txt` on the site**, both locales
  (`/llms.txt`, `/pt/llms.txt`, and the `-full` pair). The short one is an index of every page
  with its one-line description; the full one is the whole documentation as plain Markdown,
  code blocks intact, links rewritten to absolute. Both are generated from the same content
  the site renders, so they cannot drift, and both are written by `trilha export`.
- `App.Export` now writes a path whose last segment has a dot as the file itself, instead of
  `<path>/index.html`. That is what puts `llms.txt` in the static export, and it is the rule
  any `/robots.txt` or `/feed.xml` route needed.

### Fixed

- The ruler's own guard was accepting a fixture that does not compile as proof that the
  hidden test fails. `TestFixturesFailWithoutTheAgent` now demands `--- FAIL` — the hidden
  test actually running and failing — and treats a failure coming from `go vet` on the
  untouched fixture as a broken ruler, with a message saying so. `make bench-agent-dry` stops
  on it too, instead of printing an expected failure that was really a compile error.

### The ruler

`make bench-agent` before, `make bench-agent-agents` after — same fixture, same agent, same
model (Claude Code, Opus 4.8), three runs per scenario, median, 12/12 green on both sides. The
only difference is the `AGENTS.md` this release writes:

| Scenario | Turns | Cache read | Time (s) | Cost (US$) |
|---|---:|---:|---:|---:|
| `comments` | 39 → 32 (-18%) | 2241k → 1573k (-30%) | 421 → 328 (-22%) | 2.54 → 1.81 (-29%) |
| `contact-form` | 30 → 25 (-17%) | 1074k → 592k (-45%) | 185 → 120 (-35%) | 1.28 → 0.81 (-37%) |
| `cognito` | 18 → 13 (-28%) | 442k → 385k (-13%) | 73 → 73 (-1%) | 0.57 → 0.48 (-15%) |
| `pagination` | 16 → 15 (-6%) | 496k → 332k (-33%) | 101 → 70 (-31%) | 0.71 → 0.43 (-40%) |

Sixty lines of `AGENTS.md` take between 15% and 40% off the bill of a feature, and the cheapest
saving is the one the file was written for: the agent stops reading the project to find out
what the project already is. `bench/agent/RESULTS.md` has the full tables.

## 0.35.0 — 2026-09-06

### Added
- **The response headers can belong to the host**
  ([#52](https://github.com/emersonjoe/trilha/issues/52)). An app mounted inside a server
  that already answers for its responses now says so in one line:
  `Security.Delegated = true` writes none of the seven headers — not the six that had an
  `Off`, and not the `X-Content-Type-Options` that had none — so the host's
  `Content-Security-Policy` is the only one on the response. `Security.Nonce
  func(*http.Request) string` is the other half: `c.Nonce()` and `trilha.NonceAttr(c)` then
  carry the nonce the host already published in its own policy, instead of one this app
  invented that the browser has never heard of. An empty answer renders no attribute at all
  rather than `nonce=""`. `Delegated` is a decision and not a default — the zero value still
  writes every header, so a hand-written `Security{...}` cannot turn them off by omission —
  and the boot logs the delegation once.
- **The CSRF cookie, field and header have names you choose**
  ([#54](https://github.com/emersonjoe/trilha/issues/54)). `Config.CSRF` sets `Cookie`,
  `Field` and `Header`; an empty one keeps the constant it had, so renaming one is one line.
  Two hidden inputs called `_csrf` on the same page — the host's and the app's — is a bug
  nobody sees until a form posts the wrong token, and this is what gets out of the way of it.
  The name given is the one `CSRFInput`, `CSRFToken`, the check, the CORS allow-list and the
  test client all use.
- **Dependencies by type: `trilha.Provide` and `trilha.Use`**
  ([#55](https://github.com/emersonjoe/trilha/issues/55)). `trilha.Provide(a, v)` files a
  value under its type in `Setup`, and `trilha.Use[T](b)` reads it back from either the
  `*Ctx` of a handler or the `*App` itself, which is what `Setup` and a test have in hand. A
  type nobody provided panics at the call, naming the type, instead of surfacing later as a
  nil somewhere else. The type is the key, so a seam is declared by writing it:
  `trilha.Provide[Mailer](a, SMTPMailer{...})` files the interface. `Values()` stays for glue
  by name.
- The OpenAPI deduction follows a method on a value, not only a package function: a handler
  that reaches its store through `Use` still publishes the schema of what it answers. The
  index now knows methods and the type parameters of a generic function.

### What changes for you
Nothing breaks. `examples/blog` moved off package variables — `internal/posts` is a
`*posts.Store` created in `Setup`, provided, and read with `Use` in every page and route —
because that is the shape that survives two apps in one process, and the example is what
people copy. If your `Setup` keeps state in package variables and you ever mount a second app
in the same binary, or build a second app in a test, that state is shared; `Provide`/`Use` is
the fix, and both apps then get their own.

## 0.33.0 — 2026-09-06

### Added

- `bench/agent`: the ruler of Fase 5. `make bench-agent` copies `examples/blog` or
  `examples/sso` into a module of its own, runs a coding agent (`claude -p`) on four fixed
  tasks — an API route with `Bind`, a page with a `ui` form, switching the login provider to
  Cognito, paginating the post list — and records tokens in and out, turns, denied tool
  calls, time and cost, then decides pass/fail with a hidden test. Three runs per scenario,
  median in `bench/agent/RESULTS.md`; `make bench-agent-dry` proves every hidden test fails
  on the untouched fixture without spending a token. Trilha before × Trilha after, never
  against another framework (#45).
- Reference → Performance: section "Cost per feature for an agent", both locales.
- First measurement in `bench/agent/results.json` (Claude Code 2.1.212, Opus 4.8, 12/12 runs
  green). Median turns and wall time: `comments` 39 / 421 s, `contact-form` 30 / 185 s,
  `cognito` 18 / 73 s, `pagination` 16 / 101 s. This is the "before" every Fase 5 item is
  measured against.
## 0.32.0 — 2026-09-06

### Added
- **The generated file takes the package the folder declares**
  ([#51](https://github.com/emersonjoe/trilha/issues/51)). An app no longer has to be
  `package main`: if the directory being generated already declares `package crm`,
  `trilha gen` writes `trilha_gen.go` into that package and exports
  `func NewApp() *trilha.App` instead of a `main` nobody asked for. The binary that already
  exists mounts it in one line — `mux.Handle("/", crm.NewApp().Handler())` — and there is no
  hand-written registration file to grow silently, because `gen --check` keeps catching the
  folder someone added without generating. `--package <name>` forces the package for a first
  run; after that the existing file remembers it. `trilha dev` and `trilha build` refuse a
  package other than `main` and say why: the host binary is what runs.
- **Middleware for a single method**
  ([#56](https://github.com/emersonjoe/trilha/issues/56)). Besides `Middleware`,
  `middleware.go` may now export `MiddlewareGET`, `MiddlewarePOST`, `MiddlewarePUT`,
  `MiddlewarePATCH` and `MiddlewareDELETE`. They are inherited down the subtree exactly like
  `Middleware`, and the chain of a request is `Middleware` first, then the one for its
  method, both outermost first — so the rule that guards a write stops living inside the
  handler as an `if` next to the business logic, and a mixed page/API route stops paying for
  it on reads. A method middleware that no route in the subtree serves is an error,
  `E_UNUSED_METHOD_MIDDLEWARE`: a rule that guards nothing is worse than no rule. The route
  inspector in `trilha dev` lists the per-method chains next to the route-wide ones.

### Fixed
- **`app/error.go` answers every status, not only 500**
  ([#53](https://github.com/emersonjoe/trilha/issues/53)). A 403, a 401, a 409 or a 422 that
  reaches the framework now renders the app's own error page at its own status, wrapped by
  the root layout, instead of the framework's plain page; 404 keeps going to `not_found.go`,
  and an API route keeps answering `problem+json`. `trilha.StatusOf(err)` is exported for
  the `switch` that page needs, since the page receives the error and not the code. The
  internal page stays as the net underneath: with no `error.go`, or with one that fails, the
  status is still the right one.

## 0.31.0 — 2026-09-06

### Added
- **`auth.Clerk(frontendAPI, clientID, clientSecret, redirectURL)`**
  ([#41](https://github.com/emersonjoe/trilha/issues/41)), the last of the provider
  shortcuts. It takes the Frontend API URL from the dashboard —
  `verb-noun-00.clerk.accounts.dev` in development, `clerk.your-domain.com` in production,
  with or without the scheme and the trailing slash — and produces the one issuer Clerk's
  discovery document declares, which is where people get it wrong.
- The shortcut is deliberately half of what the other three are, because that is all Clerk
  offers, and it says so instead of pretending parity: Clerk's `id_token` carries the
  organization (`org_id`) and no role, so roles fall back to the generic `roles`/`groups`
  pair and a configured claim goes in `Options.RoleClaims`; and Clerk publishes no
  `end_session_endpoint` (backchannel and frontchannel logout are both off), so `Logout`
  clears the local session and writes in the log that the Clerk session was left open.
- The three questions that had kept this open were answered against a real discovery
  document rather than the documentation, and the answers are recorded on the issue:
  discovery exists and is complete, the issuer has no trailing slash, and there is neither a
  role claim nor an end-session endpoint.
- `examples/sso` accepts `SSO_PROVIDER=clerk` with `SSO_FRONTEND_API`; `trilha audit` now
  looks for a hard-coded secret in `auth.Clerk` too, which it did not do for a constructor
  it did not know. The authentication chapter and the `auth` reference cover Clerk in both
  languages, and the role table in the chapter — which had been missing Cognito since
  0.11.0 — lists every shortcut.

## 0.30.0 — 2026-09-06

### Added
- **`Pagination` and `Tooltip` in the `ui` kit**
  ([#39](https://github.com/emersonjoe/trilha/issues/39)). `ui.Pagination(ui.Pages{...})`
  renders page navigation as real links, so a page can be shared, reloaded and indexed: the
  current page is a `<span>` with `aria-current` instead of a link to where the visitor
  already is, the first page has no *previous* (nothing is rendered rather than a disabled
  link), and a window of seven slots always keeps the first and the last page with an
  ellipsis over each gap, so the footer does not grow with the table. A list with one page
  renders nothing. The `Prev`, `Next` and `Label` fields carry the user-visible text, so the
  kit does not have to pick a language.
- `ui.Tooltip(text, ...)` attaches a hint to what it wraps. The text goes into `title`, which
  is the browser's own tooltip and works with `ui.js` off; with the script on the page the
  `title` is removed — two tooltips is worse than none — a bubble with `role="tooltip"` takes
  its place, the target gets `aria-describedby`, and the hint answers to hover, keyboard
  focus and touch, closing with Escape (WCAG 1.4.13). The bubble is clamped to the viewport,
  and its text is written with `textContent`, never `innerHTML`.
- The kit chapter and the `ui` reference cover both in the two languages, with a live demo
  (`ui-paginacao`); `ui.hydrate(el)` now also arms the tooltips of what was inserted later.

### Changed
- The unminified budget for `ui.js` goes from 10 KB to 12 KB. The tooltip is the first
  component since the kit shipped to need script of its own — a hint that cannot be
  dismissed is not accessible — and `ui.css` stays inside its 25 KB.

## 0.29.0 — 2026-09-06

### Added
- **Cookbook: the nine recipes every app writes**
  ([#38](https://github.com/emersonjoe/trilha/issues/38)). A third top-level section of the
  site, next to Learn and Reference, for the question that shows up on the second day: how do
  I open a database, keep someone logged in, receive a file, paginate a list, send an e-mail,
  run a task every hour, put the whole thing in a container? Each page answers one of those
  with the trade-offs written down — the pool sized once per process and closed on shutdown,
  the session that costs one indexed query and can be revoked now, the upload served from a
  sandboxed mount, offset pagination and the cursor that replaces it, the mailer behind an
  interface so a test sends nothing, the ticker that stops with the app, the distroless image
  and the two probes.
  Two more pages close the set: a **production checklist** read from top to bottom before the
  first deploy (what `trilha audit` finds for you, what it cannot see, and the two things to
  prepare for the bad day) and a **migration guide** — plain `net/http` to Trilha one route at
  a time, with both systems in the same process, plus what to run when moving between minor
  versions.
  Every Go block on those pages is copied from `examples/cookbook`, which is part of the
  repository's module: `go vet ./...` compiles it and a site test checks that each block still
  appears, character for character, in the file it came from. A recipe that stops compiling
  breaks the build before it can mislead anyone.

## 0.28.0 — 2026-09-06

### Added
- **Route inspector in `trilha dev`**
  ([#37](https://github.com/emersonjoe/trilha/issues/37)). While the dev server runs,
  `/_trilha/routes` shows the map of the app: every route in the order the router decides,
  with kind, methods, source folder, the layouts that wrap it (outermost first) and the
  middlewares that run before it. A box at the top answers "who serves this path?" — the
  pattern that wins and the value of each parameter, resolved by an `http.ServeMux` built from
  your own patterns rather than by a second implementation of the precedence rules.
  The page is served by the dev supervisor, not by the app: it is not in the binary
  `trilha build` produces, and the same URL in production is a 404 like any other.

## 0.27.0 — 2026-09-06

### Added
- **`trilha generate page|route|component`**
  ([#36](https://github.com/emersonjoe/trilha/issues/36)). The command takes the URL and writes
  the folder the convention asks for: `trilha generate page /blog/{slug}` creates
  `app/blog/slug_/page.go` with `c.Param("slug")` already read, `trilha generate route
  /api/itens/{id}` creates `app/api/itens/id_/route.go`, and `trilha generate component Aviso`
  creates `internal/components/aviso.go` (`--dir` for another folder). What comes out compiles,
  and `trilha_gen.go` is regenerated at the end, so the URL answers before the editor is open.
  The package name is the one already declared in the folder when there is one, otherwise it
  is derived from the folder (`slug_` → `slug`, `relatorio.csv` → `relatoriocsv`, `type` →
  `type_`). An existing file needs `--force`; page and route in the same folder is refused
  with or without it, because that is a convention, not a preference.

## 0.26.0 — 2026-09-06

### Added
- **A written public API, and a test that guards it**
  ([#35](https://github.com/emersonjoe/trilha/issues/35)). `API.md` (and
  `docs/pt-BR/API.md`) says which packages the stability promise covers — `trilha`, `h`, `ui`,
  `tmpl`, `ai`, `ai/mcp`, `auth`, `cache` — and what it does not: `internal/`, the exact output
  of the CLI, the HTML the `ui` components emit, the generated `trilha_gen.go`. It also says
  what counts as a breaking change, and the cycle a symbol goes through before it disappears:
  a `Deprecated:` note naming the replacement, a line in this file, and at least one minor
  version of coexistence.
- **`api/current.txt`**: the exported surface of those eight packages, one line per symbol, in
  the format of Go's own `api/go1.txt` — no parameter names, no documentation, sorted.
  `TestSuperficiePublica` compares it on every `make test` and fails listing what came in and
  what went out; `make api` rewrites it after an intentional change, and the diff of that file
  is what makes a removal visible in review instead of in someone else's build. A symbol
  carrying `Deprecated:` shows up marked. A second test refuses a public package that nobody
  added to the list.
- `GOVERNANCE.md`, the reference overview on the site and `CONTRIBUTING.md` point at the
  document, in both languages; the constitution's principle IV now carries the boundary and the
  deprecation cycle (version 1.4.0).


## 0.25.0 — 2026-09-06

### Added
- **A written threat model** ([#34](https://github.com/emersonjoe/trilha/issues/34)).
  `SECURITY-MODEL.md` (and `docs/pt-BR/SECURITY-MODEL.md`) lists the assets, the trust
  boundaries, the actors and the threats by STRIDE, each pointing at the control that answers
  it — and, just as importantly, at what stays **open**: domain authorization, secrets at
  rest, an audit trail of business actions, volumetric attacks. Linked from `SECURITY.md` and
  from the security chapter.
- **`Config.AllowedHosts`**: a request whose `Host` is not in the list is answered with 400
  before the router, the probes and CORS, which is what keeps a forged `Host` out of the
  absolute URLs your app builds (password-reset links, invitation e-mails) and out of any
  cache in front. The port and the case do not count; `*.example.com` allows one extra label;
  `localhost` and the loopback addresses always pass in `Dev`. `TRILHA_ALLOWED_HOSTS=a,b`
  fills it from the environment. **An empty list keeps today's behaviour.**
- **A `host` security event** on every refusal, in the log, in
  `trilha_security_events_total` and in `Config.OnSecurityEvent`.
- **`trilha audit`** warns when the app declares no `AllowedHosts`.

## 0.24.0 — 2026-09-06

### Added
- **`-race` and fuzzing in CI** ([#33](https://github.com/emersonjoe/trilha/issues/33)). Two
  new jobs run on every push: `go test -race ./...` and 20 seconds on each fuzz target. A
  concurrency test (`TestConcorrencia`) hits one app from 32 goroutines — login, signed page,
  API route, static file, `/metrics` — so the detector has real contention to look at instead
  of a suite that answers one request at a time.
- **Six fuzz targets, one invariant each**: `FuzzRouteMatch` (no target crashes the app or
  serves a file from outside `public/`), `FuzzBindForm` and `FuzzBindJSON` (no error implies
  every `validate` rule holds), `FuzzSignedVerify` (a cookie is only accepted if some key
  would have produced it, and only until it expires), `FuzzParseTraceparent` (the trace id is
  empty or hex that came from the header) and `FuzzRenderEscapes` (what goes into `h.Text` or
  an attribute comes back escaped).
- **`make race`, `make fuzz` and `make fuzz-long`**, with `scripts/fuzz.sh` running the list
  of targets — `go test -fuzz` takes one target of one package at a time. `FUZZTIME=2m make
  fuzz` for a longer round; a failure lands in `testdata/fuzz/<Target>/` and gets committed
  with the fix, so `go test ./...` replays it forever.
- **A "Race and fuzzing" section in the Testing chapter** and the new commands in
  `CONTRIBUTING.md`, in both languages.

## 0.23.0 — 2026-09-06

### Added
- **Test helpers in `package trilha`: the client every app was writing by hand**
  ([#32](https://github.com/emersonjoe/trilha/issues/32)). `trilha.TestRequest(t, app, method,
  target, opts...)` sends one request through the real path — mux, middlewares, layouts, CSRF,
  error negotiation — and returns a `*TestResponse` with chainable assertions
  (`WantStatus`, `WantContains`, `WantHeader`, `JSON(&v)`, `Cookie(name)`). No assertion returns
  an `error`: a failure stops the test printing the target, the status and the body.
- `trilha.NewTestClient(t, app)` keeps the cookies the app sets, so a flow (open the form,
  submit it, read the result) is three lines with `Get`, `PostForm` and `PostJSON`.
- Every request carries the CSRF cookie and, on a method with a body, the matching
  `X-CSRF-Token` header. It is not a hole: cookie and token come from the same client, which is
  what double submit asks a browser for. `WithoutCSRF()` is how a test proves the refusal.
- `trilha.TestRoute(t, route, method, target)` exercises one `route.go` with its middlewares
  and resolves `{id}` from the pattern; `trilha.TestPage(t, route, target)` renders a page with
  its layouts and hands back the node in `res.Node`, so an assertion survives a change of
  layout. Both mount a throwaway app in `Dev`; `WithApp(a)` uses yours.
- Options: `WithApp`, `WithHeader`, `WithCookie`, `WithSigned` (a cookie signed by the app's own
  signer, so the admin page needs no `POST /login` first), `WithForm`, `WithJSON`, `WithBody`
  and `WithoutCSRF`.
- `TestingT` is the interface the helpers take (`Helper`, `Fatalf`), so `package trilha` still
  never imports `testing`: the test flags stay out of the production binary. `TestResponse`
  embeds `*httptest.ResponseRecorder`, so `Code`, `Body` and `Header()` remain at hand.
- New chapter [Testing](https://trilha.dev/learn/testing) (`/pt/aprender/testes`) and the
  helper table in `reference/app`.

### Changed
- The five examples use the helpers: `examples/blog`, `examples/cadastro`,
  `examples/orcamento`, `examples/sso` and `examples/assistente` no longer carry their own
  `httptest` client, cookie jar or CSRF copy — 500 lines became 254.

## 0.22.0 — 2026-09-05

### Added
- **`trilha openapi`: the OpenAPI 3.1 document of the API routes, deduced from the code**
  ([#31](https://github.com/emersonjoe/trilha/issues/31)). No annotation to keep in sync: the
  folder gives the path and the path parameters, the exported `GET`/`POST`/`PUT`/`PATCH`/
  `DELETE` give the operations, the doc comment gives `summary` and `description`,
  `c.Bind`/`c.BindJSON` give the request body and a 422, `c.JSON(status, v)` gives the
  response with the schema of `v`, `WriteHeader` gives a bare status, `c.Header("Content-Type", …)`
  gives the media type, and `trilha.ErrNotFound`, `trilha.Errorf` and `&trilha.Problem{Status: …}`
  give the `problem+json` responses. Page routes are not described.
- The schema of a struct comes from the same `json` and `validate` tags `Bind` reads, so the
  document cannot promise what the validation refuses: `required`, `maxLength`, `enum`,
  `format` (`email`, `url`, `date-time`) and the numeric bounds. Every operation carries the
  `default` response with the `Problem` schema, the shape of every API error since 0.21.0.
- `openapi:` directives for what reading the handler cannot tell — a middleware, a `c.Query`,
  a response built elsewhere: `openapi:response <status> [type]`, `openapi:body <type>`,
  `openapi:query <name> <type> [description]` and `openapi:tag <name>`. A type nobody declares
  is an error naming the file and the handler, not an empty schema.
- `trilha openapi --check` compares the document with the file on disk and exits `1` when they
  differ, the way `gen --check` does; `-o -` writes to stdout; `--title`, `--version` and
  `--server` fill what the code cannot know.
- `examples/orcamento` declares its `mes` parameter and its tag; `examples/blog` declares the
  429 its rate-limit middleware answers.

## 0.21.0 — 2026-09-05

### Changed
- **API errors are now [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details**
  ([#30](https://github.com/emersonjoe/trilha/issues/30)), sent as
  `application/problem+json`. The mapping from the old body: `error` → `title`, `status`
  stays, `fields` stays; `type`, `instance` and `request_id` are new. A client that reads
  `fields` needs no change; one that reads `error` reads `title` instead.
- **The format of an error is negotiated with `Accept`, not guessed from the path.** A
  `KindAuto` route from `route.go` answers `problem+json` unless the client prefers
  `text/html` (ranked by `q`), wherever the route lives — the `/api/` prefix no longer
  forces JSON on a browser, and an API outside `/api/` no longer sends a page to a JSON
  client. `KindAPI` and `KindPage` still do not negotiate. For a request that matches no
  route, `Accept` decides and the `/api/` prefix is the last resort.

### Added
- `trilha.Problem`: `Type`, `Title`, `Status`, `Detail`, `Instance`, `Fields` and `Extra`
  (extension members, written at the top level). Return one from a handler to describe an
  error the client can act on; the framework fills in what is missing.
- `trilha.ProblemType func(status int) string`, for an app that documents its errors at
  URLs of its own, and `trilha.ProblemMediaType`.
- `c.Accepts(offers ...string) string`: content negotiation with `q` values, `type/*` and
  `*/*`. An absent or `*/*` `Accept` picks the first offer.
- `examples/blog` answers 409 with its own `type` and a `slug` extension member when a title
  repeats.

### Notes
- A 5xx never carries a `Detail` the framework derived, in production: the message goes to
  the log with the `request_id` (ASVS V7.4.1). In `Dev` it comes in the response. A `Detail`
  written by the handler is always sent.
- `request_id` travels in the body as well as in `X-Request-ID`, because a script from
  another origin cannot read the header without `Expose-Headers`.

## 0.20.0 — 2026-09-05

### Added
- `Config.CORS` ([#29](https://github.com/emersonjoe/trilha/issues/29)): the cross-origin
  policy of the app in one place — `Origins` (exact, or the single entry `"*"`), `Methods`,
  `Headers`, `Expose`, `Credentials` and `MaxAge`. The zero value is off: no header is added
  and `OPTIONS` keeps reaching the router.
- The `OPTIONS` preflight is answered by the framework, before the router, so it works on
  every route and on static files: 204 with `Allow-Origin`, `Allow-Methods`, `Allow-Headers`
  and `Max-Age` for an allowed origin and method, 403 otherwise.
- `examples/blog` opens `/api` to one origin, with the preflight covered by an integration
  test.

### Notes
- An unsafe or malformed policy panics in `New`, not on the first request from outside:
  `"*"` together with `Credentials`, `"*"` mixed with other origins, and an origin with a
  path, a trailing slash or no scheme.
- `Vary: Origin` goes out with every response that carries an `Origin`, so a shared cache
  never serves the allowed origin's response to somebody else.
- A **simple** request from an unlisted origin is served as usual, only without the CORS
  headers: the browser is what hides the response from the script. Only the preflight is
  refused with a status.

## 0.19.0 — 2026-09-05

### Added
- `c.File(field, trilha.FileRules{...})` ([#28](https://github.com/emersonjoe/trilha/issues/28)):
  one file from a multipart form, answered only after the three checks an upload needs.
  `MaxSize` is a limit per file, apart from `Config.MaxBodyBytes`; `Accept` is matched
  against the media type detected in the first 512 bytes of the content, never the extension
  and never what the client announced; `Optional` says whether an absent field is an error.
- `trilha.Upload`: `Name` sanitised (no directory, no separator of either platform, no
  control character, at most 100 characters, never empty or `..`), `MIME` and `Ext` from the
  content, `Size`, and `File` positioned at the start. `up.Save(dir)` writes inside `dir`
  with mode 0600 under a free name (`note.pdf`, then `note-1.pdf`), so a second upload never
  overwrites the first, and the name cannot walk out of `dir`.
- Two messages in `ValidationMessages`, `filemax` and `filetype`, translated by
  `UseValidationPTBR` like the others.
- `examples/blog` now receives the attachment through `c.File`, showing the message in the
  form (422) instead of answering 500.

### Notes
- A rule that fails is `FieldErrors` under the field's name — the same answer `Bind` gives,
  so the same 422 form flow works for uploads.
- Type detection is `http.DetectContentType`: formats that are a zip inside (`.docx`,
  `.xlsx`) come back as `application/zip` and a CSV as `text/plain`. Where the difference
  matters, the app looks at the content itself.

## 0.18.0 — 2026-09-05

### Added
- Declarative validation in `Bind` ([#27](https://github.com/emersonjoe/trilha/issues/27)):
  the `validate:"required,min=3"` tag next to the field, applied right after the values are
  converted, in the same pass. Rules: `required`, `min`, `max`, `len`, `email`, `url`,
  `oneof` and `eqfield`, each meaning what it should per type (characters for text, value
  for numbers, date for `time.Time`, items chosen for `[]string`). The answer is still
  `FieldErrors`, with every message at once.
- `trilha.Validator` (`interface{ Validate() error }`): a field type checks itself, and a
  struct's own `Validate` runs at the end, only when no field failed — which is what makes a
  check that reads two fields safe. Value and pointer receivers both work.
- `trilha.AddRule(name, func(trilha.Field) bool)` registers a rule for the tag;
  `trilha.Field` carries `Name`, `Param`, `Text`, `Value` and `Other(name)` for a rule that
  compares two fields. A name that already exists panics, and so does an unknown name in a
  tag: a typo would otherwise be a form that accepts anything in production.
- `trilha.ValidationMessages` (message per rule, with `{param}`) and
  `trilha.UseValidationPTBR()`, which also switches `BindInvalid`. Messages are read by the
  person filling the form, which is why they are the one part of the runtime that ships
  translated.
- `BindJSON` validates too, naming the field by its `json` tag so the message comes back
  under a key the client recognises.

### Notes
- `required` means "not the zero value". Where `0` or `false` is a real answer, declare the
  field as a pointer: a `*int` that arrived holding `0` is present.
- Every rule but `required` ignores an empty value, and a value that does not convert gets
  `BindInvalid` and no rule message — one message per field.
- Nothing changed for a struct without `validate` tags.

## 0.17.0 — 2026-09-05

### Added
- HTTP cache on the `Ctx` ([#26](https://github.com/emersonjoe/trilha/issues/26)):
  `c.ETag(tag)`, `c.LastModified(t)` and `c.CacheControl(v)`. The two first write their
  header and report whether the request already had that version; `true` means the `304`
  is written, so the handler returns `nil, nil` and the body never travels. Only `GET` and
  `HEAD` answer `304`; `If-None-Match` accepts a list, `*` and weak tags (RFC 9110
  §8.8.3.2), and when both are declared it is the one that decides, with the date left as
  metadata.
- Files under `static/` now carry an `ETag`: the same content fingerprint that goes in the
  `?v=` of the URL, so a second visit costs a `304` and no bytes. Two deploys of the same
  file keep the same tag.
- The post page in `examples/blog` revalidates with the post's date, alongside
  `Cache-Control: private, no-cache`.

### Notes
- Trilha does not compute an ETag from the rendered body: every response carries a fresh
  CSP nonce, so such a tag would never match twice. The version is the data's, and the
  handler is the one that knows it.

## 0.16.0 — 2026-09-05

### Added
- `cache` package ([#25](https://github.com/emersonjoe/trilha/issues/25)): an in-memory
  cache with expiry, tags and bulk invalidation. `cache.New(cache.Options{Name, MaxEntries,
  Metrics})` is created by the app, not by the framework, and the ceiling is mandatory
  (default 10 000, LRU eviction) — a cache without one is a memory leak that takes a week
  to show up. `Set`/`Get`/`Delete`/`Invalidate`/`Clear`/`Len`/`Stats` are the untyped half;
  `cache.Get[T]`, `cache.Do[T]` and `cache.Once[T]` are the typed half, because Go does not
  allow type parameters on methods.
- `cache.Do(ctx, c, key, fn)` returns the cached value or produces it with `fn`, with one
  flight per name: whoever arrives during a fetch waits for it instead of piling onto the
  database, which is what the first request after an `Invalidate` would otherwise do. An
  error is returned to everyone waiting and cached for nobody, and the cache lock is not
  held while `fn` runs, so a nested `Do` works.
- `cache.Once(c, name, fn)` answers a question once per request and forgets it with the
  response — for what a layout, a page and three components all need to know. It is not the
  cache, and cannot outlive the request the way a per-user value in a shared cache would.
- Four metric series with `Options.Metrics`, labelled by the cache's name:
  `trilha_cache_hits_total`, `trilha_cache_misses_total`, `trilha_cache_evictions_total`
  and `trilha_cache_entries`.

### Changed
- The HELP text of the five framework metrics is now in English, like the rest of the code.
  The series names are untouched, so no dashboard or alert changes.

## 0.15.0 — 2026-09-05

### Added
- `Ctx.Hijack()`, `Ctx.AllowBody(n)` and `Ctx.NoReadDeadline()`
  ([#24](https://github.com/emersonjoe/trilha/issues/24)): the two things a long connection
  and a large body were missing. Trilha's response now implements `http.Hijacker` (libraries
  type-assert on it), and `Hijack` clears the deadlines a hijacked connection would otherwise
  inherit from the server, marks the request as hijacked so the framework writes nothing more
  on it, and logs 101. `AllowBody` replaces `Config.MaxBodyBytes` for one request — call it
  from the route's `middleware.go`, since form CSRF reads the body before the handler runs —
  and going over the new limit is still a 413.
- `ui.UploadTo(id)`, `ui.UploadBar()` and `ui.UploadScript(c)`: a file upload with a progress
  bar, off until a form asks for it. The form is an ordinary
  `multipart/form-data` form with a CSRF field; with JavaScript on, the kit sends it with
  XHR, fills the `<progress>` from the browser's own progress event, fires `trilha:upload`,
  and swaps the answer in through `Trilha-Fragment` — so the same handler answers the piece
  or the whole page. A 5xx, a network error or a piece without the id submits the form for
  real. The behavior ships as a separate `public/ui.upload.js` that `ui.Head` does not load.

### Changed
- `ui.Files` now lists five names; `trilha ui --js-only` writes all three `.js` files.

### Notes
- **WebSocket stays out of core, and that is the decision.** The protocol is transport: it
  touches no route, no layout and no render, while fragmentation, control frames, the close
  handshake, UTF-8 validation, masking, backpressure and `permessage-deflate` are a few
  hundred lines the Autobahn suite tests in 500+ cases. Your app can add `coder/websocket` to
  its own go.mod — principle II binds the framework, not the app — but it cannot take those
  lines out of the framework. `Hijack` is the door, and a real handshake is covered end to
  end in `hijack_test.go`.

## 0.14.0 — 2026-09-05

### Added
- `ui.Navigate(id)`, `ui.NoNavigate()` and `ui.NavigateScript(c)`
  ([#23](https://github.com/emersonjoe/trilha/issues/23)): client navigation, off until a
  region asks for it. A click on a same-origin link inside a marked region fetches the next
  page and replaces one element of the current one, so the header, the sidebar and the
  scroll position around it do not blink. Nothing moves to the client: the address in the
  bar is the one a normal navigation would use, the route answers the same whole document,
  and without JavaScript the link is a link. Back and Forward work and restore the scroll
  position of the entry they return to; `Cmd`-click, `target`, `download` and links to
  another origin are untouched; the region gets `aria-busy` while it waits, focus moves to
  what came in, and `trilha:swap` fires, which is how an island on the new page mounts. One
  request at a time — a second click aborts the first — and a 5xx, a network error, a
  redirect or a page without that id gives up and navigates for real. The behavior ships as
  a separate `public/ui.nav.js` that `ui.Head` does not load, so an app that does not
  navigate this way downloads nothing for it.

### Changed
- `ui.Files` now lists four names and `trilha ui --js-only` writes both `.js` files.

## 0.13.0 — 2026-09-05

### Added
- `Ctx.Island(src, props, children...)`
  ([#22](https://github.com/emersonjoe/trilha/issues/22)): an interactive region inside a
  page that stays static. The server renders the children as the fallback and the browser
  loads one ES module from `public/`, whose default export is the mount function, called with
  the element and the props. The props travel as an escaped attribute and come back through
  `JSON.parse`, so a value from the database is data and never markup; props that do not
  serialize warn once and leave the fallback alone. No bundler, no global hydration: the
  module is addressed through `Asset` (content hash in the URL), only the islands on the page
  are mounted, each once, and the loader is a single inline script carrying the request nonce
  — which is what lets the default CSP accept it without `unsafe-inline`. An island that
  arrives inside a fragment mounts too: the loader listens for `trilha:swap`.

## 0.12.0 — 2026-09-05

The five oldest open issues, all from the same place: an app already running on Trilha
(Partiu, 76 routes) reporting what hurts *after* adoption.

### Added
- `func Config(cfg *trilha.Config) error` is now an accepted form in `app/setup.go`
  ([#15](https://github.com/emersonjoe/trilha/issues/15)): reading the app's own
  configuration is the operation that most often fails on boot, and it can finally fail
  where it happens — the generated file stops the boot with your message. The form without a
  return keeps working, like `Setup` (with error) and `Layout` (without) already did.
- `Config.Mounts map[string]fs.FS` ([#17](https://github.com/emersonjoe/trilha/issues/17)):
  static trees served at URL prefixes, tried before `Public`, longest prefix first, falling
  through when the file is not there. An app that already exists almost never has its disk
  tree shaped like its URL tree, and the two ways out were reorganizing the disk to please
  the router or writing an overlay `fs.FS` by hand.
- `Config.LogRequest func(c *Ctx, status int, dur time.Duration) bool`
  ([#16](https://github.com/emersonjoe/trilha/issues/16)): decides per request, with the
  response already written, what enters the access log. In the reported measurement, 74% of
  the lines were static files answered with 200. It also covers "do not log the health
  check" and "sample 1% of the traffic".
- `trilha gen --check` ([#18](https://github.com/emersonjoe/trilha/issues/18)): generates in
  memory, compares with the committed file and exits 1 showing the differing lines — one
  line in the CI, and a folder added to `app/` without `trilha gen` stops being a 404 nobody
  can explain. The generated file now also carries `//go:generate trilha gen`, and
  `trilha audit` warns when the CLI version differs from the library's in `go.mod`.

### Changed
- The missing-`TRILHA_SECRET` warning moved from every boot to the moment a cookie is
  actually signed ([#19](https://github.com/emersonjoe/trilha/issues/19)), once per cookie
  and naming it and the route. An app with its own session never signs one, and a WARN that
  appears always and never means anything is what teaches a team to stop reading WARN.
- `Asset` fingerprints files in `Mounts` too, and the `name` given to `StaticHeaders` is now
  the URL name, which is what tells one mount from another.

### Fixed
- `trilha audit` never checked calls to `auth.Cognito(...)`: 0.11.0 taught `secretArg` where
  the secret sits but the scan still looked only for `OIDC`, `EntraID` and `Keycloak`, so a
  literal Cognito secret went unreported.

## 0.11.0 — 2026-09-05

### Added
- `auth.Cognito(region, userPoolID, clientID, clientSecret, redirectURL)` (spec 020, part of
  [#41](https://github.com/emersonjoe/trilha/issues/41)): builds the issuer
  `https://cognito-idp.<region>.amazonaws.com/<userPoolID>` and reads roles from
  `cognito:groups`, with no configuration.
- `Provider.LogoutDomain`, for the one thing Amazon Cognito does outside the standard: it
  publishes no `end_session_endpoint`, so ending the session there is `GET /logout` on the
  managed login domain, with `logout_uri` instead of `post_logout_redirect_uri`. Set the
  domain and `Logout` federates; leave it empty and `Logout` clears the local session, logs
  that this is all it did, and does not pretend otherwise. Other providers ignore the field.
- `trilha audit` knows where the client secret sits in `Cognito(...)`, so a literal secret in
  that call is caught like any other.

### Changed
- The authentication chapter and the `auth` reference document the Cognito shortcut in both
  locales, and record why **Clerk** has none: its public documentation describes
  `/.well-known/jwks.json` and an `id_token` with `org_id`, but neither a
  `/.well-known/openid-configuration` — where `auth` reads every endpoint — nor a claim
  carrying the role in the organization. A shortcut built on a guess would be worse than
  none; [#41](https://github.com/emersonjoe/trilha/issues/41) stays open for it.

## 0.10.0 — 2026-09-05

### Added
- Fragments (spec 018, issues #20 and #21): `Ctx.Fragment()` returns the id the client wants
  to swap (the `Trilha-Fragment` header). On a fragment request the same route answers with
  no layouts, no document envelope and no dev server script; every HTML response now carries
  `Vary: Trilha-Fragment`, and a redirect becomes **204 with `Trilha-Location`** so the
  client navigates for real. Middleware, CSRF and status behave as before.
- `ui.Swap(id)` and `ui.NoPush()`: a marked `<a>` or `<form>` swaps element `#id` only, with
  `aria-busy` while it waits, focus on the first `[aria-invalid=true]` on 422, focus and
  caret handed back to the field in use otherwise, hydration of what came in and a
  `trilha:swap` event. On 5xx, a network error or a fragment without the id, the kit gives up
  and navigates or submits normally — with JavaScript off, link and form work as they always
  did. `window.ui.swap` and `window.ui.hydrate` do the swap by hand.
- `examples/cadastro`: a search that filters the list and a form that saves without
  reloading, with tests covering both paths (with and without the header).
- "Interactivity" chapter on the site (`/learn/interactivity`, `/pt/aprender/interatividade`)
  and reference entries for `Ctx.Fragment`, `ui.Swap` and `ui.NoPush`.

## 0.9.0 — 2026-09-05

### Added
- Internationalization (spec 015). Everything public is English by default with a Brazilian
  Portuguese translation:
  - Site: English at `/`, `/learn`, `/reference`; Portuguese at `/pt`, `/pt/aprender`,
    `/pt/referencia`. `<html lang>`, `hreflang` alternates (`en`, `pt-BR`, `x-default`) and a
    language switcher on every page. The old `/aprender/...` and `/referencia/...` URLs answer
    301 to `/pt/...`. A test keeps both locales in sync (same pages, same demos).
  - `README.md` in English + `README.pt-BR.md`; `CONTRIBUTING`, `GOVERNANCE`, `SECURITY`,
    `SUPPORT` and `CODE_OF_CONDUCT` in English with translations in `docs/pt-BR/`; issue and
    PR templates in English.
  - CLI messages in English by default, Portuguese when `TRILHA_LANG` (or `LC_ALL`,
    `LC_MESSAGES`, `LANG`) starts with `pt`. `trilha new --lang en|pt` picks the language of
    the generated texts and `<html lang>`; the default follows the CLI language.
- `App.Export` writes an HTML redirect stub (`meta refresh` + canonical + `noindex`) for
  pages that answer a same-site 3xx, so renamed URLs keep working on static hosts. Redirects
  to another origin or to the page itself are export errors.

### Changed
- Every message from the runtime, scanner, generator, dev server, scaffold, `auth`, health
  and metrics is now in English (they end up in your code, logs and terminal). `trilha.BindInvalid` defaults to
  `"invalid value"`; the export marker file says `generated by trilha export`.
- `scaffold.UIResult.Action` uses the English constants `UICreated`, `UIUpdated`, `UIKept`,
  `UIKeptTheme`, `UIModified`; the CLI translates them for display.
- Constitution 1.2.0: "English by default, Portuguese as a translation" replaces "public
  texts in Portuguese". Specs and the constitution itself stay in Portuguese; the `examples/`
  apps stay in Portuguese and the documentation says so.

## 0.8.0 — 2026-09-05

### Added
- `Ctx.Asset` and `App.Asset` (spec 017): the address of a file in `Config.Public` carries
  the content hash (`/site.css?v=8f3a1c92`), with `BasePath` applied. A request whose
  version matches gets `public, max-age=31536000, immutable`; a wrong or missing version
  keeps the previous behavior, and in `dev` nothing is immutable. The file is read once in
  production; in `dev` a `Stat` decides whether to re-read it. A path that does not exist
  comes back unversioned, with a warning.
- `trilha audit`: warning when `immutable` shows up in a project that does not use `Asset`.

### Changed
- `ui.Head`, the site layout, the examples and `trilha new` now link assets through
  `c.Asset`. This fixes the problem that started the spec: publishing the site left, for up
  to ten minutes, new HTML with old CSS and JS.

## 0.7.0 — 2026-09-05

### Added
- OpenID Connect authentication (spec 016) in the `auth` package, with no external
  dependency: `auth.OIDC`, `auth.EntraID` and `auth.Keycloak`; `Start`/`Callback`/`Logout`
  with PKCE (S256), `state` and `nonce` in 10-minute signed cookies; `id_token` validation
  against the provider's JWKS (RS256/384/512 and ES256/384, `kid` required, one-hour cache
  with key rotation throttled to one fetch per minute); session in a signed cookie with an
  absolute deadline, idle window and a new identifier on every login; optional `Store`
  (`auth.NewMemoryStore`) for immediate revocation; `Require`, `RequireRole`, `Optional` and
  `User`; roles read from each provider's place (`roles`/`groups`/`wids` on Entra ID,
  `realm_access` and `resource_access[client]` on Keycloak) and from `Options.RoleClaims`;
  federated logout when the provider publishes `end_session_endpoint`.
- `examples/sso`: protected area, required role, API answering 401 as JSON and logout, with
  integration tests. Chapter and reference on the site.
- `trilha audit`: client secret written in the code and `redirect_uri` over `http://`
  outside `localhost` (both critical).

## 0.6.0 — 2026-09-05

### Added
- Observability (spec 014): `/_trilha/health/live` and `/_trilha/health/ready` probes with
  `App.Check` (deadline, parallel execution, cache and `application/health+json` response),
  `App.HealthReport`; metrics registry `App.Metrics()` (`Counter`/`Gauge`/`Histogram` with
  labels and a cardinality cap) exposed in the Prometheus text format when
  `Observability.Metrics` is configured; framework metrics (requests, latency, in flight,
  security events, panics, runtime and `trilha_build_info`); `Ctx.TraceID` and `Ctx.Log`
  with `traceparent` propagation (W3C Trace Context); `Config.Observability` with a
  gatekeeper by token (`TRILHA_OBS_TOKEN`, constant-time comparison) or trusted network;
  three new items in `trilha audit`; chapter and reference on the site.

### Changed
- Health details and metrics are closed by default outside `dev`: without a token or a
  trusted network, the metrics endpoint answers 401 and health returns only `status`
  (NIST SP 800-53 AU-9, OWASP API Security 2023 API8).

## 0.5.3 — 2026-09-05

### Fixed
- Site: the "Form with CSRF in one line" demo did not react to submit — the
  `onclick="return false"` on the `<form>` itself cancelled every click inside it (spec 013).
  The submit is now intercepted by `tema.js` and shows the `POST → 303 → GET /eventos/<slug>`
  flow; without JavaScript the form just reloads the page.
- No inline event handlers on the site or in the examples (Trilha's default CSP blocks them):
  a leftover `onchange=""` was removed from the budget example, and a test sweeps every page
  to keep them out.

### Added
- Spec 012 (documented backlog): reduce the fixed per-request cost measured in spec 011 (CSP
  rebuilt on every request, nonce drawn even for API routes, value map always allocated, log
  formatted even when discarded).

## 0.5.2 — 2026-09-05

### Added
- Benchmarks (spec 011): `bench/` module comparing against the standard library (page, JSON,
  static file, 200 routes, middlewares), `make bench`/`make bench-results`,
  `bench/RESULTS.md`, the "Performance and comparison" page on the site and a CI job.

## 0.5.1 — 2026-09-05

### Added
- Statistics (spec 010): cookie-free page counts on the site via GoatCounter, enabled by the
  `SITE_ANALYTICS` variable; `scripts/traffic.sh` and the `traffic` workflow (daily snapshot
  of repository traffic on the `stats` branch, optional through `TRAFFIC_TOKEN`).

## 0.5.0 — 2026-09-05

### Added
- Examples (spec 009): `examples/cadastro` (medium) and `examples/orcamento` (complex), with
  READMEs and tests; "Examples" chapter on the site.
- `c.Bind(&struct)` (form or JSON, nested structs with a prefix), `trilha.FieldErrors` (422
  with `fields` in APIs), `c.Render(code, node)` (page with layouts from a POST); in the kit:
  `ui.Errors`, `ui.InvalidIf`, `ui.SelectOptions`, `ui.Checked`.

## 0.4.0 — 2026-09-05

### Added
- UI kit (spec 006): `ui` package (components, variants, Lucide icons), assets
  `public/ui.theme.css`/`ui.css`/`ui.js` copied by `trilha new` and `trilha ui`, theme
  contract compatible with shadcn/ui v4; `blog` and `assistente` examples restyled; live
  demos on the site. `h`: repeated `class` attributes are merged into one.

## 0.3.0 — 2026-09-05

### Added
- Issues #10–#14 (spec 008): `Config.DevReload` and `TRILHA_DEV_RELOAD=off` disable the
  reload script in dev; `Route.Kind` (`KindAuto`/`KindPage`/`KindAPI`) and
  `var Kind = trilha.KindPage` in `route.go`; `App.OnShutdown`, `Timeouts.Shutdown` and an
  optional `func Shutdown(a *trilha.App) error` in `setup.go`; the generator omits `main()`
  when the package already has one; folders with a dot in the name (`app.css/` → `/app.css`)
  documented and tested.

### Fixed
- `not_found.go`/`error.go`/`page.go` that write the response and return `(nil, nil)` no
  longer get a second document on top (#11).
- A `route.go` route reached by a browser (`Accept: text/html`, outside `/api/`) gets the
  HTML error page instead of JSON (#12).

## 0.2.0 — 2026-09-05

### Added
- Adoption (spec 007, issues #6–#9): optional `func Config(cfg *trilha.Config)` in
  `setup.go`, called before `trilha.New`; derived fields (`Logger`, `Secret`, `RateLimit`,
  `TrustedProxies`) reapplied when serving starts, so changes in `Setup` count;
  `trilha.NoTimeout`; `Config.StaticCacheControl` and `Config.StaticHeaders`;
  `Ctx.SetContext` and `Ctx.SetRequest`.
- AI (spec 005): `ai` package (OpenAI-compatible client with `Chat`/`Stream`, `Tool`/`Typed`,
  `Agent` with `Run`/`RunStream`, handoffs, `AsTool`, `Parallel`, `Chain`) and `ai/mcp` (MCP
  client and server over stdio and Streamable HTTP); `c.Stream()` for Server-Sent Events;
  `examples/assistente`.
- Security (spec 004): `Config.Security` (CSP with a per-request nonce, HSTS,
  Permissions-Policy, COOP), `Config.TrustedProxies` and `c.ClientIP()`, `Config.RateLimit`
  and `trilha.Limit`, signed cookies (`c.SetSigned`/`c.Signed`,
  `TRILHA_SECRET`/`TRILHA_SECRET_PREVIOUS`), `Config.Timeouts`, security events
  (`Config.OnSecurityEvent`), `c.Nonce()`/`trilha.NonceAttr`, `c.NoWriteDeadline()`,
  `a.Config()`, and the `trilha audit` command.
- `trilha export` and `App.Export`: export of static routes to HTML, with `404.html` and a
  copy of `public/`; `App.AddExportPath` for dynamic routes; `Ctx.Base()` and
  `TRILHA_BASE_PATH` for sites under a subpath.
- `trilha.Run(app)`: the generated `main` now calls it (serve or export).
- Documentation site in `site/`, built with Trilha itself and published on GitHub Pages.
- Community files: CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, SUPPORT, GOVERNANCE, issue and
  PR templates, CODEOWNERS, Dependabot.

### Changed
- `X-Forwarded-Proto`/`X-Forwarded-For` are only honored when coming from `TrustedProxies`
  (before, `X-Forwarded-Proto: https` from any origin marked cookies as `Secure`).
- The generated file calls `trilha.Run(newApp())` instead of `ListenAndServe` directly
  (regenerate with `trilha gen`).

## 0.1.0 — 2026-09-05

### Added
- File-based routing in `app/`: `page.go`, `route.go`, `layout.go`, `middleware.go`,
  `not_found.go`, `error.go`, `setup.go`; `name_` segments, `name__` catch-all, `name-`
  groups.
- Runtime: `App`, `Ctx`, errors as values, CSRF, security headers, body limit, embedded
  static files, `slog` logs.
- `h` DSL and `tmpl` adapter for `html/template`.
- CLI: `new`, `gen`, `dev` (proxy + SSE reload, no rebuild for `public/`), `build`, `routes`.
- `examples/blog` and the test suite (unit, golden, integration, e2e).
