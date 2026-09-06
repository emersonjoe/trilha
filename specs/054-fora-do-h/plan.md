# Plano — spec 054

## Fatos que decidem o desenho

1. **O nonce já existe antes do handler.** O `applySecurity` chama `c.Nonce()` para montar a
   CSP, então um template que pergunta no meio da renderização recebe o mesmo valor que o
   cabeçalho já anunciou. Não há corrida a resolver.
2. **O token é preguiçoso de propósito.** `c.CSRFToken()` cria o cookie na primeira leitura;
   guardar o valor no contexto no `newCtx` poria um `Set-Cookie` em toda resposta, inclusive
   nas de API. Por isso o que viaja no contexto é o `*Ctx`, e os leitores chamam os métodos.
3. **`SetContext` deriva o contexto.** O valor sobrevive a um middleware que empilha os seus;
   só um `SetRequest` com uma requisição feita do zero o perderia, e aí o app trocou a
   requisição inteira de propósito.
4. **`html/template` não clona um conjunto já executado.** É por isso que o `Wrap` é de nível
   de pacote e o `Shell` é o valor que sobra: com a assinatura de uma chamada só, do jeito
   que a issue sugere, o primeiro request já teria executado o template e o segundo não
   clonaria mais.
5. **Clonar por requisição seria caro.** Medido num conjunto de 21 templates (M2): clonar,
   definir o slot e executar custa ~75 µs; executar uma casca já preparada e emendar o miolo
   custa ~9 µs. A casca é preparada uma vez, e o lugar do miolo é marcado com 16 bytes de
   `crypto/rand` em hexadecimal — que o `html/template` copia inalterados em qualquer
   contexto, e que os dados do app não têm como conter.
6. **O miolo entra por escrita, não por concatenação.** A saída da casca é cortada no marcador
   e escrita em pedaços, com o filho no meio: sem cópia da página inteira e sem `template.HTML`
   em lugar nenhum além do próprio `tmpl`.
7. **Erro nunca sai pela metade.** O `Node` do `tmpl` já rende num buffer antes de escrever;
   o `Shell` faz o mesmo, e só começa a escrever depois de ter achado o marcador.

## Corte

- `NonceFrom`/`CSRFTokenFrom` devolvem `string`, não `(string, bool)`: quem não tem nonce
  recebe `""`, que é o que o `Nonce()` já devolve num app embutido.
- `Wrap` entra em pânico em vez de devolver erro, como o `Must` do mesmo pacote: um template
  que não clona é um erro de programação que aparece na carga do pacote, não numa requisição.
- Um slot chamado duas vezes escreve o miolo duas vezes; é o que o template pediu.

## Arquivos

| Arquivo | Mudança |
| --- | --- |
| `ctx.go` | `*Ctx` no contexto, `ctxOf(r)` |
| `security.go`, `csrf.go` | os dois leitores |
| `tmpl/tmpl.go`, `tmpl/tmpl_test.go` | `Wrap`, `Shell`, `HTML` |
| `csrf_test.go`, `security_test.go` | leitores, `SetContext`, requisição de fora |
| `examples/blog/app/legado-/**` | casca em `html/template` com miolo em `h` |
| docs `ctx`, `security`, `tmpl` + guia de migração nas duas línguas | referência e receita |
| `CHANGELOG.md` | entradas na 0.39.0 |
