# Spec 139 — site: demos do kit — layout e navegação

> Mudança pequena: um pacote (`site/internal/demos`) mais o conteúdo do site, sem convenção
> nova em `app/`, sem mudança na API pública.

- **Issue**: [#189](https://github.com/emersonjoe/trilha/issues/189) — layout e navegação são
  a primeira das três frentes de demos do kit; a issue é a fonte do escopo, este documento não
  a repete.
- **Branch**: `feat/demos-layout`
- **Versão**: 0.118.0

## Por quê

O kit `ui` tem 164 funções exportadas; antes desta spec, `site/internal/demos/kit.go` cobria
6 delas com demo viva (botões, formulário, card, diálogo, tabela, paginação) — tudo que entrou
depois da 0.6x só tem linha no catálogo de `reference/ui.md`, sem exemplo em `learn/`. Quem
decide usar o Trilha olhando o site não vê o `Shell` de um app interno, o `Steps` de um
formulário em etapas ou o `Empty` de uma tela sem dado — só o nome na tabela.

## O que muda

Quinze demos novas em `site/internal/demos/kit.go` (função Go registrada no `init`, código e
resultado lado a lado — o mesmo molde das seis existentes), cobrindo os componentes de layout
e navegação da issue: `Shell`+`PageHeader`, `Breadcrumb`+`Avatar`, `MenuTrigger`/`MenuItem`,
`Collapsible`, `Tabs` (demo própria, além da que já aparece dentro do card), `Grid`/`Row`/
`Stack`/`Separator`, `Skeleton`+`Progress`, `Kbd`+`Code`, `Status`, `Empty`/`EmptyError`,
`Steps`, `Defer`, `Preview`, `Confirm` e `Indicator`/`Spinner`/`NoTransition`.

As demos não ganham rota própria: seguem o mecanismo já provado do `@demo <nome>` dentro do
Markdown, o mesmo das seis existentes — só as três demos interativas mais antigas (assistente,
chat, agente) têm `/demos/<id>` próprio, e essa forma continua fora do escopo desta issue
("não invente mecanismo novo"). `learn/ui-kit.md` e `reference/ui.md` (en e pt) ganham uma
seção nova por grupo de componentes com o `@demo` embutido, e cada linha correspondente do
catálogo de `reference/ui.md` ganha um link "veja demo" para a seção.

Ajustes de sandbox, sem mudar o comportamento real do componente:

- `ui.Shell` assume `min-height: 100dvh` (é a moldura da página inteira); dentro do cartão de
  demo isso é limitado por uma regra em `site/public/site.css`
  (`.demo-saida .kit .ui-shell`), que não afeta quem usa o kit no próprio app.
- Nenhum link do resultado renderizado aponta para um caminho real do site (a checagem de
  `TestInternalLinksResolve` roda sobre o HTML de toda página): os `Href` de exemplo ficam sem
  barra inicial (`"documents"`) ou como `"#"`, e o `Source` ao lado mostra o caminho absoluto
  que um app de verdade usaria.
- `ui.Indicator`/`ui.Spinner`/`ui.NoTransition` aparecem com o código real de um gatilho
  (`ui.Swap` + `ui.PendingAfter`), mas o resultado renderizado é estático — o `Spinner` fica
  sempre visível — porque a demo não tem uma rota de fragmento de verdade por trás para
  disparar a espera sem risco de comportamento imprevisto na página publicada.

```go
// exemplo do uso final: uma das quinze funções em site/internal/demos/kit.go
add("en", Demo{
	Name:  "ui-shell",
	Title: "The frame of an internal app, with the active item",
	Source: `ui.Shell(c, ui.ShellOpts{...}, ui.PageHeader("Documents", ...))`,
	Node: func() h.Node { return wrap(kit.Shell(nil, kit.ShellOpts{...}, ...)) },
})
```

## Fora de escopo

- Demos de dados e gráficos, e das telas dos padrões: as outras duas issues da série.
- Uma rota `/demos/<id>` própria para cada demo do kit — fica para quando (se) o site
  precisar de uma URL isolada por demo; hoje o `@demo` embutido é suficiente e é o que já
  existe.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `site/internal/demos` já depende só de `h` e `ui`; nada novo entra |
| VI — teste primeiro | `TestEveryPageResponds`/`TestLocalesInSync`/`TestInternalLinksResolve` (já existentes em `site/site_test.go`) falham antes das seções novas terem `@demo` e passam depois; nenhum teste novo foi necessário porque a suíte já cobre "toda página responde" e "as duas locales têm as mesmas demos" |
| VII — segurança por padrão | Nenhum HTML novo escapa por fora do `h`; o `data:` URI da imagem de exemplo do `ui-preview` é literal, não vem de entrada externa |

## Tarefas

- [x] T001 Rodar a suíte existente (`TestEveryPageResponds`, `TestLocalesInSync`,
      `TestInternalLinksResolve`) contra o estado sem as demos novas, confirmando que
      referenciar `@demo ui-shell` etc. sem a entrada em `kit.go` quebra o build/teste
- [x] T002 As quinze demos em `site/internal/demos/kit.go` (pt e en)
- [x] T003 Seções novas em `learn/ui-kit.md` / `aprender/interface-com-ui.md` com os `@demo`;
      links "veja demo" nas linhas correspondentes de `reference/ui.md` / `referencia/ui.md`
- [x] T004 `site/public/site.css`: regra de altura para `.ui-shell` dentro do cartão de demo
- [x] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T006 `make test` verde

## Aceitação

- **SC-001**: `go test ./site/...` passa com as quinze demos novas referenciadas nas duas
  locales, mesma quantidade e mesma ordem de `@demo` (`TestLocalesInSync`).
- **SC-002**: nenhuma página do site linka para um caminho que não existe
  (`TestInternalLinksResolve` continua verde com o HTML das demos novas).
- **SC-003**: cada componente listado na issue #189 aparece em pelo menos uma demo viva e tem
  um link "veja demo" a partir da linha correspondente em `reference/ui.md` (en e pt).
