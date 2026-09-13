# Tarefas — Spec 148

- [x] T001 `routing_test.go`: `GET /ui.css` num app com `/{lang}` na raiz responde 200 e
      `text/css`; `/en` continua na rota; rota literal ganha do arquivo de mesmo endereço e o
      `Asset` avisa nesse caso; arquivo de `Mounts` também passa na frente do curinga
- [x] T002 `routing_test.go`: `Register` com `/o/{slug}/login` e `/{lang}/cards/{deckId}` não
      entra em `panic` nas duas ordens; `/o/cards/login` cai na primeira e `/en/cards/mazo-1` na
      segunda, com os `Param` certos; 405 com `Allow`, `HEAD` como `GET`, barra final
      redirecionando, `c.Pattern()` correto; padrão repetido continua `panic`
- [x] T003 `routing.go`: segmentos do padrão, casamento com captura, comparação por
      especificidade, `bestRoute`, despacho das rotas em conflito
- [x] T004 `serve.go`/`trilha.go`/`export.go`: `Register` sondando o `pathMux`, `dispatch` com o
      estático antes da rota curinga, `fallback` enxergando as rotas próprias, `export.render`
      pelo `dispatch`
- [x] T005 `assets.go`: aviso de endereço tomado por rota literal
- [x] T006 `examples/blog`: `public/blog/capa.svg` sob `/blog/{slug}`, `/oficinas/{slug}/inscricao`
      e `/{secao}/inscricao/{id}`, `trilha_gen.go` regerado, teste de integração dos dois casos
- [x] T007 Referência en + pt: precedência por segmento nas convenções e a ordem do estático em
      `app`
- [x] T008 `CHANGELOG.md` (Fixed/Changed), `version` = 0.127.0, `ROADMAP.md`
- [ ] T009 `make test` verde, `verifica-trilha.sh --sem-testes` e
      `make release VERSION=0.127.0 ISSUES="203 204"`
