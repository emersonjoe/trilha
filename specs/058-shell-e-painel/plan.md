# Plano — spec 058

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `ui/chart.go` (novo) | `Datum`, `Stat`, `StatHint`, `Bars`, `Sparkline`, `SparkOpts`, `Donut` |
| `ui/shell.go` (novo) | `NavItem`, `NavGroup`, `UserMenu`, `ShellOpts`, `Shell`, `PageHeader`, `Back` |
| `ui/ui.go` | o script inline do `Head` também aplica a sidebar recolhida |
| `ui/assets/ui.js` | o clique do recolher e o drawer do mobile (poucas linhas, arquivo já baixado) |
| `ui/assets/ui.css` | `.ui-shell`, `.ui-page-header`, `.ui-stat`, `.ui-chart`, a legenda da rosca |
| `ui/testdata/*.svg` (novo) | golden de cada gráfico |
| `internal/scaffold/templates/` | vira `base/`, `blog/` e `app/` |
| `internal/scaffold/scaffold.go` | `Data.Template`, a caminhada em duas pastas, erro de nome |
| `internal/scaffold/texts.go` | os textos do template `app` nas duas línguas |
| `cmd/trilha/new.go`, `i18n.go` | a flag `--template` e a mensagem |
| `cmd/trilha/e2e_test.go` | o projeto `app` compila, `check` verde, `go test` verde |
| `examples/orcamento` | orçado × realizado em `ui.Bars` |
| docs | referência `shell` / `charts` nas duas línguas e a trilha "um app de gestão" |

## Ordem

Os gráficos primeiro: eles não dependem de nada e o `Stat` é peça do template. O
`Shell` depois, que usa `Stat` na tela de exemplo da referência. A reorganização dos
templates por último, quando as duas metades do `ui` já existem — assim o template
`app` é escrito uma vez, contra a API final.

## O que não entra

- Eixo, escala, legenda dentro do SVG, zoom, brush, tooltip próprio: quem precisa disso
  usa ilha, como o ROADMAP já diz.
- Ícone novo no `icons_data.go` além do que o shell usa.
- A régua do #45 sobre o template novo (decisão 11).
