package recipes

// tasksRecipe is the work that does not fit inside a request: a queue, dedupe
// by key, retry, progress, and the screen that reads what the work left
// behind.
//
// Same case as the webhooks recipe. The task module has existed since 0.64.0,
// and somebody who needs it today writes a loose goroutine instead — the
// version that loses the work on the first deploy, and that nobody can retry
// because there is nothing to retry.
func tasksRecipe() Recipe {
	return Recipe{
		Name: "tasks",
		Summary: map[string]string{
			"en": "the work that does not fit in a request: queue, dedupe, retry, and the screen",
			"pt": "o trabalho que não cabe numa requisição: fila, dedupe, retry, e a tela",
		},
		Doc: "/reference/task",
		Files: []File{
			{Rel: "internal/trabalho/trabalho.go", Go: true, Body: tasksEngine},
			{Rel: "internal/trabalho/trabalho_test.go", Go: true, Body: tasksEngineTest},
			{Rel: "{{.At}}tarefas/page.go", Go: true, Body: tasksPage},
			{Rel: "tarefas_test.go", Go: true, Body: tasksTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add tasks",
			Line:   "\tif err := trabalho.Setup(a); err != nil {\n\t\treturn err\n\t}\n",
		}},
		Imports: []string{"{{.Module}}/internal/trabalho"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}tarefas. Replace the example task with yours in " +
				"internal/trabalho, and start it from the screen that owns the thing: " +
				"`trilha.Use[*task.Tasks](c).Run(c, trabalho.Nome, id)` — the key is what makes two clicks " +
				"one task. On that screen, ui.TaskProgress(c, tasks, id) shows it going.",
			"pt": "Rode `trilha dev` e abra {{.URL}}tarefas. Troque a tarefa de exemplo pela sua em " +
				"internal/trabalho, e dispare da tela dona da coisa: " +
				"`trilha.Use[*task.Tasks](c).Run(c, trabalho.Nome, id)` — a chave é o que faz dois cliques " +
				"virarem uma tarefa. Nessa tela, ui.TaskProgress(c, tasks, id) mostra ela andando.",
		},
	}
}

const tasksEngine = `// Package trabalho is the long work of this application.
//
// The engine is a value the app builds and provides, and not a package
// variable: a suite that stands up one server per test gives each one its own,
// and no test watches another's tasks finish.
package trabalho

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/task"
)

// Nome is the task this application runs. It is a constant because the name is
// an address: the page that starts it and the handler that answers it have to
// agree, and a typo would be a task queued for nobody.
const Nome = "exemplo"

// Passo is how long each stage takes. It is here so a test can ask for a fast
// engine without a sleep of its own.
var Passo = 20 * time.Millisecond

// Setup builds the engine, registers the handler and starts it.
//
// task.Setup sweeps what a previous process left hanging — a task that was
// running when the machine went down is Failed and retryable, not Running
// forever — and hangs Shutdown on the app, so a deploy in the middle of the
// work waits instead of cutting it.
func Setup(a *trilha.App) error {
	tarefas := task.New(task.Options{Logger: a.Logger()})
	tarefas.Handle(Nome, processar)
	trilha.Provide(a, tarefas)
	return tarefas.Setup(a)
}

// processar is the work itself. Four stages, because a progress bar with one
// stage is a progress bar that says nothing: what the screen shows — "2 of 4",
// with the label — is what this function said.
//
// The context is not the request's: closing the browser does not stop the
// work, and shutting the app down does.
func processar(ctx context.Context, p *task.Progress) error {
	etapas := []string{"Ler", "Conferir", "Guardar", "Avisar"}
	for i, etapa := range etapas {
		p.Step(etapa, i+1, len(etapas))
		select {
		case <-ctx.Done():
			// Ending on the context is not a failure to hide: the deploy
			// happened, and the task is retryable exactly because it says so.
			return ctx.Err()
		case <-time.After(Passo):
		}
	}
	// The key is the application's: here it stands for the thing being worked
	// on, and a key that says "falha" is how the example shows a failure on
	// the screen without anybody breaking the code.
	if strings.Contains(p.Key, "falha") {
		return errors.New("trabalho: o exemplo falhou de propósito")
	}
	return nil
}
`

