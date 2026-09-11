#!/usr/bin/env bash
# Decide em que issue esta execução trabalha, aplica os guardrails e monta o prompt da
# sessão autônoma.
#
# Chamado pelo job `resolve` de .github/workflows/auto-issue.yml, que passa tudo por variável
# de ambiente (REPO, EVENTO, PEDIDA, ROTULADA, MODELO_OVERRIDE, PADRAO, MAX_TENTATIVAS,
# MAX_SESSOES_DIA) e lê o resultado em $GITHUB_OUTPUT: issue, branch, modelo, prompt,
# tentativa, pr_existente e prosseguir.
#
# `prosseguir=nao` não é erro: é a execução dizendo que não há o que fazer — fila vazia,
# issue já em andamento, teto do dia atingido. Só o `sim` acorda os jobs seguintes.
set -euo pipefail

REPO="${REPO:?}"
EVENTO="${EVENTO:?}"
PEDIDA="${PEDIDA:-}"
ROTULADA="${ROTULADA:-}"
MODELO_OVERRIDE="${MODELO_OVERRIDE:-}"
PADRAO="${PADRAO:-main}"
MAX_TENTATIVAS="${MAX_TENTATIVAS:-2}"
MAX_SESSOES_DIA="${MAX_SESSOES_DIA:-6}"
SAIDA="${GITHUB_OUTPUT:-/dev/stdout}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

pare() {
	echo "prosseguir=nao" >>"$SAIDA"
	echo "$1"
	exit 0
}

# --- guardrail 0: sem segredo, nada anda ---------------------------------------------
# O workflow passa TEM_SEGREDO=true se ANTHROPIC_API_KEY ou CLAUDE_CODE_OAUTH_TOKEN existe.
# Descobrir isso aqui, antes do comentário de tentativa, é o que impede um segredo ausente
# de consumir as tentativas da issue de hora em hora. Falha (e não `pare`) de propósito:
# um run vermelho com o comando exato é o aviso que o dono precisa.
if [ "${TEM_SEGREDO:-true}" != "true" ]; then
	echo "::error::Nenhum segredo de autenticação no repositório. O dono precisa rodar: gh secret set CLAUDE_CODE_OAUTH_TOKEN -R $REPO  (ou ANTHROPIC_API_KEY). Ver .github/AUTOMACAO.md §1."
	echo "prosseguir=nao" >>"$SAIDA"
	exit 1
fi

# --- guardrail 1: teto de sessões por dia no repositório -----------------------------
# Cada sessão começa com um comentário "auto · tentativa" do bot na issue; contar esses
# comentários nas últimas 24 h é contar sessões, sem depender do estado de runs.
sessoes_hoje=$(bash "$ROOT/scripts/auto/contar-tentativas.sh" --dia)
if [ "$sessoes_hoje" -ge "$MAX_SESSOES_DIA" ]; then
	pare "teto do dia atingido: $sessoes_hoje sessões nas últimas 24 h (AUTO_MAX_SESSOES_DIA=$MAX_SESSOES_DIA)"
fi

# --- qual issue ----------------------------------------------------------------------
# Um número explícito vence. Sem ele — o caso do `schedule` — pega a mais antiga com a label
# `auto`, que é a ordem que uma fila tem de ter para ninguém ficar para trás.
n="${PEDIDA:-$ROTULADA}"
if [ -z "$n" ]; then
	n=$(gh issue list -R "$REPO" --label auto --state open --limit 100 \
		--json number --jq 'sort_by(.number) | .[0].number // empty')
	[ -n "$n" ] || pare "nenhuma issue aberta com a label auto"
fi

estado=$(gh issue view "$n" -R "$REPO" --json state --jq .state)
[ "$estado" = "OPEN" ] || pare "issue #$n já está $estado"

labels=$(gh issue view "$n" -R "$REPO" --json labels --jq '[.labels[].name] | join(" ")')
case " $labels " in
*" auto "*) ;;
*) pare "issue #$n não tem a label auto (foi removida entre o disparo e agora)" ;;
esac
case " $labels " in
*" auto-parada "*) pare "issue #$n está parada (label auto-parada); tire a label para retomar" ;;
esac

branch="auto/issue-$n"

