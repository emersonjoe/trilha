# Plano — spec 066

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `island.go` | `data-trilha-csrf`, o loader com o objeto `island` (post/get/swap/csrf/signal) |
| `island_test.go` | atributo, marca do loader, e o contrato do que o loader manda |
| `internal/islands/islands.go` (novo) | leitura de `c.Island` no AST e o modelo das props |
| `internal/islands/dts.go` (novo) | emissão do `islands.d.ts` |
| `internal/islands/islands_test.go` (novo) | golden do `.d.ts` sobre a árvore sintética |
| `internal/gen/gen.go`, `cmd/trilha/*` | gravar o `.d.ts` no `gen` e conferir no `--check` |
| `cmd/trilha/vendor.go` (novo) | `trilha vendor <pkg@versao>`, `--check`, `--from` |
| `cmd/trilha/audit.go` | achado de `public/vendor/` fora do lock |
| `examples/blog` | `ilha-editor.js` com `island.post`; `islands.d.ts` commitado |
| docs | referência de ilhas e do CLI, receita "um componente React como ilha" |

## Ordem

O canal primeiro, porque é o que a receita usa; o `.d.ts` depois, que descreve o canal junto
com as props; o `vendor` em seguida, independente dos dois; exemplo, auditoria e docs no fim.

## O que não entra

- Resolvedor de dependências, `node_modules`, semver: decisão 8.
- Hidratação global, ilha aninhada em ilha, streaming de props.
- Reescrita do designer de fluxos: a issue pede a receita, não o port.
