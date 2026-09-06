# Tasks: auxiliares de teste

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — núcleo (SC-002, SC-003, SC-004)

- [x] T001 Teste que falha em `testing_test.go` (raiz): `TestRequest` num app com página e
      API; `POST` de página passa no CSRF sozinho e dá 403 com `WithoutCSRF()`; `WithSigned`
      abre rota protegida; `TestPage` devolve `Node` com layout aplicado; `TestRoute` resolve
      `{id}`; `TestClient` guarda o cookie que o handler põe; `WithJSON`/`JSON(&v)`.
- [x] T002 `testing.go`: `TestingT`, `TestOption`, `TestResponse`, `TestClient`,
      `TestRequest`, `TestRoute`, `TestPage` e as oito opções.

## Bloco 2 — exemplos (SC-001)

- [x] T003 `examples/orcamento` e `examples/cadastro` (os dois menores) passam a usar os
      auxiliares; some o `client` local.
- [x] T004 `examples/assistente`, `examples/sso` e `examples/blog` idem; conferir que a soma
      das suítes encolheu.

## Bloco 3 — documentação e fechamento

- [x] T005 Página nova `learn/testing.md` + `pt/aprender/testes.md`, registrada em
      `site/internal/docs/docs.go`, com o exemplo de cada auxiliar e a nota do CSRF.
- [x] T006 `reference/app` nas duas locales: a tabela do canto de teste.
- [x] T007 `CHANGELOG.md` (0.23.0), `version` em `cmd/trilha/main.go`, ROADMAP (Fase 3, item 14).
- [x] T008 `make test` verde e `make release VERSION=0.23.0 ISSUES="32"`.
