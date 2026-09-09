// Package tarefas is the administration screen for the long work: what ran,
// how it ended, and the button to run it again.
//
// The two halves are what the module is for. POST starts the task and comes
// straight back — the request does not wait for four stages — and the table
// reads state that outlived the request that created it.
package tarefas

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/documentos"
	"github.com/emersonjoe/trilha/examples/blog/internal/tarefas"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/task"
	"github.com/emersonjoe/trilha/ui"
)

// Page lists what has run at GET /tarefas.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Tarefas")
	motor := trilha.Use[*task.Tasks](c)
	lista, err := motor.List(c.Context(), task.ListParams{Limit: 20})
	if err != nil {
		return nil, err
	}
	return h.Div(
		ui.PageHeader("Tarefas", ui.Back{Href: "/documentos", Label: "Documentos"}),
		ui.Muted(h.Text("O processamento roda fora da requisição; esta tela lê o estado que ele deixou.")),
		disparar(c),
		ui.TaskTable(c, lista, ui.TaskTableOpts{Retry: "/tarefas/tentar", CSRF: trilha.CSRFInput(c)}),
	), nil
}

// POST starts the processing of one document and goes to its screen.
func POST(c *trilha.Ctx) error {
	id := c.Form("documento")
	if _, ok := documentos.Um(id); !ok {
		return trilha.Errorf(http.StatusNotFound, "documento não encontrado")
	}
	motor := trilha.Use[*task.Tasks](c)
	// Duplo clique no botão devolve o id da que já está indo, e não uma
	// segunda: é o dedupe por chave, e é por isso que a chave é o documento.
	tarefa, err := motor.Run(c, tarefas.Nome, id)
	if err != nil {
		return err
	}
	return c.Redirect("/tarefas/" + tarefa)
}

// disparar é o formulário que escolhe um documento pendente e manda processar.
func disparar(c *trilha.Ctx) h.Node {
	pendentes, _ := documentos.Buscar(documentos.Consulta{Limite: 8})
	opcoes := make([]ui.Option, 0, len(pendentes))
	for _, d := range pendentes {
		opcoes = append(opcoes, ui.Option{Value: d.ID, Label: d.Nome + " (" + d.Status + ")"})
	}
	if len(opcoes) == 0 {
		return h.Nil
	}
	return h.Form(h.Method("post"), h.Action("/tarefas"), h.Class("ui-row"),
		trilha.CSRFInput(c),
		ui.Select(h.Name("documento"), h.Aria("label", "Documento"), ui.SelectOptions(opcoes, "")),
		ui.Button(h.Type("submit"), h.Text("Processar")),
	)
}
