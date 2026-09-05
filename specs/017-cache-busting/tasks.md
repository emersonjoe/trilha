# Tasks: 017-cache-busting

- [x] T001 `assets_test.go`: hash estável entre chamadas, muda com o conteúdo, caminho
      ausente devolve o original, `dev` relê após alteração, `prod` não relê
- [x] T002 `assets.go`: `App.Asset`, `Ctx.Asset`, cache com invalidação por `Stat` em `dev`
- [x] T003 `static.go`: `?v=` correto → `public, max-age=31536000, immutable`; divergente ou
      ausente → comportamento atual; teste dos três casos
- [x] T004 `ui.Head` e layout do site passam a usar `Ctx.Asset`; `examples/blog` também
- [x] T005 `trilha audit`: aviso de `StaticCacheControl` com `immutable` sem uso de `Asset`
- [x] T006 Documentação (referência `app`, capítulo `dev-e-producao`), CHANGELOG, versão

Concluídas na v0.8.0. Diferenças em relação ao plano:

- O aviso da auditoria olha o par `immutable` + ausência de `.Asset(` no código do projeto;
  ler `StaticCacheControl` de verdade exigiria executar o `Config()` do app.
- `App.Asset` já aplica `BasePath`, então `Ctx.Asset` é uma linha — o plano previa a
  duplicação da lógica.
