## app/painel/documentos/page.tsx → app/painel/documentos/page.go

The API stays where it is. What moves is who calls it.

| What it was | What it becomes |
|---|---|
| `fetch(...)` in a `useEffect` | the handler calls the API and answers with the data already in hand |
| `localStorage.getItem('api_token')` | the session. The token is what the login stored in it, and it never reaches the page |
| `Authorization: Bearer ${token()}` written in the browser | a header the server adds on the way out, from the session |
| `openapi.json` read by hand into a `type Document = { … }` | the Go types of the same document, generated from it, so a field that changes name stops compiling |
| `?q=` read with `useSearchParams` and sent to the API | the same: the URL is the filter, and the filter is the API's parameter — not a slice filtered in Go after asking for everything |
| the loading state, the error state | there is none to write for loading; the API's error is a screen the server renders |
| `'use client'` | nothing. If any JavaScript of your own ended up in `public/`, the page was not ported |

`Config.Upstreams` already forwards `/api/` to the same API with the same credential. That is
for what the browser itself has to fetch — a download, an island. A listing is not one of those:
it is rendered on the server, so it does not go through the proxy.
