# Implementation Plan: Grupos de rota, adaptador html/template e recarga de estáticos

**Branch**: `002-grupos-templates` | **Date**: 2026-09-05 | **Spec**: [spec.md](spec.md)

## Summary

Três incrementos independentes sobre a 001: (1) pasta `nome-` vira grupo de rota no
scanner, com detecção de padrão duplicado; (2) pacote público `tmpl` que adapta
`html/template` ao `h.Node`; (3) o watcher do `dev` classifica lotes só de `public/` e
dispara `reload` sem rebuild.

## Technical Context

**Language/Version**: Go 1.25 (mínimo 1.22)  |  **Dependencies**: stdlib
**Testing**: `go test` (scanner por tabela, golden do gerador, `tmpl` unitário, exemplo
por httptest, watcher unitário) + `scripts/measure-reload.sh` estendido para CSS

## Constitution Check

| Princípio | Como atende |
|-----------|-------------|
| I. Convenção | `nome-` documentado, com exemplo e teste; erro `E_DUPLICATE_ROUTE` e `E_BAD_SEGMENT` para grupo dinâmico |
| II. Stdlib | `html/template`, `io/fs`; nada externo |
| III. Geração explícita | grupos só mudam o `Pattern` e as listas de layouts; arquivo gerado continua determinístico (goldens) |
| IV. Contrato de handler | inalterado; `tmpl.Node` é um `h.Node` comum |
| V. Dev < 2 s | estáticos passam a < 0,5 s |
| VI. Teste primeiro | testes do scanner, do `tmpl` e do watcher antes do código |
| VII. Segurança | `html/template` mantém escape contextual; nada muda em cabeçalhos/CSRF |

## Project Structure

```text
internal/scan/scan.go        # segment kind "group"; pattern sem o grupo; duplicatas
internal/dev/watch.go        # Change{Paths, StaticOnly}; classificação do lote
internal/dev/server.go       # StaticOnly → broadcast("reload") sem rebuild
tmpl/tmpl.go                 # Node, Must
examples/blog/app/marketing-/{layout.go,precos/page.go,sobre/page.go}
examples/blog/app/painel-/{layout.go,middleware.go,painel/page.go,relatorio/{page.go,relatorio.html}}
testdata/apps/{groups,err_group_dup,err_group_dynamic}
```

## Design

- **Scanner**: `parseSegment` devolve `kind=3 (group)` para sufixo `-`; `patternOf`
  ignora grupos; um grupo com `_`/`__` antes do `-` é `E_BAD_SEGMENT`. Ao final, agrupa
  rotas por `Pattern` e emite `E_DUPLICATE_ROUTE` com os dois `Dir`. Alias de import
  continua derivado do caminho real (`app_marketing_`).
- **Ambiguidade**: só segmentos dinâmicos contam; grupos irmãos são livres.
- **tmpl.Node**: executa `t.ExecuteTemplate(&buf, name, data)`; erro → `error` do render
  (o `writeHTML` já bufferiza, então a resposta é 500 limpo).
- **Watcher**: `Watch` passa a emitir `Change{Paths []string; StaticOnly bool}`;
  `StaticOnly` = todos os caminhos começam com `public/` **e** a existência de `public/`
  não mudou (tinha arquivos antes e depois). O servidor faz `broadcast("reload")` e loga
  `↻ public/x (estático)`.

## Complexity Tracking

Sem violações.
