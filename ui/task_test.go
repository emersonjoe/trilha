package ui

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/task"
)

// tarefas devolve um motor parado e o store dele: os testes da interface
// querem o estado que a tela desenha, e não a execução — o que roda tem teste
// no pacote task.
func tarefas(t *testing.T) (*task.Tasks, task.Store) {
	t.Helper()
	store := task.Memory()
	return task.New(task.Options{Store: store,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}), store
}

// grava põe uma tarefa no estado que a tela precisa desenhar.
func grava(t *testing.T, store task.Store, tarefa task.Task) {
	t.Helper()
	if err := store.Save(context.Background(), tarefa); err != nil {
		t.Fatal(err)
	}
}

func desenha(t *testing.T, n func(c *trilha.Ctx) h.Node) (string, *httptest.ResponseRecorder) {
	t.Helper()
	app := trilha.New(trilha.Config{Env: trilha.Prod, Locale: "pt-BR",
		Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	rec := httptest.NewRecorder()
	var out string
	app.Register(trilha.Route{Pattern: "/x", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			s, err := h.Render(n(c))
			if err != nil {
				t.Fatal(err)
			}
			out = s
			return c.Text(200, "ok")
		}}})
	app.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	return out, rec
}

// Rodando: o passo, o N/M e a barra na proporção certa.
func TestTaskProgressMostraOPasso(t *testing.T) {
	tk, store := tarefas(t)
	grava(t, store, task.Task{ID: "t1", Name: "processar", Key: "doc-1", State: task.Running,
		Step: "Classificar", N: 2, Of: 4, Queued: time.Now()})

	got, rec := desenha(t, func(c *trilha.Ctx) h.Node { return TaskProgress(c, tk, "t1") })
	for _, quero := range []string{`Classificar (2/4)`, `role="progressbar"`, `width:50%`, `data-trilha-poll="2s"`} {
		if !strings.Contains(got, quero) {
			t.Fatalf("faltou %q em:\n%s", quero, got)
		}
	}
	// Enquanto está indo, a página continua perguntando.
	if rec.Header().Get("Trilha-Poll") == "stop" {
		t.Fatal("parou de perguntar com a tarefa rodando")
	}
}

// Terminada: a tela para de perguntar. Uma aba esquecida aberta custaria uma
// requisição a cada dois segundos para sempre.
func TestTaskProgressParaNoFim(t *testing.T) {
	for _, estado := range []task.State{task.Done, task.Failed, task.Interrupted} {
		t.Run(string(estado), func(t *testing.T) {
			tk, store := tarefas(t)
			grava(t, store, task.Task{ID: "t1", Name: "processar", State: estado,
				Err: "o OCR não leu a página 3", Queued: time.Now()})
			got, rec := desenha(t, func(c *trilha.Ctx) h.Node { return TaskProgress(c, tk, "t1") })
			if rec.Header().Get("Trilha-Poll") != "stop" {
				t.Fatal("continuou perguntando depois do fim")
			}
			if estado != task.Done && !strings.Contains(got, "o OCR não leu a página 3") {
				t.Fatalf("a falha não apareceu:\n%s", got)
			}
		})
	}
}

// Sem contagem de passos a barra é indeterminada: 0% parado diz "quebrado".
func TestTaskProgressSemContagemEhIndeterminada(t *testing.T) {
	tk, store := tarefas(t)
	grava(t, store, task.Task{ID: "t1", Name: "processar", State: task.Running, Queued: time.Now()})
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node { return TaskProgress(c, tk, "t1") })
	if !strings.Contains(got, "ui-progress-idle") {
		t.Fatalf("barra determinada sem ter o que contar:\n%s", got)
	}
	if strings.Contains(got, "width:0%") {
		t.Fatal("desenhou 0%, que a pessoa lê como travado")
	}
	// Barra indeterminada precisa dizer o que está esperando, senão é um
	// retângulo animado para quem usa leitor de tela.
	if !strings.Contains(got, `aria-label="Rodando"`) {
		t.Fatalf("sem rótulo acessível:\n%s", got)
	}
}

// Id que não existe mais não pode virar página em branco nem laço de polling.
func TestTaskProgressComIdSumidoParaEExplica(t *testing.T) {
	tk, _ := tarefas(t)
	got, rec := desenha(t, func(c *trilha.Ctx) h.Node { return TaskProgress(c, tk, "nao-existe") })
	if rec.Header().Get("Trilha-Poll") != "stop" {
		t.Fatal("ficou perguntando por uma tarefa que não existe")
	}
	if !strings.Contains(got, "não está mais no registro") {
		t.Fatalf("não explicou:\n%s", got)
	}
}

// A tabela mostra o que rodou e oferece o botão só para o que já acabou —
// "tentar de novo" numa tarefa rodando é um segundo disparo.
func TestTaskTableSoOfereceRetryDoQueAcabou(t *testing.T) {
	// A tabela recebe as linhas prontas: não precisa do motor.
	rodando := task.Task{ID: "t1", Name: "processar", Key: "doc-1", State: task.Running,
		Queued: time.Now(), Started: time.Now(), By: "ana@exemplo.com"}
	falhou := task.Task{ID: "t2", Name: "processar", Key: "doc-2", State: task.Failed,
		Err: "sem OCR", Queued: time.Now(), Started: time.Now(), Ended: time.Now()}

	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return TaskTable(c, []task.Task{rodando, falhou}, TaskTableOpts{
			Retry: "/tarefas/retry", CSRF: trilha.CSRFInput(c)})
	})
	if strings.Count(got, "ui-task-retry") != 1 {
		t.Fatalf("botões de retry = %d:\n%s", strings.Count(got, "ui-task-retry"), got)
	}
	if !strings.Contains(got, `value="t2"`) {
		t.Fatal("o botão não aponta para a tarefa que falhou")
	}
	// O POST tem de levar o CSRF, senão o app recusa o próprio botão.
	if !strings.Contains(got, "csrf") {
		t.Fatalf("sem CSRF no formulário:\n%s", got)
	}
	for _, quero := range []string{"doc-1", "doc-2", "ana@exemplo.com", "Rodando", "Falhou"} {
		if !strings.Contains(got, quero) {
			t.Fatalf("faltou %q na tabela", quero)
		}
	}
}

// Sem Retry configurado não se desenha botão nenhum: uma tela que oferece
// tentar de novo e não consegue é pior que uma que não oferece.
func TestTaskTableSemRotaNaoDesenhaBotao(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return TaskTable(c, []task.Task{{ID: "t2", Name: "x", State: task.Failed}}, TaskTableOpts{})
	})
	if strings.Contains(got, "ui-task-retry") {
		t.Fatalf("desenhou botão sem para onde postar:\n%s", got)
	}
}

func TestTaskTableVaziaDiz(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return TaskTable(c, nil, TaskTableOpts{})
	})
	if !strings.Contains(got, "Nada rodou ainda") {
		t.Fatalf("%s", got)
	}
}
