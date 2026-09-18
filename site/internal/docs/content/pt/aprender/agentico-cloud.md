---
title: Desenvolvimento agentic — o control plane
description: Compartilhe a fila entre um time e uma frota de workers com o trilha-cloud, e como é o contrato se você rodar o seu.
---

Um runner numa máquina serve uma pessoa. Um time com várias máquinas, vários projetos e
agentes trabalhando em paralelo precisa de um lugar que responda *o que está na fila*, *quem
está executando o quê*, *que evidência aquela execução deixou* e *quem pediu* — sem que o
código de projeto nenhum saia da máquina que o guarda. Isso é o **trilha-cloud**, e é privado:
protocolo e runner são abertos para que qualquer ferramenta os fale; o control plane é onde
operar uma frota custa alguma coisa.

```text
                    trilha-spec (protocolo)
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
   trilha-runner      Claude Code      outro runner
          │                │                │
          └────────────────┼────────────────┘
                           ▼
                     trilha-cloud
     projetos · fila · frota · evidência · auditoria
```

O trilha-cloud é ele mesmo um app Trilha — rotas por arquivo, `trilha gen`,
[`auth.APIKeys`](/pt/aprender/autenticacao), `c.Audit` —, o que faz deste capítulo também o
maior exemplo do framework em uso.

## O fluxo

Se você tem acesso ao repositório, um processo é o control plane inteiro:

```bash
export TRILHA_SECRET=$(trilha secret)
export TRILHA_CLOUD_ADMIN_TOKEN=troque-me
export TRILHA_CLOUD_DATA=./data/cloud.json
make dev                                  # http://localhost:3000
```

O operador registra um projeto e emite uma chave para os workers. Administração fica atrás do
token do ambiente — a primeira chave tem que vir de algum lugar — e todo o resto atrás de
chaves com escopo:

```bash
curl -X POST localhost:3000/api/admin/projects -H "Authorization: Bearer troque-me" \
     -d '{"name":"agenda","org":"aprender","repo":"git@github.com:voce/agenda"}'
curl -X POST localhost:3000/api/admin/keys -H "Authorization: Bearer troque-me" \
     -d '{"name":"notebook","project":"agenda","expires_days":30,"scopes":["runs:read","runs:write"]}'
# {"key":"tc_…"}   mostrada uma vez; o segredo é temperado com TRILHA_SECRET e nunca guardado
```

Quem tem chave enfileira uma task — a mesma `TASK-002` dos capítulos anteriores:

```bash
curl -X POST localhost:3000/api/runs -H "Authorization: Bearer tc_…" \
     -d '{"project":"agenda","task_id":"TASK-002"}'
# {"id":"run-000001","status":"queued",…}
```

E um worker — um `trilha-runner` numa máquina com checkout da agenda — pega:

```bash
cd agenda
trilha-runner worker --cloud http://localhost:3000 --token tc_… --project agenda --once
# worker notebook on http://localhost:3000, project agenda
# TASK-002 → review (driver exec, 48s)
# branch trilha/task-002 in .trilha/runs/TASK-002/wt
# ✓ go test ./... (exit 0)
```

O worker pediu a próxima execução, executou localmente exatamente como no
[capítulo do runner](/pt/aprender/agentico-runner) e reportou status, branch, commit e os
registros de evidência. Abra `http://localhost:3000`: o painel mostra o projeto, a execução em
`review` com o branch, e a frota — `notebook`, ocioso, visto há segundos. A aprovação do
revisor é `DELETE /api/runs/run-000001` (review → done): a mesma decisão humana que o
protocolo mantém fora das mãos do agente.

Todo passo está na trilha de auditoria, com a chave que o fez como ator:

```bash
curl localhost:3000/api/admin/audit -H "Authorization: Bearer troque-me" | jq '.[].action'
# "run.closed" "run.finished" "run.claimed" "run.enqueued" "apikey.emitiu" "project.created"
```

## Reproduza o fluxo homologado

O repositório do Cloud contém um aceite executável do ecossistema inteiro, não
um fixture pronto. Cada execução cria uma aplicação e um protocolo novos:

