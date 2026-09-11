# Automação de issues — da label `auto` à PR mesclada sem revisão humana

Uma issue deste repositório que recebe a label `auto` é resolvida pelo Claude Code dentro do
GitHub Actions: ele escreve o código numa branch, um job independente roda gofmt + `go vet` +
`go test` + `-race` (os mesmos passos do `ci.yml`), a PR é mesclada por squash e o `Closes #n`
fecha a issue. Este documento é o manual e o registro das decisões.

Peças: `.github/workflows/auto-issue.yml` e `scripts/auto/*.sh`. É a mesma automação do
`emersonjoe/farol` (`docs/automacao-issues.md` lá), que por sua vez veio da PR #161 deste
repositório e do `emersonjoe/verba`. A sessão automática **nunca faz release**: versão, tag e
`make release` continuam com a sessão humana ("um dono da `main` por vez"); a automação só
mescla PRs pequenas na `main`, e o próximo release humano as leva junto.

## 1. O que o dono do repositório faz uma vez

1. **Gravar o segredo de autenticação** — um dos dois, lendo do teclado (o valor não passa
   pela área de transferência nem pelo histórico do shell):

   ```bash
   gh secret set CLAUDE_CODE_OAUTH_TOKEN -R emersonjoe/trilha
   ```

   Cole o token gerado por `claude setup-token` (assinatura Max) quando o `gh` pedir e
   termine com Enter e Ctrl-D. Ou, para cobrar por uso na plataforma (tem prioridade quando
   os dois existem):

   ```bash
   gh secret set ANTHROPIC_API_KEY -R emersonjoe/trilha
   ```

   Conferir que o valor não ficou vazio: `gh secret list -R emersonjoe/trilha` mostra a data,
   não o tamanho — então o teste é disparar uma issue pequena (seção 4). Um segredo vazio
   dá o erro "Either ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN, or workload identity
   federation … is required" já no início do passo do Claude.

2. **Permitir que o Actions abra PRs** — Settings → Actions → General → Workflow permissions
   → "Read and write" e marcar "Allow GitHub Actions to create and approve pull requests".
   Por linha de comando (confira com `gh api repos/emersonjoe/trilha/actions/permissions/workflow`):

   ```bash
   gh api -X PUT repos/emersonjoe/trilha/actions/permissions/workflow -f default_workflow_permissions=write -F can_approve_pull_request_reviews=true
   ```

3. **Labels** (criar se não existirem: `gh label create auto -R emersonjoe/trilha` etc.): `auto` (gatilho), `complexa` (modelo maior),
   `auto-parada` (saiu da fila).

4. **Opcional — variáveis do repositório** para ajustar os tetos sem editar o workflow:

   ```bash
   gh variable set AUTO_MAX_TENTATIVAS -R emersonjoe/trilha --body 2
   gh variable set AUTO_MAX_SESSOES_DIA -R emersonjoe/trilha --body 6
   gh variable set AUTO_MAX_USD -R emersonjoe/trilha --body 15
   ```

## 2. Como funciona

| Etapa | Job | O que faz |
|---|---|---|
| Gatilho | — | `issues: labeled` com a label `auto`; `workflow_dispatch` com `issue` (e `modelo` opcional); `schedule` de hora em hora (`23 * * * *`) que pega a issue `auto` mais antiga sem PR em andamento. |
| Escolha | `resolve` | `scripts/auto/resolve-issue.sh`: aplica os guardrails (seção 3), escolhe a issue, decide o modelo (`claude-sonnet-5`; `claude-opus-5` se a issue tem `complexa`; o input manual vence) e monta o prompt = preâmbulo de autonomia + título + corpo da issue. |
| Sessão | `sessao` | Go 1.25, comentário "🤖 auto · tentativa k/N" na issue, `anthropics/claude-code-action@v1` com `--max-turns 400`, `--max-budget-usd`, ferramentas Bash/Read/Edit/Write/MultiEdit/Glob/Grep/WebFetch. Depois `garantir-pr.sh` abre a PR se a sessão não abriu e garante o `Closes #n`. |
| Portão | `verifica` | Checkout limpo da branch; gofmt, `go vet`, `go test ./...`, `go test -race ./...`, `cd bench && go test ./...`. É o único portão da PR: uma PR aberta pelo `GITHUB_TOKEN` não dispara o `ci.yml`. |
| Mescla | `mescla` | `mesclar-e-encadear.sh`: PR em rascunho → para e marca `auto-parada`; senão `gh pr merge --squash --delete-branch`, tira a label `auto`, confere que a issue fechou e chama `proxima-issue.sh`, que dispara **uma** próxima issue por `gh workflow run`. |
| Falha | `falha` | `registrar-falha.sh`: comenta na PR (link da execução) ou na issue; se era a última tentativa, `auto-parada` e PR em rascunho. |

