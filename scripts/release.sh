#!/usr/bin/env bash
# Fecha uma spec: testa, funde na main pelo remoto, marca a versão, publica e
# fecha as issues. O script não troca de branch — a main pode estar checada em
# outro worktree, e o worktree de quem roda fica onde está.
# Uso: scripts/release.sh 0.11.0 [--issues "20 21"] [--dry-run]
#
# Rode a partir do branch da spec, com tudo commitado, o `version` de
# cmd/trilha/main.go já atualizado e o CHANGELOG já com a seção da versão —
# as notas da release saem de lá, não são escritas duas vezes.
set -euo pipefail

VERSION=${1:-}
shift || true
ISSUES=""
DRY=0
while [ $# -gt 0 ]; do
	case "$1" in
	--issues) ISSUES=${2:-}; shift 2 ;;
	--dry-run) DRY=1; shift ;;
	*) echo "argumento desconhecido: $1" >&2; exit 2 ;;
	esac
done

case "$VERSION" in
"" | -*) echo "uso: scripts/release.sh X.Y.Z [--issues \"20 21\"] [--dry-run]" >&2; exit 2 ;;
esac
if ! printf '%s' "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
	echo "versão inválida: $VERSION (esperado X.Y.Z, sem o v)" >&2
	exit 2
fi

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
TAG="v$VERSION"

run() {
	if [ "$DRY" = 1 ]; then
		printf '  [dry-run] %s\n' "$*"
	else
		"$@"
	fi
}

# --- verificações antes de mexer em qualquer coisa -------------------------

BRANCH=$(git rev-parse --abbrev-ref HEAD)
[ "$BRANCH" != "main" ] || { echo "rode a partir do branch da spec, não da main" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "há mudanças não commitadas; commite antes" >&2; exit 1; }
grep -q "^const version = \"$VERSION\"$" cmd/trilha/main.go ||
	{ echo "cmd/trilha/main.go não está na versão $VERSION" >&2; exit 1; }
grep -q "^## $VERSION — " CHANGELOG.md ||
	{ echo "CHANGELOG.md não tem a seção '## $VERSION — <data>'" >&2; exit 1; }
# O resumo do topo do ROADMAP.md anunciou a 0.41.0 até a 0.100.0 sair (#159): quem chegava
# pelo roadmap planejava contra uma fronteira de sessenta versões atrás. Conferir aqui é o
# que impede isso de acontecer de novo — o script não escreve o arquivo porque exige árvore
# limpa, então quem fecha a spec atualiza a linha e a release confirma.
grep -qE "^## Onde o Trilha está \(.*v$VERSION\)$" ROADMAP.md ||
	{ echo "ROADMAP.md: a seção 'Onde o Trilha está' não nomeia a $VERSION" >&2; exit 1; }
# Uma tag que já existe *neste commit* é um passo já dado — o ritual foi
# interrompido no meio e está sendo retomado. Apontando para outro commit
# continua sendo recusa: é a versão sendo remarcada em cima de outra coisa.
RETOMANDO=0
if TAGGED=$(git rev-parse -q --verify "refs/tags/$TAG^{commit}"); then
	[ "$TAGGED" = "$(git rev-parse HEAD)" ] ||
		{ echo "a tag $TAG já existe e aponta para $TAGGED, não para o HEAD" >&2; exit 1; }
	RETOMANDO=1
fi
command -v gh >/dev/null || { echo "gh não encontrado no PATH" >&2; exit 1; }

# A main de verdade é a do remoto: é contra ela que o push vai ser conferido.
git fetch -q origin main || { echo "não deu para buscar a main do remoto" >&2; exit 1; }
[ -z "$(git log --merges origin/main..HEAD)" ] ||
	{ echo "o branch tem merge commit, e o ruleset da main recusa; rebase antes: git rebase origin/main" >&2; exit 1; }
git merge-base --is-ancestor origin/main HEAD ||
	{ echo "o branch está atrás da main; rebase antes: git rebase origin/main" >&2; exit 1; }