| Componente | O que o aceite exercita |
|---|---|
| `trilha` | `new`, `openapi`, os seis gates de `check`, `ctx`, `build`, a página e a API gerada |
| `trilha-spec` | `init`, `spec new`, `task add`, `task move`, `doctor`, `context` e `evidence` |
| `trilha-runner` | worker do Cloud, worktree isolado, driver `echo`, checks, branch e commit |
| `trilha-cloud` | projeto, chave, fila, claim, resultado, revisão, aprovação, auditoria, portal e persistência |

Rode o aceite completo no checkout do `trilha-cloud`:

```bash
make homologate
# ou inclua o gate do Cloud e o aceite do portal em navegador headless
make check-all
```

O protocolo é gerado pela CLI pública em vez de ser copiado do repositório:

```bash
trilha-spec init "$APP_DIR"
cd "$APP_DIR"
trilha-spec spec new "Homologate the Trilha ecosystem"
trilha-spec task add "Execute the end-to-end ecosystem flow" \
  --spec 001-homologate-the-trilha-ecosystem \
  --agent coder \
  --accept "trilha-runner creates TRILHA_RUN.md inside an isolated worktree" \
  --accept "the generated Trilha application remains green" \
  --check "test -f TRILHA_RUN.md" \
  --check "go test ./..."
trilha-spec task move TASK-001 ready
trilha-spec doctor
trilha-spec context TASK-001 --json
```

Depois o Cloud enfileira a task gerada e chama um worker real:

```bash
trilha-runner worker \
  --cloud http://127.0.0.1:3901 \
  --token tc_… \
  --project homologation-app \
  --name homologation-worker \
  --once \
  --driver echo
```

O aceite exige que `TASK-001` chegue a `review` com o branch
`trilha/task-001`, um commit e evidências aprovadas para
`test -f TRILHA_RUN.md` e `go test ./...`. A mesma evidência é lida por
`trilha-spec evidence TASK-001 --json` e pela API do Cloud. Depois, o script
aprova a execução, valida os seis eventos de auditoria desde a criação do
projeto até `run.closed`, reinicia o plano de controle e confirma a
persistência do status final `done`.

Para exercitar a interface, inicie o Cloud com `make dev`, abra
`http://localhost:3000/?lang=pt-BR`, use **Configurar acesso** para guardar o
token administrativo e a chave gerada na sessão do navegador, registre a
aplicação, enfileire `TASK-001`, consulte as evidências depois que o worker
terminar e selecione **Aprovar**. O código da aplicação permanece no
checkout do worker durante todo o procedimento.

O driver determinístico `echo` mantém este aceite independente de provedor de
modelo. Drivers de IA, sandboxes remotos, billing e implantação de produção
pertencem a ambientes de aceite separados.

A parte de navegador também está disponível como `make homologate-ui`. Ela
opera o portal real pelo Chrome DevTools: configura acesso, registra projeto,
emite uma chave, enfileira execução, abre evidências protegidas e aprova a
revisão.

## Product Studio: crie o produto sem usar CLI

No Trilha Cloud 0.2.0, o operador pode habilitar um workspace gerenciado e o
usuário faz o ciclo inteiro pela interface. A CLI continua existindo por trás
do Cloud, mas deixa de ser uma responsabilidade de quem está criando o produto.

O operador inicia o Cloud uma vez, informando onde os projetos podem ser
criados e quais binários homologados serão usados:

```bash
cd trilha-cloud
mkdir -p data workspaces
export TRILHA_SECRET="$(trilha secret)"
export TRILHA_CLOUD_ADMIN_TOKEN='troque-por-um-token-administrativo'
export TRILHA_CLOUD_DATA="$PWD/data/cloud.json"
export TRILHA_CLOUD_WORKSPACE_ROOT="$PWD/workspaces"
export TRILHA_BIN="$(command -v trilha)"
export TRILHA_RUNNER_BIN="$(command -v trilha-runner)"
make dev
```

Abra `http://localhost:3000/?lang=pt-BR`, selecione **Configurar acesso** e
informe o token administrativo. Depois selecione **Novo produto** e preencha:

- nome `cadastro-usuarios` e organização `trilha`;
- descrição `Cadastro de usuários com login e troca de senha`;
- módulo `example.com/trilha/cadastro-usuarios`;
- template `app`, idioma `Português` e driver `Echo` para uma homologação
  determinística — use `AI` somente quando o provedor estiver configurado no
  ambiente do runner.

