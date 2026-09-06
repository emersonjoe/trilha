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
! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null ||
	{ echo "a tag $TAG já existe" >&2; exit 1; }
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

# --- o ritual ---------------------------------------------------------------

echo "==> make test"
run make test

# O fast-forward é o próprio push: o remoto recusa se não for um, e ninguém
# precisa da main checada aqui para isso acontecer.
echo "==> main pelo remoto"
run git push origin HEAD:main

# A tag vem depois do push da main, para não sobrar tag local apontando para um
# commit que a fusão recusou.
echo "==> tag e push"
run git tag -a "$TAG" -m "$TAG"
run git push origin "$TAG"

echo "==> release no GitHub"
if [ "$DRY" = 1 ]; then
	printf '  [dry-run] gh release create %s --title %s --notes <CHANGELOG %s>\n' "$TAG" "$TAG" "$VERSION"
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
echo "  - ROADMAP.md: riscar o item e marcar 'Entregue na $TAG'"
echo "  - avisar a outra sessão, se houver uma trabalhando neste repositório"
