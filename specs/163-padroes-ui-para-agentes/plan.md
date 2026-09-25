# Implementation Plan: Padrões de UI para agentes

**Branch**: `163-padroes-ui-para-agentes` | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)

## Summary

Seis `page.go` compiláveis em `examples/patterns/`, levados para `ui.Patterns()` por um arquivo
gerado; `trilha ui patterns`, `get_pattern` no MCP (projeto e site), a seção "usar este padrão"
nas demos, o catálogo `uidoc` apontando os padrões e um golden de render por padrão.

## Constitution Check

- [x] II — só stdlib; nenhum `require`.
- [x] III — o arquivo gerado é determinístico e commitado (`make golden`).
- [x] IV — superfície nova só `ui.Pattern`/`ui.Patterns`, com doc comment e uso em `examples/`.
- [x] VI — compilação, orçamento, nomes reais, goldens e bilíngue testados.
- [x] VII — snippets revisados por teste (CSRF, sem `h.Raw`).
- [x] Idioma — código e dados do padrão em inglês; site com tradução pt testada.

## Design

```
examples/patterns/{listwithfilter,asyncform,approvalinbox,dashboardchart,masterdetail,uploadprogress}/page.go
ui/patterns.go             # Pattern, Patterns(): nome, resumo, componentes, contrato, a11y
ui/patterns_snippets.go    # gerado: o texto de cada page.go
ui/patterns_test.go        # Current(-update), Compile, Budget, NameRealComponents, KeepProtections
internal/uidoc/            # Component.Patterns; golden de render por padrão
cmd/trilha/ui.go           # trilha ui patterns [--json] [nome]; describe mostra os padrões
ai/mcp/context.go          # get_pattern em ContextTools
site/internal/docsmcp/     # get_pattern
site/internal/demos/       # Demo.Pattern + seção "usar este padrão" + tabela pt
site/internal/docs/content/{en,pt}/reference/mcp.md, cli.md, ui.md
```

Blocos: T01 → T05 (superfície junto) → T02 → T03 → T06 → T04 → T07.
