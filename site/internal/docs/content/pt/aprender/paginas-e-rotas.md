---
title: Páginas e rotas
description: Como pastas viram URLs, incluindo segmentos dinâmicos, catch-all e grupos.
---

Você já viu que `app/eventos/page.go` responde `/eventos`. Este capítulo cobre o resto do
mapeamento: parâmetros na URL, caminhos de tamanho variável e pastas que agrupam sem
aparecer na URL.

## Segmento dinâmico: `nome_`

Cada evento terá uma página própria em `/eventos/encontro-go`. Em vez de uma pasta por
evento, crie uma pasta cujo nome termina com `_`:

```text
app/eventos/slug_/page.go   →   GET /eventos/{slug}
```

Dentro da página, o valor vem de `c.Param`:

```go
package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	slug := c.Param("slug")
	c.SetTitle("Evento " + slug)
	return h.H1(h.Textf("Evento: %s", slug)), nil
}
```

O nome do parâmetro é o nome da pasta sem o `_`. Uma pasta `id_` dá `c.Param("id")`.

:::nota
Por que não `[slug]` como em outros frameworks? Porque a pasta vira um **pacote Go**, e o
caminho de import de um pacote não aceita colchetes, chaves nem cifrão. O sufixo `_` é
legal, aparece em `go list ./...` e não confunde o shell.
:::

## Catch-all: `nome__`

Duas barras no final capturam tudo o que vier depois, com as barras internas:

```text
app/docs/caminho__/page.go   →   GET /docs/{caminho...}
```

`GET /docs/guia/instalacao` chega com `c.Param("caminho") == "guia/instalacao"`. Uma
pasta catch-all precisa ser folha: nada pode existir abaixo dela.

## Quem vence quando há empate

Rotas literais vencem as dinâmicas. Com `app/eventos/novo/page.go` e
`app/eventos/slug_/page.go`, `/eventos/novo` vai para a primeira e `/eventos/qualquer-outra`
para a segunda. Duas pastas dinâmicas irmãs (`a_` e `b_` no mesmo nível) são um erro de
geração, porque não haveria como escolher.

## Grupos de rota: `nome-`

Às vezes você quer que várias páginas compartilhem um layout ou um middleware sem que isso
apareça na URL. Uma pasta terminada em `-` é um **grupo**:

```text
app/organizador-/middleware.go    ← vale para tudo abaixo
app/organizador-/painel/page.go   → GET /painel   (sem "organizador" na URL)
app/organizador-/eventos/page.go  → GET /eventos  ✗ conflita com app/eventos/page.go
```

O gerador recusa duas pastas que produzam a mesma URL (`E_DUPLICATE_ROUTE`), então o
segundo exemplo acima não compila.

## Deixando a CLI fazer a tradução

Nada acima precisa ser digitado à mão. O `trilha generate` recebe a URL e grava a pasta que a
convenção pede, já compilando:

```bash
trilha generate page /eventos/{slug}   # app/eventos/slug_/page.go
trilha generate route /api/eventos     # app/api/eventos/route.go
```

A página já vem com `c.Param("slug")` lido, e o `trilha_gen.go` é regerado no fim, então a URL
responde antes de você abrir o editor. As flags estão em
[CLI](/pt/referencia/cli#trilha-generate).

## O que o gerador faz com isso

Rode `trilha routes` a qualquer momento para ver a tabela:

```text
MÉTODOS   PADRÃO             ORIGEM
GET       /                  app/page.go
GET       /eventos           app/eventos/page.go
GET       /eventos/{slug}    app/eventos/slug_/page.go
GET       /painel            app/organizador-/painel/page.go
```

Essa tabela vira código Go em `trilha_gen.go`: um `a.Register(trilha.Route{...})` por
linha, importando cada pacote. Se você renomear `Page`, é o compilador quem reclama, não o
servidor em produção.

## Desafio

Crie a página de detalhe `app/eventos/slug_/page.go` que mostre o slug, e uma página
`app/eventos/hoje/page.go`. Confirme com `trilha routes` que `/eventos/hoje` aponta para a
pasta literal e não para a dinâmica.

:::solucao
As duas páginas seguem o formato de `Page`. A saída de `trilha routes` deve conter:

```text
GET  /eventos/hoje      app/eventos/hoje/page.go
GET  /eventos/{slug}    app/eventos/slug_/page.go
```

A ordem alfabética coloca `/eventos/hoje` antes, mas o que decide a precedência é o
roteador: literal antes de dinâmico, sempre.
:::
