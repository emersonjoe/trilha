# Plano — spec 057

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `list.go` (novo) | `ListParams`, `Href`, `Offset`, `Limit`, `Asc`, `TotalPages`, `Restrict` |
| `bind.go` | caso especial: depois de achatar uma `ListParams`, chamar `after(form)` |
| `live.go` (novo) | `Ctx.PollEvery`, `Ctx.PollStop`, o header `Trilha-Poll` |
| `stream.go` | `Stream.Notify(name)` |
| `ui/list.go` (novo) | `Column[T]`, `Columns[T]`, `ListState`, `ListSelect`, `DataTable` |
| `ui/live.go` (novo) | `Poll`, `Live`, `On`, `LiveScript` |
| `ui/assets/ui.live.js` (novo) | polling com pausa/recuo e o `EventSource` multiplexado |
| `ui/ui.go` | `//go:embed` e `Files` com o `ui.live.js` |
| `ui/assets/ui.css` | `.ui-table th a`, `[aria-sort]`, a barra de seleção |
| `cmd/trilha/audit.go` | aviso de `ui.Live` sem `Auth` |
| `examples/blog` | rota de listagem com `DataTable`; `Poll` no status do anexo |
| docs | referência `listings` / `listagens` e `live` / `vivo`, receita nas duas línguas |

## Ordem

`ListParams` primeiro, porque `DataTable` depende dela; `ui.DataTable` depois, com os
testes de árvore; `Poll`/`Live`/`On` em seguida, que não dependem de nenhum dos dois; o
exemplo e a auditoria por último, quando as duas metades já existem.

## O que não entra

- Bus de eventos, fila ou registro de assinantes: é do app.
- Ordenação por mais de uma coluna: a URL fica ilegível e nenhuma das telas de onde a
  issue veio pede.
- Virtualização, coluna redimensionável, exportação: é tabela de dados, não planilha.
