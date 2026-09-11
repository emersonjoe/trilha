#!/usr/bin/env bash
# Mescla a PR da sessão, limpa a label e chama a próxima issue da fila.
#
# Roda no job `mescla`, depois de `verifica` — chegar aqui já significa suíte verde.
set -euo pipefail

REPO="${REPO:?}"
PR="${PR:?}"
ISSUE="${ISSUE:?}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# Rascunho é o jeito de a sessão dizer "fiquei bloqueado". A PR fica para uma pessoa e o
# encadeamento para aqui de propósito: enfileirar a próxima esconderia o problema.
if [ "$(gh pr view "$PR" -R "$REPO" --json isDraft --jq .isDraft)" = "true" ]; then
	gh pr comment "$PR" -R "$REPO" --body "PR em rascunho: a sessão se declarou bloqueada. O encadeamento para aqui até alguém olhar."
	exit 0
fi

gh pr merge "$PR" -R "$REPO" --squash --delete-branch

# O "Closes #n" fecha a issue na mesclagem, mas a label continua lá e o agendamento a
# pegaria de novo daqui a uma hora. Tirar a label é o que aposenta a issue da fila.
for _ in 1 2 3 4 5; do
	estado=$(gh issue view "$ISSUE" -R "$REPO" --json state --jq .state)
	[ "$estado" = "CLOSED" ] && break
	sleep 3
done
gh issue edit "$ISSUE" -R "$REPO" --remove-label auto || true
if [ "${estado:-}" != "CLOSED" ]; then
	# A PR foi mesclada mas o "Closes" não pegou: fechar à mão é melhor que deixar a
	# issue aberta com o trabalho já na branch padrão.
	gh issue close "$ISSUE" -R "$REPO" --comment "Mesclada na PR #$PR."
fi

bash "$ROOT/scripts/auto/proxima-issue.sh"
