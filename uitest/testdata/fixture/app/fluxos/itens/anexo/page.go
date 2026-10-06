// Package anexo is an upload inside the region whose route redirects: the
// upload follows like a ui.Swap form (#291).
package anexo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"example.com/uitest/internal/itens"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Attach")
	return h.Div(ui.NavigateScript(c), ui.UploadScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("Attach")),
			h.Form(h.ID("envio"), h.Method("post"), h.Action("/fluxos/itens/anexo"), h.Enctype("multipart/form-data"),
				ui.UploadTo("envio"), trilha.CSRFInput(c),
				h.Input(h.ID("arquivo"), h.Type("file"), h.Name("arquivo")),
				h.Button(h.Type("submit"), h.Text("Send"))))), nil
}

func POST(c *trilha.Ctx) error {
	f, hdr, err := c.Request().FormFile("arquivo")
	if err != nil {
		return err
	}
	f.Close()
	itens.Add(hdr.Filename)
	c.Flash("success", "Attached: "+hdr.Filename)
	return c.Redirect("/fluxos/itens")
}
