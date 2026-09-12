# Spec 132 — ui: List em pt-BR e Alert na ordem certa

> Mudança pequena: um pacote (`ui`), sem convenção nova em `app/`, sem mudança incompatível
> na API pública.

- **Issues**: #174 — o `H4` do título entra depois dos filhos em `Alert`, então a descrição
  aparece antes do título no DOM (e sem ícone, o título cai na coluna do ícone); #175 —
  `ui.List` (o `DataTable`) escreve "Filter" e "N results" mesmo com `Config.Locale =
  "pt-BR"`. A issue é a fonte do escopo; este documento não a repete.
- **Branch**: `feat/mig-s05-ui-locale-alert`
- **Versão**: 0.111.0

## Por quê

`ui.Date`, `ui.Number`, `ui.Bytes` e boa parte dos componentes (`csv.go`, `settings.go`,
`apikeys.go`, `search.go`...) já leem `langOf(c)` e escrevem `pt-BR` quando a app pede. O
`DataTable` — a tela mais repetida de qualquer app de gestão — ficou de fora: o botão de
filtro e o rodapé de contagem são strings fixas em inglês, escritas antes de o pacote ter
esse costume. Numa aplicação inteira em português (como `examples/blog`, que já roda com
`Locale: "pt-BR"`), toda listagem termina em "23 results" e um botão "Filter".

Separadamente, `ui.Alert` monta o título (`H4`) depois dos filhos que o chamador passou, e o
CSS do grid não fixa `grid-column`/`grid-row`. Sem ícone, a descrição ocupa a única célula da
linha 1 e o título cai abaixo, à esquerda — ordem de leitura errada tanto no visual quanto no
DOM (o que o leitor de tela lê primeiro).

## O que muda

- `ui.DataTable` passa a escrever o botão de filtro e a contagem no idioma de
  `c.Locale()`, do mesmo jeito que `ui.Date`/`ui.Number` (`word(pt, en, pt-BR)`, o par que já
  usa `csv.go`, `settings.go`, `apikeys.go` — nenhum mecanismo novo):
  - `Filter` → `Filtrar`
  - `1 result` / `N results` → `1 resultado` / `N resultados`
- `ui.Alert(title, children...)` monta o `H4` do título logo após a classe base, antes de
  qualquer filho — a descrição nunca mais precede o título no DOM, com ou sem ícone.
- `ui/assets/ui.css` fixa `grid-column`/`grid-row` em `.ui-alert-title`,
  `.ui-alert-description` e `.ui-alert .ui-icon`, para a grade não depender da ordem dos
  nós: sem ícone, a coluna 1 fica vazia e some.
- Cópia do kit atualizada nos projetos que a versionam (`examples/assistente`,
  `examples/blog`, `examples/cadastro`, `examples/orcamento`, `site`), via `trilha ui
  --force --css-only`.

```go
ui.DataTable(c, cols, rows, ui.ListState{Params: q.ListParams, Total: total, Search: "Buscar"})
// com c.Locale() == "pt-BR": botão "Filtrar", rodapé "23 resultados"

ui.Alert("Título", ui.AlertDescription(h.Text("Descrição.")))
// <h4 class="ui-alert-title">Título</h4> vem antes de <div class="ui-alert-description">
```

## Fora de escopo

- `ui/empty.go` (`emptyFor`, `EmptyOpts.Title` como "No results for…") e o resto de
  `ui/search.go` que não seja o `DataTable`: a #175 os cita como achado da mesma varredura,
  mas o pedido desta sessão é só o filtro e a contagem do `List`; `emptyFor` exigiria passar
  `*trilha.Ctx` a uma função que hoje só recebe `ListState`, mudança maior que cabe numa
  sessão própria.
- Um mapa de idioma geral (`strings(lang)`) ou `ListOpts.CountFunc`/`FilterLabel`
  configuráveis pela app: a proposta alternativa da própria issue. O molde já provado no
  pacote (`word(pt bool, en, br string) string`) resolve os dois textos sem mecanismo novo;
  se mais idiomas ou mais textos aparecerem, a spec que os trouxer decide o mapa.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Só `strconv`/`strings`, já em uso; nada novo. |
| IV — API pública pequena e estável | `Alert` e `DataTable` mantêm assinatura; `count`/`filterForm` são não exportados. |
| VI — teste primeiro | `ui/list_test.go` (Locale pt-BR e en) e `ui/ui_test.go` (ordem do Alert) antes da implementação; `examples/blog` (`TestListagemComDataTable`) já exercitava o `DataTable` com `Locale: "pt-BR"` e passa a checar "Filtrar"/"23 resultados". |
| VII — segurança por padrão | Sem novo texto vindo de entrada do usuário; `h.Text` continua escapando o que já escapava. |

## Tarefas

- [x] T001 Teste que falha: `ui/list_test.go` (`TestDataTableFilterAndCountFollowLocale`) —
      pt-BR usa "Filtrar"/"1 resultado"/"N resultados"; en continua "Filter"/"N results".
- [x] T002 Teste que falha: `ui/ui_test.go` (`TestAlertTitleComesBeforeDescription`) — título
      antes da descrição no HTML, com e sem ícone; sem ícone, sem célula de ícone.
- [x] T003 Implementação: `ui/list.go` (`DataTable`, `count`, `filterForm`), `ui/ui.go`
      (`Alert`), `ui/assets/ui.css`.
- [x] T004 Uso em `examples/blog` (`app/documentos`, já com `Locale: "pt-BR"` e `ui.Alert` na
      fila) — `TestListagemComDataTable` atualizado para "Filtrar"/"23 resultados".
- [x] T005 Cópia do kit sincronizada (`trilha ui --force --css-only`) em
      `examples/assistente`, `examples/blog`, `examples/cadastro`, `examples/orcamento`,
      `site`.
- [x] T006 `CHANGELOG.md`, `version` em `cmd/trilha/main.go` (0.111.0), item do
      `ROADMAP.md`.
- [x] T007 `make test` verde.

## Aceitação

- **SC-001**: `ui.DataTable` com `Locale: "pt-BR"` renderiza "Filtrar" e "N resultados" (e "1
  resultado" no singular); com `en` (padrão), continua "Filter"/"N results".
- **SC-002**: `ui.Alert` sempre renderiza `<h4 class="ui-alert-title">` antes de
  `class="ui-alert-description"` no HTML, com ou sem ícone; sem `Icon`, nenhum nó com
  `ui-icon` aparece.
- **SC-003**: `make test` verde, incluindo `examples/blog` e `internal/uidoc` (catálogo sem
  mudança de assinatura/doc).
