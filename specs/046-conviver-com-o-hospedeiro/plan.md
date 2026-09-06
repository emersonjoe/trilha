# Plano — Conviver com o hospedeiro

## Fatos que decidem o desenho

1. **`Off` já existe e não serve para isto.** `Off` é por cabeçalho e significa "não quero
   esta política". O caso do embutido é outro — "não sou eu quem responde" — e ele inclui o
   `X-Content-Type-Options`, que hoje é incondicional. Por isso é um campo novo, não um sétimo
   `Off`.
2. **O zero valor decide o padrão.** `Delegated bool` com zero `false` mantém o app seguro para
   quem não fez nada. Um `Managed bool` com padrão `true` teria que ser preenchido em `New`, e
   quem escrevesse `Security{CSP: ...}` inteiro por literal — que é o que a própria issue
   mostra — perderia todos os cabeçalhos em silêncio. É o oposto do princípio VII.
3. **`applyConfig` roda em `New` e de novo ao subir o servidor.** É lá que os nomes do CSRF
   ganham padrão, para que `Setup` ainda possa trocá-los e para que o resto do código leia
   sempre um nome concreto, nunca uma string vazia.
4. **`c.nonce` é campo do `Ctx` e é preenchido sob demanda.** Chamar a função do hospedeiro
   dentro de `Ctx.Nonce()` mantém uma chamada por requisição, e só nas requisições que
   perguntam. Com `Delegated`, `applySecurity` sai antes de perguntar.
5. **O mapa `values` é escrito no `Setup` e lido em requisição.** `Provide` não muda isso; ele
   só escolhe a chave. Chave é `reflect.TypeOf((*T)(nil)).Elem().String()`, que distingue
   `*farol.Deps` de `farol.Deps` e carrega o pacote no nome.
6. **`Use` entra em pânico de propósito.** A alternativa (devolver o zero) é o que a issue está
   pedindo para acabar. O pânico é recuperado pelo runtime e vira 500 com a frase que nomeia o
   tipo — falha no lugar certo, uma vez, e não `nil` viajando.
7. **O exemplo é teste de integração (princípio VI).** Trocar o global do `posts` por um
   `*posts.Store` provido é o que prova a #55: a suíte passa a poder subir dois apps no mesmo
   processo.

## Fases

### Fase 1 — cabeçalhos e nonce do hospedeiro (#52)

`Security.Delegated bool` e `Security.Nonce func(*http.Request) string`; saída antecipada em
`applySecurity`; `Ctx.Nonce` consultando a função; `NonceAttr` devolvendo `h.Group()` quando o
nonce é vazio; aviso no log de subida quando `Delegated` está ligado.

### Fase 2 — nomes do CSRF (#54)

`type CSRF struct{ Cookie, Field, Header string }`, `Config.CSRF`, padrões em `applyConfig`,
leitura pelo `csrf.go`, pelo `cors.go` (cabeçalho permitido) e pelo cliente de teste.

### Fase 3 — `Provide` e `Use` (#55)

Genéricos sobre `a.values`, pânico com o nome do tipo, e o `examples/blog` deixando o estado de
pacote: `posts.Store` com as funções virando métodos, `Provide` no `Setup`, `Use` nas páginas.

### Fase 4 — documentação e release

`api/current.txt` regravado, docs nas duas locales, CHANGELOG 0.35.0, ROADMAP, `version`.

## Constitution Check

Sem desvio. Nenhuma convenção nova em `app/`, nenhuma dependência nova, nenhuma quebra da API
pública: os três itens são campos e funções novos, e as constantes do CSRF continuam onde
estão, agora como padrão.
