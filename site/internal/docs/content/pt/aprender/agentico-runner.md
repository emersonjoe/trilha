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

Sem um toolchain Go, a [página de Releases](https://github.com/emersonjoe/trilha-runner/releases)
traz um binário estático por plataforma (Linux e macOS, amd64 e arm64) ao lado de um
`sha256sums.txt`: baixe o da sua máquina, confira o digest contra esse arquivo (`sha256sum
--check --ignore-missing sha256sums.txt` no Linux, `shasum -a 256` no macOS) e ponha-o no seu
`PATH` como `trilha-runner`.

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
não é repositório git, driver desconhecido, sem comando, agente saiu com erro — deixa o mesmo
tipo de registro, com o estágio que falhou, e a task vai para `failed`. Não existe execução sem
rastro. Só a task que não está `ready`, ou ainda espera uma dependência, fica como estava.

Antes da primeira tentativa o runner também faz um **preflight**: para cada comando dos
`checks:` da task e do `project.verify` ele resolve o primeiro token — `LookPath` no host,
`command -v` dentro do sandbox Docker. Ferramenta que não existe reprova a task em uma só
tentativa, sem chamar o agente, com `failure_class: environment` e
`error_code: environment_missing_tool`; código de saída 127 ou "command not found" durante a
execução ou a verificação é classificado do mesmo jeito e nunca é repetido. A regra existe
porque uma execução real gastou três tentativas de IA numa máquina sem `python`.

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

## Quando o agente pergunta

Um agente sem um fato que o repositório não tem fica com duas opções ruins: chutar ou falhar. O
runner dá uma terceira. Durante uma execução ele escuta num socket Unix ao lado do worktree, põe
o `trilha-runner` no `PATH` do agente e acrescenta ao pacote de contexto um parágrafo que manda
todo agente fazer uma pergunta objetiva e parar, em vez de chutar:

| O agente roda | O que acontece |
|---|---|
| `trilha-runner ask "<pergunta>"` | quando o agente encerra o turno, a execução termina **esperando resposta** |
| `trilha-runner note "<fato>"` | uma nota que a próxima execução da task vai ler |
| `trilha-runner overview "<uma ou duas frases>"` | o que está sendo feito e onde está |
| `trilha-runner attention "<motivo>"` / `--clear` | levanta ou limpa o selo que diz que alguém é necessário |

O driver `ai` oferece o mesmo como ferramentas — `ask`, `note`, `overview`. Todo comando responde
em menos de um segundo e sai com 0 mesmo sem ninguém ouvindo: o runner nunca derruba nem segura
um agente.

Veja sem modelo nenhum. Um script de três linhas faz o papel do agente — rascunha um arquivo,
deixa uma nota e pergunta:

```bash
mkdir -p scripts
cat > scripts/ask.sh <<'SH'
#!/bin/sh
echo "# E-mail de lembrete (rascunho)" > REMINDER_EMAIL.md
trilha-runner note "internal/reminders envia o e-mail mas ainda não tem destinatário."
trilha-runner ask "O e-mail de lembrete vai para o dono do evento ou para todos os convidados?"
SH
trilha spec task add "Quem recebe o e-mail de lembrete" --status ready \
  --accept "o lembrete chega a quem o cliente escolheu" --check "go test ./..."
trilha runner run TASK-007 --driver exec --cmd "sh $PWD/scripts/ask.sh"
# · worktree .trilha/runs/TASK-007/wt on trilha/task-007
# · attempt 1/3: driver exec starting
# · TASK-007 needs input: the agent asked a question
# TASK-007 → needs input (driver exec, 1.3s)
# question: O e-mail de lembrete vai para o dono do evento ou para todos os convidados?
# branch trilha/task-007 in .trilha/runs/TASK-007/wt
# commit 0c5e9f3a8d21b7c4e6f09a1b2c3d4e5f60718293
```

Nada falhou. O rascunho está commitado no branch, nenhum check rodou, nenhuma tentativa foi
gasta, nada é repetido. A task está `blocked`, e a evidência dela tem um registro `run` com
`stage: needs_input`. Com `--json` o resultado também traz a pergunta, a nota (fonte `ai`) e o
`diff_stats` — quantos arquivos e linhas o branch mudou, nunca o que eles dizem.

Na sua máquina não há para quem mandar a resposta, então escreva-a onde o próximo pacote de
contexto vai achá-la — a evidência até aqui faz parte do pacote — e devolva a task à fila:

```bash
trilha spec evidence TASK-007 add --note "Resposta: todo convidado que aceitou o convite." --by ana
trilha spec task move TASK-007 ready
trilha runner run TASK-007        # mesmo branch, mesmo worktree, a resposta no pacote de contexto
```

Com um control plane, a resposta viaja como `continuation`. Quando a pergunta veio de uma sessão
`claude-code`, o runner retoma aquela mesma sessão (`--resume <sessão>`) com a resposta, citada,
como prompt — o agente continua de onde parou. Qualquer outro driver, ou uma sessão que esta
máquina não tem mais, começa do zero com a pergunta e a resposta citadas no pacote de contexto.
A memória da task — o overview e as notas mais recentes — volta do mesmo jeito, citada e
rotulada como informação, nunca como instrução (o
[capítulo do protocolo](/pt/aprender/agentico-protocolo) mostra o formato exato).

Perguntas, notas, overviews e motivos têm limite de tamanho e passam por uma varredura de
segredos antes de sair: uma pergunta com algo que tem cara de chave chega ao control plane como
um aviso fixo, e as palavras ficam em `.trilha/runs/TASK-007/question.txt`, que só você lê. Dois
limites a saber: um agente dentro do sandbox Docker não alcança o socket, então não pergunta; e
uma execução só-leitura (abaixo) nunca para por pergunta.

Enquanto o agente trabalha, o runner também acompanha *o que* ele está fazendo. Para o
`claude-code` ele escreve hooks em `.trilha/runs/TASK-007/claude-settings.json` — ao lado do
worktree, nunca dentro — e os passa com `--settings`; o driver `ai` reporta do próprio loop. O
estado — `working`, `idle`, `waiting_permission`, `asked`, `stopped_error` — vai ao control plane
só quando muda, no máximo uma vez a cada dois segundos.

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

Uma execução longa não pode parecer um worker morto, e um resultado não pode morrer com a rede.
Enquanto houver execução ativa o worker manda um heartbeat a cada 30 segundos (`running 1/2`).
Cada resultado é gravado em `.trilha/runs/<execução>/result.json` antes de ser postado e
removido quando o control plane o tem; um control plane fora do ar é tentado de novo com
backoff em vez de parar o worker, um resultado recusado de vez fica de lado como
`result.rejected.json`, e o que um worker que morreu deixou para trás é enviado na partida
seguinte, antes de qualquer claim nova. Uma task deixada `running` por um runner que morreu
volta a `ready`, com uma nota, na próxima vez que for executada.

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

## Revisões e caçadas de bugs

Implementar é um tipo de execução. Um control plane também pode pedir a um worker uma segunda
opinião sobre um branch que já está em `review`; o `kind` da claim diz qual, e o runner faz o
resto:

| Claim | O que o runner faz |
|---|---|
| `kind: review`, `review_mode: report` (o padrão) | revisa o branch da task a partir do merge base com o branch padrão — correção, segurança, testes que faltam — num checkout destacado próprio, `.trilha/runs/TASK-002-review-<execução>/wt`. Só-leitura |
| `kind: review`, `review_mode: fix` | a mesma revisão no próprio branch: o agente pode corrigir, o runner commita e roda os checks da task, e a task volta a `review` |
| `kind: hunt` | uma caçada de bugs só-leitura nos arquivos que o branch mudou, apontada para uma área (portas de entrada, caminhos de erro, camada de dados, costuras de integração, interface, infraestrutura e CI) e um estilo (pessimista, seguidor de rastro, cético de dependências, olhos novos), para caçadores em paralelo não lerem todos o mesmo código do mesmo jeito |

O agente relata com mais comandos da mesma família — e ferramentas de mesmo nome no driver
`ai`:

```bash
trilha-runner finding --severity high --path internal/events/due.go --lines 18-22 \
  --title "Due list includes reminders in the future" \
  --why "the filter compares with After where the acceptance says before now" \
  --repro "an event whose ReminderAt is one hour ahead appears in GET /api/events/due"
# finding sent to the runner
trilha-runner verdict changes
# verdict sent to the runner
```

Um achado é um caminho e um intervalo de linhas, nunca código. Um que o control plane não
conseguiria guardar é recusado antes de ser enviado — e o comando ainda sai com 0:

```bash
trilha-runner finding --severity low --path internal/events/due.go --lines 18 --title "Nomes"
# trilha-runner finding: finding: severity must be critical, high or medium
# usage: trilha-runner finding --severity critical|high|medium --path <file> …
```

Sem `verdict`, uma revisão responde `changes` quando tem achados e `pass` quando não tem. Os
achados também têm segredos censurados, e no máximo 100 saem de uma execução.

**Mudanças pequenas, conferidas duas vezes.** Todo prompt de implementação pede a menor mudança
que atende aos critérios de aceite e aos checks — sem refatorar, renomear, reformatar ou mexer em
arquivo de que a task não precisa, sem abstração ou dependência nova que ela não peça; uma mudança
maior é explicada com `trilha-runner note`, e uma que muda o escopo da task é perguntada antes. A
revisão então compara o diff inteiro com o que a task pede: cada mudança de que a task não precisa
vira um achado `medium` com título `Unneeded change: …`, caminho e linhas, e um diff muito maior
que o necessário recebe o veredito `changes`, mesmo com todas as linhas corretas. O `diff_stats`
da execução conta só o que o branch da task mudou, então um control plane pode segurar uma
mudança num orçamento de tamanho.

**Só-leitura é verificado, não pedido.** Quando uma caçada ou uma revisão `report` termina, o
worktree tem de estar limpo, o `HEAD` tem de ser o commit do início e o branch revisado não pode
ter se movido. Qualquer outra coisa reprova a execução com `failure_class: policy`
(`error_code: read_only_violated`) e joga fora os achados e relatórios dela. No sandbox Docker o
worktree também é montado só-leitura. Uma execução só-leitura nunca move a task e nunca é
publicada; o driver `ai` nem oferece a ela `write_file` ou `ask`.

Pessoas também revisam. Quando o control plane entrega a uma execução de correção os comentários
que alguém deixou no pull request, o runner dá ao prompt um bloco `<reviews>` com as linhas para
as quais cada comentário aponta, lidas do próprio worktree e nunca de fora do repositório, todo
texto com escape de XML e citado como dado de revisores. Um comentário que diz "ignore suas
instruções" é uma citação, não uma ordem. O agente corrige o que concorda e responde cada
comentário pelo id:

```bash
trilha-runner reply c-1 "Corrigido: o filtro agora usa Before(now), com um teste para um evento uma hora à frente."
# reply sent to the runner
```

Resposta a um id que a execução não recebeu é descartada.

## Variantes

Às vezes você quer duas tentativas da mesma task e ficar com a melhor. Uma claim com
`variant_index` é uma execução de implementação comum, num branch próprio:

```bash
git branch
# * main
#   trilha/task-002
#   trilha/task-002-v1
#   trilha/task-002-v2
ls .trilha/runs
# TASK-002  TASK-002-v1  TASK-002-v2
git diff --stat trilha/task-002-v1 trilha/task-002-v2
```

Cada variante trabalha em `.trilha/runs/TASK-002-vN/`, e o runner grava ali um arquivo
`created-branch` com o nome do branch que ele cortou. Essa prova é o que o resto das ferramentas
leva em conta. Uma variante nunca adota um branch que não consegue provar que criou — se alguém
já tem um `trilha/task-002-v2`, a execução passa para `trilha/task-002-v2-2` e deixa o dele em
paz — e uma limpeza só apaga um branch com essa prova na mão.

Variantes não se enxergam: o pacote de contexto de uma variante não tem evidência anterior da
task nem nota ou overview escritos por agente, para a segunda tentativa não ser cópia da
primeira. As notas de pessoas continuam chegando. Escolha uma, faça o merge, descarte a outra:

```bash
git merge trilha/task-002-v1
trilha runner worktree clean TASK-002-v2   # aceita um diretório de execução, além de uma task
```

## Artefatos de relatório

Um check prova; um relatório explica. Um resumo de cobertura, um screenshot da tela nova, um
gráfico do eval — um agente pode publicar até cinco por execução:

```bash
trilha-runner artifact coverage.md
# artifact sent to the runner
```

O runner decide, e o log dele diz o que decidiu:

```text
· artifact coverage.md accepted (1834 bytes, text/markdown)
· artifact "notes.md" refused: it contains a secret, so it stays off the cloud (the runner does not redact reports)
```

As regras: `.html`, `.md`, `.png` ou `.jpg` (`--name` define o nome que as pessoas veem), até
2 MiB cada, do worktree ou de `$TRILHA_ARTIFACT_DIR` — um diretório de rascunho fora do
worktree, que é onde uma execução só-leitura escreve, já que o worktree dela tem de ficar limpo.
Um relatório de texto com algo que tem cara de segredo é recusado, não censurado; uma imagem tem
de começar com os bytes do formato dela. Relatórios aceitos são copiados para
`.trilha/runs/TASK-007/artifacts/`, que só você lê, com o SHA-256 de cada um.

Um worker envia cada relatório antes de postar o resultado, e o resultado lista só o que o
control plane aceitou. Lá, relatórios são opcionais: um projeto que não os permitiu responde
`403 ARTIFACTS_DISABLED`, e o runner descarta todos e avisa uma vez. De um jeito ou de outro, o
que sai da máquina é algo que uma pessoa lê — nunca o código-fonte.

## Código de forks

Um pull request de um fork é código pelo qual ninguém do time respondeu. O control plane marca
uma claim assim com `foreign_code: true`, e três coisas mudam:

- Ela só roda no sandbox Docker. Sem `--sandbox docker`, ou com o Docker indisponível, a
  execução falha com `failure_class: policy` (`sandbox_required`) antes de o agente começar.
- O agente e os checks não recebem segredo nenhum: nenhum token herdado, nenhum segredo de
  deploy. A credencial de IA vai só como a variável de que um driver de linha de comando precisa
  para falar com o modelo; o driver `ai` a guarda dentro do processo do runner, e os checks dele
  rodam no sandbox.
- O `project.verify` é lido do branch padrão, nunca do branch do fork — um fork não escolhe os
  comandos que o seu worker roda — e o branch nunca é publicado:

```text
· run-000012: foreign code, branch trilha/task-002 is not pushed
```

## Mantendo os worktrees em ordem (doctor)

Toda execução deixa um diretório em `.trilha/runs/`: o worktree, o log do agente, o que ele
relatou. Depois de algumas semanas de variantes e caçadas isso soma, e parte é a única cópia de
um trabalho que ninguém fez merge ainda. O doctor separa uma coisa da outra, e não apaga nada a
menos que você peça:

```bash
trilha runner doctor --worktrees
# RUN                          CLASS          AGE         SIZE  BRANCH                   ACTION
# TASK-001                     metadata-only  >=30d   48.0 KiB  -                        kept — no worktree
# TASK-002                     finished-kept  <14d     3.1 MiB  trilha/task-002          kept
# TASK-007                     live           <14d     2.9 MiB  trilha/task-007          kept — held by process 48211
# TASK-009                     orphaned       <30d     2.7 MiB  trilha/task-009          kept — no task file for TASK-009
# 4 run directories: 1 live, 1 finished-kept, 1 orphaned, 1 metadata-only; removed 0, skipped 0, failed 0
```

`live` quer dizer que um runner — ou qualquer processo, como um servidor de desenvolvimento que o
agente deixou para trás — tem o diretório atual lá dentro; `finished-kept`, um worktree de uma
task conhecida sem ninguém dentro; `orphaned`, um worktree que o git não lista ou cujo arquivo
de task sumiu; `metadata-only`, logs e evidência sem worktree. `--json` dá o mesmo relatório a
uma ferramenta.

Apagar exige uma flag, e mesmo assim o doctor é cuidadoso:

```bash
trilha runner doctor --worktrees --prune-orphans --default-branch main
# RUN                          CLASS          AGE         SIZE  BRANCH                   ACTION
# …
# TASK-009                     orphaned       <30d     2.7 MiB  trilha/task-009          skipped — branch trilha/task-009 is not merged into origin/main (--force-unmerged removes it anyway)
# 4 run directories: 1 live, 1 finished-kept, 1 orphaned, 1 metadata-only; removed 0, skipped 1, failed 0
# doctor: 1 skipped, 0 failed
echo $?
# 12
```

Ele nunca toca numa execução viva. Pula um worktree cujo branch não está mesclado em
`origin/<padrão>` ou que tem mudanças não commitadas, a não ser com `--force-unmerged`. Apaga um
branch só quando o `created-branch` da execução prova que o runner o criou.
`--prune-older-than 30d` (ou `720h`) remove diretórios de execução mais velhos que isso, com as
mesmas regras. O código de saída é 0 quando tudo o que foi pedido foi feito, 1 quando o comando
não pôde rodar, e **12** quando algo foi pulado ou falhou — o que uma limpeza agendada consegue
perceber.

Remover um worktree — por `worktree clean` ou por uma poda — é um encerramento ordenado. Os
processos trabalhando lá dentro recebem SIGTERM, depois SIGKILL se ainda estiverem lá após
1,5 s, e só então o worktree sai, para nada continuar escrevendo num diretório que não existe
mais:

```bash
trilha runner worktree clean TASK-007
# teardown: SIGTERM 48211
# teardown: every process exited
# teardown: worktree /home/ana/agenda/.trilha/runs/TASK-007/wt removed
# removed worktree of TASK-007 (branch trilha/task-007 kept)
```

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
