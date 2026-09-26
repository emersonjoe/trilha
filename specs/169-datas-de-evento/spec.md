# Spec 169 — Datas de evento

**Feature Branch**: `169-datas-de-evento` | **Created**: 2026-09-26 | **Status**: Entregue (0.150.0)

- **Issue**: #289 — a issue é a fonte do escopo (continuação da #288, spec 168).
- **Versão**: 0.150.0

## Por quê

A #288 deu um `DateFormat` por tabela. Na tela de referência do Prosa, cada tabela escreve
duas perguntas diferentes: "quando foi criada?" responde com o dia, "quando falhou?" precisa
da hora. Com um formatador só, a app escolhia qual coluna ficava fora do formato.

## O que muda

`APIKeysOpts.LogFormat` e `WebhooksOpts.LogFormat`, com a assinatura do `DateFormat`,
desenham as colunas de evento: o último uso da chave e o "quando" das entregas. Criação continua
no `DateFormat`. Nil cai no `DateFormat`, então uma tela da 0.149.0 desenha igual.

```go
ui.WebhooksPanel(c, subs, entregas, ui.WebhooksOpts{DateFormat: diaCurto, LogFormat: diaEHora})
```

## Fora de escopo

- Uma assinatura com o tipo da coluna (`func(*trilha.Ctx, time.Time, ui.DateColumn)`): mudaria a
  da 0.149.0 uma versão depois; a issue descarta pelo mesmo motivo.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo no `go.mod` |
| IV — API pública | dois campos novos em structs de opção; aditivo, `api/current.txt` atualizado |
| VI — teste primeiro | os testes do kit não compilavam e o do `blog` falhava antes do código |

## Segurança e privacidade (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras**: nenhuma nova; o formatador é código da própria aplicação no render e recebe o
  mesmo `Ctx` e instante que o `DateFormat`.
- **Impacto**: a saída é `h.Node`, com o escape do `h` (ASVS V1.2).
- **Exceções**: nenhuma.

## Tarefas

- [x] **T01** — testes: `TestAPIKeysTableLogFormatDesenhaOUltimoUso`,
  `TestPainelLogFormatDesenhaAsEntregas`.
- [x] **T02** — os campos e `orFormat`.
- [x] **T03** — `examples/blog/app/webhooks` com `LogFormat: diaEHora`;
  `TestCadastrarPelaTelaEReceberOEvento` exige a entrega com a hora e a assinatura com o dia.
- [x] **T04** — docs bilíngues (`webhook`, `auth`), `api/current.txt`, catálogo do `uidoc`,
  CHANGELOG, ROADMAP, versão.

## Aceitação

- **SC-001** — com os dois campos, criação sai pelo `DateFormat` e evento pelo `LogFormat`; sem
  `LogFormat`, as duas saem pelo `DateFormat` (os testes da #288 continuam passando).
- **SC-002** — `make test` verde.