![Formulário do Product Studio para criar a aplicação](/docs/agentic-cloud/cloud-product-studio-create.png "O usuário descreve o produto na UI; nenhum comando de geração é digitado por ele.")

Ao selecionar **Criar produto**, o Cloud executa operações fixas e auditáveis:

1. o **Trilha** gera a aplicação `app`, que já inclui login, convite, cadastro,
   perfil e troca de senha;
2. o **Trilha Spec** inicializa `.trilha/`, grava a spec aprovada
   `001-product-foundation` e cria `TASK-001` pronta;
3. o Cloud inicializa o Git e registra o ponto de partida;
4. ao selecionar **Iniciar build**, o **Trilha Runner** abre o worktree,
   executa a task e devolve branch, commit, log e evidências ao **Trilha Cloud**.

Para revisar, use **Emitir chave** com os escopos `runs:read` e `runs:write`,
selecione **Usar nesta sessão**, abra **Detalhes** e confira cada check. O botão
**Aprovar** encerra `review → done` e grava `run.closed` na auditoria.

![Evidências da execução aprovada no Trilha Cloud](/docs/agentic-cloud/cloud-product-studio-evidence.png "A UI mostra task, worker, branch, commit, checks e log antes da decisão humana.")

O modo gerenciado é opt-in. Nomes são validados, cada workspace precisa ser
filho direto de `TRILHA_CLOUD_WORKSPACE_ROOT`, entradas do usuário nunca viram
shell e os segredos do Cloud são removidos do ambiente dos processos filhos.
Somente credenciais do provedor necessárias ao driver `AI` chegam ao runner.

Operadores reproduzem os aceites do mesmo fluxo com:

```bash
make homologate-studio  # Trilha → Spec → Runner → Cloud → aprovação
make homologate-ui      # fluxo real do portal em Chrome
make check-all          # segurança + ecossistema + Product Studio + UI
```

## Tutorial: cadastro de usuários de ponta a ponta

Este roteiro cria uma aplicação Trilha real com login próprio, convite de
usuários e troca de senha; descreve o trabalho com `trilha-spec`; executa a
task com `trilha-runner`; e usa o `trilha-cloud` para fila, evidências e
aprovação. O Cloud recebe metadados da execução, nunca o código-fonte da
aplicação.

Use dois diretórios irmãos e três terminais: um para o Cloud, um para o app e
outro para o worker. Os exemplos reservam `localhost:3000` para o Cloud e
`localhost:3100` para a aplicação.

### 1. Inicie e configure o Trilha Cloud

Com um checkout autorizado do repositório `trilha-cloud`:

```bash
cd trilha-cloud
mkdir -p data
export TRILHA_SECRET="$(trilha secret)"
export TRILHA_CLOUD_ADMIN_TOKEN='troque-por-um-token-administrativo'
export TRILHA_CLOUD_DATA="$PWD/data/cloud.json"
make dev
```

Abra `http://localhost:3000/?lang=pt-BR`. Em **Configurar acesso**, informe
primeiro o mesmo `TRILHA_CLOUD_ADMIN_TOKEN`. A API key pode ficar vazia até ser
emitida. As credenciais ficam somente no `sessionStorage` daquele navegador e
são enviadas como Bearer tokens às APIs do Cloud.

![Diálogo Configurar acesso do Trilha Cloud](/docs/agentic-cloud/cloud-configure-access.png "Configure o token administrativo e, depois de emiti-la, a API key do worker.")

### 2. Gere a aplicação de cadastro de usuários

Em outro terminal, na pasta que guardará o projeto:

```bash
trilha new cadastro-usuarios \
  --module example.com/cadastro-usuarios \
  --template app \
  --lang pt \
  --agents
cd cadastro-usuarios
```

O template `app` já entrega o fluxo que será homologado:

| Rota | Responsabilidade |
|---|---|
| `/entrar` | confere e-mail e senha e abre a sessão |
| `/admin/usuarios` | lista pessoas, define papel e emite convite |
| `/convite/{token}` | deixa a pessoa convidada definir a primeira senha |
| `/perfil` | altera nome, solicita troca de e-mail, troca senha e lista sessões |
| `/sair` | encerra a sessão atual |

