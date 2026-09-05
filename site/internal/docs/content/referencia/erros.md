---
title: Erros
description: Os valores de erro que o Trilha entende e como cada um vira resposta.
---

Handlers devolvem `error`. O Trilha traduz:

| Valor | Página (`page.go`) | API (`route.go`) |
|---|---|---|
| `nil` | resposta escrita pelo handler; 204 se nada foi escrito | idem |
| `trilha.ErrNotFound` (ou erro que o embrulha) | 404 com `not_found.go` | `{"error":"Not Found","status":404}` |
| `*trilha.RedirectError` via `trilha.Redirect(url)` (303) ou `trilha.RedirectCode(url, code)` | redirecionamento | redirecionamento |
| `*trilha.HTTPError` via `trilha.Errorf(code, fmt, a...)` | página simples com o status e a mensagem (4xx) | `{"error":"mensagem","status":code}` |
| qualquer outro `error` | 500 com `error.go`; detalhe só em dev | `{"error":"Internal Server Error","status":500}` |
| `panic` no handler | recuperado e tratado como 500; stack só em dev | idem |

### Página ou JSON?

A coluna é decidida por rota, com um desempate por requisição:

- `page.go` → sempre página.
- `route.go` → JSON, **exceto** numa navegação de navegador: `Accept` com `text/html` e sem
  `application/json`, fora de `/api/`. Assim um `route.go` que serve HTML mostra a página
  de erro em vez de `{"error":...}`; `fetch` sem `Accept` (`*/*`), `curl` e clientes JSON
  continuam recebendo JSON.
- `route.go` pode fixar o comportamento exportando `var Kind = trilha.KindPage` (página
  sempre, e CSRF exigido em `POST`/`PUT`/`PATCH`/`DELETE`) ou `trilha.KindAPI` (JSON sempre).

### Responder por conta própria

`not_found.go`, `error.go` e `page.go` podem escrever a resposta inteira e devolver
`(nil, nil)`: o Trilha não põe nada em cima. Serve para um 404 em texto puro
(`http.NotFound(c.Writer(), c.Request())`), outro `Content-Type` ou outro status. Se a
função devolve `nil` **sem** escrever, vale a página simples do framework (404/500); em
`page.go`, 204.

Mensagens de `HTTPError` com código 5xx nunca são mostradas ao cliente. Todo erro 5xx vai
para o log com o `request_id`.

```go
if ev, ok := eventos.Buscar(slug); !ok {
	return trilha.ErrNotFound
}
if vagas < 0 {
	return trilha.Errorf(422, "vagas não pode ser negativo")
}
return c.Redirect("/eventos/" + ev.Slug)
```

Erros de `c.BindJSON` e `c.FormErr` já são `HTTPError` (400 ou 413): basta devolvê-los.

## FieldErrors

`trilha.FieldErrors` é `map[string]string` (campo → mensagem) que implementa `error`.
Devolvido de um handler responde **422**: JSON com `"fields"` em rotas de API, página de
erro em páginas. Um formulário normalmente não o devolve: valida, e no erro chama
`c.Render(422, …)` mostrando cada mensagem no campo (`ui.Errors`, `ui.InvalidIf`).

| Método | Papel |
|---|---|
| `Add(campo, msg)` | registra (a primeira mensagem do campo vence) |
| `Has(campo) bool`, `Get(campo) string` | consulta |
| `Any() bool` | há erros? |
| `OrNil() error` | `nil` quando vazio, para `return errs.OrNil()` |
