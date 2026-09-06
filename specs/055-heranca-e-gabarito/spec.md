# Spec 055 — o que a árvore sabe e o `Ctx` não conta

- **Issues**: [#42](https://github.com/emersonjoe/trilha/issues/42),
  [#43](https://github.com/emersonjoe/trilha/issues/43)
- **Branch**: `055-heranca-e-gabarito`
- **Versão**: 0.39.0 (com as specs 051 a 054)

## Por quê

As duas issues saíram da mesma adoção (Partiu, terceira leva) e do mesmo lugar: a árvore de
pastas decide coisas que depois ninguém consegue perguntar de volta.

1. **Não dá para saber qual rota casou.** O `Ctx` dá o caminho concreto
   (`/v/cmtk…/orcamento`) e um parâmetro por vez, mas não o gabarito
   `/v/{viagemId}/orcamento`. Isso morde no log de acesso, que num app com id na URL grava um
   valor novo por viagem — sem agrupamento e com a cardinalidade clássica de um rótulo por
   registro, não por tela —, e morde numa ponte para código que já existe, onde cada
   `route.go` repete a própria rota como texto porque o handler não tem como perguntar. No
   Partiu foram 66 arquivos com a string colada e um teste com `go/ast` escrito só para
   impedir que ela divergisse da pasta em que está. O dado já existe: o `wrap` guarda o
   `*Route` em `c.route`.
2. **`Kind` é a única coisa da árvore que não se herda.** `layout.go` e `middleware.go` valem
   para a subárvore inteira — é isso que faz a convenção de pastas valer a pena. `Kind`, não:
   ele é declarado por arquivo de rota, então "este ramo do site é página, não API" precisa
   ser repetido em cada folha. No Partiu foram 40 arquivos com o mesmo bloco e uma exceção. E
   `Kind` não é só como o erro é renderizado: **é o que liga o CSRF**. Uma rota de escrita
   nova que nasce sem a linha nasce sem CSRF, em silêncio — a issue traz o repro de duas
   linhas em que o mesmo `POST` sem token dá 403 no `page.go` e 200 no `route.go`.

## O que muda

### `c.Pattern()`

```go
func GET(c *trilha.Ctx) error { return ponte.Rodar(c) } // era ponte.Rodar(c, "GET /v/{viagemId}/orcamento")
```

- `Pattern() string` devolve o gabarito da rota que casou, `""` para o que o fallback
  respondeu (arquivo estático, 404, redirecionamento de barra).
- O log de acesso passa a gravar **os dois**: `path` (o caminho concreto, para quem investiga
  um caso) e `route` (o gabarito, para quem agrega). O `LogRequest` já recebe o `*Ctx`, então
  ganha o gabarito sem mudar de assinatura.

### `Kind` herdado pela subárvore

```go
// app/painel/kind.go
package painel

var Kind = trilha.KindPage // vale para esta pasta e para tudo abaixo dela
```

- `var Kind` declarado no pacote de uma pasta vale para os `route.go` daquela pasta e de
  todas as descendentes; a declaração mais funda ganha. É a mesma regra do `layout.go` e do
  `middleware.go`.
- `kind.go` é o nome do arquivo para a pasta que não tem `route.go` nenhum — a raiz de uma
  subárvore precisa poder falar sem ter rota própria. Não é um arquivo especial para o
  varredor: qualquer arquivo do pacote serve, `kind.go` é onde quem lê vai procurar.
- Uma rota de `page.go` continua sendo página: um `Kind` herdado não transforma página em
  API. A herança existe para os `route.go`, que são os que nascem API por padrão.
- O `trilha audit` ganha o aviso que faltava: `route.go` com método de corpo, sem `Kind`
  efetivo e sem `CSRFForAPI`, num app que serve páginas, é quase sempre uma escrita aberta a
  formulário de outro site.

## Superfície

| Onde | O quê |
| --- | --- |
| `ctx.go` | `Pattern()` |
| `serve.go` | `route` no registro de acesso |
| `internal/scan` | `Kind` herdado pela subárvore (`KindRef` no lugar de `HasKind`) |
| `internal/gen` | `Kind:` vindo do pacote que declarou |
| `cmd/trilha/audit.go` | aviso da escrita sem CSRF |
| `examples/blog` | `app/legado-/kind.go` e uma ação de escrita em `route.go` sob ele |

## Fora de escopo

- Redefinir o que `Config.CSRFForAPI` quer dizer. Ele continua dizendo o que diz — "cobre
  CSRF também na API" —; a herança é a forma melhor de dizer a outra coisa ("aqui não tem
  API"), e as duas conviverem por uma versão é mais barato do que renomear uma opção de
  segurança.
- Erro do varredor para um `var Kind` que nenhuma rota lê. Ele não faz mal, e a pasta que o
  declara hoje pode ganhar um `route.go` amanhã.
- `Pattern()` para o fallback. Estático e 404 dividem um rótulo só nas métricas justamente
  porque o caminho concreto é entrada do usuário; o mesmo vale aqui.

## Constitution Check

- **Convenção nova em `app/`**: `Kind` herdado — teste no varredor, rota no `examples/blog` e
  teste de integração, os três nesta spec.
- **Zero dependências.**
- **Inglês no código e no público, pt-BR na spec**; docs nas duas línguas no mesmo commit.
- Gerador determinístico, arquivo gerado commitado (`examples/blog/trilha_gen.go`).

## Critérios

- **SC-001** `c.Pattern()` devolve `/blog/{slug}` numa requisição a `/blog/ola-mundo`.
- **SC-002** `c.Pattern()` devolve `""` no que o fallback respondeu, sem pânico.
- **SC-003** O registro de acesso traz `path` e `route`, e o `route` é o gabarito.
- **SC-004** `var Kind` numa pasta vale para um `route.go` descendente que não o declara.
- **SC-005** A declaração mais funda ganha da mais rasa.
- **SC-006** Um `route.go` que declara o próprio `Kind` continua ganhando.
- **SC-007** Um `Kind` herdado não transforma uma rota de `page.go` em API.
- **SC-008** O arquivo gerado importa o pacote que declarou o `Kind`, e só quando alguém lê.
- **SC-009** No `examples/blog`, um `POST` em `route.go` sob o `kind.go` responde 403 sem
  token e 303 com ele.
- **SC-010** `trilha audit` avisa da escrita em `route.go` sem CSRF e cala quando há `Kind`.
- **SC-011** `make test` verde.
