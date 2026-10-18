// Package combo has one combobox with a short list, filtered in the browser.
// Its behavior is ui.combo.js, which ui.js loads only on a page that has one
// (spec 177).
package combo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("City")),
		ui.Combobox(ui.ComboboxOpts{Name: "cidade", Options: []ui.Option{
			{Value: "poa", Label: "Porto Alegre"},
			{Value: "pal", Label: "Palmas"},
			{Value: "rec", Label: "Recife"},
		}})), nil
}
