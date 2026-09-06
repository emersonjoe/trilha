---
title: cache
description: Options, Key, Cache, Do, Get e Once — a API do pacote cache, com os padrões e o que cada campo muda.
---

`import "github.com/emersonjoe/trilha/cache"` — cache em memória com prazo, tags e
invalidação em lote, mais um memo por requisição. O pacote importa o runtime; o runtime
não importa ele, então um app que nunca o menciona não carrega nada dele.

## Criando

```go
func New(o Options) *Cache
```

| Campo de `Options` | Padrão | O que faz |
|---|---|---|
| `Name string` | `"cache"` | rótulo das séries de métrica; dê um nome a cada cache |
| `MaxEntries int` | `10000` | teto; a entrada usada há mais tempo é despejada |
| `Metrics *trilha.Metrics` | `nil` | registro onde publicar, em geral `a.Metrics()` |

Não existe `Close`: nada roda em segundo plano. Uma entrada vencida sai quando é lida ou
quando o teto a empurra, então um cache em que ninguém toca não custa nada além da
memória que já ocupa.

## Chaves

```go
type Key struct {
	Name string
	TTL  time.Duration
	Tags []string
}
```

`Name` é o endereço: nomes iguais são a mesma entrada, e tudo o que muda a resposta tem
de estar nele. `TTL` zero ou menos é sem prazo. `Tags` agrupam entradas para o
`Invalidate`; regravar uma entrada troca as tags dela, não as soma.

## Lendo e escrevendo

```go
func (c *Cache) Set(k Key, v any)
func (c *Cache) Get(name string) (any, bool)
func (c *Cache) Delete(names ...string) int
func (c *Cache) Invalidate(tags ...string) int
func (c *Cache) Clear()
func (c *Cache) Len() int
func (c *Cache) Stats() Stats
```

`Delete` e `Invalidate` devolvem quantas entradas removeram. `Stats` traz `Hits`,
`Misses`, `Evictions` e `Entries` — os mesmos quatro números das métricas, para uma
página de saúde ou um teste.

Todo método é seguro a partir de qualquer goroutine.

## Acesso tipado

Go não permite parâmetro de tipo em método, então a metade tipada do pacote são funções
de pacote:

```go
func Get[T any](c *Cache, name string) (T, bool)
func Do[T any](ctx context.Context, c *Cache, k Key, fn func(context.Context) (T, error)) (T, error)
```

`Get[T]` devolve o valor só quando o tipo guardado bate; valor escrito sob outro tipo é
ausência, não pânico — um deploy que mudou uma struct não pode derrubar o app.

`Do` devolve o valor guardado ou o produz com `fn`, guardando o resultado sob a `k`. O
erro volta para quem chamou e não é guardado para ninguém. Só um `fn` roda por nome de
cada vez: quem chega durante uma busca espera por ela e lê a mesma resposta, então a
primeira requisição depois de um `Invalidate` não vira tropel. O mutex do cache não fica
seguro durante o `fn`, então um `Do` dentro de outro funciona.

## Por requisição

```go
func Once[T any](c *trilha.Ctx, name string, fn func() (T, error)) (T, error)
```

Responde a uma pergunta uma vez por requisição e a esquece com a resposta. Não é cache e
não recebe `*Cache`: use para o que o layout, a página e três componentes precisam saber
— quem está logado, acima de tudo — em vez de passar o valor por todas as assinaturas. O
erro também fica guardado, então uma busca que falhou é tentada uma vez.

Guardar no `*Cache`, sob um nome fixo, um valor que é de um usuário serve esse valor para
o próximo visitante; o `Once` é o que não consegue fazer isso.

## Métricas

Com `Options.Metrics` preenchido, quatro séries aparecem na exposição, rotuladas com
`cache` no valor de `Options.Name`:

| Série | Tipo | Significado |
|---|---|---|
| `trilha_cache_hits_total` | contador | leituras respondidas da memória |
| `trilha_cache_misses_total` | contador | leituras que não acharam ou acharam vencido |
| `trilha_cache_evictions_total` | contador | entradas derrubadas pelo teto |
| `trilha_cache_entries` | medidor | entradas guardadas neste instante |

Despejos subindo sem parar é `MaxEntries` baixo demais para o espaço de chaves em uso.
