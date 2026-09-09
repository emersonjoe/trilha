package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/documentos"
	"github.com/emersonjoe/trilha/task"
)

// clienteComTarefas é o cliente de teste com o motor de tarefas domado: os
// estágios sem espera, e o motor desligado no fim.
//
// As duas metades são a mesma lição. O ritmo vem do ambiente, lido uma vez
// quando o app é construído, porque uma variável de pacote que um teste
// escreve enquanto o worker de outro lê é uma corrida — e foi assim que o
// detector a encontrou. E o Shutdown é o que impede o motor de um teste de
// continuar mexendo na lista de documentos durante o teste seguinte.
func clienteComTarefas(t *testing.T) *client {
	t.Helper()
	t.Setenv("BLOG_TASK_STEP", "1ms")
	documentos.Reset()
	c := newClient(t, "prod")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		trilha.Use[*task.Tasks](c.app).Shutdown(ctx)
	})
	return c
}

// ate segura o teste até a condição valer, para não depender de sleep.
func ate(t *testing.T, porque string, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(3 * time.Millisecond)
	}
	t.Fatalf("esperei demais por: %s", porque)
}

// primeiroDoc devolve o id de um documento da lista, que é a chave da tarefa.
func primeiroDoc(t *testing.T) (id, nome string) {
	t.Helper()
	lista, _ := documentos.Buscar(documentos.Consulta{Limite: 50})
	if len(lista) == 0 {
		t.Fatal("o exemplo não tem documentos")
	}
	return lista[0].ID, lista[0].Nome
}

// #111 — o processamento roda fora da requisição: o POST volta na hora, e o
// estado continua andando depois que a resposta foi embora.
func TestTarefaRodaForaDaRequisicao(t *testing.T) {
	c := clienteComTarefas(t)

	id, _ := primeiroDoc(t)
	inicio := time.Now()
	rec := c.PostForm("/tarefas", url.Values{"documento": {id}})
	rec.WantStatus(http.StatusSeeOther)
	if passou := time.Since(inicio); passou > time.Second {
		t.Fatalf("o POST esperou %s: a tarefa rodou dentro da requisição", passou)
	}
	tarefa := strings.TrimPrefix(rec.Header().Get("Location"), "/tarefas/")
	if tarefa == "" {
		t.Fatalf("sem destino: %q", rec.Header().Get("Location"))
	}

	// A tela do andamento existe antes de a tarefa acabar, e é ela que a
	// pessoa olha enquanto espera.
	c.Get("/tarefas/" + tarefa).WantStatus(200).WantContains("progressbar")

	ate(t, "o documento ficar pronto", func() bool {
		d, _ := documentos.Um(id)
		return d.Status == "pronto"
	})
	// E terminada, a página para de perguntar.
	fim := c.Get("/tarefas/" + tarefa + "?fragment=trilha-task")
	fim.WantStatus(200)
	if fim.Header().Get("Trilha-Poll") != "stop" {
		t.Fatal("a página continuou perguntando depois de a tarefa terminar")
	}
}

// Duplo clique no botão não processa o documento duas vezes.
func TestDoisPostsSaoUmaTarefaSo(t *testing.T) {
	c := clienteComTarefas(t)
	id, _ := primeiroDoc(t)

	primeiro := c.PostForm("/tarefas", url.Values{"documento": {id}}).Header().Get("Location")
	segundo := c.PostForm("/tarefas", url.Values{"documento": {id}}).Header().Get("Location")
	if primeiro != segundo {
		t.Fatalf("dois disparos viraram duas tarefas: %s e %s", primeiro, segundo)
	}
}

// A falha aparece na tela com o motivo, e o botão de tentar de novo roda outra
// vez — que é o ciclo inteiro da tela de administração.
func TestFalhaApareceETentaDeNovo(t *testing.T) {
	c := clienteComTarefas(t)

	// O exemplo falha de propósito num documento com "erro" no nome.
	documentos.Importar([]documentos.Documento{{Nome: "erro-sem-texto.pdf", Tipo: "nota", Bytes: 10, Status: "fila"}})
	lista, _ := documentos.Buscar(documentos.Consulta{Q: "erro-sem-texto", Limite: 1})
	if len(lista) != 1 {
		t.Fatalf("não achei o documento plantado: %+v", lista)
	}
	ruim := lista[0].ID

	tarefa := strings.TrimPrefix(c.PostForm("/tarefas", url.Values{"documento": {ruim}}).
		Header().Get("Location"), "/tarefas/")
	ate(t, "a falha aparecer na tela", func() bool {
		return strings.Contains(c.Get("/tarefas/"+tarefa).Body.String(), "não tem texto")
	})

	// A listagem oferece o botão, e ele roda de novo com id próprio.
	c.Get("/tarefas").WantStatus(200).WantContains("Tentar de novo", ruim)
	outra := c.PostForm("/tarefas/tentar", url.Values{"id": {tarefa}})
	outra.WantStatus(http.StatusSeeOther)
	if novo := strings.TrimPrefix(outra.Header().Get("Location"), "/tarefas/"); novo == tarefa {
		t.Fatal("o retry devia ser uma execução nova, com id próprio")
	}
	// E a que falhou continua na lista: é o motivo de alguém ter apertado.
	c.Get("/tarefas").WantStatus(200).WantContains("Falhou")
}

// Documento que não existe é 404, e não uma tarefa disparada para o vazio.
func TestDispararDocumentoInexistenteE404(t *testing.T) {
	c := clienteComTarefas(t)
	c.PostForm("/tarefas", url.Values{"documento": {"nao-existe"}}).WantStatus(http.StatusNotFound)
}
