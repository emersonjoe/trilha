---
title: Dados e cache
description: De onde vem o dado, por quanto tempo vale guardar a resposta e o que a derruba — com cache.Do, tags e cache.Once.
---

O Trilha não tem ORM, não tem repositório e não tem opinião sobre o seu banco: a página
chama o seu código, e o seu código chama o que você usa. O que o framework traz é a parte
que sempre se escreve à mão e sempre se escreve errado — guardar uma resposta por um
tempo, e jogá-la fora quando ela deixa de ser verdade.

```go
import "github.com/emersonjoe/trilha/cache"
```

## O cache é seu, não do framework

Não existe `app.Cache()`. Você cria, você diz de que tamanho ele fica, e você o guarda
onde mora o código que o enche — em geral o pacote que consulta o banco:

```go
// internal/eventos/eventos.go
var Cache *cache.Cache

// app/setup.go
func Setup(a *trilha.App) error {
	eventos.Cache = cache.New(cache.Options{
		Name:       "eventos",
		MaxEntries: 500,
		Metrics:    a.Metrics(),
	})
	return nil
}
```

`MaxEntries` tem padrão (10 000) e não tem como dizer "sem limite". Cache sem teto é
vazamento de memória que leva uma semana para aparecer: toda chave que alguém consegue
inventar — um termo de busca, um filtro na query string — vira uma entrada que nunca sai.
Batendo no teto, a entrada usada há mais tempo é despejada.

## `Do`: o valor, ou o jeito de buscá-lo

`cache.Do` é o pacote inteiro em uma chamada. Ele devolve o que está guardado, ou executa
a sua função e guarda o que ela devolveu:

```go
func Proximos(ctx context.Context) ([]Evento, error) {
	return cache.Do(ctx, Cache, cache.Key{
		Name: "eventos:proximos",
		TTL:  5 * time.Minute,
		Tags: []string{"eventos"},
	}, func(ctx context.Context) ([]Evento, error) {
		return db.Proximos(ctx)
	})
}
```

| Campo da `Key` | O que é |
|---|---|
| `Name` | o endereço do valor; nomes iguais são a mesma entrada |
| `TTL` | por quanto tempo ele vale; `0` (ou menos) é sem prazo |
| `Tags` | rótulos para invalidar em lote depois |

O nome é uma decisão, não um detalhe: tudo o que muda a resposta tem de estar nele. Uma
lista que depende da página e de quem está logado é `posts:pagina:2:usuario:42`, e não
`posts` — chave de cache que esquece o usuário é como o dado de um vai parar na tela do
outro.

O erro volta para quem chamou e não é guardado para ninguém. A requisição seguinte tenta
de novo.

### Uma busca de cada vez

No instante em que uma chave quente vence, todas as requisições que a queriam chegam
juntas e todas vão ao banco. O `Do` não deixa: a primeira executa a função, as outras
esperam por ela e leem a mesma resposta. É uma busca por chave, não importa quantas
requisições estejam na fila atrás dela.

## O que derruba

Tempo é o jeito fraco de invalidar — cinco minutos de lista errada são cinco minutos de
alguém lendo um post que já foi apagado. O jeito forte é avisar:

```go
func Criar(ctx context.Context, e Evento) error {
	if err := db.Inserir(ctx, e); err != nil {
		return err
	}
	Cache.Invalidate("eventos")
	return nil
}
```

`Invalidate` derruba toda entrada que carrega aquela tag, seja qual for o nome, e devolve
quantas derrubou. `Delete(nomes...)` derruba por nome, e `Clear()` esvazia tudo.

Ponha a chamada ao lado da escrita, nunca ao lado da leitura. Um cache é invalidado por
quem mudou o dado — o código que lê não tem como saber que alguma coisa mudou.

## `Once` não é o cache

O layout quer saber quem está logado. O cabeçalho quer o mesmo. Dois componentes dentro
da página também. O `cache.Once` responde à pergunta uma vez por requisição:

