#!/usr/bin/env bash
# Registra uma falha da automação e decide se a issue continua na fila.
#
# Roda no job `falha`, quando `sessao` ou `verifica` quebraram. Três casos:
#   - a sessão não publicou branch/PR: comenta na issue;
#   - a suíte falhou na PR: comenta na PR com o link da execução;
#   - em qualquer dos dois, se esta era a última tentativa, a issue sai da fila (label
#     `auto-parada`) e a PR, se existir, vira rascunho — nada é mesclado, e o schedule não
#     volta a gastar tokens na mesma issue.
set -euo pipefail

REPO="${REPO:?}"
ISSUE="${ISSUE:?}"
BRANCH="${BRANCH:?}"
PR="${PR:-}"
TENTATIVA="${TENTATIVA:-1}"
MAX_TENTATIVAS="${MAX_TENTATIVAS:-2}"
EXECUCAO="${EXECUCAO:?}"

if [ -z "$PR" ]; then
	PR=$(gh pr list -R "$REPO" --head "$BRANCH" --state open --json number --jq '.[0].number // empty')
fi

ultima=""
if [ "$TENTATIVA" -ge "$MAX_TENTATIVAS" ]; then
	ultima=" Era a última tentativa ($TENTATIVA/$MAX_TENTATIVAS): a issue saiu da fila com a label \`auto-parada\`."
fi

if [ -n "$PR" ]; then
	printf 'A verificação automática falhou: %s\n\nA PR fica aberta e nada é mesclado sem a suíte verde.%s\n' \
		"$EXECUCAO" "${ultima:-" A próxima sessão da issue retoma esta branch com o erro no prompt."}" |
		gh pr comment "$PR" -R "$REPO" --body-file -
else
	printf '🤖 auto · a sessão terminou sem PR (branch não publicada ou sem diferença para a branch padrão): %s\n%s\n' \
		"$EXECUCAO" "${ultima:-"A próxima sessão tenta de novo."}" |
		gh issue comment "$ISSUE" -R "$REPO" --body-file -
fi

if [ -n "$ultima" ]; then
	gh issue edit "$ISSUE" -R "$REPO" --remove-label auto --add-label auto-parada || true
	if [ -n "$PR" ]; then
		gh pr ready "$PR" -R "$REPO" --undo || true
	fi
	printf '🤖 auto · parada: %s tentativas sem PR mesclada. Saiu da fila; alguém precisa olhar. Para tentar de novo, tire a label `auto-parada` e ponha `auto`.\n' \
		"$TENTATIVA" | gh issue comment "$ISSUE" -R "$REPO" --body-file -
fi

echo "falha registrada (issue #$ISSUE, PR ${PR:-nenhuma}, tentativa $TENTATIVA/$MAX_TENTATIVAS)"
