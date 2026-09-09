// Package assistentepassos is what the three screens of the wizard share: the
// indicator and the one field helper, so no screen invents its own.
package assistentepassos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Passos is the list, in order. It lives here so the three screens cannot
// disagree about how many steps there are or what they are called.
var Passos = []ui.Step{
	{Label: "Dados", Href: "/assistente/dados"},
	{Label: "Endereço", Href: "/assistente/endereco"},
	{Label: "Revisão"},
}

// Campo is label + input bound to the model and to the error map, the same
// helper the one-page form of this example uses.
func Campo(id, label, value string, errs trilha.FieldErrors, attrs ...h.Node) h.Node {
	return ui.Field(id, label, ui.Input(append([]h.Node{h.ID(id), h.Name(id), h.Value(value), ui.InvalidIf(errs, id)}, attrs...)...), ui.Errors(errs, id))
}