Os pontos centrais ficam em `app/entrar/page.go`,
`app/admin/usuarios/page.go`, `app/convite/token_/page.go` e
`app/perfil/page.go`. O primeiro administrador é semeado por ambiente:

```bash
export TRILHA_SECRET='use-um-segredo-com-pelo-menos-32-bytes'
export ADMIN_EMAIL='admin@example.com'
export ADMIN_PASSWORD='senha-segura-123'
```

### 3. Configure o gate `make check`

`trilha check` executa os gates do framework (`gen`, `gofmt`, `vet`, `test`,
`audit` e, quando existe um documento versionado, `openapi`). Um `Makefile`
pequeno deixa o binário substituível em CI sem duplicar a política:

```make
TRILHA ?= trilha

.PHONY: check
check:
	$(TRILHA) check
```

Rode o gate com as mesmas variáveis usadas pela aplicação:

```bash
TRILHA_SECRET="$TRILHA_SECRET" \
ADMIN_EMAIL="$ADMIN_EMAIL" \
ADMIN_PASSWORD="$ADMIN_PASSWORD" \
make check
```

Para testar outra versão da CLI sem editar o arquivo:

```bash
make check TRILHA='go run github.com/emersonjoe/trilha/cmd/trilha@v0.123.0'
```

### 4. Descreva a entrega com `trilha-spec`

Inicialize o protocolo e gere a spec:

```bash
trilha-spec init .
trilha-spec spec new "Cadastro e autenticação de usuários"
```

Complete `.trilha/project.md` com os comandos do projeto e edite
`.trilha/specs/001-cadastro-e-autenticacao-de-usuarios.md` para registrar o
problema, a mudança, o que ficou fora e os critérios de aceite. Em seguida,
gere uma task executável:

```bash
trilha-spec task add "Validar cadastro login e troca de senha" \
  --spec 001-cadastro-e-autenticacao-de-usuarios \
  --agent coder \
  --accept "administrador consegue entrar" \
  --accept "administrador consegue convidar usuário" \
  --accept "usuário convidado define a primeira senha e consegue entrar" \
  --accept "usuário consegue trocar a própria senha" \
  --check "go test ./..." \
  --check "make check"

trilha-spec task move TASK-001 ready
trilha-spec doctor
trilha-spec context TASK-001 --json
```

`doctor` valida a estrutura do protocolo. `context` mostra exatamente o pacote
que o runner entregará ao agente: projeto, constituição, spec, task e perfil do
agente.

### 5. Versione o ponto de partida

O runner cria um worktree e uma branch por task, portanto precisa de um
repositório Git com um commit limpo:

```bash
git init
git add .
git commit -m "inicia aplicação de cadastro de usuários"
```

### 6. Registre o projeto no portal

No Trilha Cloud, selecione **Registrar projeto** e informe:

- **Nome:** `cadastro-usuarios`;
- **Organização:** o identificador do seu time, por exemplo `trilha`;
- **Repositório:** a URL Git ou uma referência local compreensível para o
  operador, por exemplo `local:///workspace/cadastro-usuarios`.

![Diálogo Registrar projeto](/docs/agentic-cloud/cloud-register-project.png "O Cloud identifica o projeto; o checkout continua na máquina do worker.")

O mesmo passo pode ser automatizado:

```bash
curl -X POST http://localhost:3000/api/admin/projects \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"cadastro-usuarios","org":"trilha","repo":"local:///workspace/cadastro-usuarios"}'
```

### 7. Emita a chave do worker

Selecione **Emitir chave**, use o nome `cadastro-usuarios-worker` e mantenha os
escopos `runs:read` e `runs:write`. O segredo `tc_…` aparece uma única vez;
guarde-o em um gerenciador de segredos. No portal, **Usar esta chave** também a
preenche na configuração da sessão atual.

![Diálogo Emitir API key](/docs/agentic-cloud/cloud-issue-key.png "A chave do worker precisa ler a fila e publicar o resultado.")

Pela API, a operação equivalente é:

```bash
curl -X POST http://localhost:3000/api/admin/keys \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"cadastro-usuarios-worker","project":"cadastro-usuarios","expires_days":30,"scopes":["runs:read","runs:write","deployments:write","secrets:read"]}'
# copie o campo key da resposta para TRILHA_CLOUD_API_KEY
```

