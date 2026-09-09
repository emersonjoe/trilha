package ui

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Section is what SettingsForm needs of a trilha.Settings — its form, its
// current values and the address it posts to. It is an interface so that the
// kit does not have to name the generic type of somebody else's configuration.
type Section interface {
	Schema() trilha.Schema
	Values() map[string]string
	Key() string
}

// SettingsForm is the administration screen of one section, drawn from the
// struct that declares it.
//
//	// app/admin/pipeline/page.go
//	func Page(c *trilha.Ctx) (h.Node, error) { return ui.SettingsForm(c, config.Cfg, nil), nil }
//	func POST(c *trilha.Ctx) error           { return config.Cfg.Update(c) }
//
// One field per field, of the type the tags asked for: a select where there is
// a oneof, a number with its range, a checkbox for a bool. It is the same
// ui.SchemaForm every data-defined form uses, so the label, the help and the
// message say the same things here as anywhere else.
//
//	see: trilha.Settings, trilha.SchemaOf, ui.SchemaForm
func SettingsForm(c *trilha.Ctx, s Section, errs map[string]string, children ...h.Node) h.Node {
	body := append([]h.Node{
		h.Class("ui-stack"), h.Method("post"), trilha.CSRFInput(c),
		SchemaForm(s.Schema(), s.Values(), errs),
	}, children...)
	if len(children) == 0 {
		body = append(body, Submit(h.Text(word(langOf(c) == "pt-BR", "Save", "Salvar"))))
	}
	return h.Form(body...)
}
