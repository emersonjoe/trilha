# Implementation Plan: 009-exemplos

Constitution Check: I (só convenções existentes; exemplos + testes), II (stdlib; `Bind`
usa `reflect` para decodificar structs, como `encoding/json` — não para descobrir rotas),
III (nada em runtime), IV (`Bind` e `FieldErrors` são adições ao `Ctx`/erros), V (sem
impacto), VI (testes por exemplo), VII (validação no servidor; escape via `h`).

Ordem: US3 primeiro (Bind, FieldErrors, ui.Value/Errors/SelectOptions) com testes, depois
`examples/cadastro`, depois `examples/orcamento`, docs.

Estrutura:
```
examples/cadastro/ app/{layout,page,setup}.go app/api/cidades/route.go internal/{clientes,validar} public/{app.js,style.css} cadastro_test.go README.md
examples/orcamento/ app/{layout,page,setup}.go app/contas/codigo_/page.go app/lancamentos/route.go(POST) app/api/relatorio.csv/route.go internal/{plano,componentes} orcamento_test.go README.md
```
`app/api/relatorio.csv/` usa a convenção de pasta com ponto (spec 008).