O preâmbulo do prompt diz à sessão: branch `auto/issue-<n>`; commits sem `Co-Authored-By`;
ler o `CLAUDE.md`; ficar no escopo; **não fazer release**; rodar `make test` e `-race` antes de terminar; parar após 3
rodadas de correção sobre o mesmo erro; `gh pr create` com título `#<n> · <título>` e
`Closes #<n>`; se bloquear, comentar na issue e abrir como `--draft`; nunca pedir confirmação.

Permissões do workflow: `contents`, `pull-requests`, `issues`, `actions: write` e
`id-token: write`. A `main` tem ruleset que proíbe merge commit (squash é permitido) — a
PR #161 desta mesma automação foi mesclada pelo `GITHUB_TOKEN` por squash, então o caminho
funciona. Se o ruleset passar a exigir review ou checks, o `gh pr merge` vai falhar com "not
mergeable": a saída é dar bypass ao bot no ruleset ou trocar o `GITHUB_TOKEN` do job `mescla`
por um token de app/PAT com permissão de bypass — o `ci.yml` não roda em PR do bot, então
um check obrigatório nunca ficaria verde sozinho.

## 3. Guardrails contra laço infinito e gasto inútil

O risco real não é uma sessão que roda para sempre — isso `--max-turns` e `timeout-minutes`
resolvem. É a **fila** repetindo a mesma issue que falha de hora em hora, ou uma sessão que
publica uma branch vazia e é contada como progresso. Cada guardrail existe por um caso desses.

| Guardrail | Onde | Decisão |
|---|---|---|
| Uma execução por issue | `concurrency` + `resolve` | Grupo `auto-issue-<n>`; o `resolve` ainda recusa se há PR aberta ou outro run com `issue #n` no nome (janela entre disparar e a PR existir). |
| Uma sessão por vez no repositório | `resolve` | Se outro run do workflow está em andamento, este só segue se a issue dele tem número **maior**; se é menor (ou é o schedule), este para sem gastar nada e volta pelo encadeamento/schedule. Rotular oito issues de uma vez dispara oito runs, mas só o de menor número vira sessão. Duas sessões paralelas partiriam da mesma `main` e a segunda mescla cairia em conflito. |
| Teto de tentativas por issue | `resolve`, `falha` | Cada sessão comenta "🤖 auto · tentativa k/N" **antes** do passo do Claude; o `resolve` conta esses comentários (`contar-tentativas.sh`). Ao atingir `AUTO_MAX_TENTATIVAS` (padrão **2**), a issue perde `auto`, ganha `auto-parada`, a PR vira rascunho e a fila segue. Contar comentários e não runs faz o número sobreviver a runner morto e cancelamento. |
| Retomada única | `resolve` | PR aberta e **não** rascunho = a verificação falhou. A issue ganha uma retomada na mesma branch com o erro no prompt ("leia os comentários da PR"). Rascunho = sessão se declarou bloqueada → para até um humano. |
| Teto de sessões por dia | `resolve` | Soma dos comentários "tentativa" do bot nas últimas 24 h em todo o repositório ≥ `AUTO_MAX_SESSOES_DIA` (padrão **6**) → `prosseguir=nao`. O schedule tenta de novo na hora seguinte; nada é perdido, só adiado. |
| Sessão limitada | `sessao` | `--max-turns 400`, `--max-budget-usd` (`AUTO_MAX_USD`, padrão 15; só conta com chave da plataforma — com token de assinatura o custo é 0 e o freio que vale é o de turnos), `timeout-minutes: 150`. |
| Branch sem diff | `garantir-pr.sh` | Branch publicada sem diferença para a `main` não vira PR: o job falha e a tentativa conta. |
| Laço dentro da sessão | prompt | "Se a suíte continuar vermelha depois de 3 rodadas de correção sobre o mesmo erro, pare, abra a PR como rascunho e explique na issue." |
| Encadeamento em série | `proxima-issue.sh` | Dispara **uma** issue por vez, sempre por `workflow_dispatch`. Fila em série mantém a `main` mesclável e o gasto previsível; o schedule é a rede de segurança se um encadeamento morrer. |
| Segredo ausente | `resolve` | O workflow passa `TEM_SEGREDO` e o `resolve` falha com o comando exato **antes** do comentário de tentativa. Sem isso, um segredo vazio consumiria as duas tentativas de cada issue sem produzir nada. |
| Fronteira de confiança | `resolve` (`if:`) | Só a label `auto` dispara. Quem pode rotular pode escrever no repositório; o corpo da issue vira prompt de um agente com shell. Nunca trocar o gatilho para `opened`. |
| Escopo | prompt | Não refatorar fora da issue, não abrir outras issues, não tocar em `deploy/` nem em segredos. |

Para **retomar** uma issue parada: tire `auto-parada`, ponha `auto`. Os comentários antigos
continuam contando — se quiser zerar, apague-os ou suba `AUTO_MAX_TENTATIVAS`.

