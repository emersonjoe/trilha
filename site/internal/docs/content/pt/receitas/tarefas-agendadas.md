---
title: Tarefas agendadas
description: Um ticker que sobe com o app e para com ele, um tique por vez, um pânico que não derruba o processo — e a linha em que o cron passa a ser a resposta certa.
---

Algum trabalho não tem requisição por trás: expirar sessões, mandar um resumo, tentar de novo o
que falhou. Enquanto há uma instância, uma goroutine com um ticker é a resposta inteira, e ela
mora dentro do app, com o mesmo pool e o mesmo logger.

## O formato

```go
// Job is something that runs on a schedule inside the process. It is the
// right shape while one instance runs it; the moment there are two, the
// answer is a queue or a lock in the database, not a second ticker.
type Job struct {
	Name  string
	Every time.Duration
	Run   func(context.Context) error
}
```

```go
// Start runs the job until the context is cancelled. A tick that arrives
// while the previous run is still going is dropped, not queued: a job that
// takes longer than its interval must not pile up copies of itself.
func Start(ctx context.Context, log *slog.Logger, j Job) {
	t := time.NewTicker(j.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runJob(ctx, log, j)
		}
	}
}
```

Um `time.Ticker` descarta um tique quando ninguém está recebendo, e é esse o comportamento que
você quer: uma tarefa que demora mais que o intervalo não pode acumular cópias de si mesma. O
laço chama a tarefa e só depois volta a esperar.

```go
// runJob keeps one tick from taking the process down. A panic in a
// background task is a bug, and a bug in one task should not stop the ones
// that work.
func runJob(ctx context.Context, log *slog.Logger, j Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("job panicked", "job", j.Name, "panic", r)
		}
	}()
	start := time.Now()
	if err := j.Run(ctx); err != nil {
		log.Error("job failed", "job", j.Name, "error", err, "took", time.Since(start))
		return
	}
	log.Info("job done", "job", j.Name, "took", time.Since(start))
}
```

O `recover` não é otimismo sobre o seu código. Um pânico numa goroutine derruba o processo
inteiro — servidor HTTP incluído — então um bug solto no resumo noturno pararia o site.
Registrado e pulado, ele custa uma execução.

## Subindo e parando

```go
// SetupJobs starts the tasks and makes the shutdown wait for them. A job
// killed halfway is a row written and an e-mail not sent, and it is the
// hardest kind of bug to reproduce.
func SetupJobs(a *trilha.App) error {
	ctx, stop := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, j := range []Job{
		{Name: "expire-sessions", Every: time.Hour, Run: expireSessions},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Start(ctx, a.Logger(), j)
		}()
	}
	a.OnShutdown(func(*trilha.App) error {
		stop()
		wg.Wait()
		return nil
	})
	return nil
}
```

O contexto é cancelado no desligamento e o `WaitGroup` faz o processo esperar o tique em
andamento. Sem ele, um deploy mata uma tarefa no meio: a linha foi escrita, o e-mail não foi, e
esse é o tipo de bug mais difícil de reproduzir.

A tarefa em si é uma função comum que recebe um contexto, então um teste chama ela direto, sem
temporizador nenhum no meio:

```go
// expireSessions is an ordinary function taking a context: nothing about it
// knows it runs on a timer, so a test calls it directly.
func expireSessions(ctx context.Context) error {
	_, err := DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	return err
}
```

## Quando "uma instância" deixa de ser verdade

No momento em que há duas instâncias, as duas ticam. Toda tarefa roda duas vezes, e "mandar o
resumo" vira "mandar o resumo duas vezes".

| Situação | O que fazer |
|---|---|
| uma instância | esta receita |
| mais de uma, tarefa idempotente | mantenha; rodar duas vezes não muda nada |
| mais de uma, tarefa que só pode rodar uma vez | um lock no banco |
| trabalho que precisa sobreviver a um restart | uma fila, não um ticker |

O lock é menor do que parece — uma linha por nome de tarefa, um update condicional que só dá
certo para a instância que chegar primeiro:

```sql
UPDATE job_locks SET locked_until = now() + interval '5 minutes', owner = $1
WHERE name = $2 AND locked_until < now();
```

Se o update não tocou nenhuma linha, outra instância está rodando, e esta pula o tique.

:::nota
E não há nada de errado em um `cron` chamando uma rota com token, ou um timer do systemd
rodando o seu binário com um subcomando. A vantagem é a separação: uma tarefa que trava não
segura uma vaga no servidor. O custo é mais uma coisa para publicar.
:::
