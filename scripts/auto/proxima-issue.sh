#!/usr/bin/env bash
# Dispara a próxima issue da fila `auto`, se houver uma livre.
#
# O encadeamento é sempre por `gh workflow run`, nunca por gatilho `pull_request`: uma PR
# aberta com o GITHUB_TOKEN não dispara outros workflows (é a proteção do GitHub contra
# recursão), então esperar pelo evento seria esperar para sempre. O dispatch com o mesmo
# GITHUB_TOKEN funciona — por isso o workflow pede `actions: write`.
#
# Dispara uma por vez: a próxima sai quando esta mesclar. Fila em série é o que mantém a
# branch padrão sempre mesclável, o rebase fora do caminho e o gasto previsível. Issues com
# PR aberta (em rascunho ou esperando retomada) e issues em execução são puladas; a
# retomada de uma PR que falhou fica para o schedule, que passa pelos tetos do resolve.
#
# Uso: proxima-issue.sh            → dispara
#      proxima-issue.sh --dry-run  → só diz o que faria
set -euo pipefail

REPO="${REPO:-emersonjoe/trilha}"
WORKFLOW="${WORKFLOW:-auto-issue.yml}"
DRY="${1:-}"

abertas=$(gh pr list -R "$REPO" --state open --limit 100 --json headRefName --jq '.[].headRefName')
rodando=$(gh run list -R "$REPO" --workflow "$WORKFLOW" --limit 40 --json displayTitle,status \
	--jq '.[] | select(.status=="in_progress" or .status=="queued" or .status=="waiting") | .displayTitle' 2>/dev/null || true)
fila=$(gh issue list -R "$REPO" --label auto --state open --limit 100 --json number --jq 'sort_by(.number) | .[].number')

for n in $fila; do
	grep -qx "auto/issue-$n" <<<"$abertas" && { echo "#$n: já tem PR aberta"; continue; }
	grep -q "issue #$n\$" <<<"$rodando" && { echo "#$n: já está em execução"; continue; }
	if [ "$DRY" = "--dry-run" ]; then
		echo "#$n: PRONTA"
	else
		gh workflow run "$WORKFLOW" -R "$REPO" -f "issue=$n"
		echo "#$n: disparada"
	fi
	exit 0
done

echo "fila vazia ou tudo em andamento"
