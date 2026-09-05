# Tasks: 017-cache-busting

- [ ] T001 `assets_test.go`: hash estável entre chamadas, muda com o conteúdo, caminho
      ausente devolve o original, `dev` relê após alteração, `prod` não relê
- [ ] T002 `assets.go`: `App.Asset`, `Ctx.Asset`, cache com invalidação por `Stat` em `dev`
- [ ] T003 `static.go`: `?v=` correto → `public, max-age=31536000, immutable`; divergente ou
      ausente → comportamento atual; teste dos três casos
- [ ] T004 `ui.Head` e layout do site passam a usar `Ctx.Asset`; `examples/blog` também
- [ ] T005 `trilha audit`: aviso de `StaticCacheControl` com `immutable` sem uso de `Asset`
- [ ] T006 Documentação (referência `app`, capítulo `dev-e-producao`), CHANGELOG, versão