### 8. Enfileire a task e conecte o runner

No portal, selecione **Nova execução**, escolha `cadastro-usuarios` e informe
`TASK-001`. Pela API:

```bash
export TRILHA_CLOUD_API_KEY='tc_…'
curl -X POST http://localhost:3000/api/runs \
  -H "Authorization: Bearer $TRILHA_CLOUD_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"project":"cadastro-usuarios","task_id":"TASK-001"}'
```

No checkout da aplicação, execute um worker. O driver `echo` torna esta
homologação determinística: ele prova o protocolo, o worktree, o commit e os
checks sem depender de um provedor de IA.

```bash
cd cadastro-usuarios
TRILHA_SECRET="$TRILHA_SECRET" \
ADMIN_EMAIL="$ADMIN_EMAIL" \
ADMIN_PASSWORD="$ADMIN_PASSWORD" \
trilha-runner worker \
  --cloud http://localhost:3000 \
  --token "$TRILHA_CLOUD_API_KEY" \
  --project cadastro-usuarios \
  --workspace-root /var/lib/trilha-runner/workspaces \
  --repo git@github.com:trilha/cadastro-usuarios.git \
  --default-branch main \
  --push \
  --name cadastro-usuarios-worker \
  --once \
  --driver echo
```

O resultado esperado é `TASK-001 → review`, uma branch
`trilha/task-001`, um commit e evidências aprovadas para `go test ./...` e
`make check`. Abra **Detalhes**, confira os comandos, códigos de saída, branch,
commit e log; só então selecione **Aprovar**.

![Evidências da execução em revisão](/docs/agentic-cloud/cloud-run-review.png "A revisão mostra a task, o worker, a branch, o commit, cada check e o log antes da aprovação.")

![Execução concluída no portal](/docs/agentic-cloud/cloud-run-done.png "Depois da aprovação, a execução fica concluída e o worker volta a ficar ocioso.")

### 9. Teste login, cadastro por convite e troca de senha

Inicie a aplicação em uma porta diferente da usada pelo Cloud:

```bash
cd cadastro-usuarios
TRILHA_SECRET="$TRILHA_SECRET" \
ADMIN_EMAIL="$ADMIN_EMAIL" \
ADMIN_PASSWORD="$ADMIN_PASSWORD" \
PORT=3100 trilha dev
```

Abra `http://localhost:3100/entrar` e entre com `admin@example.com` e
`senha-segura-123`.

![Login da aplicação gerada](/docs/agentic-cloud/users-login.png "O administrador inicial vem de ADMIN_EMAIL e ADMIN_PASSWORD.")

Abra `http://localhost:3100/admin/usuarios`, informe o e-mail e o nome da
pessoa e selecione **Convidar**. A pessoa nasce inativa e sem senha; o
administrador recebe um link temporário, não escolhe a senha dela.

![Cadastro de usuário por convite](/docs/agentic-cloud/users-admin.png "A tela mostra o link de uso único e a pessoa ainda inativa.")

Abra o link `/convite/{token}` em uma janela privada. A pessoa convidada define
uma senha com pelo menos 12 caracteres; o token é consumido, a conta fica ativa
e o navegador volta para o login.

![Definição da primeira senha](/docs/agentic-cloud/users-invite.png "A senha nasce com o próprio usuário e o link deixa de valer depois do uso.")

Depois de entrar, abra `http://localhost:3100/perfil`. No card **Senha**,
informe a atual e uma nova senha. A aplicação encerra as sessões e exige novo
login com a senha nova.

![Troca de senha no perfil](/docs/agentic-cloud/users-password.png "A troca pede a senha atual e encerra a sessão ao concluir.")

![Confirmação após trocar a senha](/docs/agentic-cloud/users-password-changed.png "O logout após a troca confirma que a nova credencial precisa ser usada.")

### 10. Limites deste exemplo

O template usa stores em memória para manter o exemplo pequeno. Reiniciar a
aplicação apaga usuários, convites, sessões e alterações de senha; o
administrador inicial volta a ser criado a partir das variáveis de ambiente.
Antes de produção, substitua esses stores por persistência real, configure o
envio de e-mail para convites e troca de endereço, use HTTPS e guarde os
segredos fora do repositório.

