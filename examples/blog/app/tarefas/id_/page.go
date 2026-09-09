// Package id_ is the screen that watches one task: GET /tarefas/{id}.
//
// It is the whole point of the module in one page. The fragment asks for
// itself every two seconds while the task is alive and stops the moment it
// ends, and without JavaScript it is the state as of the load — old, never
// broken, and reloading is the refresh.
package id_

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/task"
	"github.com/emersonjoe/trilha/ui"
)

// Page draws the progress, and only the progress when asked for the fragment.
func Page(c *trilha.Ctx) (h.Node, error) {
	motor := trilha.Use[*task.Tasks](c)
	id := c.Param("id")
	if c.Fragment() == "trilha-task" {
		return ui.TaskProgress(c, motor, id), nil
	}
	t, err := motor.Get(c.Context(), id)
	if err != nil {
		return nil, trilha.Errorf(http.StatusNotFound, "tarefa não encontrada")
	}
	c.SetTitle("Processando " + t.Key)
	return h.Div(
		ui.PageHeader("Processando "+t.Key, ui.Back{Href: "/tarefas", Label: "Tarefas"}),
		ui.Muted(h.Text("Esta página se atualiza sozinha e para quando a tarefa termina.")),
		ui.Card(ui.CardContent(ui.TaskProgress(c, motor, id))),
		h.P(h.A(h.Href("/tarefas"), h.Text("Voltar para as tarefas"))),
		ui.LiveScript(c),
	), nil
}
