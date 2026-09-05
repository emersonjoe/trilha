#!/usr/bin/env bash
# Mede o ciclo editar→ver do `trilha dev` (SC-002: < 2 s).
# Uso: scripts/measure-reload.sh [dir-do-app] [porta]
set -euo pipefail
APP=${1:-examples/blog}
PORT=${2:-3999}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT/$APP"
MARK="marcador-$(date +%s)"
PAGE=app/page.go
mkdir -p "$ROOT/.trilha"
cp "$PAGE" "$ROOT/.trilha/page.go.bak"
trap 'mv "$ROOT/.trilha/page.go.bak" "$PAGE"; kill -INT $DEV 2>/dev/null || true; sleep 0.5' EXIT

# Binário da CLI (não `go run`): assim o SIGINT do trap chega ao próprio dev server.
go build -o "$ROOT/.trilha/cli" "$ROOT/cmd/trilha"
"$ROOT/.trilha/cli" dev --addr "127.0.0.1:$PORT" >"$ROOT/.trilha-dev.log" 2>&1 &
DEV=$!
for _ in $(seq 1 100); do
  curl -sf "http://127.0.0.1:$PORT/" >/dev/null 2>&1 && break
  sleep 0.1
done
curl -sf "http://127.0.0.1:$PORT/" | grep -q "_trilha/events" || { echo "sem script de reload"; exit 1; }

# 1) edição válida
sed -i '' "s/Trilha\"))/Trilha $MARK\"))/" "$PAGE"
START=$(python3 -c 'import time;print(time.time())')
while ! curl -sf "http://127.0.0.1:$PORT/" | grep -q "$MARK"; do sleep 0.05; done
END=$(python3 -c 'import time;print(time.time())')
printf "editar→ver: %.2f s\n" "$(echo "$END - $START" | bc)"

# 1b) CSS: só reload, sem rebuild (SC-002 da 002: < 0,5 s)
CSS=public/style.css
cp "$CSS" "$ROOT/.trilha/style.css.bak"
LOG_BEFORE=$(grep -c "✓ pronto" "$ROOT/.trilha-dev.log")
echo "/* $MARK */" >> "$CSS"
START=$(python3 -c 'import time;print(time.time())')
while ! curl -sf "http://127.0.0.1:$PORT/style.css" | grep -q "$MARK"; do sleep 0.02; done
END=$(python3 -c 'import time;print(time.time())')
printf "css→ver: %.2f s\n" "$(echo "$END - $START" | bc)"
sleep 0.6
LOG_AFTER=$(grep -c "✓ pronto" "$ROOT/.trilha-dev.log")
[ "$LOG_BEFORE" = "$LOG_AFTER" ] && echo "css: sem rebuild" || echo "css: REBUILD INDEVIDO"
mv "$ROOT/.trilha/style.css.bak" "$CSS"
sleep 0.6

# 2) erro de compilação aparece no navegador
echo "func quebrado( {" >> "$PAGE"
for _ in $(seq 1 100); do
  curl -s "http://127.0.0.1:$PORT/" | grep -q "Erro de compilação" && break
  sleep 0.1
done
curl -s -o /dev/null -w "erro de compilação: HTTP %{http_code}\n" "http://127.0.0.1:$PORT/"

# 3) corrigir volta sozinho
cp "$ROOT/.trilha/page.go.bak" "$PAGE"
for _ in $(seq 1 100); do
  curl -sf "http://127.0.0.1:$PORT/" | grep -q "<h1>Trilha</h1>" && { echo "recuperou após correção"; break; }
  sleep 0.1
done