Para repetir o aceite automatizado do próprio Cloud, incluindo navegador
headless, rode no repositório `trilha-cloud`:

```bash
make check-all
```

### 11. Opere ambientes, segredos, deploy e rollback pela UI

Depois que o worker estiver conectado, a operação diária não exige terminal:

1. Em **Specs**, selecione **Executar** numa task pronta. O Cloud cria a execução e entrega ao
   runner um bundle versionado com projeto, repositório, rodada, etapas, critérios e checks.
2. O runner atualiza um checkout dedicado, materializa `.trilha/` usando o Trilha Spec, executa
   a task num worktree e publica os branches `trilha/spec-*` e `trilha/task-*` quando `--push`
   está habilitado.
3. Em **Ambientes**, crie `production`, informe a URL HTTPS e o nome do perfil permitido no
   runner, por exemplo `production`.
4. Em **Segredos**, informe uma variável por linha (`NOME=valor`). O navegador só envia os
   valores; o Cloud grava AES-256-GCM e depois mostra apenas os nomes.
5. Selecione **Publicar**, informe um commit ou tag imutável e acompanhe o estado. Depois de uma
   entrega bem-sucedida, **Rollback** agenda a revisão anterior.

O perfil de entrega fica somente na VPS, em `/etc/trilha-runner/delivery.json`:

```json
{
  "profiles": {
    "production": {
      "deploy": ["/usr/local/libexec/trilha/deploy-product"],
      "rollback": ["/usr/local/libexec/trilha/rollback-product"],
      "health_url": "https://cadastro-usuarios.eoslab.com.br/health/ready",
      "timeout_seconds": 600
    }
  }
}
```

O Cloud não envia comandos, chaves Git nem acesso ao socket Docker. O serviço roda como usuário
dedicado, sem sudo e com escrita restrita a `/var/lib/trilha-runner`. O processo de entrega
recebe um ambiente mínimo, e qualquer segredo que apareça na saída é mascarado antes do log ser
devolvido ao Cloud.

![Ambientes e entregas no portal](/docs/agentic-cloud/cloud-environments.png "O ambiente mostra apenas nomes de segredos, revisão atual, perfil permitido e ações auditáveis.")

![Portal responsivo em 390 pixels](/docs/agentic-cloud/cloud-mobile.png "Specs, Kanban, ambientes, execuções e frota continuam operáveis em uma tela móvel.")

### 12. Pause um projeto e deixe o disjuntor puxar o freio

Cancelar para uma execução. Pausar para um projeto: as execuções na fila continuam na fila, na
mesma ordem, os workers seguem enviando heartbeat, e `POST /api/runs/next` responde `204` para
aquele projeto até um humano retomar. Execuções gerenciadas iniciadas em **Specs** recebem
`409` enquanto o projeto está pausado.

1. Em **Projetos**, selecione **Pausar** no projeto. O diálogo pede um motivo (até 240
   caracteres) e uma confirmação; o motivo vai para a auditoria e é o que a próxima pessoa lê
   antes de retomar.
2. A linha mostra o selo **Pausado**, o motivo, quem pausou e quando. A página **Visão geral**
   do produto mostra o mesmo no painel **Freio da frota**.
3. Selecione **Retomar** e confirme. A fila volta a ser entregue na mesma ordem. Retomar é
   sempre uma decisão humana; nada retoma um projeto automaticamente.

O mesmo freio, pela API:

```bash
curl -sS -X POST "$CLOUD/api/admin/projects/cadastro-usuarios/pause" \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"reason":"a spec 004 muda o schema; segurar até a revisão"}'

curl -sS -X POST "$CLOUD/api/admin/projects/cadastro-usuarios/resume" \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN"
```

As duas rotas exigem o token de admin ou uma sessão de operador. Uma chave de worker — mesmo
com `runs:write` — recebe `403`: o worker entrega trabalho, não decide se a frota trabalha.

**O disjuntor** pausa o projeto sozinho quando os números dizem que ele está queimando dinheiro.
Na **Visão geral** do produto, abra **Configurar disjuntor** e defina os limiares que quiser;
zero desativa um limiar:

