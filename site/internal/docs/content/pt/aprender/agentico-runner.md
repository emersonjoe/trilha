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
# claude-code
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

## Um check que mede: métricas como evidência

Nem toda aceitação é um sim ou um não. Acurácia de triagem num conjunto rotulado, nota de
tradução, p95 de latência, violações de acessibilidade — cada um é um número contra um limiar, e
um número enterrado na saída de um comando é invisível para quem revisa a task. Então a bateria
imprime uma linha de JSON por número e o runner transforma cada uma num registro `eval`:

```bash
cat > eval/triagem.sh <<'SH'
#!/bin/sh
python eval/triagem.py           # o que quer que meça
echo '{"metric":"triage_top1","value":0.87,"threshold":0.85,"comparator":">=","dataset":{"id":"triage-v3","manifest":"eval/golden/manifest.json"}}'
SH
trilha spec task add "Triagem fica acima de 0.85" --status ready \
  --accept "acurácia top-1 de pelo menos 0.85" --check "sh eval/triagem.sh"
trilha runner run TASK-005 --driver echo
# · eval triage_top1=0.87 >= 0.85 (passed=true)
# TASK-005 → review (driver echo, 1.2s)
```

Os comparadores são `>=`, `<=`, `>`, `<`, `==` e `!=`. **Uma métrica que não alcança o limiar
reprova a task mesmo que o script saia com 0** — o código de saída diz que a bateria rodou, a
métrica diz se o resultado é bom o bastante. O `dataset` opcional nomeia um manifesto; o runner
faz o hash do manifesto, nunca dos dados, então um conjunto dourado grande ou privado também
fica fixado. Qualquer outra linha que o script imprima é saída comum.

## O sandbox: quando os checks precisam de serviços

O worktree cerca o que o agente pode *tocar*. Ele não fornece o que os checks *precisam*: uma
suíte de API que quer Postgres falha nele, e com razão. O manifesto do agente declara o que
subir, e `--sandbox docker` sobe:

```markdown
---
name: coder
role: Implementa uma task no worktree dela e produz evidência.
driver: exec
command: claude -p -
tools: [read, write, run]
sandbox: {"image":"golang:1.22","services":[{"name":"postgres","image":"pgvector/pgvector:pg16","env":{"POSTGRES_PASSWORD":"trilha","POSTGRES_DB":"acervo"},"ready":["pg_isready","-U","postgres"]}]}
---
```

```bash
trilha runner run TASK-006 --sandbox docker
# · sandbox: service postgres started
# · sandbox: service postgres ready
# · sandbox: trilha-task-006 running golang:1.22
# · sandbox docker: commands run in /workspace
```

Os serviços sobem numa rede própria e respondem pelo nome — o check conecta em `postgres`, não
numa porta da sua máquina. O agente e os checks rodam num container nessa rede, e a evidência
registra o comando embrulhado, então o registro diz onde rodou.

Quatro coisas são do runner e não do manifesto, de propósito: o worktree é o único caminho
gravável que sobrevive (o sistema de arquivos raiz é somente leitura, `/tmp` morre com o
container), os limites de recurso são fixados pelo runner (uma CPU, 1 GiB de memória, 256 PIDs,
`cap-drop ALL` e `no-new-privileges`), o agente roda como o usuário dono do worktree e não como
root, e nada monta o socket do Docker. Um sandbox que fala com o daemon não
é sandbox. O que foi criado é removido depois, mesmo quando a
execução falhou no meio, então `docker ps` está vazio no fim.

A terceira dessas não é boa educação, é o que faz a primeira funcionar. Derrubar todas as
capabilities tira o `CAP_DAC_OVERRIDE`, e aí root dentro do container para de burlar as
permissões de arquivo — então um agente root não conseguiria escrever no worktree de jeito
nenhum, porque o worktree é seu. Rodar como o dono dele é o que mantém gravável o único caminho
gravável.

Existe uma quarta coisa que você nunca precisa arranjar, e vale saber por quê. A credencial do
seu projeto entra no container **uma vez**, por um arquivo que só você pode ler, quando o
container sobe. Nada é passado na linha do `docker exec`, porque aquela linha é a argv de um
processo na sua própria máquina — e o `ps` mostra os argumentos de um processo para qualquer
usuário dela. Segredo que anda em linha de comando é segredo que você publicou localmente.

Uma máquina sem Docker não perde nada do que tinha: `--sandbox` vale `none` por padrão e o
worktree continua sendo o sandbox.

## Esperando por outro repositório

Uma task de produto pode depender de uma task de framework que mora em outro repositório. Diga
isso em `depends_on` com o alias na frente — `trilha:TASK-004` — e diga ao `next` onde está o
outro checkout:

```bash
trilha runner next --repo trilha=../trilha
# · TASK-005: waiting:trilha:TASK-004 (running)
# error: runner: no task is ready with every dependency done
```

A task não é oferecida, é reportada como `blocked` com o motivo em evidência, e a própria fila a
devolve para `ready` assim que a task do outro repositório estiver `done`. Uma dependência que o
runner não consegue resolver também bloqueia — ele nunca lê como pronto o que não consegue ver.
Um worker conectado ao Cloud resolve a mesma dependência contra o control plane, não contra um
caminho.

## Worker persistente e entrega

O worker conectado ao Cloud pode manter checkouts dedicados em `--workspace-root`, materializar
bundles versionados do Trilha Spec e publicar os branches de spec e implementação com `--push`.

Uma frota raramente é de um tipo só de máquina, então o worker diz o que ele é:

```bash
trilha runner worker --cloud https://cloud.exemplo --token "$TOKEN" --project acervo \
  --label docker --label region:br --capacity 2
```

Os labels e a capacidade viajam no heartbeat e na claim, junto das execuções em voo, para o
control plane mandar uma execução cujos checks precisam de Postgres para o host que tem Docker,
e manter um projeto cujos dados não podem sair do país num worker daquela região. O payload
também leva as execuções em voo e as versões do runner e dos drivers, para uma claim
incompatível ser recusada antes de tocar em qualquer código. Uma execução que este host não
pode honrar é recusada e reportada, nunca executada em silêncio. Com `--capacity 2` o worker
toca duas execuções ao mesmo tempo, cada uma no seu worktree.

A mesma claim pode trazer o acesso ao modelo do próprio projeto — provedor, endpoint, credencial
e os hosts em que essa credencial pode ser gasta. Ele chega ao agente como ambiente do processo e
em nenhum outro lugar — o driver `claude-code` sempre executa `claude -p -` e recebe a
credencial como `CLAUDE_CODE_OAUTH_TOKEN` ou `ANTHROPIC_API_KEY` — e é redigido da saída. Ele
substitui a configuração do próprio worker só nessa execução. Quando o projeto lista hosts
permitidos, um
endpoint fora deles é recusado *antes da primeira requisição*: a evidência é um registro `run`
com `stage: policy` e a task vai para `failed`. Residência é imposta pelo runner, não por
confiar na configuração de cada worker.

Deploy e rollback usam `--delivery-config`: somente comandos cadastrados localmente podem rodar;
o Cloud nunca fornece uma linha de shell. Um perfil que é uma stack `compose` com banco declara
a própria sequência — `steps[]` em ordem, um `migrate` que roda imediatamente antes do passo
marcado `switch`, e `health[]` por serviço — então uma migração que falha aborta a entrega com a
revisão anterior ainda servindo, e um serviço que nunca fica saudável dispara o rollback. O
rollback reverte o schema só quando a migração se declarou reversível; caso contrário é
só-imagem e o log diz que o schema mantém a forma nova.

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
