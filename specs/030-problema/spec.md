# Spec 030 — Erro de API no padrão RFC 9457 e negociação de conteúdo

- **Issue**: [#30](https://github.com/emersonjoe/trilha/issues/30) (ROADMAP, Fase 3, item 12)
- **Branch**: `030-problema`
- **Versão**: 0.21.0

## Por quê

O erro de API do Trilha hoje é `{"error":"Not Found","status":404}` — um formato que só o
Trilha entende. Existe um padrão para isso desde 2016 (hoje RFC 9457,
`application/problem+json`), e ele já é lido por cliente gerado, gateway, painel de erro e
teste de contrato. Enquanto o corpo for caseiro, cada consumidor da API escreve o adaptador
dele.

O segundo problema é como o framework decide **em que formato** o erro sai. Hoje é o
caminho: `/api/` no começo da URL, mais um `strings.Contains(accept, "application/json")`.
Quem põe a API em `/v2/` recebe página HTML no lugar do JSON, e quem navega com o navegador
até uma rota de `route.go` fora de `/api/` recebe JSON no lugar da página. O cliente já diz
o que quer ler, no `Accept`; o servidor é que não estava escutando direito.

## O que muda

O corpo do erro das rotas de API passa a ser `problem+json`:

```json
{"type":"about:blank","title":"Unprocessable Entity","status":422,
 "instance":"/api/posts","request_id":"01J...","fields":{"title":"obrigatório"}}
```

E o handler pode devolver o problema pronto, com os membros de extensão que quiser:

```go
return &trilha.Problem{
	Type:   "https://exemplo.com/probs/sem-saldo",
	Title:  "Sem saldo",
	Status: http.StatusPaymentRequired,
	Detail: "A conta tem R$ 3,00 e a operação custa R$ 10,00.",
	Extra:  map[string]any{"saldo": 300},
}
```

### Superfície

| Símbolo | Papel |
|---|---|
| `Problem` | `Type`, `Title`, `Status`, `Detail`, `Instance`, `Fields`, `Extra`; é um `error` |
| `c.Accepts(offers ...string) string` | negociação de conteúdo: a melhor oferta segundo o `Accept`, ou `""` |
| `ProblemType func(status int) string` | opcional: a URI do `type` por status, para quem documenta os erros |

## Decisões

- **O corpo antigo sai, não fica ao lado.** `{"error":...}` vira `title`. Manter as duas
  chaves no mesmo JSON seria manter as duas para sempre, que é exatamente o que o RFC existe
  para acabar. É 0.x, a mudança está anunciada no CHANGELOG com o de-para, e `fields`
  continua igual — o formulário que lê `fields` não muda uma linha.
- **`Accept` decide, o tipo da rota é o padrão.** `KindAPI` responde `problem+json` sempre e
  `KindPage` responde HTML sempre — é o que esses dois nomes significam. Só a rota `KindAuto`
  negocia: `route.go` responde `problem+json`, a não ser que o `Accept` prefira `text/html`
  (navegador na barra de endereço); `page.go` responde HTML, e não negocia, porque um
  fragmento trocado na página precisa de HTML mesmo quando o `fetch` diz outra coisa.
- **A negociação é ranqueada, com `q`.** `text/html;q=0.9, application/json` prefere JSON,
  e o `*/*;q=0.8` que todo navegador manda não empata mais com o `text/html` que ele mandou
  antes. `Accept` ausente ou `*/*` não é preferência: vale o padrão da rota.
- **O caminho sobrevive em um lugar só: o 404 de rota que não existe.** Ali não há rota para
  perguntar o tipo, e o `Accept` pode estar mudo (`curl` manda `*/*`). O prefixo `/api/`
  entra como último recurso, depois da negociação, e está documentado como tal.
- **Produção não conta o que deu errado (ASVS V7.4.1).** Em 5xx, `title` é o texto do status
  e `detail` fica vazio; a mensagem do erro vai para o log, com o `request_id`. Em `Dev`, e
  só nele, `detail` traz a mensagem — é a mesma escolha que a página de erro já faz.
- **`request_id` no corpo, não só no cabeçalho.** O `X-Request-ID` existe desde sempre, mas
  cabeçalho de resposta não é legível por script de outra origem sem `Expose-Headers`. No
  corpo, o suporte recebe o identificador de quem abriu o chamado.

## Requisitos

- **FR-001** Erro em rota de API sai com `Content-Type: application/problem+json` e corpo
  com `type` (`about:blank` por padrão), `title`, `status`, `instance` e `request_id`.
- **FR-002** `FieldErrors` continua virando 422 com o mapa em `fields`.
- **FR-003** `HTTPError` com mensagem e status < 500 põe a mensagem em `detail`.
- **FR-004** 5xx em `Prod` não traz `detail` nem o texto do erro; em `Dev`, traz.
- **FR-005** Um `*Problem` devolvido pelo handler define status, `type`, `title`, `detail` e
  os membros de `Extra`, achatados no objeto de cima.
- **FR-006** `c.Accepts` ordena por `q`, entende `tipo/*` e `*/*`, ignora oferta com `q=0` e
  devolve `""` quando nada serve.
- **FR-007** Rota `KindAuto` de `route.go` responde HTML quando o `Accept` prefere
  `text/html`, em qualquer caminho — sem depender de `/api/`; `KindAPI` e `KindPage` não
  negociam.
- **FR-008** No 404 sem rota, o `Accept` decide; empatado ou ausente, o prefixo `/api/`
  decide; fora dele, a página 404.

## Fora de escopo

- **Negociação do corpo de sucesso** (`c.Render` escolhendo entre HTML e JSON). O handler que
  serve dois formatos pergunta com `c.Accepts` e escreve o que quiser; adivinhar por ele é
  onde a resposta certa vira surpresa.
- **`application/problem+xml`.** Ninguém pediu.
- **Catálogo de `type` por status.** Fica o gancho `ProblemType`; a URI é do app, que é quem
  tem a página para documentar cada erro.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — SSR primeiro | a página de erro HTML continua sendo o padrão de quem navega |
| II — só biblioteca padrão | `encoding/json`, `net/http`, `strings`, `strconv` |
| III — coerência com Go | `Problem` é um `error`; `errors.As` continua valendo |
| IV — convenção nova tem teste e uso no exemplo | `examples/blog` devolve um `Problem` com `type` próprio e o teste cobre |
| VI — teste primeiro | `problem_test.go` vermelho antes de `problem.go` |
| VII — compatibilidade | mudança de formato anunciada no CHANGELOG (0.x); `fields` preservado |

## Tarefas

- [x] T001 `problem_test.go` vermelho: corpo e content-type, `fields`, `detail` por ambiente,
      `Problem` do handler com `Extra`, `Accepts` com `q`, negociação por tipo de rota, 404.
- [x] T002 `problem.go` (tipo, `MarshalJSON`, negociação) e ligação em `render.go`/`serve.go`.
- [x] T003 `examples/blog`: erro de API com `type` próprio e teste de integração.
- [x] T004 Documentação nas duas locales: `learn/api` (o formato e a negociação) e
      `reference/errors` (a tabela do `Problem` e do `Accepts`).
- [x] T005 `CHANGELOG.md` (0.21.0) com o de-para, `version`, ROADMAP item 12.
- [x] T006 `make test` verde e `make release VERSION=0.21.0 ISSUES="30"`.

## Aceitação

- **SC-001** `GET /api/posts/nao-existe` responde 404 `application/problem+json` com `title`,
  `status`, `instance` e `request_id`.
- **SC-002** `POST /api/posts` com título vazio responde 422 com `fields.title`, como antes.
- **SC-003** Um `panic` em rota de API responde 500 sem nenhum trecho do erro em `Prod`, e
  com a mensagem em `detail` no `Dev`.
- **SC-004** A mesma rota de `route.go`, fora de `/api/`, responde página HTML para
  `Accept: text/html,*/*;q=0.8` e `problem+json` para `Accept: */*`.
- **SC-005** `c.Accepts("text/html", "application/json")` com
  `Accept: text/html;q=0.2, application/json;q=0.8` devolve `application/json`.
