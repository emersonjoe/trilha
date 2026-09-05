#!/usr/bin/env bash
# Tráfego do repositório nos últimos 14 dias (API do GitHub, via `gh` já autenticado).
# Uso: scripts/traffic.sh [owner/repo] [--json]
set -euo pipefail
repo="${1:-emersonjoe/trilha}"
if [[ "${1:-}" == "--json" ]]; then repo="emersonjoe/trilha"; fi
json=$( [[ "${2:-${1:-}}" == "--json" ]] && echo 1 || echo 0 )

views=$(gh api "repos/$repo/traffic/views")
clones=$(gh api "repos/$repo/traffic/clones")
paths=$(gh api "repos/$repo/traffic/popular/paths")
refs=$(gh api "repos/$repo/traffic/popular/referrers")
meta=$(gh api "repos/$repo" --jq '{stars: .stargazers_count, forks: .forks_count, watchers: .subscribers_count, open_issues: .open_issues_count}')

if [[ "$json" == 1 ]]; then
  jq -cn --arg date "$(date -u +%F)" --argjson views "$views" --argjson clones "$clones" --argjson paths "$paths" --argjson refs "$refs" --argjson meta "$meta" \
    '{date: $date, views: {count: $views.count, uniques: $views.uniques}, clones: {count: $clones.count, uniques: $clones.uniques}, paths: $paths, referrers: $refs} + $meta'
  exit 0
fi

echo "== $repo (últimos 14 dias)"
echo "$meta" | jq -r '"estrelas \(.stars) · forks \(.forks) · watchers \(.watchers) · issues abertas \(.open_issues)"'
echo "$views"  | jq -r '"visitas: \(.count) (\(.uniques) únicos)"'
echo "$clones" | jq -r '"clones:  \(.count) (\(.uniques) únicos)"'
echo "-- caminhos mais vistos"
echo "$paths" | jq -r '.[] | "\(.count)\t\(.uniques)\t\(.path)"' | head -10
echo "-- referenciadores"
echo "$refs" | jq -r '.[] | "\(.count)\t\(.uniques)\t\(.referrer)"' | head -10
