---
title: Charts
description: ui.Stat, ui.Bars, ui.Sparkline and ui.Donut draw a dashboard on the server, in SVG, with no JavaScript.
---

A dashboard is mostly four drawings, and none of them needs a charting library. These
four render on the server as SVG: they arrive with the page, they print, they work with
JavaScript off, and they cost nothing to download.

They are deliberately small. Axes, zoom, tooltips and brushing are not here — that is an
island with a real library, and the framework does not stand in the way of one.

See it live: [demo](/learn/ui-kit#four-numbers-and-the-drawings-beside-them).

## The number arrives formatted

`ui.Datum` carries `Label`, `Value` and `Text`. `Value` is what the drawing measures;
`Text` is what a person reads. The framework has no locale, so money and percentages are
formatted by the app:

```go
data := []ui.Datum{
	{Label: "Rent", Value: 3200, Text: "R$ 3.200,00"},
	{Label: "Food", Value: 1450.5, Text: "R$ 1.450,50"},
}
```

With `Text` empty the value is written as a plain number, rounded to two decimals.

## Stat

The big number of a card, with its label and an optional hint:

```go
ui.Card(ui.Stat("Open", "12", ui.StatHint("3 more than last week")))
```

`Stat` is text, not a drawing: it is `<div>`s and it inherits the font of the page.

## Bars, Sparkline, Donut

```go
ui.Bars(data, ui.ChartTitle("Spending by category"))
ui.Sparkline([]float64{4, 9, 7, 12, 11, 15}, ui.SparkOpts{})
ui.Donut(data, ui.ChartTitle("Share of the month"))
```

`SparkOpts{Width, Height}` defaults to 120×32 — a sparkline belongs inside a line of
text, not on a screen of its own. Colours come from the theme (`--chart-1` to
`--chart-5`, cycling), so a chart changes with the rest of the page and there is no
palette to keep in sync.

## What a screen reader gets

Every chart renders an invisible `<table class="ui-sr">` with the same numbers. That
table is the accessible version, which is why the SVG itself is `aria-hidden="true"` —
with one exception: pass `ui.ChartTitle`, and the SVG becomes `role="img"` with a
`<title>`, because now it has a name worth announcing.

`ui.SparklineTitle` is the sparkline's version of the same thing.

## A series that does not cooperate

An empty series, a series of zeros and a single point all have a defined drawing: the
frame is rendered, the numbers table is rendered, and nothing divides by zero. A
dashboard on the first day of the month is a normal screen, not a stack trace.
