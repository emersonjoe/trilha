---
title: Planilhas (CSV)
description: Exportar uma planilha que o Excel abre sem estragar, e ler uma de volta dizendo qual linha e qual coluna estão erradas.
---

Toda aplicação interna acaba fazendo isto duas vezes: um botão que baixa a lista e uma tela que
a recebe de volta. As duas falham nos mesmos poucos lugares. Na ida: sem BOM, e todo acento
abre como caractere estranho; vírgula onde o Excel de quem vai abrir espera ponto e vírgula, e
o arquivo abre como uma coluna só; data num formato que a planilha lê como texto. Na volta:
"erro no arquivo", que deixa alguém procurando uma data errada entre quatro mil linhas no olho.

`c.CSV` e `trilha.BindCSV` são essas duas metades.

## Exportando

```go
// Row is the spreadsheet and the form at once. The csv tag is the heading in
// the file; the validate tag is the same rule a screen would apply, written
// once and holding in both places.
type Row struct {
	Code   string  `csv:"code"   validate:"required,max=20"`
	Name   string  `csv:"name"   validate:"required"`
	Parent string  `csv:"parent"`
	Weight float64 `csv:"weight" validate:"min=0"`
	id     string  `csv:"-"` // never exported, never read
}

// ExportPlan sends the whole plan as a file the person can open. The BOM, the
// separator and the date format follow Config.Locale, which is the difference
// between a spreadsheet that opens in columns and one that opens as a single
// column of mojibake.
func ExportPlan(c *trilha.Ctx) error {
	rows, err := plan(c)
	if err != nil {
		return err
	}
	return c.CSV("classification-plan.csv", rows)
}
```

O cabeçalho de cada coluna é a tag `csv`, ou o nome do campo quando não há tag; `csv:"-"` deixa
o campo de fora, e campo não exportado nunca esteve. O que o locale decide:

| | `Locale: "en"` | `Locale: "pt-BR"` |
|---|---|---|
| separador | `,` | `;` |
| data | `2026-09-08 15:04` | `08/09/2026 15:04` |
| decimal | `1234.5` | `1234,5` |
| booleano | `yes` / `no` | `sim` / `não` |

Os dois começam com BOM UTF-8 e terminam as linhas com CRLF, que é o que a RFC 4180 diz e o que
o Excel no Windows lê.

:::note
`Config.TimeZone` é o fuso em que a data aparece. Sem ele todo horário é UTC — o que está
correto e também está três horas adiantado para quem vai ler.
:::

## Duzentas mil linhas

Passe um canal no lugar da fatia e o arquivo é escrito enquanto é produzido: a memória é de uma
linha, e o download começa antes de a consulta terminar.

```go
// StreamPlan is the same export when the answer is two hundred thousand rows:
// a channel is written as it is produced, so the memory is one row and the
// person sees the download start immediately.
//
// The goroutine has to end even when the download does not: the browser that
// closes the connection cancels the request, and a producer that never learns
// that is a goroutine leak with a database cursor attached to it.
func StreamPlan(c *trilha.Ctx) error {
	ch := make(chan Row)
	go func() {
		defer close(ch)
		for _, r := range everyRow(c) {
			select {
			case ch <- r:
			case <-c.Context().Done():
				return
			}
		}
	}()
	return c.CSV("classification-plan.csv", (<-chan Row)(ch))
}
```

O `c.CSV` já tira o prazo de escrita, então uma exportação longa num link ruim não é derrubada
como handler travado. O que ele não pode fazer por você é encerrar o produtor: o `select` no
`c.Context().Done()` acima é a razão inteira de este trecho ser maior que o anterior.

## Importando

```go
// ImportPlan reads the file back and says which cell is wrong. The separator
// and the BOM are detected, the header matches by tag in any order, and each
// row goes through the same validate tags a form would.
func ImportPlan(c *trilha.Ctx) error {
	up, err := c.File("file", trilha.FileRules{
		Accept:  []string{"text/csv", "text/plain"},
		MaxSize: 5 << 20,
	})
	if err != nil {
		return err
	}
	defer up.Close()

	var rows []Row
	res, err := trilha.BindCSV(up, &rows)
	if err != nil {
		// A file that is not a file at all — empty, or unreadable. It belongs
		// on the field the person used, not in a 500.
		return trilha.FieldErrors{"file": err.Error()}
	}
	if !res.OK() {
		return c.Render(http.StatusUnprocessableEntity, ui.CSVErrors(c, res, ui.CSVErrorsOpts{
			Action: ui.ButtonLink("/import", h.Text("Choose another file")),
		}))
	}
	if err := save(c, rows); err != nil {
		return err
	}
	return c.Redirect("/import/done")
}
```

O que o `BindCSV` decide por você, e por quê:

- **O separador e o BOM são detectados**, então um handler só atende o arquivo que o Excel
  escreveu aqui e o que um script escreveu em qualquer outro lugar.
- **O cabeçalho casa pela tag**, ignorando maiúsculas e espaços em volta, em qualquer ordem.
  Quem mudou uma coluna de lugar não errou.
- **Coluna que nenhum campo reivindica é aviso**, não erro, e vai para `res.Warnings`. Planilha
  ganha coluna o tempo todo; recusar o arquivo por isso só ensina a apagar colunas antes de
  subir.
- **Coluna obrigatória faltando é uma mensagem na linha 1**, não a mesma mensagem em cada uma
  de dez mil linhas.
- **Só as linhas que passam entram na fatia.** Depois do `res.OK()` ela é o arquivo inteiro;
  depois de uma falha ela não tem nada que valha salvar.
- **Linha em branco é pulada**, e o `MaxRows` (100 mil por padrão) é erro em vez de corte —
  metade de uma importação dizendo que deu certo é pior que uma que falha.
- **A data é lida em `dd/mm/aaaa` além do ISO**, porque é o que a planilha brasileira exporta.

## A tela do erro

`res.Errors` é `[]trilha.CSVError{Line, Column, Message}` — a linha como o editor da pessoa
numera, com o cabeçalho como linha 1, e a coluna pelo cabeçalho que está no arquivo. Dá para
procurar pelas duas metades, que é justamente o ponto.

O `ui.CSVErrors` desenha isso como uma tabela com os vinte primeiros:

```go
// ShowCSVErrors is the shortest form of the error screen: the default table,
// the first twenty cells, no button.
func ShowCSVErrors(c *trilha.Ctx, res trilha.CSVResult) error {
	return c.Render(http.StatusUnprocessableEntity, ui.CSVErrors(c, res))
}
```

| Linha | Coluna | Problema |
|---|---|---|
| 4 | `code` | precisa ter no máximo 20 caracteres |
| 4 | `name` | obrigatório |
| 17 | `weight` | precisa ser 0 ou mais |

As mensagens são as da validação, então o `UseValidationPTBR` traduz a importação junto com
todos os formulários.

## De onde vem o código

De [`examples/cookbook/csv.go`](https://github.com/emersonjoe/trilha/blob/main/examples/cookbook/csv.go),
e a ida e a volta são exercitadas de ponta a ponta no exemplo do blog, em
[`app/documentos/planilha`](https://github.com/emersonjoe/trilha/blob/main/examples/blog/app/documentos/planilha/route.go):
o arquivo que o `GET` escreve é o arquivo que o `POST` aceita de volta.
