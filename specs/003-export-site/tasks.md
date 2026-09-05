# Tasks: Exportação estática e site de documentação

- [x] T001 Testes `export_test.go` (rotas estáticas, extra paths, dinâmicas ignoradas, 404.html, public copiado, erro em rota falha, marcador de limpeza) e `Ctx.Base()`
- [x] T002 Implementar `export.go` (Export, AddExportPath, Base) e hook `TRILHA_EXPORT` no gerador (goldens)
- [x] T003 `cmd/trilha/export.go` + e2e (`new` → `export`)
- [x] T004 `site/internal/md`: Markdown mínimo (títulos, parágrafos, listas, código cercado com realce Go, inline code, negrito, links, tabelas, citações, `:::desafio`) com testes
- [x] T005 `site/`: layout (sidebar, sumário, tema, prev/next), home com demos "código → resultado", páginas Aprender (8) e Referência (7), 404, `public/site.css` + `theme.js`
- [x] T006 Teste de integração do site: todas as rotas 200, cada capítulo tem "Desafio", links internos resolvem
- [x] T007 Workflow `pages.yml` + habilitar Pages via API; verificar URL pública
- [x] T008 Verificação visual no navegador (desktop/móvel, claro/escuro) e ajustes; README aponta para o site
