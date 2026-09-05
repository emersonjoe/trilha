#!/bin/sh
# Prints bench/RESULTS.md: environment + benchmark table.
set -e
cpu=$(sysctl -n machdep.cpu.brand_string 2>/dev/null || grep -m1 "model name" /proc/cpuinfo | cut -d: -f2 | sed 's/^ //')
echo "# Resultados de referência"
echo
echo "Gerado em $(date -u +%F) por \`make bench-results\` — $(go version | cut -d' ' -f3), $cpu, $(uname -sm)."
echo "Custo por requisição em processo (\`httptest\`), só o framework; leia a metodologia em"
echo "https://emersonjoe.github.io/trilha/referencia/desempenho."
echo
echo '```'
go test -run XXX -bench . -benchmem -count=3 2>/dev/null | grep "^Benchmark" | sort
echo '```'