const tasksEngineTest = `package trabalho

import (
	"context"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/task"
)

// A tarefa anda pelos passos e termina. O que a tela mostra é o que esta
// função disse, então é isto que vale testar.
func TestProcessarAndaPelosPassos(t *testing.T) {
	// Um Progress sem motor atrás é o que um teste precisa: o Step vira um
	// no-op, e o que se testa aqui é o trabalho, não a gravação dele.
	if err := processar(context.Background(), &task.Progress{ID: "t-1", Name: Nome, Key: "doc-1"}); err != nil {
		t.Fatal(err)
	}
}

// E uma chave que pede falha falha: é assim que a tela de exemplo mostra o
// caminho triste sem ninguém quebrar o código.
func TestChaveComFalha(t *testing.T) {
	err := processar(context.Background(), &task.Progress{ID: "t-2", Name: Nome, Key: "doc-falha"})
	if err == nil || !strings.Contains(err.Error(), "prop") {
		t.Fatalf("err = %v", err)
	}
}
`

const tasksPage = `// Package tarefas is the administration screen for the long work: what ran,
// how it ended, and the button to run it again.
//
// The two halves are what the module is for. POST starts the task and comes
// straight back — the request does not wait for four stages — and the table
// reads state that outlived the request that created it.
package tarefas

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/task"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/trabalho"
)

// Page lists what has run at GET {{.URL}}tarefas.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.tasks_title}}")
	tarefas := trilha.Use[*task.Tasks](c)
	lista, err := tarefas.List(c.Context(), task.ListParams{Limit: 20})
	if err != nil {
		return nil, err
	}
	return h.Div(
		ui.PageHeader("{{.T.tasks_title}}"),
		ui.Muted(h.Text("{{.T.tasks_desc}}")),
		h.Form(h.Method("post"), h.Action("{{.URL}}tarefas"), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			ui.Field("chave", "{{.T.tasks_key}}", ui.Input(h.ID("chave"), h.Name("chave"), h.Required())),
			h.Div(ui.Submit(h.Text("{{.T.tasks_run}}"))),
		),
		ui.TaskTable(c, lista, ui.TaskTableOpts{Retry: "{{.URL}}tarefas", CSRF: trilha.CSRFInput(c)}),
	), nil
}

// POST starts a task, or retries one, and comes straight back. It does not
// wait for the work: that is the whole point of the module.
func POST(c *trilha.Ctx) error {
	tarefas := trilha.Use[*task.Tasks](c)
	if id := c.Form("id"); id != "" {
		if _, err := tarefas.Retry(c, id); err != nil {
			return err
		}
		return c.Redirect("{{.URL}}tarefas")
	}
	// Two clicks on the button give back the id of the one already going, and
	// not a second: that is the dedupe by key, and it is why the key is the
	// thing being worked on and not a random number.
	if _, err := tarefas.Run(c, trabalho.Nome, c.Form("chave")); err != nil {
		return err
	}
	return c.Redirect("{{.URL}}tarefas")
}
`

const tasksTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// A tela dispara e volta na hora; a tabela lê o estado que o trabalho deixou.
func TestTarefaRodaEAparece(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())

	c.Get("{{.URL}}tarefas").WantStatus(http.StatusOK)
	c.PostForm("{{.URL}}tarefas", url.Values{"chave": {"doc-1"}}).WantStatus(http.StatusSeeOther)

	// O trabalho acontece fora da requisição, então a tela é consultada até
	// ele terminar — que é exatamente como uma pessoa a usa.
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		corpo := c.Get("{{.URL}}tarefas").WantStatus(http.StatusOK).Body.String()
		if contem(corpo, "doc-1") && !contem(corpo, "running") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("a tarefa não terminou em cinco segundos")
}

func contem(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
`
