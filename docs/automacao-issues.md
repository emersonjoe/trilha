# Automação de issues: da label `auto` à PR mesclada

Uma issue com a label `auto` é resolvida sem humano no circuito: o Claude Code escreve o
código no GitHub Actions, um job independente roda a suíte e o lint, a PR é mesclada e a
issue fecha na mesclagem. Este arquivo diz o que o dono do repositório precisa fazer uma
vez, como acompanhar, o que acontece quando trava, e por que cada decisão é a que é.

- Workflow: [`.github/workflows/auto-issue.yml`](../.github/workflows/auto-issue.yml)
- Scripts: [`scripts/auto/`](../scripts/auto/)

## O que o dono precisa fazer uma vez

**1. Gravar o segredo de autenticação.** Um dos dois; a chave da plataforma tem
prioridade quando os dois existem.

```bash
# Chave da plataforma (console.anthropic.com), cobrada por uso:
gh secret set ANTHROPIC_API_KEY -R emersonjoe/trilha

# Ou o token da assinatura Max, gerado por `claude setup-token`:
gh secret set CLAUDE_CODE_OAUTH_TOKEN -R emersonjoe/trilha
```

Sem valor na linha de comando: o `gh` lê do teclado e o segredo não passa pela área de
transferência, pelo histórico do shell nem pela lista de processos. Cole quando ele
pedir e tecle Enter.

**2. Deixar o Actions abrir PR.** Settings → Actions → General → Workflow permissions →
marque *Allow GitHub Actions to create and approve pull requests*. Pela linha de comando:

```bash
gh api -X PUT repos/emersonjoe/trilha/actions/permissions/workflow \
  -f default_workflow_permissions=write -F can_approve_pull_request_reviews=true
```

Sem isso, o `gh pr create` dentro do job falha com `GitHub Actions is not permitted to
create or approve pull requests`.

**3. Nada além disso.** A label `auto` é criada na primeira vez que alguém a aplica, e a
`complexa` também. O workflow não precisa de App, de PAT nem de runner próprio.

## Como usar

| Quero | Faço |
|---|---|
| Resolver uma issue | ponho a label `auto` nela |
| Resolver uma issue agora, sem mexer em label | `gh workflow run auto-issue.yml -f issue=123` |
| Usar o modelo grande | ponho também a label `complexa` (ou `-f modelo=claude-opus-5`) |
| Tirar uma issue da fila | removo a label `auto` |
| Ver o que a fila faria | `bash scripts/auto/proxima-issue.sh --dry-run` |

O agendamento de hora em hora pega a issue `auto` aberta mais antiga que não tenha PR
aberta nem execução em andamento. É rede de segurança, não o caminho normal: o caminho
normal é a label e o encadeamento.

## Como acompanhar

```bash
gh run list --workflow auto-issue.yml -R emersonjoe/trilha   # as execuções
gh run watch -R emersonjoe/trilha                            # acompanha a atual ao vivo
gh run view <id> --log -R emersonjoe/trilha                  # o log inteiro
gh pr list -R emersonjoe/trilha --search "head:auto/issue-"  # as PRs da automação
```

`show_full_output: true` está ligado, então o raciocínio e as ferramentas da sessão saem
no log do job — é lá que se vê o que ela decidiu, não só o que ela escreveu.

## O que acontece quando trava

O desenho tem um lugar só onde a automação para, e ele é visível:

- **A sessão se declarou bloqueada.** Ela comenta o motivo na issue e abre a PR como
  rascunho. O job `mescla` vê `isDraft`, comenta na PR e não mescla. A fila para: a
  próxima issue não é disparada, de propósito — enfileirar em cima de um problema é como
  se perde a tarde.
- **A suíte falhou.** O job `verifica` comenta na PR com o link da execução e sai com
  erro. A PR fica aberta, nada é mesclado, a fila para.
- **A sessão não publicou a branch.** O job falha com `a sessão da issue #n não publicou
  a branch auto/issue-n`. Normalmente é `--max-turns` estourado: releia o log, e se a
  issue era grande demais ponha a label `complexa` e dispare de novo.
- **A sessão publicou a branch e esqueceu a PR.** O passo *Garantir a PR* abre uma. O
  mesmo passo acrescenta `Closes #n` se a PR não tiver — sem isso a issue ficaria aberta
  com o trabalho mesclado.

