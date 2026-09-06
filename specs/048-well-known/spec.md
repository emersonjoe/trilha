# Spec 048 — `/.well-known/`: a pasta com ponto que precisa virar rota

- **Issue**: [#75](https://github.com/emersonjoe/trilha/issues/75) — a issue é a fonte do
  escopo; aqui só o contrato e a ordem de execução.
- **Branch**: `048-well-known`
- **Versão**: 0.38.0

## Por quê

Hoje `app/.well-known/oauth-authorization-server/route.go` **desaparece**. O scanner pula
todo diretório que começa com ponto (`internal/scan/scan.go:307`) e não diz nada: `trilha
gen` termina com sucesso, `trilha routes` não lista a rota, o app sobe e responde 404 numa
URL que uma RFC exige. Quem escreveu o handler procura o erro no handler.

São dois defeitos no mesmo lugar:

1. **O silêncio.** Qualquer pasta pulada pode estar escondendo um `page.go` ou um
   `route.go`, e o único sintoma é um 404 em produção. O scanner sabe a resposta na hora do
   `gen` e não fala.
2. **A pasta legítima.** `/.well-known/` não é um detalhe de implementação de quem escreve o
   app: é onde sete especificações mandam publicar documento (RFC 8414, RFC 9728, OIDC
   Discovery, RFC 8555, RFC 9116, apple-app-site-association, W3C change-password). Sem
   exceção nomeada, o jeito de servir essas URLs no Trilha é não usar `app/`.

É o irmão da #14: ponto **no meio** do nome já vira caminho fixo (`app.css` → `/app.css`);
ponto **no começo** some.

## O que muda

### `.well-known` deixa de ser pulada

```text
app/.well-known/security.txt/route.go   →  GET /.well-known/security.txt
app/.well-known/oauth-authorization-server/route.go
                                        →  GET /.well-known/oauth-authorization-server
```

É a **única** exceção: `.git/`, `.idea/`, `.trilha/` continuam invisíveis. Dentro dela valem
as mesmas convenções de sempre (`route.go`, `page.go`, `middleware.go`, subpasta = segmento).
Como `.well-known` não é identificador Go, o pacote declara outro nome
(`package wellknown`) — a mesma regra do `app.css` da #14, e o gerador já importa tudo com
alias.

### Uma pasta pulada que esconde rota vira erro

```text
error: app/.oauth/route.go: route.go is inside ".oauth", a directory the scanner skips: a
       name that starts with a dot is not a route (.well-known is the only exception)
  → rename the directory without the leading dot, or start it with "_" if you meant to park it
```

Código novo `E_HIDDEN_ROUTE`, com conserto na tabela de `fixes` (a mesma da 0.37.0), emitido
por `trilha gen`, `trilha dev` e `trilha check`.

O erro é **só para o ponto**. `_x/` e `testdata/` continuam em silêncio porque são a forma
documentada de guardar uma pasta fora do roteamento — e são, agora, o conserto que a mensagem
oferece.

### As outras duas varreduras aprendem a exceção

`internal/openapi` (índice de tipos) e `internal/dev` (watcher) pulam ponto pelo mesmo
motivo. Sem a exceção nos três, o tipo de resposta de uma rota `.well-known` não entra no
OpenAPI e editá-la não recarrega o dev server. A regra passa a morar num lugar só:
`scan.WellKnown`.

## Fora de escopo

- **Escape geral de segmento literal** (`ponto-well-known/` ou `var Pattern`) — a issue mede
  o custo e não recomenda agora; a exceção nomeada resolve o caso real sem abrir a porta para
  `.git/` virar rota.
- **Erro para `_x/` e `testdata/`** — ver acima: são parking documentado.
- **`go vet ./...` e `go test ./...` não enxergam o pacote.** É regra da ferramenta Go
  (padrão `...` não casa caminho com ponto), não do Trilha: o pacote **compila** normalmente
  porque `trilha_gen.go` o importa pelo caminho explícito. Documentado, não contornado.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `path/filepath`, `io/fs`, `strings`; nada novo |
| III — convenção nova precisa de teste no scanner + rota no `examples/blog` + teste de integração | T001/T002 (scanner), T005 (`app/.well-known/security.txt`), T006 (e2e) |
| VI — teste primeiro | cada bloco começa pelo teste que falha |
| VII — segurança por padrão | a exceção é uma lista de um nome; nenhuma outra pasta oculta passa a ser servida |
| IX — mensagem de erro com conserto | `E_HIDDEN_ROUTE` nasce com entrada em `fixes` (`TestEveryCodeHasFix`) |

## Aceitação

- **SC-001** `app/.well-known/security.txt/route.go` produz a rota `/.well-known/security.txt`
  no `Result` do scanner e no `trilha_gen.go`, com alias válido.
- **SC-002** `app/.git/`, `app/.idea/` e `app/.trilha/` continuam fora do resultado.
- **SC-003** Um `route.go` ou `page.go` dentro de pasta pulada por ponto produz
  `E_HIDDEN_ROUTE` com o caminho do arquivo e conserto não vazio; `_x/` e `testdata/` não
  produzem erro.
- **SC-004** `examples/blog` serve `GET /.well-known/security.txt` com `200` e
  `text/plain`, e o `trilha_gen.go` commitado do exemplo contém a rota.
- **SC-005** Editar um arquivo sob `app/.well-known/` marca o snapshot do watcher como sujo.
- **SC-006** O tipo devolvido por uma rota `.well-known` aparece no documento do `openapi`.
- **SC-007** Documentação nas duas locales diz que `.well-known` é a exceção e que o resto
  do ponto continua ignorado — e agora reclamando quando esconde rota.
