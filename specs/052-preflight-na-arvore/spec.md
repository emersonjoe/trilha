# Spec 052 — o preflight na árvore: `OPTIONS`, `var CORS` por rota e o segredo que ninguém assina

- **Issues**: [#76](https://github.com/emersonjoe/trilha/issues/76) e
  [#78](https://github.com/emersonjoe/trilha/issues/78) (a mesma lacuna, relatada duas vezes),
  [#77](https://github.com/emersonjoe/trilha/issues/77)
- **Branch**: `052-preflight-na-arvore`
- **Versão**: 0.39.0 (junto com a spec 051)

## Por quê

As três issues nasceram do mesmo dia: o Partiu subiu para a v0.38.0 para usar o
`/.well-known/` na árvore, que a #75 tinha acabado de destravar, e teve de voltar atrás
duas vezes.

1. **O preflight não tem porta.** `internal/scan/scan.go` reconhece cinco métodos. Um
   `func OPTIONS` num `route.go` é descartado em silêncio: o `trilha gen` passa, o
   `trilha routes` não mostra o método, e em produção o `fallback` do `serve.go` devolve 405
   com `Allow` antes de rodar middleware ou handler — não há saída pelo lado do app. O
   `a.Register` roteia OPTIONS sem problema hoje; quem está curto é só o varredor. Os
   documentos de descoberta da RFC 8414 e da RFC 9728 são buscados de outra origem por um
   cliente que manda `MCP-Protocol-Version` — cabeçalho não-simples, logo preflight. Sem
   OPTIONS, o documento existe e mesmo assim não é alcançável. O `Config.CORS` não resolve:
   é política do app inteiro, e abrir as outras ~85 rotas para consertar três seria trocar
   uma lacuna por uma superfície.

2. **O portão reprova quem não usa o que ele protege.** O `trilha check` da 0.37.0 falha no
   `audit` com `TRILHA_SECRET not set` num app que nunca chama `SetSigned`. Como o `check`
   para no primeiro erro, o `openapi` — o passo que de fato pega regressão — nunca roda.
   Definir um segredo que nada assina é pior do que não ter: entra no `.env`, entra na
   rotação, e um dia alguém o gira e não acontece nada.

## O que muda

### `func OPTIONS` é um handler como os outros

`scan.Methods` passa a ter seis métodos. Vale em `route.go` e em `page.go`, como qualquer
método de corpo, e vale para `MiddlewareOPTIONS` — a lista do varredor é uma só, e o resto
do framework já lê dela.

`HEAD` fica de fora de propósito: o `net/http.ServeMux` responde HEAD com o handler do GET
desde o Go 1.22, e registrar um segundo seria duas verdades para a mesma requisição.

### O descarte calado vira erro (`E_UNROUTABLE_METHOD`)

`func HEAD`, `func TRACE`, `func CONNECT` num `route.go` continuam não sendo rota — mas
agora param a geração com linha e conserto, em vez de sumirem. É a mesma classe de defeito
que o `E_HIDDEN_ROUTE` da 0.38.0 fechou, e o pedido explícito da #78.

### `var CORS = trilha.CORS{...}` no `route.go`

A política de origem cruzada de **uma** rota, escrita onde a rota mora:

```go
package security

var CORS = trilha.CORS{Origins: []string{"*"}, Methods: []string{"GET"}}

func GET(c *trilha.Ctx) error { ... }
```

- O varredor enxerga o `var` exportado (como já faz com `Kind`), o gerador escreve
  `CORS: &pkg.CORS` no `trilha.Route`, e o `App` monta a política uma vez, no `Register`.
- O preflight é respondido pela rota: 204 com `Access-Control-Allow-*` quando a origem e o
  método estão na lista, 403 quando não estão — a mesma decisão do `Config.CORS`, com o
  mesmo código.
- Toda resposta daquela rota carrega o `Access-Control-Allow-Origin` e o `Vary: Origin`.
- Se o arquivo **também** exporta `func OPTIONS`, o handler escrito à mão ganha: o caso
  comum é declarativo, o caso esquisito continua possível. É o "(b) com (a) por baixo" da
  #78.
- `var CORS` num `page.go` é erro (`E_CORS_ON_PAGE`): página é navegação do próprio site, e
  um `var` ignorado em silêncio é exatamente o que esta spec veio consertar.
- O `Config.CORS` do app inteiro continua onde estava e roda antes do roteador — **exceto**
  na rota que declara a própria política: aí ela decide sozinha. Sem essa exceção, um app que
  já tem `Config.CORS` com lista exata (como o `examples/blog`) nunca conseguiria abrir três
  caminhos: o preflight morreria no 403 do app antes de chegar à rota, que é exatamente o
  caso das issues.

### O `audit` pergunta se o app assina alguma coisa

`TRILHA_SECRET` ausente é **crítico** quando o código-fonte chama `SetSigned`, `Signed`,
`NewSigner`, declara `Secret:` na `Config` ou importa `trilha/auth`; caso contrário é
**aviso**, com o texto dizendo o que o ligaria. Segredo curto demais segue crítico em
qualquer caso: quem definiu quis usar.

## Superfície

| Onde | O quê |
| --- | --- |
| `internal/scan` | `OPTIONS` em `Methods`; `Route.HasCORS`; `E_UNROUTABLE_METHOD`; `E_CORS_ON_PAGE` |
| `internal/gen` | `CORS: &pkg.CORS` no literal da rota |
| `trilha` (runtime) | `Route.CORS *CORS`; política por rota no `Register` |
| `internal/openapi` | OPTIONS não vira operação (é mecânica de CORS, não API) |
| `cmd/trilha/audit.go` | segredo ausente é aviso quando nada assina |
| `examples/blog` | `/.well-known/security.txt` com `var CORS` e `func OPTIONS` |

## Fora de escopo

- `HEAD` no varredor (o `ServeMux` já serve HEAD pelo GET).
- Reescrever o `Config.CORS` do app: a política por rota é somada, não substituta.
- `--allow` / `TRILHA_AUDIT_IGNORE` na #77: a alternativa barata da issue deixa de ser
  necessária quando a checagem sabe olhar o código.
- Fazer o `trilha check` seguir depois de um passo falho — o `audit` deixa de falhar aqui,
  e a ordem dos passos é assunto de outra issue.

## Constitution Check

- **Convenção nova em `app/`** (`func OPTIONS`, `var CORS`): teste no varredor, rota no
  `examples/blog` e teste de integração — os três nesta spec.
- **Zero dependências**: nada entra.
- **Gerador determinístico**: o `CORS:` sai do mesmo lugar que o `Kind:`, e o
  `trilha_gen.go` do exemplo é commitado.
- **Inglês no código e no público, pt-BR na spec**: docs nas duas línguas no mesmo commit.

## Critérios

- **SC-001** `func OPTIONS` num `route.go` aparece em `trilha routes`, no `trilha_gen.go` e
  responde a requisição (nada de 405).
- **SC-002** `MiddlewareOPTIONS` guarda esse método e só ele.
- **SC-003** `func HEAD` num `route.go` para a geração com `E_UNROUTABLE_METHOD`, com linha.
- **SC-004** `var CORS` num `route.go` responde ao preflight com 204 e os cabeçalhos certos;
  origem de fora da lista recebe 403; requisição simples recebe `Allow-Origin` e `Vary`.
- **SC-005** `func OPTIONS` escrito à mão prevalece sobre o `var CORS`.
- **SC-006** `var CORS` num `page.go` é `E_CORS_ON_PAGE`.
- **SC-007** Uma rota com `var CORS` não muda em nada as outras rotas do app, e a política
  dela vale mesmo num app que já tem `Config.CORS` com lista exata.
- **SC-008** `trilha audit` num app que não assina nada: segredo ausente é aviso e o comando
  termina em zero; o mesmo app com `SetSigned` no código: crítico, como antes.
- **SC-009** `trilha openapi` não documenta OPTIONS.
- **SC-010** `make test` verde; `trilha_gen.go` do `examples/blog` regenerado e commitado.