# --- guardrail 2: teto de tentativas por issue ---------------------------------------
tentativas=$(bash "$ROOT/scripts/auto/contar-tentativas.sh" "$n")
if [ "$tentativas" -ge "$MAX_TENTATIVAS" ]; then
	gh issue edit "$n" -R "$REPO" --remove-label auto --add-label auto-parada >/dev/null
	printf '🤖 auto · parada: %s tentativas sem PR mesclada (AUTO_MAX_TENTATIVAS=%s). Saiu da fila; alguém precisa olhar. Para tentar de novo, tire a label `auto-parada` e ponha `auto`.\n' \
		"$tentativas" "$MAX_TENTATIVAS" | gh issue comment "$n" -R "$REPO" --body-file -
	pare "issue #$n esgotou as tentativas ($tentativas/$MAX_TENTATIVAS); marcada auto-parada"
fi
tentativa=$((tentativas + 1))

# --- já está sendo trabalhada? -------------------------------------------------------
# A PR aberta é o sinal que sobrevive ao fim da execução que a criou. Rascunho = a sessão se
# declarou bloqueada: para aqui, é caso para uma pessoa. PR pronta e não mesclada = a
# verificação falhou; a issue ganha UMA retomada na mesma branch, com a falha no prompt.
pr_existente=""
aberta=$(gh pr list -R "$REPO" --head "$branch" --state open --json number,isDraft --jq '.[0] // empty')
if [ -n "$aberta" ]; then
	num=$(jq -r .number <<<"$aberta")
	rascunho=$(jq -r .isDraft <<<"$aberta")
	[ "$rascunho" = "false" ] || pare "issue #$n tem a PR #$num em rascunho (sessão bloqueada); espera revisão humana"
	pr_existente="$num"
fi

# Fila em série: uma sessão por vez em todo o repositório, não só por issue. Duas sessões
# em paralelo partiriam da mesma main e a segunda mescla cairia em conflito — e, na
# migração, as issues são dependentes por construção. Regras:
#   - outro run com ESTA issue no nome → fila dupla, para;
#   - outro run em andamento cuja issue tem número MENOR (ou é o schedule "mais antiga")
#     → ele tem prioridade; este para e o encadeamento/schedule volta a esta issue depois.
#     Isso resolve a rajada de N issues rotuladas de uma vez: só a de menor número segue;
#   - outro run com issue de número MAIOR → este segue e aquele é quem para.
# O run predecessor no encadeamento ainda aparece em andamento por alguns segundos depois
# de disparar este (o job mescla está terminando), por isso a espera curta antes de decidir.
if [ -n "${GITHUB_RUN_ID:-}" ]; then
	for _ in 1 2 3 4 5 6 7 8; do
		outros=$(gh run list -R "$REPO" --workflow auto-issue.yml --limit 40 \
			--json databaseId,displayTitle,status \
			--jq ".[] | select(.status==\"in_progress\" or .status==\"queued\" or .status==\"waiting\") \
				| select(.databaseId != $GITHUB_RUN_ID) | \"\\(.databaseId) \\(.displayTitle)\"" 2>/dev/null || true)
		[ -n "$outros" ] || break
		sleep 15
	done
	if [ -n "$outros" ]; then
		grep -q "issue #$n\$" <<<"$outros" && pare "issue #$n já está em execução ($(grep "issue #$n\$" <<<"$outros" | cut -d' ' -f1))"
		while read -r id titulo; do
			[ -n "$id" ] || continue
			outra=$(sed -n 's/.*issue #\([0-9][0-9]*\)$/\1/p' <<<"$titulo")
			if [ -z "$outra" ] || [ "$outra" -lt "$n" ]; then
				pare "fila em série: run $id ($titulo) tem prioridade; esta issue volta pelo encadeamento ou pelo schedule"
			fi
		done <<<"$outros"
	fi
fi

# --- modelo --------------------------------------------------------------------------
# Sonnet dá conta do tamanho de issue que esta automação recebe; `complexa` é o botão para
# quando não dá, e o input manual vence os dois.
modelo="$MODELO_OVERRIDE"
if [ -z "$modelo" ]; then
	modelo=claude-sonnet-5
	case " $labels " in *" complexa "*) modelo=claude-opus-5 ;; esac
fi

