---
title: task
description: Tasks, Options, Handle, Run, Progress, Store e as duas telas — a API do pacote task, com os padrões e o que cada campo muda.
---

`import "github.com/emersonjoe/trilha/task"` — o trabalho que não cabe dentro de uma requisição:
os quatro estágios de processar um documento, o lote que alguém disparou, o relatório que leva
um minuto. Ele guarda estado, então a tela tem o que mostrar.

## Isto não é uma fila

As tarefas vivem em **um processo**. Duas réplicas são dois motores, e a mesma tarefa roda duas
vezes; nada sobrevive a uma queda, exceto o registro dizendo que foi interrompida. Essa é a
forma honesta, e ela cobre o que uma aplicação interna faz. Mais que isso pede uma fila de
verdade — o `Store` e o registro de handlers são a costura para pôr uma atrás.

## O motor

```go
func New(o Options) *Tasks                       // não inicia nada
func (t *Tasks) Handle(name string, fn Func)     // antes do Setup
func (t *Tasks) Setup(a *trilha.App) error       // varre, inicia e pendura o Shutdown no app
func (t *Tasks) Shutdown(ctx context.Context) error
```

O `New` não abre nada, então mora onde os outros valores do app são construídos. O `Setup` faz
duas coisas além de iniciar os workers: pendura o `Shutdown` no app e marca como
**interrompido** tudo que o store ainda chama de `queued` ou `running` — aquilo pertence a um
processo que não existe mais, e tarefa presa em "rodando" para sempre é o bug clássico.

| `Options` | Padrão | O que faz |
|---|---|---|
| `Store Store` | `Memory()` | onde o estado mora |
| `Workers int` | 1 | quantas tarefas rodam ao mesmo tempo |
| `Queue int` | 128 | quantas podem esperar; fila cheia é erro no `Run`, não uma fatia que cresce |
| `Shutdown time.Duration` | 15 s | quanto esperar antes de cancelar o context de quem está rodando |
| `Logger *slog.Logger` | `slog.Default()` | uma linha por tarefa, e a pilha de um panic |

Um worker é o padrão honesto: o trabalho que vem para cá costuma disputar a mesma CPU ou o mesmo
serviço externo.

## Registrar e disparar

```go
type Func func(ctx context.Context, p *Progress) error

func (t *Tasks) Run(c *trilha.Ctx, name, key string) (id string, err error)
func (t *Tasks) Retry(c *trilha.Ctx, id string) (string, error)
func (t *Tasks) Cancel(ctx context.Context, id string) error
func (t *Tasks) Get(ctx context.Context, id string) (Task, error)
func (t *Tasks) List(ctx context.Context, p ListParams) ([]Task, error)
```

`Handle` e `Run` são separados por causa do `Retry`: uma closure passada no `Run` vive enquanto o
processo vive, e a tarefa que alguém quer tentar de novo é justamente a que morreu no deploy.
Registrada por nome, o botão funciona depois de um reinício.

**Duas execuções com o mesmo nome e a mesma chave, com uma viva, são uma só** — o id que volta é
o da que já está indo. É o duplo clique no botão, e o motivo de a chave existir. Terminada, a
chave fica livre de novo.

Não existe `Args`. A chave é o id — o documento, o lote — e o resto a função busca na sua própria
tabela, que é a mesma troca que o `Ctx.Link` faz.

O `Retry` devolve um id **novo** e deixa a execução que falhou no store: uma tela que engole a
falha perde o motivo de alguém ter apertado o botão. Tentar de novo algo ainda vivo devolve o id
dele, em vez de começar uma segunda.

O `Cancel` cancela o context da tarefa. Não é um kill: quem ignora o context roda até o fim, e
quem respeita para onde olhou.

O context que a função recebe **não é o da requisição** — fechar o navegador não interrompe o
trabalho — e ele cancela quando o `Shutdown` perde a paciência. Um `panic` vira erro gravado, com
a pilha no log, porque um panic em goroutine de fundo derrubaria o servidor web junto.

## Progress e Task

```go
func (p *Progress) Step(label string, n, of int)   // p.Step("Classificar", 2, 4)

type Progress struct{ ID, Name, Key string }

type Task struct {
	ID, Name, Key string
	State         State  // Queued, Running, Done, Failed, Interrupted
	Step          string
	N, Of         int
	Err           string // já é string: a tela mostra
	By            string // do Ctx.Actor, então diz quem apertou o botão
	Queued, Started, Ended time.Time
}

func (t Task) Percent() int          // -1 quando não há o que contar
func (t Task) Took() time.Duration
func (s State) Live() bool           // na fila ou rodando
```

O `Step` escreve no store, então custa uma escrita: chame uma vez por estágio, não uma por linha.
A falha ao escrever não é devolvida — perder uma atualização de andamento não é motivo para
falhar um trabalho que está indo bem, e o próximo `Step` corrige o quadro.

O `Percent` devolve **-1** em vez de 0 quando não há contagem, e o kit desenha uma barra
indeterminada para isso: barra parada em zero se lê como quebrada.

## Store

```go
type Store interface {
	Save(ctx context.Context, t Task) error
	Get(ctx context.Context, id string) (Task, error)
	List(ctx context.Context, p ListParams) ([]Task, error)
	Active(ctx context.Context, name, key string) (string, error)
	Interrupt(ctx context.Context, at time.Time) (int, error)
}

func Memory() Store
```

Quatro dos cinco são óbvios; o `Interrupt` é o que uma aplicação não teria pensado em escrever.
Ele roda uma vez, na subida.

**Não há implementação SQL aqui**, e é a mesma escolha de todo store deste framework: uma que
viesse pronta teria de escolher dialeto de placeholder e ser dona de um DDL, e o framework não é
dono do seu esquema. A [receita](/pt/receitas/tarefas) traz o arquivo inteiro.

O `ListParams` filtra a tela de administração — `Name`, `Key`, `State`, `Limit`, `Offset` — e o
valor zero dele é "as últimas cinquenta de tudo", mais nova primeiro.

## As telas

```go
func ui.TaskProgress(c *trilha.Ctx, tasks *task.Tasks, id string) h.Node
func ui.TaskTable(c *trilha.Ctx, tasks []task.Task, o ui.TaskTableOpts) h.Node
```

O `TaskProgress` é um `ui.Poll` que para sozinho: enquanto a tarefa está viva o fragmento pede a
si mesmo de dois em dois segundos, e no instante em que ela acaba a rota responde
`Ctx.PollStop`. Sem JavaScript ele é o estado de quando a página carregou — velho, nunca
quebrado. O id vem da aplicação, que sabe qual tarefa é daquela tela; passar um que a pessoa
mandou seria deixá-la acompanhar o trabalho de outra.

O `TaskTableOpts` recebe `Retry` (para onde o botão posta; sem ele, não há botão), `CSRF`
(`trilha.CSRFInput(c)`, obrigatório com o `Retry`) e `Empty`. O botão só aparece para tarefas que
já acabaram — "tentar de novo" numa que está rodando é um segundo disparo.

O [`trilha add tasks`](/pt/referencia/cli#trilha-add) escreve o motor, as duas telas e o teste.

@demo ui-tarefas

## Erros

O `ErrUnknownTask` é um nome que ninguém registrou: erro no `Run`, e não uma tarefa parada para
sempre esperando uma função que não existe. O `ErrNotFound` é um id que o store não tem.

## O que não está aqui

Prioridade, agendamento e retentativa automática. Agendar é um ticker — veja
[tarefas agendadas](/pt/receitas/tarefas-agendadas) — e tentar de novo sozinho esconde a falha em
vez de mostrá-la.
