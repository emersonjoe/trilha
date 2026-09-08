---
title: A React component as an island
description: Vendor preact and htm, mount an existing component inside c.Island, and let it talk back with island.post.
---

A team arrives with a component that already works — a date picker, an editor, a chart — and
the question is where it goes. It goes inside an island: the page stays server-rendered, one
element of it is React, and nothing else on the site pays for it.

## The module comes down once

There is no bundler and no install step, so the runtime is a file in the repository like any
other asset:

```bash
trilha vendor preact@10.19.3
trilha vendor htm@3.1.1
```

That writes `public/vendor/preact.js` and `public/vendor/htm.js` and records the version, the
sha256 and the URL in `vendor.lock`, which is committed. `trilha vendor --check` re-hashes
them in the CI, and `trilha audit` says something if a file shows up under `public/vendor/`
that the lock does not name.

Preact plus `htm` is the pair that needs no build: `htm` is JSX as a tagged template, so the
component is written the way the team writes it and the browser reads it directly. A team on
React proper vendors `react` and `react-dom` the same way and mounts with `createRoot`.

## The page

Nothing about the page is React-aware. It renders the fallback and hands the component its
props, as a struct so the types come out on the other side:

```go
// EditorProps is what the editor island gets. It is a struct, not a map, so
// `trilha gen` can write public/islands.d.ts and the editor on the JavaScript
// side knows the same names the page does.
type EditorProps struct {
	// PalavrasPorMinuto is the reading speed the counter divides by.
	PalavrasPorMinuto int `json:"palavrasPorMinuto"`
	// Rascunho is where the island saves without leaving the page.
	Rascunho string `json:"rascunho"`
}
```

The children of `c.Island` are the fallback: the form the visitor gets with the script
blocked, still on its way, or broken. That is the part a React app usually does not have.

## The island mounts the component

```js
// public/ilha-editor.js
import { h, render } from "/vendor/preact.js";
import htm from "/vendor/htm.js";

const html = htm.bind(h);

function Editor({ props, island }) {
  const [aviso, setAviso] = useState("");
  const salva = async (texto) => {
    try {
      const r = await island.post(props.rascunho, { corpo: texto });
      setAviso(`saved · ${r.palavras} words`);
    } catch (e) {
      setAviso(e.name === "IslandInvalid" ? e.fields.corpo : "could not save");
    }
  };
  return html`<${Campo} onSave=${salva} aviso=${aviso} ppm=${props.palavrasPorMinuto} />`;
}

/** @type {import("/islands.d.ts").IslandMount<"/ilha-editor.js">} */
export default function (el, props, island) {
  render(html`<${Editor} props=${props} island=${island} />`, el);
}
```

`island` is the third argument the loader passes: `island.post` sends JSON with the CSRF
token already on it, a `422` comes back as an `IslandInvalid` whose `.fields` is the same
object the form would have shown, and `island.signal` is aborted when the element leaves the
page — which is where a React `useEffect` cleanup would otherwise leak.

The `IslandMount` type comes from `public/islands.d.ts`, written by `trilha gen` from the
`c.Island` calls in `app/`. The editor then checks that `props.rascunho` exists and that
`props.palavrasPorMinuto` is a number, without anyone writing the types twice.

## The route it talks to

The other side is a `route.go` like any other. It is an API — its errors are problem+json,
which is what the island reads — but its client is the page, with the page's cookies, so it
asks for the token:

```go
func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error {
	return trilha.RequireCSRF(c, next)
}
```

`c.BindJSON` validates the body with the same `validate` tags the form uses, and the
`FieldErrors` it returns become the `422` the island already knows how to show.

## What this is not

It is not a single-page app with a Go backend. The routing, the layouts, the navigation and
every other screen stay on the server; React is one element of one page, and the page works
without it. If the answer to "which parts are islands?" is "all of them", the thing being
built is a SPA, and Trilha is the wrong tool for it — that is a real answer, not a missing
feature.