# Notas da release: a seção da versão no CHANGELOG, sem o cabeçalho.
NOTES=$(awk -v v="## $VERSION — " '
	index($0, v) == 1 { on = 1; next }
	on && /^## / { exit }
	on { print }
' CHANGELOG.md | sed -e '/./,$!d')
[ -n "$NOTES" ] || { echo "seção $VERSION vazia no CHANGELOG.md" >&2; exit 1; }

echo "==> $TAG a partir de $BRANCH${ISSUES:+, fechando issues: $ISSUES}"
[ "$RETOMANDO" = 1 ] && echo "==> a tag $TAG já existe neste commit: retomando o que falta"

# FUNDIDA marca o ponto sem volta. Depois do push da main uma falha deixa uma
# release pela metade, e quem lê o erro de curl que a produziu não tem como
# saber disso: o script precisa dizer o que já foi e o que falta. Foi assim que
# a 0.102.0 e a 0.103.0 pararam — a credencial empurrava branch e não tag.
FUNDIDA=0
restante() {
	status=$?
	[ "$status" != 0 ] && [ "$FUNDIDA" = 1 ] || return 0
	echo >&2
	echo "release.sh: a main já foi fundida em $(git rev-parse --short HEAD), e o ritual parou." >&2
	echo >&2
	echo "Falta, e é só rodar de novo — o que já foi feito é pulado:" >&2
	echo "  scripts/release.sh $VERSION${ISSUES:+ --issues \"$ISSUES\"}" >&2
	echo >&2
	echo "Ou à mão:" >&2
	# A tag agora, e não o RETOMANDO do começo: o passo que falhou foi o push,
	# e nesse caso a tag local já existe — mandar criá-la de novo é mandar
	# rodar um comando que erra.
	git rev-parse -q --verify "refs/tags/$TAG" >/dev/null ||
		echo "  git tag -a $TAG -m $TAG" >&2
	echo "  git push origin $TAG" >&2
	echo "  gh release create $TAG --title $TAG --notes-file -   # a seção $VERSION do CHANGELOG" >&2
	for n in $ISSUES; do
		echo "  gh issue close $n --comment \"Entregue na $TAG.\"" >&2
	done
}
trap restante EXIT

# --- o ritual ---------------------------------------------------------------

echo "==> make test"
run make test

# O fast-forward é o próprio push: o remoto recusa se não for um, e ninguém
# precisa da main checada aqui para isso acontecer.
echo "==> main pelo remoto"
run git push origin HEAD:main
[ "$DRY" = 1 ] || FUNDIDA=1

# A tag vem depois do push da main, para não sobrar tag local apontando para um
# commit que a fusão recusou. A consequência é que tudo daqui para baixo pode
# falhar com a main já fundida, e é o `restante` acima que conta isso.
echo "==> tag e push"
[ "$RETOMANDO" = 1 ] || run git tag -a "$TAG" -m "$TAG"
run git push origin "$TAG"

# Criar ou, quando a release já existe, escrever as notas nela. Retomar é o
# motivo; o outro caso é a release criada sem corpo — que acontece de verdade,
# porque `gh release create` com um --notes-file vazio publica assim mesmo.
echo "==> release no GitHub"
if [ "$DRY" = 1 ]; then
	printf '  [dry-run] gh release create|edit %s --notes <CHANGELOG %s>\n' "$TAG" "$VERSION"
elif gh release view "$TAG" >/dev/null 2>&1; then
	printf '%s\n' "$NOTES" | gh release edit "$TAG" --notes-file -
else
	printf '%s\n' "$NOTES" | gh release create "$TAG" --title "$TAG" --notes-file -
fi

for n in $ISSUES; do
	echo "==> fecha #$n"
	run gh issue close "$n" --comment "Entregue na $TAG."
done

# A main local é conveniência, não fonte de verdade: se estiver checada em outro
# worktree o Git recusa, e a release já saiu de qualquer jeito.
if [ "$DRY" = 1 ]; then
	printf '  [dry-run] git fetch origin main:main\n'
elif ! git fetch -q origin main:main 2>/dev/null; then
	echo "==> a main local não foi adiantada (checada em outro worktree); dê 'git pull --ff-only' lá"
fi

echo
echo "Falta o que nenhum script escreve por você:"
echo "  - ROADMAP.md: riscar o item da fase e marcar 'Entregue na $TAG'"
echo "  - avisar a outra sessão, se houver uma trabalhando neste repositório"
