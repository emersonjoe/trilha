// Package anexos shows an upload with progress that still works without
// JavaScript, and a route that raises the body limit only for itself.
package anexos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/anexos"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /anexos — the whole page, or just the list when the client
// asks for the fragment.
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "lista" {
		return lista(), nil
	}
	c.SetTitle("Anexos")
	return ui.Card(
		ui.CardHeader(
			h.H1(h.Class("ui-card-title"), h.Text("Anexos")),
			ui.CardDescription("O arquivo sobe com barra de progresso; sem JavaScript, o formulário envia como sempre."),
		),
		ui.CardContent(
			// O formulário é um formulário: action, método e enctype de
			// sempre. ui.UploadTo diz qual pedaço a resposta troca, e o
			// script só entra se o navegador o executar.
			h.Form(h.Method("post"), h.Action("/anexos"), h.Enctype("multipart/form-data"),
				h.Class("ui-stack"), ui.UploadTo("lista"),
				trilha.CSRFInput(c),
				ui.Field("arquivo", "Arquivo", ui.Input(h.ID("arquivo"), h.Name("arquivo"), h.Type("file"), h.Required()),
					ui.Help("Até 8 MB — o limite desta rota, não o do app.")),
				ui.UploadBar(),
				ui.Submit(h.Text("Enviar")),
			),
			lista(),
			ui.UploadScript(c),
		),
	), nil
}

// POST receives the file. Middleware already raised the limit for this route;
// here it only answers the piece or the whole page, depending on who asked.
func POST(c *trilha.Ctx) error {
	if err := c.FormErr(); err != nil {
		return err
	}
	f, hdr, err := c.Request().FormFile("arquivo")
	if err != nil {
		return err
	}
	defer f.Close()
	anexos.Add(hdr.Filename, hdr.Size)
	if c.Fragment() != "" {
		return c.Render(200, lista())
	}
	return c.Redirect("/anexos")
}

// lista is the swapped piece: it carries the id the form points at.
func lista() h.Node {
	itens := anexos.All()
	if len(itens) == 0 {
		return h.Ul(h.ID("lista"), h.Class("ui-list"), h.Li(h.Class("ui-muted"), h.Text("Nada enviado ainda.")))
	}
	rows := make([]h.Node, 0, len(itens))
	for _, a := range itens {
		rows = append(rows, h.Li(
			h.Strong(h.Text(a.Nome)),
			h.Text(" — "),
			h.Span(h.Class("ui-muted"), h.Text(a.Tamanho())),
		))
	}
	return h.Ul(append([]h.Node{h.ID("lista"), h.Class("ui-list")}, rows...)...)
}
