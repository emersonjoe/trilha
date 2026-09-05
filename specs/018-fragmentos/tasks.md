# Tasks: 018-fragmentos

- [ ] T001 `fragment_test.go`: `Ctx.Fragment`, resposta sem layout e sem `<!doctype>`,
      `Vary`, script de dev ausente, redirecionamento → 204 + `Trilha-Location`, `Ctx.Render`
      em 422 sem layout
- [ ] T002 `ctx.go` + `render.go`: `Fragment()`, salto de layouts e do envelope, `Vary`
- [ ] T003 Redirecionamento em requisição de fragmento vira 204 + `Trilha-Location`
- [ ] T004 `ui/assets/ui.js`: clique em `<a>`, envio de `<form>`, troca, `aria-busy`, foco,
      histórico e recuo para navegação; `ui.Swap` e estilo de ocupado
- [ ] T005 `examples/cadastro`: busca na lista e envio sem recarga, com teste dos dois
      caminhos (com e sem cabeçalho)
- [ ] T006 Documentação: capítulo de interatividade, referência (`Ctx`, `ui`), CHANGELOG,
      versão; fechar #20 e #21
