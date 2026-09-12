---
title: Desenvolvimento agentic — o protocolo
description: Descreva a próxima feature da agenda como tasks que um agente executa, com critérios de aceite, checks e evidência, usando o trilha-spec.
---

O [capítulo anterior](/pt/aprender/ia-e-agentes) pôs agentes *dentro* da agenda: um chat,
ferramentas, um servidor MCP. Este e os dois seguintes viram a pergunta ao contrário: como um
agente trabalha *na* agenda — pega uma task, faz no próprio branch, prova, entrega a um revisor?

O Trilha responde com três ferramentas, e este capítulo é sobre a primeira:

| Ferramenta | O que é | Onde |
|---|---|---|
| **trilha-spec** | o protocolo aberto: o que fazer, em arquivos que qualquer agente lê | [github.com/emersonjoe/trilha-spec](https://github.com/emersonjoe/trilha-spec) |
| **trilha-runner** | como roda na sua máquina: worktree, agente, checks, evidência | [github.com/emersonjoe/trilha-runner](https://github.com/emersonjoe/trilha-runner) |
| **trilha-cloud** | o control plane de um time e de uma frota de workers | privado |

O protocolo não depende de nada — nem deste framework —, então o diretório `.trilha/` que ele
escreve é legível pelo Claude Code, pelo Codex, por um script ou por uma pessoa com um editor.

## Instalar e inicializar

```bash
go install github.com/emersonjoe/trilha-spec/cmd/trilha-spec@latest
cd agenda
trilha-spec init --name agenda --description "Agenda de eventos da trilha Aprender"
```

A partir do Trilha 0.124, `trilha spec …` é a mesma chamada que `trilha-spec …`: a CLI do
framework entrega qualquer comando que não conhece ao `trilha-<nome>` do seu `PATH`, como o
`git` faz. As duas grafias aparecem abaixo; use a que preferir.

```text
.trilha/
├── .gitignore        ignora runs/ e cache/; o resto é commitado
├── project.md        o que é este projeto, para um agente que acabou de chegar
├── constitution.md   as regras que toda task obedece
├── specs/            o que construir e por quê
├── tasks/            TASK-001.md … as unidades executáveis
├── agents/           coder.md, reviewer.md — quem pode executar, com o quê
├── context/          documentos extras que todo agente recebe
└── evidence/         a prova que cada task produziu
```

:::nota
O `trilha dev` guarda o cache de build em `.trilha/cache/`, que o framework ignora sozinho. Se
você inicializou o protocolo com um Trilha anterior ao 0.124, rode `trilha-spec doctor` depois
do primeiro `trilha dev`: ele avisa quando o `.gitignore` do cache está escondendo o protocolo
do git.
:::

Abra `.trilha/project.md` e preencha as duas coisas que um agente não adivinha — como rodar e
como testar:

```markdown
---
name: agenda
description: Agenda de eventos da trilha Aprender
default_agent: coder
verify:
  - go vet ./...
  - go test ./...
---

# agenda

Um app Trilha: as rotas moram em `app/`, `trilha gen` regenera o `trilha_gen.go`,
`trilha check` é o portão. Roda com `trilha dev`.
```

`verify` é a lista de comandos que toda task roda além dos próprios checks.

## Uma especificação e suas tasks

A feature: **lembretes** — um evento pode ter um lembrete, e a API lista os eventos cujo
lembrete venceu.

```bash
trilha spec new "Lembretes de evento"
# created 001-lembretes-de-evento (.trilha/specs/001-lembretes-de-evento.md)
```

Escreva o *porquê* e o *o quê* nesse arquivo; as tasks apontam para ele. Depois corte em
trabalho que um agente pega sozinho:

```bash
trilha spec task add "Campo de lembrete no Event" --spec 001-lembretes-de-evento --status ready \
  --accept "Event tem ReminderAt (time.Time) e o formulário aceita" \
  --check "go test ./internal/events/..."

trilha spec task add "GET /api/events/due" --spec 001-lembretes-de-evento --status ready \
  --depends TASK-001 \
  --accept "a rota responde os eventos cujo ReminderAt é anterior a agora" \
  --accept "trilha openapi --check passa" \
  --check "go test ./..." --check "trilha openapi --check"
```

Cada task é um arquivo Markdown com um front matter pequeno — legível, diffável, promptável:

```markdown
---
id: TASK-002
title: GET /api/events/due
status: ready
spec: 001-lembretes-de-evento
depends_on:
  - TASK-001
acceptance:
  - a rota responde os eventos cujo ReminderAt é anterior a agora
  - trilha openapi --check passa
checks:
  - go test ./...
  - trilha openapi --check
created: "2026-09-12T14:03:11Z"
---
```

`checks` são programa e argumentos, nunca um shell — um check que precisa de pipe diz isso com
`sh -c "…"`. É o que torna a evidência confiável: o que rodou é exatamente o que está escrito.

## O grafo decide o que roda

```bash
trilha spec task next
# TASK-001  Campo de lembrete no Event
trilha spec task move TASK-002 running
# error: task TASK-002: cannot run, waiting on TASK-001
trilha spec task graph
```

`next` responde as tasks `ready` com toda dependência `done`, em ordem de dependência. Uma task
percorre uma vida estrita — `idea → spec → ready → running → verify → review → done`, com
`blocked` e `failed` como as duas saídas — e uma transição que pula etapa é recusada. O grafo
sai em Mermaid (`--dot` para Graphviz), então cabe num README ou num pull request.

## O que o agente recebe

```bash
trilha spec context TASK-001
```

O *pacote de contexto* é um documento Markdown só, na ordem que um leitor precisa: o projeto,
a constituição, o manifesto do próprio agente (papel, ferramentas permitidas, restrições), a
especificação, a task com critérios de aceite e checks, as dependências e seus status, a
evidência até aqui, e todo arquivo de `.trilha/context/`. `--json` é o mesmo para uma
ferramenta. Termina dizendo ao agente para não marcar a task como done — isso é decisão do
revisor.

Faça a primeira task você mesmo agora, à mão, do jeito que o próximo capítulo deixará um
agente fazer:

```bash
trilha spec task move TASK-001 running
# … acrescente ReminderAt em internal/events e no formulário, com teste …
trilha spec task move TASK-001 verify
trilha spec verify TASK-001
# ✓ go test ./internal/events/... (exit 0)
# ✓ go vet ./... (exit 0)
# ✓ go test ./... (exit 0)
# evidence: 3 record(s) in .trilha/evidence/TASK-001
# TASK-001 is now review
```

## Evidência

Cada check deixou um registro JSON — o comando, onde rodou, o código de saída, a saída e seu
SHA-256, quem rodou e quando:

```bash
trilha spec evidence TASK-001
# #1   ✓ check    trilha-spec verify   go test ./internal/events/... (exit 0)
# #2   ✓ check    trilha-spec verify   go vet ./... (exit 0)
# #3   ✓ check    trilha-spec verify   go test ./... (exit 0)
trilha spec evidence TASK-001 add --note "Revisei o diff; o formulário valida a data." --by ana
trilha spec task move TASK-001 done
trilha spec task next
# TASK-002  GET /api/events/due
```

Um registro nunca é editado; correção é registro novo. Essa é a ideia inteira do protocolo:
uma task está pronta quando seus critérios de aceite têm evidência, não quando um chat diz.

## O mesmo por MCP

Tudo acima está disponível a qualquer host MCP — Claude Code, Cursor, o `ai.Agent` do
capítulo anterior — por um servidor stdio:

```json
{ "mcpServers": { "trilha": { "command": "trilha-spec", "args": ["mcp", "--write"] } } }
```

Só leitura por padrão (`trilha_list_tasks`, `trilha_get_task`, `trilha_next`, `trilha_context`,
`trilha_graph`); `--write` acrescenta `trilha_move`, `trilha_evidence` e `trilha_verify`.
Ferramenta não oferecida não pode ser chamada — a mesma postura do
[`trilha mcp`](/pt/referencia/cli#trilha-mcp).

O protocolo completo — formatos de arquivo, tabela de transições, esquema da evidência — está
em [docs/pt-BR/protocol.md](https://github.com/emersonjoe/trilha-spec/blob/main/docs/pt-BR/protocol.md).

## Desafio

Acrescente uma terceira task, "E-mail de lembrete", que depende de **ambas** TASK-001 e
TASK-002, com um check que ainda não pode passar. Mostre que `task next` não a lista enquanto
TASK-002 está aberta, leve TASK-002 até `done`, e então verifique a task nova e veja-a cair em
`failed` com a evidência que diz por quê.

:::solucao
```bash
trilha spec task add "E-mail de lembrete" --spec 001-lembretes-de-evento --status ready \
  --depends TASK-001,TASK-002 \
  --accept "um e-mail sai quando um lembrete vence" \
  --check "go test ./internal/reminders/..."
trilha spec task next            # só TASK-002: TASK-003 espera por ela
trilha spec task move TASK-002 running
trilha spec task move TASK-002 verify
trilha spec verify TASK-002 && trilha spec task move TASK-002 done
trilha spec task next            # TASK-003
trilha spec task move TASK-003 running
trilha spec task move TASK-003 verify
trilha spec verify TASK-003
# ✗ go test ./internal/reminders/... (exit 1)
# TASK-003 is now failed
trilha spec evidence TASK-003    # o registro carrega a saída do compilador e seu hash
trilha spec task move TASK-003 ready   # failed → ready é o único caminho de volta
```
`failed` não é beco sem saída: a evidência diz o que consertar, e `ready` põe a task de volta
na fila para a próxima tentativa — sua ou, no próximo capítulo, de um agente.
:::
