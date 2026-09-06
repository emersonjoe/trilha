# Tasks: `generate` com contrato

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — achar o tipo (SC-002, SC-003)

- [x] T001 Teste que falha em `internal/scaffold/types_test.go`: num projeto sintético,
      `findType("Comment")` acha o pacote, o caminho de importação e os campos com as tags;
      nome ausente devolve "não achei"; nome em dois pacotes devolve erro com os dois
      caminhos; `posts.Comment` desempata.
- [x] T002 `internal/scaffold/types.go`: a varredura com as mesmas pastas puladas do
      `internal/openapi` (com a exceção `scan.WellKnown`), os campos com `json`, `form` e
      `validate`, e o valor de exemplo por campo a partir das tags.

## Bloco 2 — o contrato da rota (SC-001, SC-002, SC-009)

- [x] T003 Teste que falha em `internal/scaffold/generate_test.go`: `--methods GET,POST` com
      parâmetro, `--bind` com tipo ausente e com tipo presente; método desconhecido ou
      repetido é recusa antes de escrever; sem flags, a saída de hoje não muda.
- [x] T004 `internal/scaffold/generate.go`: `Methods`, `Bind`, `Module` em `GenOptions` e os
      templates de rota com contrato.
- [x] T005 Goldens `testdata/golden/generate/route-*.golden` e o `-update` do pacote;
      `internal/scaffold` no alvo `golden` do `Makefile`.

## Bloco 3 — o formulário e o layout (SC-004, SC-005, SC-008)

- [x] T006 Teste que falha: `--form Contact` gera `CSRFInput`, um campo por campo, 422 com
      `FieldErrors` e redirect; `--layout` grava o que falta, recusa caminho não ancestral e
      arquivo que não é `layout.go`; `--lang pt` troca comentário e não troca identificador.
- [x] T007 `internal/scaffold`: o template de página com formulário, o `--layout` e os
      comentários por idioma em `texts.go`.
- [x] T008 Goldens `page-form.golden`, `page-form-pt.golden`, `layout.golden`.

## Bloco 4 — `generate test` (SC-006, SC-007)

- [x] T009 Teste que falha em `internal/scaffold/gentest_test.go`: um caso por método
      existente, alvo com o parâmetro, corpo montado das tags quando o tipo é resolvível,
      URL sem rota é erro.
- [x] T010 `internal/scaffold/gentest.go` e o `kind` `test` no `cmd/trilha/generate.go`.
- [x] T011 e2e em `cmd/trilha/e2e_test.go`: rota com `--methods/--bind`, página com `--form`,
      os dois `generate test`, e `trilha check` verde sem edição.

## Bloco 5 — fechamento (SC-010)

- [x] T012 `usage` e ajuda das flags nos dois idiomas; `AGENTS.md` do scaffold com as flags e
      `TestAgentsMatchesUsage` verde.
- [x] T013 Documentação nos dois idiomas (referência da CLI e a página de agentes) e
      `site/internal/docs` em sincronia.
- [x] T014 CHANGELOG (0.39.0) e ROADMAP (item 27). Régua (#45): a issue pede o número antes e
      depois no cenário 1, e a linha de base commitada é anterior à 0.35.0 — a medição custa
      cerca de uma hora e API de verdade dos dois lados, então fica combinada com o usuário
      antes de gastar; a 0.39.0 não publica número que não foi medido.
