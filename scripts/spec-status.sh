#!/bin/sh
# Lists every specs/*/spec.md whose status disagrees with the repository's tags.
#
#   missing   no **Status** line and no **Versão** that names a release (or says "nenhuma")
#   draft     **Status** is Draft or Rascunho
#   ready     **Status** says "Pronta para release" but that version is already tagged
#   untagged  the status or version names a release that has no tag yet (listed, not failing:
#             a CHANGELOG section without a tag is the release's problem, not the spec's)
#
# Prints one line per flagged spec (`spec :: reason :: what the file says`) and exits 1 when a
# spec is missing, draft or ready-but-tagged, so it can run as a gate. `-q` omits the summary.
set -eu
cd "$(dirname "$0")/.."

tags=$(git tag --list 'v*')
tagged() { printf '%s\n' "$tags" | grep -qx "v$1"; }
version_in() { printf '%s\n' "$1" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n 1; }

flagged=0
failing=0
total=0
for f in specs/*/spec.md; do
	total=$((total + 1))
	spec=${f#specs/}
	spec=${spec%/spec.md}
	status=$(grep -oE -m1 '\*\*Status\*\*: *[^|]*' "$f" | sed 's/\*\*Status\*\*: *//; s/ *$//')
	versao=$(grep -oE -m1 '\*\*Versão\*\*: *.*' "$f" | sed 's/\*\*Versão\*\*: *//')
	v=$(version_in "$status")
	[ -n "$v" ] || v=$(version_in "$versao")
	reason=""
	case "$status" in
	*Draft* | *Rascunho*) reason="draft" ;;
	*"Pronta para release"*)
		if [ -n "$v" ] && tagged "$v"; then reason="ready but v$v is tagged"; fi
		;;
	"")
		case "$versao" in
		*nenhuma*) ;;
		*) [ -n "$v" ] || reason="missing" ;;
		esac
		;;
	esac
	if [ -n "$reason" ]; then failing=$((failing + 1)); fi
	if [ -z "$reason" ] && [ -n "$v" ] && ! tagged "$v"; then reason="untagged v$v"; fi
	if [ -n "$reason" ]; then
		flagged=$((flagged + 1))
		printf '%s :: %s :: %s\n' "$spec" "$reason" "${status:-${versao:-<none>}}"
	fi
done
[ "${1:-}" = "-q" ] || printf '%d of %d specs flagged, %d failing\n' "$flagged" "$total" "$failing" >&2
[ "$failing" -eq 0 ]