titulo=$(gh issue view "$n" -R "$REPO" --json title --jq .title)
gh issue view "$n" -R "$REPO" --json body --jq .body >/tmp/auto-issue-body.md

# --- saída ---------------------------------------------------------------------------
{
	echo "issue=$n"
	echo "branch=$branch"
	echo "modelo=$modelo"
	echo "tentativa=$tentativa"
	echo "pr_existente=$pr_existente"
	echo "prosseguir=sim"
} >>"$SAIDA"

retomada=""
if [ -n "$pr_existente" ]; then
	retomada="
ATENÇÃO — esta é uma RETOMADA. A branch $branch já existe e tem a PR #$pr_existente aberta;
a verificação automática dela falhou. Antes de qualquer coisa leia os comentários da PR
(\`gh pr view $pr_existente --comments\`) e o log da execução apontado neles, corrija a causa
e faça push na mesma branch. Não abra outra PR. Se a causa não estiver ao seu alcance,
converta a PR em rascunho (\`gh pr ready $pr_existente --undo\`) e explique na issue.
"
fi

# O delimitador é aleatório porque o corpo da issue é texto de outra pessoa e pode conter
# qualquer coisa, inclusive a palavra que fecharia o heredoc cedo demais.
fim="FIM_$(openssl rand -hex 8)"
{
	echo "prompt<<$fim"
	cat <<PREAMBULO
Você está executando de forma autônoma no GitHub Actions. Não há humano para responder:
nunca peça confirmação, decida e registre a decisão no corpo da PR.

Regras desta execução (tentativa $tentativa de $MAX_TENTATIVAS):
- Trabalhe na branch $branch (crie a partir de origin/$PADRAO se não existir; se já existir,
  continue nela).
- Commits sem trailer Co-Authored-By, e sem nenhum outro trailer de coautoria. Mensagens em
  português, no padrão do histórico (\`git log --oneline -20\`).
- Leia o CLAUDE.md do repositório e siga as convenções dele. A constituição está em
  .specify/memory/constitution.md.
- Fique no escopo da issue: não refatore o que ela não pede, não abra outras issues, não
  mexa em segredos. Se descobrir algo fora do escopo, registre no corpo da PR.
- NÃO faça release: não mexa em versão, tag, CHANGELOG de versão nem \`make release\` — o
  release é da sessão humana. Registre a mudança no CHANGELOG na seção "Unreleased" se ela
  existir; senão, só no corpo da PR.
- Mudança de API pública precisa de: teste, entrada no API.md se ele documenta o pacote, e
  a convenção da constituição (rota no examples/blog quando for convenção nova).
- Rode \`make test\` (gofmt + go vet + go test ./...) e \`go test -race ./...\` antes de
  terminar. Só termine com a suíte verde — um job independente vai rodá-la de novo e a PR
  não é mesclada se falhar.
- Guardrail contra laço: se a suíte continuar vermelha depois de 3 rodadas de correção sobre
  o mesmo erro, PARE de tentar, faça push do que estiver consistente, abra a PR como
  rascunho e explique o erro na issue. Não repita o mesmo comando esperando outro resultado.
- Ao terminar, faça push e abra a PR para $PADRAO com \`gh pr create\`, título
  "#$n · $titulo" e corpo contendo a linha "Closes #$n". A PR será testada e mesclada
  automaticamente, então escreva o corpo para quem for ler o histórico depois: o que mudou,
  as decisões tomadas e o que ficou de fora.
- Se ficar bloqueado, comente o motivo na issue com \`gh issue comment $n\`, faça push do que
  estiver consistente e abra a PR como rascunho (\`--draft\`) — rascunho é o sinal de "parei
  aqui" e impede a mesclagem.
- Ferramentas disponíveis: go 1.25, git, gh, make. A CLI do próprio repositório roda com
  \`go run ./cmd/trilha <comando>\`.
$retomada
A issue #$n, que é a fonte do escopo:

## $titulo

PREAMBULO
	cat /tmp/auto-issue-body.md
	echo
	echo "$fim"
} >>"$SAIDA"

echo "issue #$n · $titulo · modelo $modelo · branch $branch · tentativa $tentativa/$MAX_TENTATIVAS${pr_existente:+ · retomada da PR #$pr_existente}"