Para retomar depois de arrumar à mão: `gh workflow run auto-issue.yml -f issue=<n>`, ou
`bash scripts/auto/proxima-issue.sh` para a fila voltar a andar.

## Erros conhecidos

| Sintoma | Causa e solução |
|---|---|
| `Workflow initiated by non-human actor` | A execução foi disparada pelo próprio workflow, e a action recusa bot por padrão. É o que `allowed_bots: "github-actions"` resolve — se voltar, é porque essa entrada saiu do workflow. |
| `401 OAuth access token is invalid` | O segredo foi colado errado (quebra de linha, espaço no fim, token de outra conta). Regrave com `gh secret set`, lendo do teclado; não dá para conferir o valor pelo GitHub depois. |
| `rate_limit_error` num `curl` direto com token de assinatura | Normal, e não indica limite atingido: o token de `claude setup-token` é para a CLI e para a action, não para chamada crua à API. Não é sinal de problema no workflow. |
| `GitHub Actions is not permitted to create or approve pull requests` | Falta o passo 2 acima. |
| A PR foi mesclada e a issue continua aberta | O `Closes #n` não estava no corpo. O `mesclar-e-encadear.sh` fecha à mão nesse caso; se aconteceu, vale ver por que a PR saiu sem a linha. |

## As decisões, e por quê

**A label `auto` é a fronteira de confiança.** O corpo da issue entra no prompt de um
agente com shell, então quem escreve o corpo manda no agente. Aplicar label exige
permissão de escrita no repositório: quem pode rotular já podia commitar. Por isso o
gatilho é `labeled` e não `opened` — uma issue de terceiro não executa nada sozinha.

**O encadeamento é sempre por `workflow_dispatch`.** Uma PR aberta com o `GITHUB_TOKEN`
não dispara outros workflows — é a proteção do GitHub contra recursão — então esperar
pelo evento `pull_request` seria esperar para sempre. `gh workflow run` com o mesmo
token funciona, e é por isso que o workflow pede `actions: write`.

**O job `verifica` repete o que o `ci.yml` já faz.** Pela mesma razão: o `ci.yml` roda em
`pull_request` e não dispara nessas PRs. Sem o `verifica`, a PR chegaria à mesclagem sem
portão nenhum. Ele roda gofmt, `go vet`, `go test ./...`, o módulo `bench` e o detector
de corrida; windows, fuzz e `govulncheck` ficam para o push na branch padrão, que
dispara o `ci.yml` de verdade — são lentos e o que eles pegam não é o que uma sessão
autônoma erra.

**Checkout limpo antes de testar.** O `verifica` clona a branch de novo em vez de
reaproveitar a árvore da sessão: o que é testado é o que está publicado, e não um
arquivo que ficou para trás sem commit.

**Squash, e a branch some.** Uma issue vira um commit na branch padrão. O histórico da
sessão (dez commits de tentativa) não é histórico que alguém vá ler.

**Sonnet por padrão, Opus com `complexa`.** O tamanho de issue que esta automação recebe
não precisa do modelo grande, e a label é o botão para quando precisa. O input `modelo`
vence os dois, para quando se quer testar.

**Uma execução por issue, uma issue por vez.** `concurrency` por número impede duas
execuções na mesma issue; o `proxima-issue.sh` dispara uma só. Fila em série mantém a
branch padrão sempre mesclável e tira o rebase do caminho.

**`prosseguir=nao` não é falha.** Quando não há o que fazer — fila vazia, issue já com
PR, issue já em execução — o `resolve` sai verde e os jobs seguintes não rodam. Execução
vermelha tem de significar problema de verdade, senão ninguém olha mais.

**Sem branch protection.** Hoje a branch padrão não tem regra, e por isso o
`gh pr merge --squash` com o `GITHUB_TOKEN` funciona. Se um dia tiver: o `GITHUB_TOKEN`
não satisfaz *Require a pull request before merging* com aprovação obrigatória, porque
ele não pode aprovar a própria PR. As saídas, em ordem de preferência — (a) exigir
status checks em vez de aprovação humana, e listar o job `verifica` entre eles;
(b) marcar o app do Actions em *Allow specified actors to bypass required pull
requests*; (c) trocar o `GITHUB_TOKEN` por um GitHub App próprio com permissão de
conteúdo e PR, e gerar o token com `actions/create-github-app-token`. A opção (a) é a
única que mantém o portão de teste de pé.
