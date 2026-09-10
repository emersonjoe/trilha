# Spec 104 — a receita `tasks`: o trabalho que não cabe numa requisição

- **Issue**: [#116](https://github.com/emersonjoe/trilha/issues/116) — a issue é a fonte do escopo.
- **Branch**: `104-receita-tasks`
- **Versão**: 0.84.0

## Por quê

Mesmo caso da `webhooks`: o módulo `task` existe desde a 0.64.0 — fila, dedupe por chave, retry,
progresso, `ui.TaskTable`, `ui.TaskProgress` — e quem tem um projeto e precisa disso hoje escreve
uma goroutine solta, que é a versão que perde o trabalho no primeiro deploy.

## O que muda

```
$ trilha add tasks
  + internal/trabalho/trabalho.go     o motor, e uma tarefa de exemplo com passos
  + internal/trabalho/trabalho_test.go
  + app/tarefas/page.go               o que rodou, como terminou, e o botão de tentar de novo
  + tarefas_test.go
  ~ app/setup.go                      trabalho.Setup(a)
```

O que a receita escreve, e por que:

- **O motor é do app, num `Provide`.** Cada teste tem o seu, e nenhum teste vê a tarefa do outro.
- **A tarefa de exemplo tem passos**, porque um `Progress` que ninguém chama é uma barra que
  ninguém entende: a tela mostra "2 de 4" porque a tarefa disse.
- **O `Setup` varre o que um processo anterior deixou pendurado** e pendura o `Shutdown` no app —
  um deploy no meio de um processamento espera em vez de cortar.
- **O `Run` deduplica por chave.** Dois cliques no botão devolvem o id da tarefa que já está indo,
  e não uma segunda: é o que separa uma fila de um jeito de dobrar o trabalho.

## Fora de escopo

- **Store em SQL.** Memória aqui, tabela na receita do cookbook, como em todas.
- **Cron.** Tarefa por horário é outra coisa; esta é trabalho disparado por alguém.
- **A tela de uma tarefa só, com `ui.TaskProgress`.** Ela pertence à tela do recurso que disparou
  a tarefa, e essa tela é do app; o `Next` diz a linha.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o módulo `task` e o kit |
| VI — teste primeiro | a receita escreve dois testes, e o e2e da CI aplica ela num projeto novo |
| Determinismo | a receita não inventa concorrência: o motor é o do módulo |

## Tarefas

- [x] T001 Teste que falha: `add tasks` escreve os quatro e liga o `Setup`
- [x] T002 A receita: motor, tarefa de exemplo, tela, testes
- [x] T003 O e2e da CI aplicando `tasks` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.84.0`

## Aceitação

- **SC-001** `trilha add tasks` num projeto novo passa `trilha check` sem uma edição.
- **SC-002** A tela dispara uma tarefa e ela termina.
- **SC-003** Dois disparos com a mesma chave devolvem a mesma tarefa.
- **SC-004** Uma tarefa que falha aparece como falha, com o botão de tentar de novo.
