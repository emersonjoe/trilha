---
title: Trabalho que sobrevive à requisição
description: trilha/task — os quatro estágios de processar alguma coisa, com uma tela que mostra o andamento, um panic que não derruba o processo e um store onde o estado fica.
---

Todo app de gestão tem a tela que dispara algo longo e depois fica perguntando "acabou?". O que
se escreve para isso é uma linha:

```go
go func() { processa(docID) }()
```

Essa linha tem quatro defeitos, e os quatro aparecem em produção, não na sua máquina.

O erro não vai para lugar nenhum — ninguém fica sabendo que falhou. O `context` da requisição
morre quando o navegador fecha, então metade do trabalho para pela metade. Um `panic` ali dentro
derruba o processo inteiro, servidor web junto, porque goroutine de fundo não tem quem a
recupere. E a tela não tem o que mostrar, porque não existe estado — existe uma goroutine.

## Isto não é uma fila

Antes de tudo, porque decide se o módulo serve para você: as tarefas vivem em **um processo**.
Duas réplicas são dois destes, e a mesma tarefa roda duas vezes. Nada sobrevive a uma queda,
exceto o registro dizendo que foi interrompida.

Essa é a forma honesta da coisa, e ela cobre o que uma aplicação interna faz de verdade. Se você
precisa de trabalho espalhado por máquinas, ou tentado de novo pelo worker de outra pessoa, você
quer uma fila de verdade — e o `Store` é a costura para pôr uma atrás.

## Registrar e disparar

```go
// Novo builds the engine with the handler already registered.
func Novo() *task.Tasks {
	t := task.New(task.Options{Workers: 2})
	t.Handle(Nome, processar)
	return t
}
```

`Handle` e `Run` são separados de propósito, e o motivo é o `Retry`. Uma closure passada no
`Run` vive enquanto o processo vive; depois de um reinício, a tarefa interrompida não teria
função nenhuma para tentar de novo — e a tarefa que alguém quer tentar de novo é justamente a
que morreu no deploy. Registrada por nome, o botão funciona.

```go
	motor := tarefas.Novo()
	trilha.Provide(a, motor)
	if err := motor.Setup(a); err != nil {
		return err
	}
```

O `Setup` faz duas coisas que valem saber. Ele pendura o `Shutdown` no app, então um deploy no
meio de um processamento espera em vez de cortar. E marca como **interrompido** tudo que o store
ainda chama de `queued` ou `running`: aquilo pertence a um processo que não existe mais, e
tarefa presa em "rodando" para sempre é o bug clássico.

O motor é um valor que o app provê, e não uma variável de pacote — o mesmo motivo dos stores.
Uma suíte que sobe um servidor por teste dá a cada um o seu, e nenhum teste vê a tarefa do
outro.

```go
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
```

A chave é o que faz duas execuções serem a mesma. Com uma viva, um segundo `Run` devolve o id da
primeira — que é o duplo clique no botão, e o motivo de a chave existir.

## O trabalho

```go
// processar is the work. It reports each stage with Progress.Step and looks at
// the context between them: a task that never looks is a task the shutdown has
// to cancel by force.
func processar(ctx context.Context, p *task.Progress) error {
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
		case <-time.After(Passo):
		}
		// Um documento com "erro" no nome falha de propósito: é o caminho que
		// o exemplo precisa mostrar tanto quanto o que dá certo.
		if i == 1 && strings.Contains(strings.ToLower(doc.Nome), "erro") {
			documentos.Marcar(p.Key, "fila")
			return errors.New("não consegui classificar: o arquivo não tem texto")
		}
	}
	documentos.Marcar(p.Key, "pronto")
	return nil
}
```

Não existe `Args`: a chave é o id, e o resto a função busca na sua própria tabela. É a mesma
troca que o `c.Link` faz, e pelo mesmo motivo — o que viaja fica pequeno e continua verdadeiro.

O `Step` escreve no store, então custa uma escrita. Chame uma vez por estágio, não uma vez por
linha.

Um `panic` lá dentro vira erro gravado, com a pilha no log. Perder o servidor web porque um
documento tinha uma página ruim não é uma troca que alguém faria de propósito.

## A tela

```go
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
```

O `ui.TaskProgress` é um `ui.Poll` que para sozinho: enquanto a tarefa está viva o fragmento
pede a si mesmo de dois em dois segundos, e no instante em que ela acaba a rota responde
`Ctx.PollStop` e o navegador para. Sem JavaScript ele é o estado de quando a página carregou —
velho, nunca quebrado — e recarregar é a atualização.

