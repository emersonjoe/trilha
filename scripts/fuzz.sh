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

SAIDA=$(mktemp)
trap 'rm -f "$SAIDA"' EXIT

# corpus_de conta os arquivos que o alvo tem hoje; um achado acrescenta um.
# O alvo que nunca achou nada não tem a pasta, e com pipefail um find que falha
# derruba o script inteiro — daí o caminho explícito para o zero.
corpus_de() {
	[ -d "$1" ] || { echo 0; return 0; }
	find "$1" -type f | wc -l
}

# roda executa um alvo, mostra a saída e a guarda para ser lida depois.
roda() {
	local pacote=$1 alvo=$2 st
	set +e
	go test "$pacote" -run "^${alvo}\$" -fuzz "^${alvo}\$" \
		-fuzztime "$FUZZTIME" -fuzzminimizetime "$MINIMIZE" 2>&1 | tee "$SAIDA"
	st=${PIPESTATUS[0]}
	set -e
	return "$st"
}

# so_prazo diz se a reprovação foi o prazo do -fuzztime estourando dentro de uma
# iteração, e não uma entrada. As duas coisas são distinguíveis: um achado
# escreve o arquivo em testdata/fuzz/<Alvo>/ e anuncia onde escreveu. Sem esses
# dois sinais, e com a mensagem do prazo, não houve achado nenhum (#86).
so_prazo() {
	local corpus=$1 antes=$2
	[ "$antes" = "$(corpus_de "$corpus")" ] || return 1
	grep -q "context deadline exceeded" "$SAIDA" || return 1
	! grep -q "Failing input written to" "$SAIDA"
}

for entrada in "${ALVOS[@]}"; do
	pacote=${entrada%%:*}
	alvo=${entrada##*:}
	corpus="$pacote/testdata/fuzz/$alvo"
	antes=$(corpus_de "$corpus")
	echo "==> $alvo ($pacote) por $FUZZTIME"
	roda "$pacote" "$alvo" && continue

	# Uma segunda passada, e só para essa assinatura. Um travamento de verdade
	# repete e continua reprovando; o que não repete era a máquina, não o
	# código — e um trabalho que reprova por acaso ensina a reexecutar sem ler.
	so_prazo "$corpus" "$antes" || exit 1
	echo "==> $alvo: prazo estourado sem entrada nova; segunda passada (#86)"
	antes=$(corpus_de "$corpus")
	roda "$pacote" "$alvo" && continue
	so_prazo "$corpus" "$antes" &&
		echo "==> $alvo: o prazo estourou duas vezes seguidas — isto não é acaso" >&2
	exit 1
done
