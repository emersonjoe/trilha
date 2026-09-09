package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/task"
)

// TaskProgress is the bar that watches one task: the step, how far along, and
// the failure when there was one.
//
//	// app/documentos/id_/page.go
//	func Page(c *trilha.Ctx) (h.Node, error) {
//		if c.Fragment() == "tarefa" {
//			return ui.TaskProgress(c, app.Tarefas, id), nil
//		}
//		…
//	}
//
// It is a Poll that stops on its own: while the task is alive the fragment
// asks again every two seconds, and the moment it finishes the route answers
// Ctx.PollStop and the browser stops asking. Without JavaScript it is the
// state as of when the page loaded — old, never broken — and reloading is the
// refresh.
//
// The id comes from the application, which knows which task belongs to the
// screen; passing one the visitor sent would be letting them watch somebody
// else's work.
func TaskProgress(c *trilha.Ctx, tasks *task.Tasks, id string) h.Node {
	lang := langOf(c)
	if id == "" || tasks == nil {
		return h.Div(h.ID("trilha-task"))
	}
	t, err := tasks.Get(c.Context(), id)
	if err != nil {
		c.PollStop()
		return h.Div(h.ID("trilha-task"), Poll("2s", ""),
			Muted(h.Text(taskWords[lang]["gone"])))
	}
	// The polling ends with the work: a page that keeps asking about a task
	// that finished an hour ago is a page that costs a request every two
	// seconds for as long as somebody leaves the tab open.
	if !t.State.Live() {
		c.PollStop()
	}

	corpo := []h.Node{h.ID("trilha-task"), Poll("2s", ""), h.Class("ui-task")}
	switch t.State {
	case task.Done:
		corpo = append(corpo,
			h.P(h.Class("ui-task-step"), h.Text(taskWords[lang]["done"])),
			Progress(1, 1))
	case task.Failed, task.Interrupted:
		titulo := taskWords[lang]["failed"]
		if t.State == task.Interrupted {
			titulo = taskWords[lang]["interrupted"]
		}
		corpo = append(corpo, Alert(titulo, Destructive(),
			AlertDescription(h.Text(t.Err))))
	default:
		corpo = append(corpo, h.P(h.Class("ui-task-step"), h.Text(taskStep(t, lang))))
		if t.Of > 0 {
			corpo = append(corpo, Progress(t.N, t.Of))
		} else {
			// No count is not zero progress: an indeterminate bar says "going"
			// where a bar stuck at 0% says "broken".
			corpo = append(corpo, h.Div(h.Class("ui-progress ui-progress-idle"),
				h.Role("progressbar"), h.Aria("label", taskStep(t, lang)), h.Div()))
		}
	}
	return h.Div(corpo...)
}

func taskStep(t task.Task, lang string) string {
	rotulo := t.Step
	if rotulo == "" {
		rotulo = taskWords[lang][string(t.State)]
	}
	if t.Of > 0 && t.N > 0 {
		return rotulo + " (" + strconv.Itoa(t.N) + "/" + strconv.Itoa(t.Of) + ")"
	}
	return rotulo
}

// TaskTableOpts is what the administration screen needs beyond the rows.
type TaskTableOpts struct {
	// Retry is where the "try again" button posts to; the button is not drawn
	// when it is empty, because a screen that offers to retry and cannot is
	// worse than one that does not offer.
	Retry string
	// CSRF is the hidden input, from trilha.CSRFInput. Required with Retry:
	// a POST without it is a POST the app will refuse.
	CSRF h.Node
	// Empty is what to say when there is nothing. Left empty it says so in
	// the app's language.
	Empty string
}

// TaskTable is the administration screen: what ran, how it ended, and the
// button to run it again.
//
//	ui.TaskTable(c, tarefas, ui.TaskTableOpts{Retry: "/tarefas/retry", CSRF: trilha.CSRFInput(c)})
//
// It is a plain table and not a DataTable because the columns are fixed and
// the sort is always the same: newest first is the only order anybody wants
// from a list of what just happened.
func TaskTable(c *trilha.Ctx, tasks []task.Task, o TaskTableOpts) h.Node {
	lang := langOf(c)
	w := taskWords[lang]
	if len(tasks) == 0 {
		vazio := o.Empty
		if vazio == "" {
			vazio = w["empty"]
		}
		return Empty(EmptyOpts{Title: vazio})
	}
	linhas := make([]h.Node, 0, len(tasks))
	for _, t := range tasks {
		linhas = append(linhas, h.Tr(
			h.Td(h.Text(t.Name), h.Br(), Muted(h.Text(t.Key))),
			h.Td(taskBadge(t, lang)),
			h.Td(h.Text(taskStep(t, lang))),
			h.Td(Date(c, t.Queued, Relative())),
			h.Td(h.Text(t.Took().String())),
			h.Td(h.Text(t.By)),
			h.Td(taskRetry(t, o, w)),
		))
	}
	return h.Div(h.Class("ui-table-wrap"), h.Table(h.Class("ui-table"),
		h.Thead(h.Tr(
			h.Th(h.Text(w["task"])), h.Th(h.Text(w["state"])), h.Th(h.Text(w["step"])),
			h.Th(h.Text(w["when"])), h.Th(h.Text(w["took"])), h.Th(h.Text(w["by"])), h.Th(),
		)),
		h.Tbody(linhas...),
	))
}

func taskBadge(t task.Task, lang string) h.Node {
	// Three variants and not five: the kit has the ones it has, and inventing
	// a colour here would be a class no stylesheet defines.
	variante := Secondary()
	switch t.State {
	case task.Done:
		variante = Outline()
	case task.Failed, task.Interrupted:
		variante = Destructive()
	}
	return Badge(variante, h.Text(taskWords[lang][string(t.State)]))
}

func taskRetry(t task.Task, o TaskTableOpts, w map[string]string) h.Node {
	if o.Retry == "" || t.State.Live() {
		return h.Nil
	}
	return h.Form(h.Method("post"), h.Action(o.Retry), h.Class("ui-inline-form"),
		o.CSRF,
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(t.ID)),
		Button(Ghost(), h.Type("submit"), h.Text(w["retry"])),
	)
}

// taskWords is the text this kit writes on its own. It is two languages and
// not a translation mechanism, which is the same trade the rest of the kit
// makes: an app in a third language writes its own screen, and it is a table.
var taskWords = map[string]map[string]string{
	"en": {
		"queued": "Queued", "running": "Running", "done": "Done",
		"error": "Failed", "interrupted": "Interrupted",
		"failed": "It did not finish", "gone": "This task is no longer on record.",
		"task": "Task", "state": "State", "step": "Step", "when": "Started",
		"took": "Took", "by": "By", "retry": "Try again",
		"empty": "Nothing has run yet.",
	},
	"pt-BR": {
		"queued": "Na fila", "running": "Rodando", "done": "Pronto",
		"error": "Falhou", "interrupted": "Interrompida",
		"failed": "Não terminou", "gone": "Esta tarefa não está mais no registro.",
		"task": "Tarefa", "state": "Estado", "step": "Passo", "when": "Começou",
		"took": "Levou", "by": "Por", "retry": "Tentar de novo",
		"empty": "Nada rodou ainda.",
	},
}
