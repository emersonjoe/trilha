#!/usr/bin/env bash
# Garante que a sessão terminou com uma PR aberta, e imprime o número dela.
#
# A sessão recebe a ordem de abrir a PR, mas uma sessão que bate no --max-turns ou que se
# enrola no fim deixa a branch publicada e nenhuma PR. Isso pararia o encadeamento em
# silêncio, que é o pior jeito de parar: abrir a PR aqui faz o trabalho chegar ao portão de
# teste de qualquer jeito.
#
# Guardrail: branch publicada sem nenhuma diferença para a branch padrão não vira PR — é uma
# sessão que não fez nada, e o job falha para a tentativa contar.
set -euo pipefail

BRANCH="${BRANCH:?}"
ISSUE="${ISSUE:?}"
PADRAO="${PADRAO:-main}"
SAIDA="${GITHUB_OUTPUT:-/dev/stdout}"

git fetch -q origin
if ! git ls-remote --exit-code --heads origin "$BRANCH" >/dev/null; then
	echo "::error::a sessão da issue #$ISSUE não publicou a branch $BRANCH"
	exit 1
fi

if git diff --quiet "origin/$PADRAO...origin/$BRANCH" 2>/dev/null; then
	echo "::error::a branch $BRANCH não tem nenhuma diferença para $PADRAO: a sessão não produziu código"
	exit 1
fi

n=$(gh pr list --head "$BRANCH" --state open --json number --jq '.[0].number // empty')
if [ -z "$n" ]; then
	titulo=$(gh issue view "$ISSUE" --json title --jq .title)
	gh pr create --head "$BRANCH" --base "$PADRAO" \
		--title "#$ISSUE · $titulo" \
		--body "PR aberta pelo workflow porque a sessão publicou a branch e não a abriu.

Closes #$ISSUE"
	n=$(gh pr list --head "$BRANCH" --state open --json number --jq '.[0].number // empty')
fi
[ -n "$n" ] || { echo "::error::não deu para abrir nem encontrar a PR de $BRANCH"; exit 1; }

# O "Closes #n" é o que fecha a issue na mesclagem. Se a sessão abriu a PR sem ele, a issue
# ficaria aberta com o trabalho já mesclado — então ele é acrescentado aqui.
corpo=$(gh pr view "$n" --json body --jq .body)
if ! grep -qiE "(close[sd]?|fix(e[sd])?|resolve[sd]?) +#$ISSUE\b" <<<"$corpo"; then
	gh pr edit "$n" --body "$corpo

Closes #$ISSUE"
	echo "acrescentado 'Closes #$ISSUE' ao corpo da PR"
fi

echo "numero=$n" >>"$SAIDA"
echo "PR #$n"
