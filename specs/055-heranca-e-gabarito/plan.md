# Plano — spec 055

## Fatos que decidem o desenho

1. **O `*Route` já está no `Ctx`.** O `wrap` faz `c.route = r` antes de qualquer middleware,
   então o `Pattern()` é um getter e vale desde o primeiro middleware da cadeia. O fallback
   monta o `Ctx` sem rota, e é por isso que ele devolve `""` em vez de mentir.
2. **O log já tem o gabarito ao lado.** O `observe` do `wrap` recebe `r.Pattern` para a
   métrica; o `logRequest`, que roda na mesma função, não o gravava. Não é dado novo, é um
   campo que faltava.
3. **`Kind` é uma variável de pacote, não de arquivo.** O varredor lê `pkg.vars["Kind"]`, que
   já é verdadeiro para qualquer arquivo do pacote da pasta. A herança, então, não é uma
   convenção de arquivo: é passar a referência pela recursão do `walk`, do mesmo jeito que
   `layouts` e `mws` descem.
4. **O gerado precisa saber de qual pacote vem.** Hoje `HasKind` é um `bool` e o gerador
   escreve `Kind: <alias da própria rota>.Kind`. Com herança o pacote é outro, então o que
   desce é um `*Ref` — e o `import` só entra no arquivo gerado quando uma rota de fato o lê,
   senão uma pasta que declara `Kind` e não tem descendente com `route.go` deixaria um import
   sem uso no código gerado.
5. **Página é página.** `kindOf` já responde `kindPage` para uma rota com `Page != nil`
   quando `Kind` é `KindAuto`; se um `Kind` herdado descesse até um `page.go`, um
   `KindAPI` de um ramo de API transformaria a página em JSON. O varredor não anexa `Kind` a
   uma rota de `page.go` — o que também deixa a raiz `app/` livre para declarar `KindPage`
   sem tocar em nada.
6. **O audit lê o código, não o ambiente.** Depois da spec 052 ele já roda o `scan.Scan` para
   conferir o gerado; a checagem nova sai da mesma varredura, sem ler arquivo de novo.

## Corte

- `Pattern()` devolve `string`, não `(string, bool)`: `""` é a resposta do fallback e não há
  terceira possibilidade.
- O campo do log chama-se `route`, não `pattern`: é o nome que um sistema de métricas usa
  para esse rótulo, e o `path` ao lado já diz que o outro é o concreto.
- `HasKind bool` some da `scan.Route` em vez de conviver com o `*Ref`: são a mesma pergunta,
  e o `internal/scan` não é superfície pública.
- O aviso do `audit` é `warn`, não `critical`: existe app que serve HTML e recebe `POST` de
  um cliente próprio, e reprovar o `trilha check` dele seria a repetição do #77.

## Arquivos

| Arquivo | Mudança |
| --- | --- |
| `ctx.go` | `Pattern()` |
| `serve.go` | `route` nos dois ramos do `logRequest` |
| `trilha.go` | doc do `Route.Kind` com a herança |
| `internal/scan/scan.go` | `KindRef` descendo pelo `walk` |
| `internal/gen/gen.go` | `Kind:` a partir do `KindRef` |
| `cmd/trilha/audit.go`, `cmd/trilha/i18n.go` | aviso da escrita sem CSRF |
| `internal/scan/scan_test.go`, `internal/gen/*` | herança, precedência, página intacta |
| `examples/blog/app/legado-/kind.go`, `.../legado/apagar/route.go` | a convenção em uso |
| `examples/blog/blog_test.go`, `ctx_test.go`, `serve_test.go` | integração e log |
| docs `ctx`, `routing`/`rotas`, `cli` nas duas línguas | referência |
| `CHANGELOG.md` | entradas na 0.39.0 |
