# Plano — spec 063

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `ui/*.go` | doc comment nos 38 símbolos sem, exemplo nos 7 que a régua pede |
| `internal/uidoc/uidoc.go` (novo) | `Component`, `Components()`, `Lookup(name)`, embed do catálogo |
| `internal/uidoc/extract.go` (novo) | leitura do pacote `ui` com `go/doc` (só em teste/geração) |
| `internal/uidoc/catalog.json` (novo, gerado) | o catálogo commitado |
| `internal/uidoc/uidoc_test.go` (novo) | `-update`, doc obrigatório, exemplo obrigatório |
| `cmd/trilha/ui.go` | subcomando `describe`, texto e `--json` |
| `cmd/trilha/i18n.go` | as frases novas |
| `cmd/trilha/e2e_test.go` | `describe` no fluxo de ponta a ponta |
| `examples/cookbook/next.go` (novo) | os trechos reais da página nova |
| `site/.../cookbook/from-next.md`, `.../receitas/do-next.md` (novos) | o guia |
| `internal/scaffold/agents/AGENTS.{en,pt}.md` | a linha do `describe` |
| docs de CLI (`cli.md` nas duas línguas), `CHANGELOG.md`, `api/current.txt` | |

## Ordem

Os doc comments primeiro, porque o extrator só é útil depois que a fonte está completa; o
`internal/uidoc` em seguida, com o `-update`; a CLI depois, que só formata; o guia por último,
porque cita o comando.

## O que não entra

- Descrever `h`, `trilha` ou `ai`: o pedido é o `ui`, e a régua de exemplo não serve para um
  pacote de runtime.
- Tradução do catálogo: doc comment é em inglês por constituição, e traduzir na hora seria um
  segundo lugar para envelhecer.
- Conversão automática de código React: é o `migrate next` da #59, não este.