| Limiar | Dispara quando |
|---|---|
| `max_cost_per_hour` | o custo estimado das tentativas concluídas na última hora passa dele |
| `max_failure_rate` | nas últimas `failure_window` execuções concluídas (padrão 10) a fração de falhas passa dele; espera a janela encher |
| `max_repeated_failure_class` | esse número de tentativas seguidas falhou com o mesmo `failure_class` |

```bash
curl -sS -X PUT "$CLOUD/api/admin/projects/cadastro-usuarios/breaker" \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"max_cost_per_hour":5,"max_failure_rate":0.5,"failure_window":10,"max_repeated_failure_class":3}'
```

Ao cruzar um limiar, o projeto é pausado com o motivo `breaker:<limiar>` e
`paused_by: circuit breaker`; um humano não consegue escrever um motivo que comece com
`breaker:`, então a trilha nunca mente sobre quem puxou o freio. O registro de auditoria
`project.paused` carrega o limite, o valor medido e os sinais daquele momento
(`cost_last_hour`, `failure_rate`, `repeated_failure_class`), os mesmos números que o painel
**Freio da frota** e `GET /api/admin/products/{name}/metrics` mostram. Toda pausa, retomada e
mudança de limiar está em `GET /api/admin/audit`.

### 13. Evidência além do exit code: evals, atestações e quórum

Um check que sai com `0` prova que o comando rodou. Não prova que o modelo continua preciso,
nem que as pessoas que precisam assinar assinaram. Três coisas fecham essa lacuna.

**Evals com limiar.** Um registro de evidência com `kind: "eval"` carrega a métrica em `meta`.
O runner reporta como qualquer outra evidência; o Cloud aplica o gate em
`POST /api/runs/{id}/result` e marca o run como `failed` quando o valor não atinge o limiar,
seja qual for o `passed` que o worker declarou:

```json
{"kind":"eval","task":"TASK-002","seq":2,"passed":true,
 "meta":{"metric":"triage_top1","value":"0.71","threshold":"0.85","comparator":">="}}
```

O run termina com `failure_class: eval_below_threshold`, `error_code: EVAL_BELOW_THRESHOLD` e a
mensagem `eval below threshold: triage_top1=0.71 >= 0.85`. Comparadores: `>=` (padrão), `>`,
`<=`, `<`, `==`.

**Uma política de revisão com quórum.** Decida quantas atestações, de quais papéis, e se
precisam ser assinadas:

```bash
curl -sS -X PUT "$CLOUD/api/admin/projects/cadastro-usuarios/review" \
  -H "Authorization: Bearer $TRILHA_CLOUD_ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"quorum":2,"roles":["uat"],"require_signature":true}'
```

A partir daí `DELETE /api/runs/{id}` responde `409 control: review quorum is incomplete: 0 of 2 attestations,
missing roles uat` até o quórum ser atingido. `GET /api/runs/{id}/quorum` mostra quem atestou e
o que falta.

**Atestações assinadas.** Cada revisor tem um par de chaves Ed25519; a chave pública é
registrada no projeto (`POST /api/admin/projects/{name}/attestation-keys` com `{id, public_key,
owner}`, base64 da chave de 32 bytes). A assinatura cobre o JSON canônico
`{"at":…,"by":…,"role":…,"run":…,"statement":…,"task":…}` com chaves ordenadas, então uma
assinatura nunca serve para outro run:

```bash
curl -sS -X POST "$CLOUD/api/runs/$RUN/attest" \
  -H "Authorization: Bearer $WORKER_KEY" -H 'Content-Type: application/json' \
  -d '{"by":"maria","role":"uat","statement":"Aceito em UAT em 2026-09-18",
       "at":"2026-09-18T14:00:00Z","key_id":"uat-maria","signature":"<base64>"}'
```

Uma atestação sem assinatura só é aceita pela sessão do operador no navegador; uma chave sem
assinatura recebe `403` e um registro de auditoria `run.attestation_refused`. Uma atestação por
par `(by, role)`.

**Tendência e exportação.** O quadro da spec mostra um gráfico por rodada (tarefas concluídas,
checks aprovados, evals dentro do limiar, média de cada métrica) renderizado como SVG no
servidor; os mesmos números vêm de `GET /api/admin/specs/{id}/trend`. Para um auditor,
`GET /api/admin/projects/{name}/evidence?format=csv&spec=<id>` baixa as linhas de evidência e
atestação exatamente como a tela lista (`format=json` para máquinas).

