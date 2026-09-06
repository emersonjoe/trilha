# Plano: auxiliares de teste

## Fatos que decidem o desenho

1. **`testing` não pode entrar no grafo do runtime.** Importar `testing` num arquivo normal
   registra as flags `-test.*` no binário de produção e polui o `flag.CommandLine` de quem
   usa o framework. Daí a interface `TestingT` com `Helper()` e `Fatalf(...)`: `*testing.T` a
   satisfaz sem que ninguém precise saber disso. `httptest` não importa `testing`, então
   pode entrar.
2. **O caminho de verdade já está montado.** `App.Handler()` devolve o mux com middlewares,
   CSRF, layouts e negociação de erro. Um auxiliar que refaça esse pipeline testaria outra
   coisa. Então tudo — `TestRequest`, `TestRoute`, `TestPage` — termina em
   `handler.ServeHTTP(rec, req)`.
3. **`TestRoute` e `TestPage` só precisam de um `App` qualquer.** `Register` monta o mux a
   partir da `Route`, inclusive os `{id}`. Um `New(Config{Env: Dev})` descartável basta, e
   `WithApp` cobre quem quer o seu (com `Values`, `Signer`, config).
4. **O nó renderizado não sai do handler.** A resposta de uma página é HTML. Para `TestPage`
   devolver `Node`, o auxiliar envolve a `Route.Page` numa função que guarda o nó antes de
   devolvê-lo — o layout continua sendo aplicado pelo pipeline, porque quem chama os layouts
   é o `App`. Um campo, sem gancho novo no runtime.
5. **CSRF de duplo envio não precisa do servidor.** O cookie e o token são o mesmo valor
   gerado pelo cliente; o app só compara. Então o auxiliar gera um valor de 32 bytes por
   cliente, manda no cookie e, nos métodos com corpo, no cabeçalho — e o `WithoutCSRF`
   simplesmente não manda nada.
6. **Cookie assinado precisa do `Signer` do app.** `WithSigned` só funciona quando há app
   (sempre há: `TestRoute`/`TestPage` montam um). O valor é assinado com `a.signer` e uma
   validade de uma hora, que é o que um teste precisa.
7. **O pote de cookies é do cliente, não do pedido.** `TestRequest` é um tiro só; quem
   precisa de fluxo usa `NewTestClient`. Internamente é o mesmo caminho: o tiro só é um
   cliente descartável.
8. **Asserção que devolve `*TestResponse` encadeia.** `res.WantStatus(200).WantContains("x")`
   lê melhor do que três `if`. Toda falha chama `Fatalf` com o corpo junto, porque o corpo é
   o que explica a falha.

## Fases

1. **Núcleo.** `testing_test.go` na raiz com o que a spec promete (SC-002 a SC-004), depois
   `testing.go`.
2. **Exemplos.** Trocar os cinco `client` locais pelos auxiliares, um exemplo por vez,
   rodando a suíte do exemplo a cada troca.
3. **Documentação.** Página nova `learn/testing` + `pt/aprender/testes` (registro em
   `site/internal/docs/docs.go`), `reference/app` citando o canto de teste.
4. **Fechamento.** CHANGELOG, `version`, ROADMAP, release.

## Riscos

- **A API cresce demais.** Mitigação: nada entra que não seja usado por pelo menos um dos
  cinco exemplos depois da fase 2. O que sobrar sai antes da release.
- **`TestPage` com layout que ignora `children`.** O nó guardado é o da página, não o do
  layout; a documentação diz isso em uma linha.
- **Exemplo que dependia do pote de cookies de um jeito sutil** (`sso` guarda estado de
  OIDC). Mitigação: converter por último e manter o teste do fluxo inteiro.
