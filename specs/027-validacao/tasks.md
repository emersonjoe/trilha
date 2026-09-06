# Tasks: Validação declarativa no `Bind`

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — motor (FR-001…FR-004, FR-007, FR-010)

- [x] T001 Teste que falha em `validate_test.go`: as sete regras nos tipos da tabela; regra
      além de `required` ignora vazio; `AddRule` novo vale na tag e nome repetido entra em
      pânico; `UseValidationPTBR` troca as mensagens.
- [x] T002 `validate.go`: `Field`, `Validator`, `AddRule`, `ValidationMessages`,
      `UseValidationPTBR`, as regras e a passada.

## Bloco 2 — ligação com o `Bind` (FR-005, FR-006, FR-008, FR-009)

- [x] T003 Teste que falha em `bind_test.go`: struct aninhada com prefixo põe a mensagem no
      nome certo; conversão que falha não recebe mensagem de regra; tipo com `Validate()`;
      struct com `Validate()` só é chamada sem erro de campo; `BindJSON` valida igual.
- [x] T004 `bind.go`: marca do campo inválido, coleta e chamada da validação nos dois
      caminhos.

## Bloco 3 — exemplos (SC-005, SC-006)

- [x] T005 Teste que falha em `examples/blog/blog_test.go`: publicar sem título devolve 422
      com a mensagem no formulário.
- [x] T006 `examples/blog/app/blog/novo/page.go`: `Bind` com tags e `Render(422)`.
- [x] T007 Teste que falha em `examples/orcamento/orcamento_test.go`: mensagens em português
      para campo vazio, e a regra de negócio (conta inexistente) continua de pé.
- [x] T008 `examples/orcamento`: tags no `Lancamento`, `Validar` só com regra de negócio,
      `UseValidationPTBR()` no `Setup`, tipo com `Validate()` para o valor.

## Bloco 4 — documentação e fechamento

- [x] T009 `learn/forms` + `pt/aprender/formularios`: seção de validação (tag, tipo próprio,
      regra própria, a fronteira com a regra de negócio).
- [x] T010 `reference/ctx` (`Bind`) + `reference/validation` nas duas locales, com a tabela
      de regras e as mensagens.
- [x] T011 `CHANGELOG.md` (0.18.0), `version` em `cmd/trilha/main.go`, ROADMAP (Fase 2,
      item 8).
- [ ] T012 `make test` verde e `make release VERSION=0.18.0 ISSUES="27"`.