```go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	usuario, err := cache.Once(c, "usuario", func() (*usuarios.Usuario, error) {
		return usuarios.Buscar(c.Context(), auth.From(c).Subject)
	})
	…
}
```

Nada guardado aqui sobrevive à resposta, e é exatamente esse o ponto. Ele não recebe
`*Cache`, nem TTL, nem tag, porque não há o que vencer: o valor morre com a requisição
que o criou. Use `Once` quando a alternativa é passar um valor por seis assinaturas de
função, e `Do` quando a resposta é a mesma para todo mundo.

Não troque um pelo outro. O nome de um usuário no `Do` sob a chave `"usuario"` é o nome
desse usuário servido para a próxima pessoa que abrir a página.

## Vendo funcionar

Com `Options.Metrics`, quatro séries aparecem no `/metrics`, rotuladas pelo nome do
cache:

```
trilha_cache_hits_total{cache="eventos"} 1043
trilha_cache_misses_total{cache="eventos"} 61
trilha_cache_evictions_total{cache="eventos"} 0
trilha_cache_entries{cache="eventos"} 61
```

Acertos sobre acertos mais erros é a taxa de acerto — abaixo de 50 % o TTL está curto
demais ou a chave carrega algo que não devia. Despejos subindo é teto baixo demais: o
cache está jogando fora justamente o que iam lhe pedir.

## O cache que o navegador guarda

O cache de cima poupa ao servidor uma ida ao banco. Este poupa à rede uma resposta inteira: o
navegador já tem a página e só pergunta se ela mudou.

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	p, ok := trilha.Use[*posts.Store](c).Get(c.Param("slug"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	c.CacheControl("private, no-cache")
	if c.ETag(p.Atualizado.UTC().Format(time.RFC3339Nano)) {
		return nil, nil // a cópia do navegador está em dia: 304, sem corpo
	}
	c.SetTitle(p.Title)
	return view(p), nil
}
```

`ETag` escreve a etiqueta e diz se a requisição já a trazia. Quando diz que sim, o `304` já foi
escrito: devolva `nil, nil` — um corpo ali seria jogado fora. `LastModified` faz o mesmo com uma
data, e `CacheControl` escreve o cabeçalho como você digitou. `no-cache` não quer dizer "não
guarde"; quer dizer "guarde, mas me pergunte antes de reusar", que é justamente o que provoca o
`304`.

A etiqueta é uma versão do dado, não um hash da página — e a Trilha não calcula uma por você.
Toda resposta carrega um nonce novo de CSP, então um hash do HTML nunca bateria duas vezes. Serve
qualquer coisa que se mexa quando o dado se mexe: `updated_at`, um número de revisão, os ids do
que foi renderizado.

> Uma etiqueta que esquece quem está lendo é o mesmo defeito de uma chave de cache que esquece o
> usuário. Se a página muda com o visitante, ponha isso na etiqueta ou não mande nenhuma.

Os arquivos em `static/` já fazem isso sozinhos: a impressão digital do `?v=` é a ETag deles, então
a segunda visita custa um `304` e nenhum byte.

## Desafio

A página de detalhe do evento vai ao banco a cada visita. Guarde-a por uma hora, com uma
tag que permita ao `Salvar` derrubar só aquele evento e outra que derrube a seção
inteira.

:::solution
Tags são uma lista, então uma entrada pode pertencer a mais de um grupo:

```go
func Buscar(ctx context.Context, slug string) (Evento, error) {
	return cache.Do(ctx, Cache, cache.Key{
		Name: "evento:" + slug,
		TTL:  time.Hour,
		Tags: []string{"eventos", "evento:" + slug},
	}, func(ctx context.Context) (Evento, error) {
		return db.Buscar(ctx, slug)
	})
}

func Salvar(ctx context.Context, e Evento) error {
	if err := db.Salvar(ctx, e); err != nil {
		return err
	}
	// O evento mudou: a página dele, e toda lista em que ele aparece.
	Cache.Invalidate("evento:"+e.Slug, "eventos")
	return nil
}
```
:::
