---
title: Erros
description: Os valores de erro que o Trilha entende e como cada um vira resposta.
---

Handlers devolvem `error`. O Trilha traduz:

| Valor | Página (`page.go`) | API (`route.go`) |
|---|---|---|
| `nil` | resposta escrita pelo handler; 204 se nada foi escrito | idem |
| `trilha.ErrNotFound` (ou erro que o embrulha) | 404 com `not_found.go` | 404 `problem+json`, `"title":"Not Found"` |
| `*trilha.RedirectError` via `trilha.Redirect(url)` (303) ou `trilha.RedirectCode(url, code)` | redirecionamento | redirecionamento |
| `*trilha.HTTPError` via `trilha.Errorf(code, fmt, a...)` | o status, com o `error.go` (4xx) | o status, com a mensagem em `detail` (4xx) |
| qualquer outro `error` | 500 com `error.go`; detalhe só em dev | 500, com `detail` só em dev |
| `*trilha.Problem` | o status, com o `error.go` | o problema, do jeito que foi escrito |
| `panic` no handler | recuperado e tratado como 500; stack só em dev | idem |

### Página ou problem+json?

A coluna é decidida por rota; o desempate é o cabeçalho `Accept`, ranqueado por `q`:

- `page.go` → sempre página. Um fragmento trocado na página precisa de HTML mesmo quando o
  `fetch` diz outra coisa.
- `route.go` → `problem+json`, **exceto** quando o `Accept` prefere `text/html` a
  `application/json` — o navegador na barra de endereço. O caminho não entra na conta: um
  `route.go` dentro de `/api/` mostra a página de erro para o navegador como qualquer outro.
- `Accept` ausente, ou `*/*` (`fetch`, `curl`), não é preferência: quem decide é o tipo da
  rota.
- `var Kind = trilha.KindPage` (sempre página, com CSRF exigido em
  `POST`/`PUT`/`PATCH`/`DELETE`) ou `trilha.KindAPI` (sempre `problem+json`, diga o `Accept` o
  que disser) fixa o comportamento. Ele é herdado pela subárvore inteira, então um `kind.go`
  na raiz de um ramo decide todo `route.go` abaixo dele; veja
  [Convenções de arquivo](/pt/referencia/convencoes#o-kind-segue-a-subarvore).
- Sem rota nenhuma (404) não há tipo para perguntar: decide o `Accept` e, quando ele está
  mudo, o prefixo `/api/` é o último recurso.

### Uma página para todo status menos o 404

O `app/error.go` responde **todo** status de erro, não só os 5xx: um 403 num app com papéis
é a resposta mais comum depois do 200, e merece o menu, o texto e o layout do app. O
`app/not_found.go` continua com o 404 — ele existe e é o lugar.

A assinatura não muda; o status vem do erro:

```go
func Error(c *trilha.Ctx, err error) (h.Node, error) {
	switch trilha.StatusOf(err) {
	case http.StatusForbidden:
		return painel.Negado(c), nil
	default:
		return painel.Erro(c), nil
	}
}
```

`trilha.StatusOf(err)` diz o status que o framework vai mandar — a mesma classificação da
tabela acima. (`c.Status` é um setter; a página recebe o erro, não o código, e é por isso que
a função existe.)

A página do próprio framework continua como rede, com o texto de sempre: para o app que não
tem `error.go` e para o `error.go` que falha. Rota de API (`KindAPI`) segue intocada:
`problem+json` como antes.

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

## Problem

Erro de API é *problem details*, do [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457),
enviado como `application/problem+json`:

```json
{"type":"about:blank","title":"Unprocessable Entity","status":422,
 "instance":"/api/posts","request_id":"01J…","fields":{"title":"obrigatório"}}
```

Devolva um `*trilha.Problem` para dizer mais do que um status:

```go
return &trilha.Problem{
	Type:   "https://exemplo.com/probs/sem-saldo",
	Title:  "Sem saldo",
	Status: http.StatusPaymentRequired,
	Detail: "A conta tem R$ 3,00 e a operação custa R$ 10,00.",
	Extra:  map[string]any{"saldo": 300},
}
```

| Campo | Papel |
|---|---|
| `Type` | URI que nomeia o tipo de problema; padrão `about:blank` |
| `Title` | resumo curto, igual em toda ocorrência; padrão o texto do status |
| `Status` | status HTTP |
| `Detail` | o que aconteceu **desta** vez; é lido por uma pessoa |
| `Instance` | esta ocorrência; padrão o caminho da requisição |
| `Fields` | os `FieldErrors` de um 422 |
| `Extra` | membros de extensão, escritos no objeto de cima (o `saldo` do exemplo) |

`trilha.ProblemType` (um `func(status int) string`) preenche o `Type` de todo problema que
não trouxer um — para o app que documenta os próprios erros em uma URL sua.

Em produção, um 5xx nunca leva `Detail`, e a mensagem vai para o log com o `request_id`; em
`Dev`, ela vem na resposta. O `Detail` que **você** escreveu é seu e sai sempre: a regra é
sobre o que o framework vazaria, não sobre o que você decidiu contar.

## Negociação de conteúdo

`c.Accepts(ofertas...)` devolve a oferta que o cliente prefere, ranqueada pelos `q` do
`Accept`, ou `""` quando ele não aceita nenhuma. `Accept` ausente ou `*/*` não é preferência,
então ponha o seu padrão primeiro:

```go
switch c.Accepts("text/html", "application/json") {
case "application/json":
	return c.JSON(200, ev)
default:
	return c.Render(200, pagina(ev))
}
```

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
