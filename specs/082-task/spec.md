# Spec 082 — módulo trilha/task

- **Issue**: [#111](https://github.com/emersonjoe/trilha/issues/111) — a issue é a fonte do escopo.
- **Branch**: `082-task`
- **Versão**: 0.64.0

## Por quê

Seis telas do Acervo disparam algo longo e ficam perguntando "acabou?". O que o iniciante
escreve para isso é uma linha:

```go
go func() { processa(docID) }()
```

E essa linha tem quatro defeitos que só aparecem em produção. O erro não vai para lugar
nenhum — ninguém sabe que falhou. O `context` do request morre quando o navegador fecha, então
metade das tarefas termina pela metade. Um `panic` lá dentro derruba o processo inteiro, e não
só a tarefa. E a tela não tem o que mostrar, porque não existe estado: o que existe é uma
goroutine.

O ROADMAP diz "fila no núcleo: não", e continua certo. Só que o que falta aqui não é fila. É
*"rode isto depois de responder, e me deixe mostrar o andamento"*.

## O que muda

Módulo opcional `trilha/task`: tarefas **de um processo**, estado na `Store`, sem fila entre
máquinas.

```go
// app/setup.go
var Tarefas = task.New(task.Options{Store: task.Memory(), Workers: 2})

func Setup(a *trilha.App) error {
	Tarefas.Handle("processar-documento", processar)
	return Tarefas.Setup(a)
}

func processar(ctx context.Context, p *task.Progress) error {
	p.Step("OCR", 1, 4)
	if err := ocr.Run(ctx, p.Key); err != nil {
		return err
	}
	p.Step("Classificar", 2, 4)
	…
}
```

```go
// disparar
id, err := Tarefas.Run(c, "processar-documento", docID)

// mostrar: uma barra que se atualiza sozinha e para sozinha no fim
ui.TaskProgress(c, Tarefas, id)
```

**`Handle` e `Run` são separados de propósito**, e é o `Retry` que decide isso. Uma closure
passada no `Run` existe enquanto o processo existe; depois de um reinício, a tarefa
interrompida não teria função nenhuma para tentar de novo. Com a função registrada por nome, o
botão "tentar de novo" funciona inclusive para o que morreu no deploy — que é justamente
quando alguém aperta esse botão. A chave (`docID`) chega como `p.Key`: o resto o app busca na
tabela dele, como no `c.Link`.

O que o módulo garante:

- a função roda **fora da requisição**, num `context` que não morre quando o navegador fecha e
  que cancela no `Shutdown`;
- `Run` com o mesmo nome e a mesma chave, enquanto uma está viva, devolve a que já existe em
  vez de uma segunda — o duplo-clique no botão não processa o documento duas vezes;
- `panic` vira `error` gravado, com a pilha no log, e nunca derruba o processo;
- `Setup` marca como `interrupted` tudo que ficou `queued` ou `running` na subida. É o bug
  clássico: tarefa `running` para sempre, de um processo que não existe mais;
- `Shutdown` espera as tarefas em andamento até o prazo, e cancela o `context` delas depois;
- `c.Audit` registra quem disparou e como terminou.

| Símbolo | O que é |
|---|---|
| `New(Options) *Tasks`, `(*Tasks).Setup(a)`, `Shutdown(ctx)` | o motor |
| `Handle(nome, Func)`, `Run(c, nome, chave) (id, error)` | registrar e disparar |
| `Func func(ctx, *Progress) error`, `Progress.Step(rótulo, n, de)` | a tarefa e o andamento |
| `Get`, `List(ListParams)`, `Retry(c, id)` | a tela de administração |
| `Store`, `Memory()` | onde o estado mora |
| `ui.TaskProgress(c, tasks, id)`, `ui.TaskTable(c, tarefas, opts)` | as duas telas |
| `ErrUnknownTask`, `ErrNotFound` | nome sem `Handle`, id que não existe |

O `ui.TaskProgress` é um `ui.Poll` que chama `c.PollStop()` quando a tarefa termina: sem
JavaScript ele é o estado de quando a página carregou — velho, nunca quebrado.

## Fora de escopo

- **Fila entre processos.** Está escrito no doc do pacote, não no rodapé: duas réplicas são
  duas filas, e a mesma tarefa roda duas vezes. Quem precisa disso usa uma fila de verdade, e o
  `Store` mais o `Runner` são a costura por onde trocar.
- **`task.SQL(db)`.** A issue pede, e é o único item que fica de fora: nenhum store deste
  repositório traz implementação SQL — `LinkStore`, `auth.KeyStore`, `SettingsStore` e
  `DraftStore` são interface mais memória, com o SQL na receita. Um `SQL()` aqui teria de
  escolher dialeto de placeholder e ser dono de um DDL, que é exatamente o que o framework não
  faz. A receita mostra a implementação inteira.
- **Prioridade, agendamento, retentativa automática.** Agendar é o cron (#112 e o que já
  existe); tentar de novo sozinho esconde a falha em vez de mostrá-la.
- **Aviso do `trilha dev` sobre reiniciar.** O `interrupted` na subida já conta a história no
  lugar certo — a tela —, e um aviso no terminal a cada recarga vira ruído em uma tarde.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `context`, `sync`, `log/slog`, `time` |
| VI — teste primeiro | dedupe, panic, shutdown, interrupted e cancelamento têm teste cada um |
| VII — segurança por padrão | tarefa não herda o `context` do request; `Retry` é POST com CSRF; a tela de administração é do app guardar |
| Convenção nova | rota no `examples/blog` + teste de integração |

## Tarefas

- [x] T001 Teste que falha: `Run` duas vezes com a mesma chave dá um id só
- [x] T002 `Tasks`, `Progress`, `Store`, `Memory`
- [x] T003 Testes: panic vira erro, shutdown espera, cancelamento chega, `interrupted` na subida
- [x] T004 `Setup`, `Shutdown`, `Retry`, `List`
- [x] T005 `ui.TaskProgress` e `ui.TaskTable`
- [x] T006 Uso no `examples/blog` + teste de integração
- [x] T007 Receita en + pt (com o store SQL inteiro), referência en + pt
- [x] T008 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.64.0 --issues "111"`

## Aceitação

- **SC-001** Dois `Run` com o mesmo nome e chave, com uma viva, devolvem o mesmo id.
- **SC-002** `panic` dentro da tarefa vira `state=error` com a mensagem, e o processo continua.
- **SC-003** Fechar o navegador não interrompe a tarefa; o `Shutdown` interrompe.
- **SC-004** Depois de um reinício, nada fica `running`: vira `interrupted`, e o `Retry`
  funciona porque a função está registrada por nome.
- **SC-005** A tela mostra o passo e o N/M, e para de perguntar quando acaba.
- **SC-006** `TestNoExternalDeps` continua verde.
