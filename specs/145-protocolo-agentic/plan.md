# Plano — Spec 145

**Branch**: `145-protocolo-agentic` | **Spec**: [spec.md](spec.md) | **Issue**: #232

## Summary

Duas mudanças pequenas na CLI (cache em `.trilha/cache/`, despacho `trilha-<nome>`) e três
capítulos no site. Nenhuma API pública muda; nenhuma convenção nova em `app/`.

## Technical Context

Go 1.22+, stdlib. Arquivos: `cmd/trilha/main.go` (+ `external_test.go`), `cmd/trilha/i18n.go`,
`cmd/trilha/export.go`, `internal/dev/server.go`, `internal/scaffold/templates/base/gitignore.tmpl`,
`.gitignore`, `site/internal/docs/docs.go`, `site/internal/docs/content/{en/learn,pt/aprender}/agentic-*.md`,
`site/internal/docs/content/{en/reference/conventions,pt/referencia/convencoes}.md`.

## Constitution Check

Na spec. Sem violação; sem *Complexity Tracking*.

## Decisões

- **Despacho no `default:`**, não antes do `switch`: um comando do framework nunca é sombreado
  por um `trilha-<nome>` do `PATH`.
- **`TRILHA_PARENT_VERSION`** no ambiente do filho: o `trilha-spec` pode avisar quando o
  framework é anterior ao que ele espera, sem parsear saída.
- **Mensagem do `audit` cita `.trilha/cache/`**; o teste continua sendo `Contains(".trilha")`,
  então `.trilha/` antigo e `.trilha/cache/` novo passam.
- **Capítulos na Learn, não uma seção nova**: os testes do site já exigem desafio + solução e
  paridade de locales para capítulos; uma seção nova pediria rotas, exportação e navegação.
