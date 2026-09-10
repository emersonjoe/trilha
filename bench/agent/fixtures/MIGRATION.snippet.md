## app/documentos/page.tsx → app/documentos/page.go

| What it was | What it becomes |
|---|---|
| `useSearchParams` for `q`, `tipo`, `sort`, `dir`, `page` | `trilha.ListParams` embedded in the screen's own query struct, read by `c.Bind` |
| `<table>` with sortable headers and `aria-sort` | `ui.DataTable` with `ui.Columns`; `Sort: true` is what makes a column orderable, and the kit writes `aria-sort` |
| `useState` + `apiGet` + `setInterval(…, 5000)` | the handler reads the store and answers; `ui.Poll("5s", "")` asks for the fragment again |
| the loading state | there is none: the server answers with the data already in it |
| the pagination `<nav>` rebuilding the query string | `ui.ListState` — the kit's links carry the filter, which is the part hand-written ones lose |
| `'use client'` | nothing. If any JavaScript of your own ended up in `public/`, the page was not ported |

The API stays where it is: this screen used to call `/api/documents` from the browser with the
session's token in reach of anybody with the developer tools open. In Trilha the handler reads on
the server, and nothing about the credential reaches the page.
