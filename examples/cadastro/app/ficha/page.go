// Package ficha shows a form the product defined at runtime: the fields come
// as JSON — from a table, from the settings of a tenant, from the step of a
// workflow — and there is no Go struct to fill.
package ficha

import (
	"encoding/json"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// esquemaJSON is what a row of the database would hold. trilha.Schema decodes
// straight from it, so changing the form is changing the data.
const esquemaJSON = `[
  {"type": "display", "text": "Preencha com o que o cliente relatou no atendimento."},
  {"name": "assunto", "label": "Assunto", "type": "text", "required": true, "min": "3", "max": "60"},
  {"name": "canal", "label": "Canal", "type": "select", "required": true, "options": [
    {"value": "telefone", "label": "Telefone"},
    {"value": "email", "label": "E-mail"},
    {"value": "loja", "label": "Loja"}
  ]},
  {"name": "quando", "label": "Quando aconteceu", "type": "date", "required": true},
  {"name": "nota", "label": "Nota do atendimento", "type": "number", "min": "0", "max": "10", "help": "De 0 a 10."},
  {"name": "relato", "label": "Relato", "type": "textarea", "rows": 5, "required": true, "min": "10"},
  {"name": "protocolo", "label": "Protocolo", "type": "text", "pattern": "^[0-9]{4}-[0-9]{4}$", "help": "No formato 0000-0000."},
  {"name": "retornar", "label": "Precisa de retorno", "type": "checkbox"}
]`

var esquema = carregar()

func carregar() trilha.Schema {
	var s trilha.Schema
	if err := json.Unmarshal([]byte(esquemaJSON), &s); err != nil {
		panic(err)
	}
	return s
}

// Page draws the schema, empty.
func Page(c *trilha.Ctx) (h.Node, error) { return tela(c, nil, nil, false), nil }

// POST reads it with BindSchema: the same validation and the same FieldErrors
// a tagged struct gets, keyed by the field's own name.
func POST(c *trilha.Ctx) error {
	values, err := trilha.BindSchema(c, esquema)
	if errs, ok := err.(trilha.FieldErrors); ok {
		return c.Render(http.StatusUnprocessableEntity, tela(c, values, errs, false))
	}
	if err != nil {
		return err
	}
	return c.Render(http.StatusOK, tela(c, values, nil, true))
}

func tela(c *trilha.Ctx, values map[string]string, errs trilha.FieldErrors, salvo bool) h.Node {
	c.SetTitle("Ficha de atendimento")
	return ui.Card(
		ui.CardHeader(h.H1(h.Class("ui-card-title"), h.Text("Ficha de atendimento")),
			ui.CardDescription("Os campos vêm de um JSON: mudar o formulário é mudar o dado, não o código.")),
		ui.CardContent(
			h.If(salvo, ui.Alert("Ficha recebida!", ui.Icon("circle-check"))),
			h.Form(h.Method("post"), h.Action("/ficha"), h.Class("ui-stack"), h.Attr("novalidate", ""), trilha.CSRFInput(c),
				h.If(errs.Any(), ui.Alert("Corrija os campos destacados", ui.Destructive(), ui.Icon("triangle-alert"))),
				ui.SchemaForm(esquema, values, errs),
				ui.Row(ui.Submit(h.Text("Enviar ficha"))),
			),
		),
	)
}
