// Package tentar is where the "try again" button of ui.TaskTable posts.
//
// It is a route of its own because retrying is a POST and the listing is a
// GET, and putting both on /tarefas would make the retry compete with the
// button that starts a new one — two forms, one address, and whichever the
// browser submits.
package tentar

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/task"
)

// POST runs the task again and goes to the new run's screen.
func POST(c *trilha.Ctx) error {
	motor := trilha.Use[*task.Tasks](c)
	novo, err := motor.Retry(c, c.Form("id"))
	if err != nil {
		return trilha.Errorf(http.StatusNotFound, "tarefa não encontrada")
	}
	c.Flash("info", "Rodando de novo.")
	return c.Redirect("/tarefas/" + novo)
}
