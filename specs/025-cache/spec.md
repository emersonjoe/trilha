# Spec 025 — Cache com TTL, tags e invalidação explícita

- **Issue**: [#25](https://github.com/emersonjoe/trilha/issues/25) (ROADMAP, Fase 2, item 6)
- **Branch**: `025-cache`
- **Versão**: 0.16.0

## Por quê

Toda aplicação que o Trilha serve busca dado, e hoje cada projeto inventa o próprio cache —
um `map` com `sync.RWMutex` aqui, um `time.AfterFunc` ali, e ninguém sabe dizer quando o
dado sai. Sem invalidação explícita, "revalidação" é conversa: dá para guardar, não dá para
soltar na hora certa. É o que abre a Fase 2, porque cache HTTP (#26) só faz sentido em cima
de um cache de aplicação em que se confia.

O problema não é guardar. É responder três perguntas no mesmo lugar: **onde o dado é
buscado**, **por quanto tempo vale** e **o que o derruba**.

## O que muda

Um pacote novo, `cache`, que importa o `trilha` como o `auth` já faz — o runtime não sabe
que ele existe.

```go
var c = cache.New(cache.Options{Name: "app", Metrics: app.Metrics()})

// A chave carrega as três respostas: nome, validade e o que a derruba.
posts, err := cache.Do(ctx, c, cache.Key{Name: "posts:all", TTL: time.Minute, Tags: []string{"posts"}},
	func(ctx context.Context) ([]Post, error) { return db.All(ctx) })

// Gravou? Derruba quem depende disso, por tag, agora.
cache.Invalidate("posts")   // método: c.Invalidate("posts")
```

E a deduplicação, que é outra coisa e por isso tem outro nome:

```go
// O layout e a página perguntam quem é o usuário. Uma busca por requisição —
// e nada disso atravessa para a requisição seguinte.
u, err := cache.Once(c, "usuario", func() (*Usuario, error) { return usuarios.De(c) })
```

A diferença entre as duas está no meio do caminho e é o que a documentação precisa deixar
claro: **`Do` guarda entre requisições** (com TTL e tag, e um único `fn` em voo por chave,
mesmo sob concorrência); **`Once` guarda dentro de uma requisição** e morre com ela — é o
lugar do dado que não pode envelhecer nem um segundo, mas também não precisa ser buscado
duas vezes na mesma resposta.

### Superfície

| Símbolo | Papel |
|---|---|
| `cache.New(Options) *Cache` | `Name` (rótulo da métrica), `MaxEntries` (padrão 10 000), `Metrics` (opcional) |
| `c.Set(Key, v any)` | grava; `Key{Name, TTL, Tags}` |
| `c.Get(name) (any, bool)` | lê; expirado é ausente |
| `c.Delete(names ...string) int` | tira por nome |
| `c.Invalidate(tags ...string) int` | tira tudo que carrega qualquer uma das tags; devolve quantas entradas caíram |
| `c.Clear()` / `c.Len() int` / `c.Stats() Stats` | esvazia, conta, e os números de acerto/erro/despejo |
| `cache.Get[T](c, name) (T, bool)` | leitura tipada; tipo errado é ausência, não pânico |
| `cache.Do[T](ctx, c, Key, fn) (T, error)` | lê ou busca; erro não é guardado |
| `cache.Once[T](c *trilha.Ctx, name, fn) (T, error)` | uma busca por requisição |

## Decisões

- **`Do` tem voo único por chave.** Duas goroutines pedindo a mesma chave fria fazem **uma**
  busca; a segunda espera e recebe o mesmo resultado. Sem isso, o primeiro pico depois de um
  `Invalidate` vira uma estampida no banco, e a promessa de "cache em que se confia" seria
  falsa justamente na hora em que ela importa. São ~30 linhas, e é propriedade documentada
  do `Do`, não mágica em runtime.
- **`TTL <= 0` é sem prazo.** A entrada vive até uma tag derrubá-la ou o teto despejá-la. É o
  modo natural de um cache cuja invalidação é explícita; prazo e tag são independentes.
- **Sem goroutine de limpeza.** Entrada vencida sai quando é lida ou quando o teto aperta.
  Nada de `Close()`, nada de tarefa de fundo que o app precisa lembrar de parar.
- **Teto obrigatório, despejo por LRU** (`container/list`). Um cache sem teto é um vazamento
  com nome bonito (NIST SP 800-53 SC-5, o mesmo motivo do `MaxSeries` das métricas).
- **Erro não entra no cache.** `Do` que falha não guarda nada: a próxima chamada tenta de
  novo. Cache de erro é decisão de quem chama, não do cache.
- **O pacote importa o `trilha`, e não o contrário.** Métrica sai por `Options.Metrics`,
  `Once` recebe `*trilha.Ctx`. O runtime continua sem saber que cache existe — quem não usa
  não paga nem um campo no `App`.

## Requisitos

- **FR-001** `cache.New` devolve um cache pronto para uso concorrente; o zero de `Options`
  vale (nome `default`, teto 10 000, sem métrica).
- **FR-002** `Set`/`Get` respeitam o TTL: lida depois do prazo, a entrada é ausente e conta
  como erro de cache.
- **FR-003** `Invalidate(tags...)` derruba toda entrada que carrega qualquer uma das tags,
  devolve a contagem e não toca no resto. Tag desconhecida devolve 0.
- **FR-004** Uma entrada regravada troca as tags antigas pelas novas; nenhuma tag fica
  apontando para chave que não existe mais (`Len` das tags volta a zero depois de um `Clear`).
- **FR-005** O teto vale: passando de `MaxEntries`, sai a entrada usada há mais tempo, e
  `Len()` nunca passa do teto.
- **FR-006** `cache.Get[T]` com o tipo errado devolve `(zero, false)`; não entra em pânico.
- **FR-007** `cache.Do` chama `fn` uma vez por chave fria, mesmo com N goroutines em cima
  (verificado com `-race` e contador atômico); erro volta para todas e não é guardado.
- **FR-008** `cache.Once` chama `fn` uma vez por requisição; duas requisições diferentes
  chamam duas vezes; o valor não vaza de uma para a outra.
- **FR-009** Com `Options.Metrics`, o registro ganha `trilha_cache_hits_total`,
  `trilha_cache_misses_total`, `trilha_cache_evictions_total` e `trilha_cache_entries`,
  todos com o rótulo `cache`.
- **FR-010** Zero dependência externa; `go test -race` limpo.

## Fora de escopo

- **Cache HTTP (`ETag`, `Last-Modified`, `304`)** — é a #26, e é outra camada: esta guarda
  valor em memória, aquela negocia com o navegador.
- **Cache distribuído (Redis, memcached).** Levaria dependência externa para o núcleo. O que
  esta spec deixa pronto é o formato: quem precisar de rede escreve o próprio `Cache` com a
  mesma superfície e troca o valor no `Setup`.
- **Cache de página inteira / `revalidate` na rota.** Guardar resposta pronta mexe com
  layout, CSP com nonce e cookie de sessão — cada um deles é um jeito de servir a página de
  um usuário para outro. Fica para depois da #26, com o cabeçalho `Vary` no meio.
- **Persistência em disco, compressão, `Peek`, `GetOrSet` sem tipo.** Superfície pequena
  primeiro.
- **Métrica de tempo de busca.** `fn` é do app; quem quer medir mede lá.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — SSR primeiro | cache é do servidor; nada muda no HTML |
| II — só biblioteca padrão | `container/list`, `sync`, `time`; nada mais |
| III — coerência com Go | genérico onde o tipo importa, `context.Context` no `fn`, erro explícito |
| IV — convenção nova tem teste, uso no exemplo e integração | `examples/blog` passa a servir a lista de posts pelo cache e a derrubá-la ao publicar |
| VI — teste primeiro | `cache/cache_test.go` vermelho antes de `cache.go` |
| VII — compatibilidade | pacote novo; nada existente muda de assinatura |

## Aceitação

- **SC-001** Um `Do` com TTL de um minuto busca uma vez e devolve o guardado na segunda
  chamada; passado o prazo, busca de novo.
- **SC-002** `Invalidate("posts")` derruba as entradas com a tag e deixa as outras de pé,
  e o `Do` seguinte busca de novo.
- **SC-003** 50 goroutines em cima da mesma chave fria produzem **uma** chamada de `fn`, com
  `-race` limpo.
- **SC-004** No `examples/blog`, publicar um post muda a lista na resposta seguinte — sem
  esperar o TTL, porque o `Create` invalida a tag.
- **SC-005** `/metrics` do exemplo mostra `trilha_cache_hits_total{cache="blog"}` maior que
  zero depois de duas visitas à lista.
