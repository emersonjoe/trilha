# Tasks: API pública e política de depreciação

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — renderizador (SC-002, SC-004)

- [ ] T001 Teste que falha em `internal/apisurface/apisurface_test.go`: pacote sintético em
      `t.TempDir()` com func, método de ponteiro e de valor, struct com campo exportado e
      não exportado, interface, const, var, tipo função, alias e um símbolo com
      `// Deprecated:`; asserções sobre as linhas, a ordem e a ausência do não exportado e do
      `_test.go`.
- [ ] T002 `internal/apisurface/apisurface.go`: `Render` com `go/parser` + `go/printer`.

## Bloco 2 — golden da superfície (SC-002, SC-003)

- [ ] T003 `api_test.go` na raiz: `TestSuperficiePublica` compara `api/current.txt`, com `-update`
      para regravar, e mensagem de falha listando `+`/`-`; `TestPacotesPublicosNaLista` garante
      que nenhum pacote público ficou de fora.
- [ ] T004 `api/current.txt` gerado; alvo `api` no `Makefile` e `.PHONY`.

## Bloco 3 — documentos (SC-001)

- [ ] T005 `API.md`: escopo coberto e não coberto, o que conta como quebra, política de
      depreciação, o que a 1.0 muda, como rodar `make api`, e a frase sobre o que o teste não
      pega.
- [ ] T006 `docs/pt-BR/API.md`: tradução no mesmo commit.
- [ ] T007 Ponteiro em `GOVERNANCE.md` e `docs/pt-BR/GOVERNANCE.md`.

## Bloco 4 — constituição (SC-005)

- [ ] T008 `.specify/memory/constitution.md`: emenda com a fronteira e o ciclo de depreciação;
      versão 1.4.0 e `Last Amended`.

## Bloco 5 — fechamento

- [ ] T009 `CONTRIBUTING.md` e `docs/pt-BR/CONTRIBUTING.md`: `make api` no bloco de comandos e a
      regra de revisar o diff do `api/current.txt`.
- [ ] T010 `CHANGELOG.md` (0.26.0), `version` em `cmd/trilha/main.go`, ROADMAP (§1 e §20).
- [ ] T011 `make test` verde e `make release VERSION=0.26.0 ISSUES="35"`.
