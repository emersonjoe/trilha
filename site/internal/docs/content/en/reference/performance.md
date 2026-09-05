---
title: Performance and comparison
description: How much Trilha costs over the standard library, how to measure it yourself, and how it compares with other approaches.
---

## Methodology

The only number worth publishing is the **cost of the framework over the standard library**,
which is the real alternative in Go. The benchmarks live in `bench/` (a separate module, so
Trilha stays dependency-free) and measure, in process (`httptest`, no network), the same
work done two ways: with Trilha and with plain `net/http` + `html/template`.

```bash
git clone https://github.com/emersonjoe/trilha && cd trilha
make bench            # runs; make bench-results rewrites bench/RESULTS.md
```

Scenarios: page with layout and 20 items (`h` × `html/template`), JSON response, static
file (`Public` × `http.FileServer`), 200 routes with a parameter (`ServeMux` on both sides)
and a chain of 5 middlewares.

## Reference results

Apple M2, Go 1.25, 2026-09-05 (median of 3 runs; `bench/RESULTS.md` has the full output).
Values per request.

| Scenario | Stdlib | Trilha | Difference |
|---|---|---|---|
| Page (20 items, layout) | 29.4 µs · 270 allocs | 19.4 µs · 482 allocs | `h` is ~34 % faster than `html/template` here, with more allocations |
| JSON (20 items) | 4.2 µs | 7.6 µs | +3.4 µs |
| Static (1.4 KB) | 1.4 µs | 4.3 µs | +2.9 µs |
| 200 routes + parameter | 0.72 µs | 4.0 µs | +3.3 µs |
| 5 middlewares | 0.64 µs | 4.1 µs | +3.4 µs |

Honest reading: Trilha has a **fixed cost of ~3 µs and ~40 allocations per request**,
regardless of the route. It pays for: request id (random), CSP nonce, security headers,
`Ctx` with a value map, body limit, timing and **structured logging** of every request
(`slog`, which formats the line even when discarded). In a real server a database query
costs 100 µs to a few ms, and the network more; the difference disappears. If that ever
matters to you, the path is reducing allocations in `Ctx` and making logging optional per
route — and the benchmark is there to prove the gain.

The **edit → see** cycle of `trilha dev` is ~1.2 s in the blog example (Go recompilation)
and ~30 ms for changes only in `public/` (`make reload` measures on your machine).

## Comparison of approach

No third-party numbers: versions change, configurations differ and each project optimizes
for different things. What can be compared safely is the **approach**. Always check each
project's documentation; names cited are trademarks of their respective owners and there is
no affiliation.

| | Trilha | plain `net/http` | Go routers (chi, echo, gin, fiber) | templ + htmx | Next.js |
|---|---|---|---|---|---|
| Routes | by folders in `app/` (`page.go`, `route.go`) | registered by hand | registered by hand | registered by hand (with the router you choose) | by folders in `app/` |
| Nested layouts | `layout.go` per folder | manual | manual | components | `layout.tsx` |
| HTML | typed `h` DSL (escaped by default) or `html/template` | `html/template` | `html/template` or libs | `templ` (compiled) | JSX/React |
| Client interactivity | HTML + `ui.js` (200 lines) or htmx; no hydration | your choice | your choice | htmx | React (hydration, RSC) |
| Runtime dependencies | none | none | the router (+ deps) | `templ` (+ generator) | Node, React, Next |
| Dev | `trilha dev`: ~1 s reload, compile error on the page | manual `go run` | `air`/manual | `templ generate --watch` + reload | `next dev` (HMR) |
| Production | one static binary with `public/` embedded | binary | binary | binary | Node or edge; build |
| Static export | `trilha export` | manual | manual | manual | `output: 'export'` |
| Default security | CSP with nonce, HSTS, CSRF, rate limit, signed cookies, timeouts | nothing (you configure) | varies | nothing (you configure) | basic headers; CSRF in Server Actions |
| AI | `ai` (OpenAI-compatible), `ai/mcp` | — | — | — | Vercel AI SDK (package) |

When **not** to use Trilha: apps that need a highly interactive client UI (editors,
real-time dashboards with complex state) are better served by React/Next or by an SPA; and
projects that already have a Go router and mature templates gain little by switching. Trilha
shines in server-rendered business apps, content sites and APIs with a dashboard, where a
dependency-free binary and strong conventions weigh more than fine-grained interactivity.
