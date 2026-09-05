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