## 4. Como acompanhar

```bash
gh run list -R emersonjoe/trilha --workflow auto-issue.yml --limit 10
```

```bash
gh run watch -R emersonjoe/trilha <run-id>
```

```bash
gh run view -R emersonjoe/trilha <run-id> --log-failed
```

Estado da fila sem disparar nada:

```bash
REPO=emersonjoe/trilha bash scripts/auto/proxima-issue.sh --dry-run
```

Disparar uma issue específica, ou com outro modelo:

```bash
gh workflow run auto-issue.yml -R emersonjoe/trilha -f issue=12 -f modelo=claude-opus-5
```

Cada issue mostra a trilha inteira nos comentários do bot: tentativa, falha, parada. Cada
PR mesclada tem o corpo escrito pela sessão com as decisões.

## 5. Quando trava

| Sintoma | O que aconteceu | O que fazer |
|---|---|---|
| Issue com `auto-parada` | Duas tentativas sem PR mesclada, ou a sessão abriu a PR como rascunho. | Ler o comentário da sessão na issue/PR, corrigir ou detalhar a issue, tirar `auto-parada` e pôr `auto`. |
| PR aberta, não rascunho, sem mesclar | A suíte falhou no `verifica` (comentário com o link). | Esperar a retomada do schedule, ou disparar `gh workflow run … -f issue=n`. |
| Run vermelho já no job `resolve` com "Nenhum segredo de autenticação" | Segredo ausente. Nenhuma tentativa foi consumida e nenhum token gasto. | Seção 1, item 1. O schedule repete o aviso de hora em hora até o segredo existir. |
| Nada acontece ao rotular | Permissões do Actions em "read", ou workflow desativado. | Seção 1, item 2; `gh workflow list`. |
| `resolve` diz "teto do dia atingido" | ≥ `AUTO_MAX_SESSOES_DIA` sessões em 24 h. | Esperar, ou subir a variável. |
| Run com `prosseguir=nao` e fila cheia | Todas as issues `auto` têm PR aberta ou estão rodando. | Normal; o schedule volta. |

## 6. Erros já conhecidos

- **"Workflow initiated by non-human actor"** — a execução foi disparada pelo próprio
  workflow (schedule ou dispatch do encadeamento) e a action recusou. Solução já aplicada:
  `allowed_bots: "github-actions"` no passo da action.
- **"401 OAuth access token is invalid"** — o segredo `CLAUDE_CODE_OAUTH_TOKEN` foi colado
  errado (com espaço, quebra de linha, ou o token de outra conta). Regravar com
  `gh secret set CLAUDE_CODE_OAUTH_TOKEN -R emersonjoe/trilha` lendo do teclado.
- **"Either ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN, or workload identity federation …
  is required"** — o segredo não existe ou está vazio (aconteceu na Trilha em 11/09/2026:
  `gh secret list` mostrava o segredo, mas o valor chegava vazio no job). Regravar.
- **`rate_limit_error` num `curl` direto** com o token de assinatura — é o comportamento
  normal do token fora do Claude Code; não indica limite atingido nem token inválido.
- **PR do bot não dispara o `ci.yml`** — proteção do GitHub contra recursão. Por isso o job
  `verifica` existe e por isso o encadeamento é por `gh workflow run`, nunca por
  `pull_request`.

## 7. Decisões registradas

- **Squash na mesclagem**: uma issue = um commit na `main`, com o corpo da PR como mensagem.
  O histórico de tentativas da sessão não tem valor na `main`.
- **Modelo padrão Sonnet 5**, Opus 5 só com `complexa`: o custo por sessão cai muito e o
  tamanho de issue que esta fila recebe cabe no Sonnet; a label é o botão para quando não cabe.
- **Mesmos passos do `ci.yml`** no portão: uma verificação que só existe no workflow é uma
  verificação que ninguém roda local.
- **`-race` no portão** além do `make test`: o `ci.yml` roda os dois em jobs separados, e
  uma PR do bot não passa pelo `ci.yml`; então o `verifica` repete os dois.
- **Sem release automático**: a fila mescla na `main`, e só. Versão e tag são decisão da
  sessão humana que fecha a spec do dia.
- **Tentativas contadas por comentário**, não por run, e comentário escrito antes de gastar
  tokens: é a contagem que sobrevive a cancelamento e runner morto.
- **2 tentativas, 6 sessões/dia**: uma retomada resolve o caso comum (teste esquecido); a
  segunda falha é sinal de issue mal especificada, e mais tentativas só gastariam. Seis
  sessões de até 150 min cobrem um dia de fila em série com folga.
- **Sem branch protection na `main`** enquanto a automação é a única a mesclar; o dia em que
  houver, o item da seção 2 explica o que muda.
- **Encadeamento só por dispatch**, uma por vez; o schedule é rede de segurança e não motor.
