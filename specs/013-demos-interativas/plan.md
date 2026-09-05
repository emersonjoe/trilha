# Implementation Plan: 013-demos-interativas

Constitution Check: I–V sem impacto (só o site de documentação); VI teste antes da
correção (o teste de manipulador inline falha com o código atual); VII a mudança **remove**
uma prática que a CSP padrão bloqueia, alinhando o site ao que ele ensina.

1. Teste em `site/site_test.go` que varre todas as páginas procurando `on…=` (falha hoje).
2. `site/internal/demos/demos.go`: formulário sem `Onclick`, com `data-demo="form"`,
   `method="get"` e `h.Output(data-demo-saida)` com a legenda.
3. `site/public/tema.js`: listener de `submit` → `preventDefault`, *slug* do nome, texto
   `POST … → 303 … → GET /eventos/<slug>`.
4. `site/public/site.css`: `.demo-nota`.
5. `examples/orcamento/internal/componentes/componentes.go`: remover `h.Attr("onchange","")`.
6. Verificar no navegador (dev e export), merge, tag de correção.