Sem contagem de passos ele desenha uma barra **indeterminada** em vez de 0%: barra parada em
zero se lê como quebrada, e uma que anda se lê como indo.

A tela de administração é o `ui.TaskTable`, e o botão de tentar de novo é um POST próprio:

```go
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
```

O `Retry` devolve um id **novo**, e a execução que falhou continua no store. Uma tela que engole
a falha perde o motivo de alguém ter apertado o botão.

## Guardando o histórico: um store em SQL

O `Memory()` é o padrão e serve para a maioria dos apps — as tarefas também não sobrevivem a um
reinício. Uma tabela compra o histórico, e "interrompida às 14:32 pelo deploy" é uma frase que
alguém acaba precisando.

```go
// TaskSchema is the table. Two indexes and both are load-bearing: the first is
// the deduplication, which runs on every Run, and the second is the listing,
// which is the only order the screen ever asks for.
const TaskSchema = `
CREATE TABLE IF NOT EXISTS trilha_tasks (
	id       TEXT PRIMARY KEY,
	name     TEXT NOT NULL,
	key      TEXT NOT NULL,
	state    TEXT NOT NULL,
	step     TEXT NOT NULL DEFAULT '',
	n        INTEGER NOT NULL DEFAULT 0,
	of_n     INTEGER NOT NULL DEFAULT 0,
	err      TEXT NOT NULL DEFAULT '',
	by_whom  TEXT NOT NULL DEFAULT '',
	queued   TIMESTAMPTZ NOT NULL,
	started  TIMESTAMPTZ,
	ended    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS trilha_tasks_active ON trilha_tasks (name, key) WHERE state IN ('queued','running');
CREATE INDEX IF NOT EXISTS trilha_tasks_recent ON trilha_tasks (queued DESC);
`
```

```go
// Active is the deduplication, and it is the one query that runs on every
// Run: the partial index above exists for this line.
func (s TaskSQL) Active(ctx context.Context, name, key string) (string, error) {
	var id string
	err := s.DB.QueryRowContext(ctx,
		`SELECT id FROM trilha_tasks WHERE name = $1 AND key = $2 AND state IN ('queued','running') LIMIT 1`,
		name, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
```

```go
// Interrupt runs once, at boot, and is the reason a table beats memory here:
// it is the row that says the deploy caught this one halfway, instead of a
// screen waiting forever for a process that is gone.
func (s TaskSQL) Interrupt(ctx context.Context, at time.Time) (int, error) {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE trilha_tasks SET state = 'interrupted', ended = $1,
			err = 'the process stopped before this finished'
		WHERE state IN ('queued','running')`, at)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
```

O arquivo inteiro está em `examples/cookbook/tasks.go`, com sabor Postgres; SQLite e MySQL são o
mesmo arquivo com `?` no lugar de `$1`.

Não existe `task.SQL(db)` no módulo pelo mesmo motivo que nenhum store deste framework traz um:
ele teria de escolher dialeto de placeholder e ser dono de um DDL, e o framework não é dono do
seu esquema.

:::note
O dedupe segura um lock entre a checagem e a escrita, que é o que o torna verdadeiro em vez de
provável — e honesto só porque isto roda em um processo. Com store em SQL e duas réplicas,
acrescente um índice único em `(name, key) WHERE state IN ('queued','running')` e o banco passa
a garantir o que o lock não garante.
:::

## Testando

O teste do exemplo é o que vale copiar, porque ele afirma o que importa — que o POST voltou
antes de o trabalho terminar:

```go
	id, _ := primeiroDoc(t)
	inicio := time.Now()
	rec := c.PostForm("/tarefas", url.Values{"documento": {id}})
	rec.WantStatus(http.StatusSeeOther)
	if passou := time.Since(inicio); passou > time.Second {
		t.Fatalf("o POST esperou %s: a tarefa rodou dentro da requisição", passou)
	}
```

Espere por uma condição, não pelo relógio. Um `time.Sleep` é lento quando passa e mentiroso
quando falha.

## Agendamento

Trabalho sem requisição atrás — expirar sessões, um resumo noturno — tem outra forma: veja
[tarefas agendadas](/pt/receitas/tarefas-agendadas). Um ticker dispara; este módulo roda.
