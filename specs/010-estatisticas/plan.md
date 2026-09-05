# Implementation Plan: 010-estatisticas

Constitution Check: II (nada no runtime; só o site e CI), VII (sem cookies; CSP do site
libera apenas a origem do script quando habilitado; segredos fora do repo).

- `site/app/setup.go`: lê `SITE_ANALYTICS`; guarda em `a.Values()["analytics"]`; CSPExtra.
- `site/internal/ui`: `Analytics(c)` no `<head>` e nota no rodapé.
- `.github/workflows/pages.yml`: passa `vars.SITE_ANALYTICS` ao export.
- `.github/workflows/traffic.yml`: cron diário; `TRAFFIC_TOKEN` (secret) só para a API;
  push na branch `stats` com o `GITHUB_TOKEN`.
- `scripts/traffic.sh`: `gh api` local.
- GOVERNANCE.md, CHANGELOG.
