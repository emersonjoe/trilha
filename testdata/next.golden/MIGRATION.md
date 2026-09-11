# Migration report

Read from `app`: 13 files with somewhere to go. The tree beside this file is the skeleton — folders, packages and signatures. What each screen does is still yours to port; this table is so you do not have to open them to find out.

## Screens

| Source | Here | URL | Client | Calls | Suggested |
|---|---|---|---|---|---|
| `app/(marketing)/about/page.tsx` (11) | `marketing-/about/page.go` | `/about` | no | — | A — no island signal of its own |
| `app/api/documents/route.ts` (15) | `api/documents/route.go` | `/api/documents` | no | — | — |
| `app/api/health/route.ts` (2) | `api/health/route.go` | `/api/health` | no | — | — |
| `app/dashboard/layout.tsx` (4) | `dashboard/layout.go` | `/dashboard` | no | — | — |
| `app/dashboard/page.tsx` (15) | `dashboard/page.go` | `/dashboard` | yes (1 useState, 1 useMemo) | GET /api/metrics?range=:range | C — pointer (line 10) and live svg (line 10) |
| `app/docs/[[...slug]]/page.tsx` (4) | `docs/slug__/page.go` | `/docs/{slug...}` | no | — | A — no island signal |
| `app/documents/[id]/page.tsx` (24) | `documents/id_/page.go` | `/documents/{id}` | yes (2 useState, 1 useEffect, 1 useRef) | GET /api/documents/:id<br>POST /api/documents/:id/reprocess<br>GET /api/documents/:id/status | B — polling (line 12) |
| `app/documents/page.tsx` (15 + 12) | `documents/page.go` | `/documents` | yes (2 useState, 1 useEffect) | GET /api/documents?q=:query | A — no island signal |
| `app/error.tsx` (6) | `error.go` | `/` | yes | — | — |
| `app/files/[...path]/page.tsx` (4) | `files/path__/page.go` | `/files/{path...}` | no | — | A — no island signal |
| `app/not-found.tsx` (4) | `not_found.go` | `/` | no | — | — |
| `app/page.tsx` (11) | `page.go` | `/` | no | — | A — no island signal |
| `app/users/[user-id]/page.tsx` (6) | `users/user_id_/page.go` | `/users/{user_id}` | no | GET /api/users/:user_id | A — no island signal |

The suggestion is mechanical, and it is here to be argued with — the reason beside each class says which line it came from: **C** when the file shows a pointer handler, a drawing surface (`<canvas>`, or an `<svg>` something actually draws on — an icon is an icon) or an editor — the browser is doing the work, so it becomes an island; **B** when it polls, opens a modal, has tabs or takes a file — the kit does that without a bundle; **A** otherwise, which is a form and a list, and the whole screen fits on the server.

## Global dependencies

Reached from a `layout.tsx`: the frame around every screen, not the work of any one of them. It is ported once — into the layout, or into a single island inside it — and it does not change the class of the screens it wraps.

- `components/Chat.tsx` (19 lines): polling — on its own it would be B.

## No equivalent

- `app/dashboard/(.)modal/page.tsx`: intercepting route: no equivalent — a modal over a page is ui.Dialog on the page that opens it ((.)modal)
- `app/dashboard/@sidebar/page.tsx`: parallel route: there is nothing like it here — render the slots as parts of the page (@sidebar)
- `app/dashboard/template.tsx`: template: a layout that remounts has no meaning without a client router
- `app/docs/[[...slug]]/page.tsx`: optional catch-all: the address without the segment needs its own page ([[...slug]])
- `app/documents/[id]/loading.tsx`: loading state: the page arrives filled, so there is no moment to fill
- `app/layout.tsx`: root layout: yours already exists and is a whole <html> document — port the head and the frame into it
- `app/loading.tsx`: loading state: the page arrives filled, so there is no moment to fill
- `app/users/[user-id]/page.tsx`: parameter renamed to a Go identifier — the URL and c.Param use the new name ([user-id])
- `middleware.ts`: middleware: becomes app/<branch>/middleware.go, or Auth.Require() — there is no automatic translation
- `next.config.ts`: rewrite: becomes a line in Config.Upstreams, with the credential the proxy injects (/api/:path* → https://api.example.com/:path*)

## What to do next

1. `trilha gen` and `go build ./...`: the skeleton compiles as it is.
2. Port the **A** screens first — they are forms and lists, and they close fast.
3. `trilha ui describe` says what the kit has, so you do not have to guess at a name.
4. The cookbook page *From Next.js to Trilha* has the React pattern beside the line that replaces it.
