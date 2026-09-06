# Plano — spec 052

## Fatos que decidem o desenho

1. **O roteador já roteia OPTIONS.** `App.Register` monta `a.mux.Handle(m+" "+pat, ...)` a
   partir do mapa de métodos, sem lista fixa. O 405 vem do `fallback` (`serve.go`), que só é
   alcançado porque nenhum handler foi registrado. Logo a mudança é do varredor, e nenhum
   caminho do runtime precisa aprender um método novo.
2. **O varredor já lê `var` exportado.** `parsePackage` guarda `info.vars`, que é como o
   `Kind` chega ao gerador. `HasCORS` custa uma linha e nenhum parser novo.
3. **A política de CORS já existe pronta.** `newCORSPolicy(CORS, csrfHeader) *corsPolicy` e
   `(*corsPolicy).handle(w, r) bool` fazem preflight, 403 e cabeçalhos. Por rota é a mesma
   política com outro dono: montada uma vez no `Register`, chamada antes do handler.
4. **`handle` devolve "já respondi".** Por isso a rota com `var CORS` é um `http.Handler`
   por fora do que o `wrap` devolve: o preflight não passa por middleware, CSRF nem Ctx, que
   é o que o browser espera de um 204 de preflight.
5. **`Methods[1:]` do `page.go` supõe que o GET é o primeiro.** `OPTIONS` entra no fim da
   lista; a ordem da lista é a ordem em que o gerador escreve os métodos, e nada mais.
6. **HEAD não é lacuna, é duplicata.** O `ServeMux` do Go 1.22 casa HEAD com o handler do
   GET. Aceitá-lo criaria duas respostas para a mesma requisição, então ele entra na lista
   dos que param a geração com mensagem, e não na dos roteáveis.
7. **O `audit` já lê o código do projeto.** `projectSource` concatena os `.go` não-teste;
   as checagens de host e de métricas já decidem por substring. O segredo passa a decidir do
   mesmo jeito, com a lista de chamadas que realmente dependem dele.

## Corte

- Preflight por rota responde **antes** do middleware, como o do app inteiro: um preflight
  que passa por autenticação é um preflight que falha.
- `var CORS` inválido continua entrando em pânico no `New`/`Register` (origem malformada,
  `"*"` com `Credentials`): a mesma decisão do `Config.CORS`, e um erro de CORS que só
  aparece na primeira requisição de fora é um erro que ninguém vê em desenvolvimento.
- O erro do método não-roteável nomeia o conserto (`rename func HEAD`... / `GET already
  answers HEAD`), no formato que a 0.37.0 fixou.

8. **O `Config.CORS` responde antes do roteador.** Descoberto no teste de integração: com a
   lista exata do `examples/blog`, o preflight do documento levava 403 do app inteiro e a
   rota nunca era chamada. O `serveHTTP` passa a olhar se o caminho tem política própria
   antes de aplicar a do app — uma consulta ao `pathMux`, só quando há política de app e a
   requisição traz `Origin`.

## Arquivos

| Arquivo | Mudança |
| --- | --- |
| `internal/scan/scan.go` | `Methods` com OPTIONS; `HasCORS`; dois códigos de erro; mensagens |
| `internal/scan/scan_test.go` | OPTIONS roteável, `E_UNROUTABLE_METHOD`, `E_CORS_ON_PAGE`, `HasCORS` |
| `internal/gen/gen.go` | `CORS: &{{.Alias}}.CORS` |
| `testdata/golden/*` | golden do gerador com a rota que tem CORS |
| `cors.go` | doc do tipo `CORS` citando o `var CORS` da rota |
| `serve.go` | `Register` monta a política e embrulha os handlers; OPTIONS gerado |
| `trilha.go` | `Route.CORS *CORS`; a rota com política própria não passa pela do app |
| `cors_test.go` | preflight por rota, 403, requisição simples, OPTIONS à mão vence |
| `internal/openapi/openapi.go` | pula OPTIONS |
| `cmd/trilha/audit.go` + `i18n.go` | aviso quando nada assina |
| `cmd/trilha/audit_test.go` | os dois lados da decisão |
| `examples/blog/app/.well-known/security.txt/route.go` | `var CORS` + `func OPTIONS` |
| `site/.../reference/conventions.md` + `pt/referencia/convencoes.md` | a linha nova |
| `site/.../learn/api.md` + `pt/aprender/api.md` | receita do preflight |
| `CHANGELOG.md` | entradas na 0.39.0 |
