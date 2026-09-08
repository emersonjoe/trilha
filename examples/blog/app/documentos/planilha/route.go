// Package planilha é a planilha dos dois lados: o GET baixa a lista no formato
// que o Excel abre, e o POST recebe o mesmo arquivo de volta dizendo qual linha
// e qual coluna estão erradas.
package planilha

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/documentos"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// linha é a planilha e o formulário ao mesmo tempo: a tag csv é o cabeçalho no
// arquivo, e a tag validate é a mesma regra que valeria numa tela — escrita uma
// vez, ela vale nos dois lugares.
type linha struct {
	Nome   string `csv:"Documento" validate:"required,max=80"`
	Tipo   string `csv:"Tipo"      validate:"required,oneof=nota recibo contrato"`
	Bytes  int    `csv:"Bytes"     validate:"min=0"`
	Status string `csv:"Status"`
}

// GET /documentos/planilha baixa a lista inteira.
func GET(c *trilha.Ctx) error {
	docs, _ := documentos.Buscar(documentos.Consulta{Limite: 1000})
	linhas := make([]linha, 0, len(docs))
	for _, d := range docs {
		linhas = append(linhas, linha{Nome: d.Nome, Tipo: d.Tipo, Bytes: d.Bytes, Status: d.Status})
	}
	return c.CSV("documentos.csv", linhas)
}

// POST recebe a planilha editada. Arquivo ilegível é uma mensagem no campo;
// célula inválida é uma lista de linha e coluna, que é o que a pessoa precisa
// para achar o erro numa planilha de quatro mil linhas.
func POST(c *trilha.Ctx) error {
	up, err := c.File("arquivo", trilha.FileRules{
		Accept:  []string{"text/csv", "text/plain"},
		MaxSize: 5 << 20,
	})
	if err != nil {
		return err
	}
	defer up.Close()

	var linhas []linha
	res, err := trilha.BindCSV(up, &linhas)
	if err != nil {
		return trilha.FieldErrors{"arquivo": err.Error()}
	}
	if !res.OK() {
		return c.Render(http.StatusUnprocessableEntity, ui.CSVErrors(c, res, ui.CSVErrorsOpts{
			Action: ui.ButtonLink("/documentos", h.Text("Voltar aos documentos")),
		}))
	}

	novos := make([]documentos.Documento, 0, len(linhas))
	for _, l := range linhas {
		novos = append(novos, documentos.Documento{Nome: l.Nome, Tipo: l.Tipo, Bytes: l.Bytes, Status: l.Status})
	}
	documentos.Importar(novos)
	return c.Redirect("/documentos")
}
