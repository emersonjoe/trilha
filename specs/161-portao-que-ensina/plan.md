# Implementation Plan: O portão que ensina

**Branch**: `161-portao-que-ensina` | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)

## Summary

`internal/checkerr` vira a fonte única dos códigos `E_*` (runtime + scanner + ferramentas),
`trilha check --json` ganha o esquema do plano com `code/hint/doc` por falha e exit codes
0/1/2, as páginas `/docs/errors` passam a ser geradas do catálogo com busca e âncoras, e o
MCP do site ganha `get_error`. Nenhuma dependência nova; `checkerr` é `internal/`.

## Constitution Check

- [x] II — só stdlib; [x] IV — `internal/` não entra em `api/current.txt`; [x] VI — catálogo
  completo testado por varredura, mapeadores com fixtures; [x] VII — portão igual ao do CI,
  vuln opt-in, sem segredo na saída; [x] idioma — código en, páginas bilíngues no mesmo
  commit.

## Design

```
internal/checkerr/         # NOVO: Doc, Docs(), ByCode() com famílias por prefixo
cmd/trilha/check.go        # códigos por problema, --json novo, exit 0/1/2
cmd/trilha/checkerr/…      # mapeadores: gofmt, vet, test, vuln (JSON), api (diff)
site/app/docs/errors/      # páginas geradas do catálogo (índice com busca + por código)
site/internal/docsmcp/     # get_error(code)
site/internal/docs/content/{en,pt}/reference/ # mcp.md, errors.md
```

Blocos: T01 (catálogo) → T02/T03 (orquestrador + schema + exit codes) → T04 (mapeadores) →
T05 (páginas) → T06 (MCP) → T07 (régua: nota de fechamento).
