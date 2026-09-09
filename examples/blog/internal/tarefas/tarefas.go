// Package tarefas is the long work of this example: processing a document in
// four stages, which is what every management app has and what nobody writes
// well the first time.
//
// The engine is a value the app builds and provides, and not a package
// variable — the same reason the posts store is. A suite that stands up one
// server per test gives each one its own, and no test watches another's tasks
// finish.
package tarefas

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/examples/blog/internal/documentos"
	"github.com/emersonjoe/trilha/task"
	"github.com/emersonjoe/trilha/webhook"
)

// Nome is the task this app runs. It is a constant because the name is an
// address: the page that starts it and the handler that answers it have to
// agree, and a typo would be a task queued for nobody.
const Nome = "processar-documento"

// motor carries what the work needs to know. It is a struct and not a package
// variable, and that is not taste: a package variable a test writes while
// another test's worker reads it is a data race, and the race detector found
// exactly that one here.
type motor struct {
	passo time.Duration
	hooks *webhook.Hooks
}

// Novo builds the engine with the handler already registered. The pace of the
// stages is read once, here, so each engine carries its own — a test asks for
// a fast one through the environment, before the app is built, and nothing is
// shared afterwards.
func Novo(hooks *webhook.Hooks) *task.Tasks {
	m := &motor{passo: 250 * time.Millisecond, hooks: hooks}
	if v := os.Getenv("BLOG_TASK_STEP"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			m.passo = d
		}
	}
	t := task.New(task.Options{Workers: 2})
	t.Handle(Nome, m.processar)
	return t
}

// processar is the work. It reports each stage with Progress.Step and looks at
// the context between them: a task that never looks is a task the shutdown has
// to cancel by force.
func (m *motor) processar(ctx context.Context, p *task.Progress) error {
	doc, ok := documentos.Um(p.Key)
	if !ok {
		return errors.New("documento não existe mais")
	}
	documentos.Marcar(p.Key, "processando")

	estagios := []string{"Extrair texto", "Classificar", "Indexar", "Concluir"}
	for i, estagio := range estagios {
		p.Step(estagio, i+1, len(estagios))
		select {
		case <-ctx.Done():
			// Interrupted is not failed, and the difference is what the screen
			// tells somebody before they press "try again".
			documentos.Marcar(p.Key, "fila")
			return ctx.Err()
		case <-time.After(m.passo):
		}
		// Um documento com "erro" no nome falha de propósito: é o caminho que
		// o exemplo precisa mostrar tanto quanto o que dá certo.
		if i == 1 && strings.Contains(strings.ToLower(doc.Nome), "erro") {
			documentos.Marcar(p.Key, "fila")
			m.avisa("documento.falhou", doc)
			return errors.New("não consegui classificar: o arquivo não tem texto")
		}
	}
	documentos.Marcar(p.Key, "pronto")
	m.avisa("documento.processado", doc)
	return nil
}

// avisa manda o evento para quem assinou. O Ctx é nulo porque aqui não há
// requisição — o trabalho já saiu dela faz tempo, que é o motivo de existir —
// e o Emit grava as entregas e volta sem esperar rede nenhuma.
//
// A falha ao avisar não derruba a tarefa: o documento foi processado, e um
// aviso que não saiu é uma entrega que a tela mostra. Fazer o contrário seria
// reprocessar um documento porque um parceiro está fora do ar.
func (m *motor) avisa(evento string, doc documentos.Documento) {
	if m.hooks == nil {
		return
	}
	if err := m.hooks.Emit(nil, evento, map[string]any{
		"id": doc.ID, "nome": doc.Nome, "tipo": doc.Tipo, "bytes": doc.Bytes,
	}); err != nil {
		slog.Default().Error("webhook", "evento", evento, "documento", doc.ID, "err", err)
	}
}
