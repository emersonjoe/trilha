#!/usr/bin/env bash
# Roda cada alvo de fuzzing por um tempo. O `go test -fuzz` aceita um alvo de um
# pacote por vez, então a lista mora aqui e não no Makefile.
#
# Uso: scripts/fuzz.sh [FUZZTIME]         (padrão: 20s, o mesmo do CI)
#      FUZZTIME=5m scripts/fuzz.sh
#
# Falha achada vira arquivo em testdata/fuzz/<Alvo>/ — commite junto com a
# correção: é a regressão que impede a volta do bug.
set -euo pipefail

FUZZTIME=${1:-${FUZZTIME:-20s}}
# Minimizar uma entrada nova pode estourar o prazo total do alvo; um teto curto
# aqui mantém o CI previsível.
MINIMIZE=${FUZZMINIMIZE:-5s}

ALVOS=(
	".:FuzzRouteMatch"
	".:FuzzParseTraceparent"
	".:FuzzSignedVerify"
	".:FuzzBindForm"
	".:FuzzBindJSON"
	"./h:FuzzRenderEscapes"
)

for entrada in "${ALVOS[@]}"; do
	pacote=${entrada%%:*}
	alvo=${entrada##*:}
	echo "==> $alvo ($pacote) por $FUZZTIME"
	go test "$pacote" -run "^${alvo}\$" -fuzz "^${alvo}\$" \
		-fuzztime "$FUZZTIME" -fuzzminimizetime "$MINIMIZE"
done
