# Tasks: Inglês por padrão, português como tradução (i18n)

**Input**: [spec.md](spec.md), [plan.md](plan.md)

## Phase 1: Constituição e runtime

- [x] T001 Emendar `.specify/memory/constitution.md` (v1.2.0): seção "Estilo, idioma e interface" com a regra de inglês por padrão + tradução pt-BR no mesmo commit; specs e constituição em pt-BR; erros de biblioteca em inglês.
- [x] T002 [TDD] `export_test.go`: caminho exportado que responde 301 interno vira `index.html` com `meta refresh` + `canonical`; 3xx externo continua erro. Implementar em `export.go`.
- [x] T003 Mensagens do runtime/scanner/gerador/scaffold em inglês (`bind.go`, `export.go`, `render.go`, `signed.go`, `trilha.go`, `internal/scan/scan.go`, `internal/scaffold/ui.go`); ajustar testes que conferem texto.

## Phase 2: Site bilíngue

- [x] T004 [TDD] `site/site_test.go`: páginas das duas locales respondem 200 com `lang` certo; `/aprender/*` e `/referencia/*` → 301 para `/pt/...`; `hreflang` e switcher apontam para a tradução da página; export cobre tudo; sincronia (seções, contagem, `@demo`).
- [x] T005 `docs.go`: `Locale`, `Section`, `Page.Locale`, `Locales`, `Get(locale, section, slug)`, `All()`, `Translation(p, locale)`, `Neighbors` por locale; conteúdo movido para `content/pt/...`; `content/en/...` criado (placeholders substituídos na fase 3).
- [x] T006 `md.go`: `Options.Locale`; títulos de callout por idioma; aliases `tip|warning|note|challenge|solution`.
- [x] T007 `demos`: `For(locale)`, `Card(locale, d)`, versões `en` de todas as demos (`demos.go`, `kit.go`).
- [x] T008 `ui`: `text.go` com `T(c, key)`; header com switcher; layout com `hreflang`, `lang`, descrição por idioma; sidebar/TOC/vizinhos/DocPage por locale; footer e nota de estatísticas.
- [x] T009 `site/internal/home`: home nas duas línguas; `site/app/page.go` e `site/app/pt/page.go`.
- [x] T010 Rotas: `site/app/learn`, `reference`, `pt/aprender`, `pt/referencia` (índice + `slug_`); `aprender`/`referencia` antigos viram 301; `middleware.go` define locale e seção; `setup.go` registra export paths (locales + antigos); `not_found.go` bilíngue; `trilha_gen.go` regenerado.
- [x] T011 `tema.js`: textos por `document.documentElement.lang`.

## Phase 3: Conteúdo em inglês

- [x] T012 `content/en/learn/`: index, pages-and-routes, layouts, html-with-h, forms, api, middleware, security, ui-kit, ai-and-agents, examples, dev-and-production, troubleshooting.
- [x] T013 `content/en/reference/`: index, conventions, ctx, h, tmpl, errors, app, security, ui, ai, mcp, cli, performance.
- [x] T014 Atualizar `content/pt` e `content/en` com `TRILHA_LANG`/`--lang` (cli) e a nota sobre exemplos em português (examples/exemplos).

## Phase 4: CLI e scaffold

- [x] T015 [TDD] `cmd/trilha/e2e_test.go`: `trilha help` em `en`/`pt`; `trilha new --lang pt` gera `h.Lang("pt-BR")` e textos em português; sem `--lang` segue `TRILHA_LANG`.
- [x] T016 `cmd/trilha/i18n.go` (`lang()`, `t()`), todos os comandos e `audit` usando a tabela.
- [x] T017 `internal/scaffold`: `Data.Lang`; templates com textos por idioma; `new.go --lang`.

## Phase 5: Repositório

- [x] T018 `README.md` (en) + `README.pt-BR.md`; links para o site por idioma.
- [x] T019 `CONTRIBUTING.md`, `GOVERNANCE.md`, `SECURITY.md`, `SUPPORT.md`, `CODE_OF_CONDUCT.md` em inglês; traduções em `docs/pt-BR/`; `.github/ISSUE_TEMPLATE/*`, `PULL_REQUEST_TEMPLATE.md` em inglês.
- [x] T020 `CHANGELOG.md` em inglês com entrada 0.6.0; `version = "0.6.0"`; `pages.yml` sem mudança de caminho (export cobre `pt/`).
- [x] T021 `make test`, `trilha export` local do site conferido no navegador (en, pt, redirect antigo), merge ff em `main`, tag `v0.6.0`, release.
