# Plano — spec 059

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `bind.go` | `bindSlice`, `bindMap`, o nome indexado e o teto `MaxItems` |
| `bindjson.go` | chave de erro no formato `items[1].qty` |
| `validate.go` | `minitems`/`maxitems` em fatia e mapa |
| `schema.go` (novo) | `SchemaField`, `Schema`, `BindSchema`, a tradução para as regras |
| `ui/schema.go` (novo) | `SchemaForm` e o campo por tipo |
| `internal/openapi` | `[]Item` e mapa no corpo do formulário |
| `examples/cadastro` | lista de dependentes; tela com esquema vindo de JSON |
| `fuzz_test.go` | índices e chaves torcidos |
| docs | referência `validation`/`validacao` e `ui`, capítulo de formulários |

## Ordem

O `Bind` primeiro, porque o `BindSchema` usa o mesmo motor; o fuzz junto com ele, que é
onde o índice torto aparece; `SchemaForm` depois; o exemplo e o OpenAPI por último.

## O que não entra

- `ui.FieldList`: decisão 11 da spec.
- Lista dentro de lista (`itens[0].tags[1]`): nenhuma das telas da issue pede, e o nome
  fica ilegível antes de ficar útil.
- Esquema com condição (`mostrar se`): o `ui.ShowWhen` já existe e é do app.
