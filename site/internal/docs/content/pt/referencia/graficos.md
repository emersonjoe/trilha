---
title: Gráficos
description: ui.Stat, ui.Bars, ui.Sparkline e ui.Donut desenham um painel no servidor, em SVG, sem JavaScript.
---

Um painel é quase sempre quatro desenhos, e nenhum deles precisa de biblioteca de
gráficos. Estes quatro são desenhados no servidor, em SVG: chegam com a página, imprimem,
funcionam com o JavaScript desligado e não custam download nenhum.

São pequenos de propósito. Eixo, zoom, tooltip e seleção de faixa não estão aqui — isso é
uma ilha com uma biblioteca de verdade, e o framework não atrapalha quem quiser uma.

Veja funcionando: [demo](/pt/aprender/interface-com-ui#quatro-numeros-e-os-desenhos-ao-lado).

## O número chega formatado

`ui.Datum` carrega `Label`, `Value` e `Text`. `Value` é o que o desenho mede; `Text` é o
que uma pessoa lê. O framework não tem locale, então dinheiro e porcentagem são
formatados pelo app:

```go
data := []ui.Datum{
	{Label: "Aluguel", Value: 3200, Text: "R$ 3.200,00"},
	{Label: "Mercado", Value: 1450.5, Text: "R$ 1.450,50"},
}
```

Com `Text` vazio o valor sai como número puro, arredondado em duas casas.

## Stat

O número grande de um card, com o rótulo e uma dica opcional:

```go
ui.Card(ui.Stat("Abertos", "12", ui.StatHint("3 a mais que na semana passada")))
```

`Stat` é texto, não desenho: são `<div>`s, e herda a fonte da página.

## Bars, Sparkline, Donut

```go
ui.Bars(data, ui.ChartTitle("Gasto por categoria"))
ui.Sparkline([]float64{4, 9, 7, 12, 11, 15}, ui.SparkOpts{})
ui.Donut(data, ui.ChartTitle("Fatia do mês"))
```

`SparkOpts{Width, Height}` vale 120×32 por padrão — sparkline é para caber dentro de uma
linha de texto, não para ocupar uma tela. As cores vêm do tema (`--chart-1` a
`--chart-5`, ciclando), então o gráfico muda junto com o resto da página e não há paleta
para manter em dia.

## O que um leitor de tela recebe

Todo gráfico desenha também uma `<table class="ui-sr">` invisível com os mesmos números.
Essa tabela é a versão acessível, e é por isso que o SVG em si é `aria-hidden="true"` —
com uma exceção: passe `ui.ChartTitle` e o SVG vira `role="img"` com um `<title>`, porque
aí ele tem um nome que vale anunciar.

`ui.SparklineTitle` é a versão disso para o sparkline.

## Uma série que não colabora

Série vazia, série de zeros e série de um ponto só têm desenho definido: a moldura sai, a
tabela de números sai e nada divide por zero. Painel no primeiro dia do mês é uma tela
normal, não um stack trace.