**Métricas e alertas.** Inicie o Cloud com `TRILHA_METRICS=/_trilha/metrics` e, em produção,
`TRILHA_OBS_TOKEN_FILE` apontando para um segredo de 32+ bytes. O Prometheus coleta
`trilha_cloud_runs{status}`, `trilha_cloud_queue_age_seconds{project}`,
`trilha_cloud_breaker_open{project}`, `trilha_cloud_workers{state}`,
`trilha_cloud_worker_last_seen_seconds{worker,project}` e `trilha_cloud_runs_awaiting_quorum`.
`deploy/eoslab/compose.observability.yml` traz Prometheus e Grafana com regras versionadas: um
worker calado por cinco minutos dispara `TrilhaWorkerStopped`, um disjuntor aberto dispara
`TrilhaBreakerOpen`, uma fila com mais de quinze minutos dispara `TrilhaQueueAging`.

## O contrato

O worker só precisa de três rotas, então outro control plane — o seu — pode implementá-las:

| Método | Caminho | Corpo | Resposta |
|---|---|---|---|
| POST | `/api/runs/next` | `{worker, project}` | `200 {id, project, task_id}`, ou `204` quando não há nada na fila |
| POST | `/api/runs/{id}/result` | `{passed, status, branch, commit, evidence[], log, error}` | `202` |
| POST | `/api/workers/heartbeat` | `{name, project, status}` | `200` |

Todas com `Authorization: Bearer <chave>`. O corpo do resultado é o `queue.Result` do
`trilha-runner`; `evidence[]` é o registro do protocolo, sem mudar. O código não viaja.

## O que está no MVP e o que não está

| | Entregue | Depois |
|---|---|---|
| Control plane | projetos, specs, rodadas, kanban, execução e revisão | organizações e times, billing |
| Frota | workers persistentes por projeto, checkout Git isolado, sincronização `.trilha` e push opcional | paralelismo configurável e sandboxes remotos |
| Entrega | ambientes, segredos cifrados, perfis locais, deploy, health check e rollback | estratégias progressivas e aprovações múltiplas |
| Governança | login, CSRF, chaves persistentes com escopo/projeto/expiração/revogação, auditoria, gate de eval, atestações assinadas com quórum, exportação de evidência e métricas Prometheus | SSO e evidência assinada pelo runner |
| Armazenamento | snapshot JSON atômico a cada escrita | SQL e alta disponibilidade |

## Desafio

Escreva o menor control plane que um `trilha-runner worker --once` aceita: um app Trilha com
as três rotas acima, uma fila que é um slice em memória, e ainda sem autenticação. Rode o
worker contra ele com o driver `echo`.

:::solucao
Três arquivos em `app/api/` de um projeto novo (`trilha new fila`):

```go
// app/api/runs/next/route.go
package next

var fila = []string{"TASK-004"} // os ids de task a entregar, em ordem

func POST(c *trilha.Ctx) error {
	var in struct{ Worker, Project string }
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	if len(fila) == 0 {
		c.Status(204)
		return nil
	}
	id := fila[0]
	fila = fila[1:]
	return c.JSON(200, map[string]string{"id": "run-1", "project": in.Project, "task_id": id})
}
```

```go
// app/api/runs/id_/result/route.go
package result

func POST(c *trilha.Ctx) error {
	var in map[string]any
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	c.Log().Info("resultado", "run", c.Param("id"), "status", in["status"], "commit", in["commit"])
	c.Status(202)
	return nil
}
```

```go
// app/api/workers/heartbeat/route.go
package heartbeat

func POST(c *trilha.Ctx) error { return c.JSON(200, map[string]string{"ok": "1"}) }
```

`trilha gen && trilha dev` num terminal; na agenda,
`trilha-runner worker --cloud http://localhost:3000 --token x --project agenda --once --driver echo`.
O worker reivindica a `TASK-004`, roda e posta o resultado que aparece no log. Um slice não é
uma fila que dois workers compartilham, e um `Authorization` que ninguém confere não é um
control plane — que é exatamente a lista do que o trilha-cloud acrescenta.
:::
