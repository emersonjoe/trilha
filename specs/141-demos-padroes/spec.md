# Spec 141 — site: demos do kit — as telas dos padrões

> Mudança pequena: um pacote (`site/internal/demos`) mais o conteúdo do site, sem convenção
> nova em `app/`, sem mudança na API pública.

- **Issue**: [#191](https://github.com/emersonjoe/trilha/issues/191) — as telas dos padrões são
  a terceira das três frentes de demos do kit; a issue é a fonte do escopo, este documento não a
  repete.
- **Branch**: `feat/demos-padroes`
- **Versão**: 0.120.0

## Por quê

`site/internal/demos/kit.go` cobre layout e navegação (spec 139) e dados e gráficos (spec 140),
mas nada das telas que as receitas `trilha add` escrevem no projeto de alguém: `AuditTable`,
`APIKeysTable`, `PolicyGrid`, `SettingsForm`, `Inbox`, `TaskTable`, `WebhooksPanel`,
`ConnectionsPanel` só aparecem no catálogo de `reference/ui.md` e na prosa do capítulo do
próprio pacote — sem uma tela renderizada ao lado do código. Quem decide usar o Trilha não vê a
fila de aprovação nem o painel de chaves de API antes de rodar `trilha add`.

## O que muda

Doze demos novas em `site/internal/demos/kit.go` (mesmo molde das demos existentes: uma função
Go registrada no `init`, código e resultado lado a lado), cobrindo os grupos da issue:

- **`ui-auditoria`** — `ui.AuditTable`, embutida no próprio capítulo de observabilidade.
- **`ui-chaves-api`** — `ui.SecretOnce` + `ui.APIKeysTable`, em "API keys" (auth).
- **`ui-uso-api`** — `ui.APIUsage`, na seção de uso de chave (auth).
- **`ui-permissoes`** — `ui.PolicyGrid`, em "A matrix people edit" (auth).
- **`ui-configuracoes`** — `ui.SettingsForm` sobre `trilha.Settings[T]`, em "Settings" (app).
- **`ui-aprovacoes`** — `ui.Inbox` + `ui.InboxBadge`, em "The screen" (approval).
- **`ui-prazos`** — `ui.DeadlineCards`/`DeadlineList`/`DeadlineBadge`, na própria
  `reference/ui.md` (onde o componente já tem prosa funda); sem receita própria.
- **`ui-versoes`** — `ui.VersionList`/`VersionBadge`/`Changed`, idem, também sem receita.
- **`ui-tarefas`** — `ui.TaskProgress` + `ui.TaskTable`, em "The screens" (task); seedado direto
  num `task.Memory()` para não depender de uma goroutine de worker no teste que renderiza.
- **`ui-webhooks`** — `ui.WebhooksPanel`, em "The screen" (webhook).
- **`ui-conexoes`** — `ui.ConnectionsPanel`/`ConnectionStatus`/`ParseConnectionForm`, na própria
  `reference/ui.md` (mesma razão de `ui-prazos`), com `SecretField` embutido no próprio
  formulário do painel.
- **`ui-avisos-csv`** — `ui.Flashes` + `ui.CSVErrors` juntas, em "Spreadsheets" (ctx): a resposta
  real de uma importação — o aviso de resumo e a tabela por célula — sem receita própria.

Cada demo com receita diz qual `trilha add <receita>` escreve aquela tela, com link para
[`trilha add`](/reference/cli#trilha-add); as duas sem receita (`ui-prazos`, `ui-versoes`) e as
duas primitivas de `Ctx` (`ui-avisos-csv`) dizem que não há receita — são o primitivo do
framework usado direto. `reference/ui.md` e o capítulo do pacote que documenta cada tela ganham
o link "see demo" (en) / "veja demo" (pt) na linha do catálogo e na prosa, nas duas locales.

```go
// exemplo do uso final: uma das doze funções em site/internal/demos/kit.go
add("en", Demo{
	Name:  "ui-auditoria",
	Title: "Who did what, in a DataTable",
	Source: `records, total := auditoria.Search(q)
return ui.AuditTable(c, records, ui.AuditOpts{...}), nil`,
	Node: func() h.Node { return auditDemo(false) },
})
```

## Fora de escopo

- Demos de layout/navegação (spec 139) e de dados/gráficos (spec 140): as outras duas issues.
- Uma rota `/demos/<id>` própria: como nas duas specs anteriores, o `@demo` embutido continua
  bastando — nenhuma destas doze precisa de uma URL isolada.
- Acrescentar uma receita `trilha add deadlines` ou `trilha add versions`: os dois primitivos
  (`trilha.Deadlines`, `trilha.Versioned[T]`) não têm — e não pedem — uma receita hoje; a demo
  documenta esse fato em vez de inventar uma.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `site/internal/demos` passa a importar também `auth` e `task`, ambos do próprio módulo (nenhuma dependência externa nova); `context` é biblioteca padrão |
| VI — teste primeiro | `TestKitPatternDemosRender` (novo, em `site/site_test.go`) falha antes das doze demos existirem — cada página do capítulo do pacote precisa responder com uma marca do componente e, onde há receita, o próprio `trilha add <receita>` |
| VII — segurança por padrão | Nenhum HTML novo escapa por fora do `h`; nenhum link do resultado renderizado é absoluto (mesma regra das specs 139/140); os stores de exemplo (`task.Memory()`, `trilha.ConnectionMemory()`) são seedados com dados fixos, sem entrada externa |

## Tarefas

- [x] T001 Teste que falha: `TestKitPatternDemosRender` em `site/site_test.go`, contra o estado
      sem as doze demos novas
- [x] T002 As doze demos em `site/internal/demos/kit.go` (pt e en)
- [x] T003 `@demo` embutido no capítulo do próprio pacote (observability, auth ×3, app ×1,
      approval, task, webhook, ui ×2, ctx) e link "see/veja demo" no catálogo de
      `reference/ui.md`, nas duas locales
- [x] T004 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T005 `make test` verde

## Aceitação

- **SC-001**: `go test ./site/...` passa com as doze demos novas nas duas locales, mesma
  quantidade e mesma ordem de `@demo` (`TestLocalesInSync`).
- **SC-002**: nenhuma página do site linka para um caminho que não existe
  (`TestInternalLinksResolve` continua verde com o HTML das demos novas).
- **SC-003**: cada componente da issue #191 aparece em pelo menos uma demo viva, dentro do
  capítulo do próprio pacote (não em `learn/ui-kit`), e a demo com receita contém o texto
  `trilha add <receita>` (`TestKitPatternDemosRender`).
- **SC-004**: `reference/ui.md` (en e pt) linka a demo de cada um dos doze grupos, a partir da
  linha do catálogo ou da prosa mais próxima do componente.
