#!/usr/bin/env bash
# Conta sessões da automação pelos comentários "🤖 auto · tentativa" que o job `sessao`
# escreve antes de gastar tokens.
#
# Uso: contar-tentativas.sh <issue>   → tentativas daquela issue
#      contar-tentativas.sh --dia     → sessões no repositório nas últimas 24 h
#
# Contar comentários, e não runs, é o que faz o número sobreviver a um run cancelado ou a um
# runner que morreu: o comentário já estava lá.
set -euo pipefail
REPO="${REPO:?}"
MARCA="auto · tentativa"

if [ "${1:-}" = "--dia" ]; then
	desde=$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-24H +%Y-%m-%dT%H:%M:%SZ)
	gh api --paginate "repos/$REPO/issues/comments?since=$desde&per_page=100" \
		--jq "[.[] | select(.user.login==\"github-actions[bot]\") | select(.body | contains(\"$MARCA\"))] | length" |
		awk '{s+=$1} END {print s+0}'
	exit 0
fi

n="${1:?issue}"
gh api --paginate "repos/$REPO/issues/$n/comments?per_page=100" \
	--jq "[.[] | select(.user.login==\"github-actions[bot]\") | select(.body | contains(\"$MARCA\"))] | length" |
	awk '{s+=$1} END {print s+0}'
