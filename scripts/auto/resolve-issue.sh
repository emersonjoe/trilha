#!/usr/bin/env bash
# Decide em que issue esta execução trabalha e monta o prompt da sessão autônoma.
#
# Chamado pelo job `resolve` de .github/workflows/auto-issue.yml, que passa tudo por
# variável de ambiente (REPO, EVENTO, PEDIDA, ROTULADA, MODELO_OVERRIDE, PADRAO) e lê o
# resultado em $GITHUB_OUTPUT: issue, branch, modelo, prompt e prosseguir.
#
# `prosseguir=nao` não é erro: é a execução dizendo que não há o que fazer — a fila está
# vazia, ou a issue já tem uma PR aberta. Só o `sim` acorda os jobs seguintes.
set -euo pipefail

REPO="${REPO:?}"
EVENTO="${EVENTO:?}"
PEDIDA="${PEDIDA:-}"
ROTULADA="${ROTULADA:-}"
MODELO_OVERRIDE="${MODELO_OVERRIDE:-}"
PADRAO="${PADRAO:-main}"
SAIDA="${GITHUB_OUTPUT:-/dev/stdout}"

pare() {
	echo "prosseguir=nao" >>"$SAIDA"
	echo "$1"
	exit 0
}

# --- qual issue -------------------------------------------------------------
# Um número explícito vence. Sem ele — o caso do `schedule` — pega a mais antiga com a
# label `auto`, que é a ordem que uma fila tem de ter para ninguém ficar para trás.
n="${PEDIDA:-$ROTULADA}"
if [ -z "$n" ]; then
	n=$(gh issue list -R "$REPO" --label auto --state open --limit 100 \
		--json number --jq 'sort_by(.number) | .[0].number // empty')
	[ -n "$n" ] || pare "nenhuma issue aberta com a label auto"
fi

estado=$(gh issue view "$n" -R "$REPO" --json state --jq .state)
[ "$estado" = "OPEN" ] || pare "issue #$n já está $estado"

branch="auto/issue-$n"

# --- já está sendo trabalhada? ----------------------------------------------
# A PR aberta é o sinal confiável: sobrevive ao fim da execução que a criou, o que o
# estado "in_progress" de um run não faz.
aberta=$(gh pr list -R "$REPO" --head "$branch" --state open --json number --jq '.[0].number // empty')
[ -z "$aberta" ] || pare "issue #$n já tem a PR #$aberta aberta"

# E o run em andamento cobre a janela entre disparar e a PR existir. O próprio run é
# ignorado pelo número; qualquer outro com esta issue no nome significa fila dupla.
if [ -n "${GITHUB_RUN_ID:-}" ]; then
	outros=$(gh run list -R "$REPO" --workflow auto-issue.yml --limit 40 \
		--json databaseId,displayTitle,status \
		--jq ".[] | select(.status==\"in_progress\" or .status==\"queued\" or .status==\"waiting\") \
			| select(.databaseId != $GITHUB_RUN_ID) | select(.displayTitle | test(\"issue #$n\$\")) \
			| .databaseId" 2>/dev/null || true)
	[ -z "$outros" ] || pare "issue #$n já está em execução ($outros)"
fi

# --- modelo -----------------------------------------------------------------
# Sonnet dá conta do tamanho de issue que esta automação recebe; `complexa` é o botão
# para quando não dá, e o input manual vence os dois.
labels=$(gh issue view "$n" -R "$REPO" --json labels --jq '[.labels[].name] | join(" ")')
modelo="$MODELO_OVERRIDE"
if [ -z "$modelo" ]; then
	modelo=claude-sonnet-5
	case " $labels " in *" complexa "*) modelo=claude-opus-5 ;; esac
fi

titulo=$(gh issue view "$n" -R "$REPO" --json title --jq .title)
gh issue view "$n" -R "$REPO" --json body --jq .body >/tmp/auto-issue-body.md

# --- saída ------------------------------------------------------------------
{
	echo "issue=$n"
	echo "branch=$branch"
	echo "modelo=$modelo"
	echo "prosseguir=sim"
} >>"$SAIDA"

# O delimitador é aleatório porque o corpo da issue é texto de outra pessoa e pode
# conter qualquer coisa, inclusive a palavra que fecharia o heredoc cedo demais.
fim="FIM_$(openssl rand -hex 8)"
{
	echo "prompt<<$fim"
	cat <<PREAMBULO
Você está executando de forma autônoma no GitHub Actions. Não há humano para responder:
nunca peça confirmação, decida e registre a decisão no corpo da PR.

Regras desta execução:
- Trabalhe na branch $branch (crie a partir de origin/$PADRAO se não existir; se já
  existir, continue nela).
- Commits sem trailer Co-Authored-By, e sem nenhum outro trailer de coautoria.
- Leia o CLAUDE.md do repositório e siga as convenções dele.
- Rode a suíte antes de terminar: \`make test\` (gofmt + go vet + go test ./...). Só
  termine com ela verde — um job independente vai rodá-la de novo e a PR não é mesclada
  se falhar.
- Ao terminar, faça push e abra a PR para $PADRAO com \`gh pr create\`, título
  "#$n · $titulo" e corpo contendo a linha "Closes #$n". A PR será testada e mesclada
  automaticamente, então escreva o corpo para quem for ler o histórico depois.
- Se ficar bloqueado, comente o motivo na issue com \`gh issue comment $n\`, faça push do
  que estiver consistente e abra a PR como rascunho (\`--draft\`) — rascunho é o sinal de
  "parei aqui" e impede a mesclagem.
- Ferramentas disponíveis: go 1.25, git, gh, make.

A issue #$n, que é a fonte do escopo:

## $titulo

PREAMBULO
	cat /tmp/auto-issue-body.md
	echo
	echo "$fim"
} >>"$SAIDA"

echo "issue #$n · $titulo · modelo $modelo · branch $branch"
