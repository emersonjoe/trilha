# Plano — spec 064

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `internal/migrate/migrate.go` (novo) | `Scan`, `Project`, `Page`, `Note`, `Files`, `Write` |
| `internal/migrate/analyze.go` (novo) | as expressões regulares: endpoints, hooks, sinais, classificação |
| `internal/migrate/report.go` (novo) | `MIGRATION.md` |
| `internal/migrate/migrate_test.go` (novo) | golden da árvore e do relatório, com `-update` |
| `internal/scaffold/generate.go` | `packageName` exportado como `PackageName` (o `migrate` usa a mesma regra) |
| `testdata/next/` (novo) | a árvore sintética |
| `testdata/next.golden/` (novo) | o `app/` esperado e o `MIGRATION.md` |
| `cmd/trilha/migrate.go` (novo) | `trilha migrate next <dir> [--out] [--report] [--dry-run] [--force]` |
| `cmd/trilha/main.go` | `case "migrate"` |
| `cmd/trilha/i18n.go` | uso e mensagens novas |
| `cmd/trilha/e2e_test.go` | o `app/` gerado compila e passa no `gen --check` |
| `Makefile` | `internal/migrate` no `golden` |
| docs | `cli.md` nas duas línguas, `from-next.md`/`do-next.md` apontando o comando |
| `CHANGELOG.md` | a entrada |

## Ordem

A tradução de caminho primeiro, porque tudo depende dela; a análise léxica em seguida, que não
depende de nada; o emissor de arquivos e o relatório sobre as duas; a CLI por último, quando o
pacote já tem golden.

## O que não entra

- Traduzir JSX para `h`: é o trabalho de quem porta, e a issue diz isso.
- Parser de TypeScript.
- Escrever o `Config.Upstreams` no `setup.go`: o relatório sugere a linha.
