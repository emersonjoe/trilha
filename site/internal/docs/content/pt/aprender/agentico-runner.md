---
title: Desenvolvimento agentic — o runner
description: Deixe um agente executar uma task da agenda no próprio worktree, verificada, com evidência, usando o trilha-runner.
---

O [protocolo](/pt/aprender/agentico-protocolo) diz o que fazer. O **trilha-runner** é como
roda numa máquina sua:

```text
Task → Agente → Worktree → Execução → Verificação → Evidência
```

Ele pega a task que está pronta, faz checkout de um branch para ela, entrega ao agente o pacote
de contexto, commita o que o agente mudou, roda os checks *no worktree*, grava cada resultado
como evidência e move a task para `review` — ou para `failed`. Sua cópia de trabalho nunca é
tocada.

## Instalar e ensaiar

```bash
go install github.com/emersonjoe/trilha-runner/cmd/trilha-runner@latest
cd agenda
trilha runner drivers
# ai
# echo
# exec
```

Antes de gastar um token, veja o pipeline com o driver que não tem modelo. O `echo` só escreve
a primeira linha do prompt em `TRILHA_RUN.md`; todo o resto — branch, commit, checks,
evidência, status — é de verdade:

```bash
trilha spec task add "Fumaça" --status ready --accept "o runner funciona" \
  --check "sh -c \"test -f TRILHA_RUN.md\""
trilha runner run TASK-004 --driver echo
# · worktree .trilha/runs/TASK-004/wt on trilha/task-004
# · driver echo starting
# · committed 3f9c1a2b7e01
# · TASK-004 is now review (3 checks, passed=true)
# TASK-004 → review (driver echo, 210ms)
# branch trilha/task-004 in .trilha/runs/TASK-004/wt
# ✓ sh -c "test -f TRILHA_RUN.md" (exit 0)
# ✓ go vet ./... (exit 0)
# ✓ go test ./... (exit 0)
```

Olhe o que ficou:

```bash
git branch                       # trilha/task-004
ls                               # sem TRILHA_RUN.md aqui: o trabalho está no branch
trilha runner worktree list
trilha spec evidence TASK-004    # três checks e um registro `run`
```

O registro `run` carrega o driver, o branch, o commit, o diff stat e o fim da saída do agente,
com um ponteiro para `.trilha/runs/TASK-004/agent.log`. Uma execução que nem chega a partir —
sem comando, agente saiu com erro — deixa o mesmo tipo de registro, com o estágio que falhou, e
a task vai para `failed`. Não existe execução sem rastro.

```bash
trilha spec task move TASK-004 done      # ou: ready, para rodar de novo no mesmo branch
trilha runner worktree clean TASK-004    # remove o checkout, mantém o branch
```

## Um agente de verdade: o driver exec

O agente é qualquer comando que lê um prompt no stdin e trabalha no diretório atual. O
manifesto dele, `.trilha/agents/coder.md`, diz como iniciá-lo e o que ele pode fazer:

```markdown
---
name: coder
role: Implements a task inside its own worktree and produces evidence.
driver: exec
command: claude -p -
tools:
  - read
  - write
  - run
constraints:
  - Stay inside the worktree of the task.
  - Run the checks listed in the task before reporting.
---
```

`codex exec -`, `aider --message-file -` ou um script seu funcionam do mesmo jeito. Agora a
segunda task da spec de lembretes, a que acrescenta `GET /api/events/due`:

```bash
trilha spec task next            # TASK-002  GET /api/events/due
trilha runner next               # roda com o comando do manifesto
```

O agente recebe o pacote de contexto — projeto, constituição, a especificação, a task, o que a
TASK-001 já fez — no stdin, trabalha em `.trilha/runs/TASK-002/wt`, e quando sai o runner
commita, roda `go test ./...` e `trilha openapi --check` nesse worktree e grava o resultado.
`TRILHA_TASK` e `TRILHA_WORKTREE` estão no ambiente do agente.

Revise como qualquer branch:

```bash
git diff main..trilha/task-002
trilha spec evidence TASK-002
trilha spec task move TASK-002 done      # aprovar
git merge trilha/task-002                # ou abra um pull request a partir do branch
```

## O driver ai: um modelo com ferramentas cercadas

Sem um agente de linha de comando, o driver `ai` roda o loop de agente do
[pacote `ai`](/pt/aprender/ia-e-agentes) deste framework em qualquer modelo que fale o
protocolo OpenAI — Ollama na sua máquina inclusive:

```bash
export OPENAI_BASE_URL=http://localhost:11434/v1 OPENAI_API_KEY=ollama TRILHA_AI_MODEL=qwen2.5-coder
trilha runner run TASK-002 --driver ai
```

O modelo recebe `read_file` e `list_files` sempre, `write_file` só quando o manifesto lista
`write`, `run` só quando lista `run` — todas recusam caminho fora do worktree — e, com
`trilha-spec` no `PATH`, as ferramentas MCP só-leitura do protocolo, para olhar sozinho as
dependências e a evidência da task. A cerca é a mesma seja qual for o driver: o worktree, as
ferramentas do manifesto e os checks.

## O que o runner não faz

Não faz push, não abre pull request, não roda várias tasks ao mesmo tempo nem põe o agente num
container. Os dois primeiros são a próxima spec do runner; os dois últimos são o
[control plane](/pt/aprender/agentico-cloud). A costura já existe: `sandbox.Sandbox` prepara um
ambiente para um job, e o único sandbox do runner local é o worktree.

## Desafio

Dê à agenda um agente `reviewer` que só pode ler, e use as restrições do manifesto para que um
coder que esqueça de rodar os testes seja pego pelos checks, não por uma pessoa. Depois quebre
um teste de propósito e rode a task: onde ela para, e o que a evidência diz?

:::solucao
O `.trilha/agents/reviewer.md` já existe desde o `init` com `tools: [read]`; um coder não
consegue pular os checks porque quem os roda é o runner, não o agente — é por isso que
`checks:` mora na task e não no prompt. Para ver:

```bash
# faça um teste falhar no worktree de que o agente vai partir
sed -i 's/want := 3/want := 4/' internal/events/events_test.go
trilha spec task move TASK-002 ready
trilha runner run TASK-002 --driver echo
# ✗ go test ./... (exit 1)
# TASK-002 is now failed
trilha spec evidence TASK-002 --json | jq '.[-2].output' | head
```
O passo do agente "deu certo" (echo sempre dá); a *verificação* falhou, no worktree, e o
registro guarda a saída do teste. A task está `failed`, o branch guarda a tentativa, e
`task move TASK-002 ready` enfileira a próxima. Conserte o teste e rode de novo: o runner
reaproveita o mesmo worktree e o mesmo branch.
:::
