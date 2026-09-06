# Tasks: `-race` e fuzzing no CI

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — concorrência (SC-001)

- [x] T001 `race_test.go` na raiz: N goroutines contra o mesmo `*App`, passando por `c.Asset`,
      métrica, rate limit, cookie assinado e `Values()`; cada resposta confere seu invariante.
- [x] T002 `go test -race -run TestConcorrencia .` verde e, com o `sync` de um ponto com estado
      removido à mão, acusando corrida (verificação manual, anotada no bloco).

## Bloco 2 — alvos de fuzzing (SC-002, SC-005)

- [x] T003 `fuzz_test.go` na raiz: `FuzzRouteMatch` (app com rota estática, `{id}`,
      `{path...}`, API e `Public`, mais arquivo isca fora da raiz servida),
      `FuzzParseTraceparent` e `FuzzSignedVerify`.
- [x] T004 `fuzz_test.go`: `FuzzBindForm` e `FuzzBindJSON` — sem erro implica `validate`
      respeitado.
- [x] T005 `h/fuzz_test.go`: `FuzzRenderEscapes` — `html.UnescapeString` do texto e do atributo
      renderizados volta ao valor original, e o resultado não traz `<` nem `"` soltos.
- [x] T006 Cada alvo rodado com fuzzing de verdade (`make fuzz`); o que aparecer vira caso em
      `testdata/fuzz/<Alvo>/`, commitado, e correção nesta spec ou issue nova.

## Bloco 3 — ferramenta e CI (SC-003, SC-004)

- [x] T007 `scripts/fuzz.sh` (lista de pacote/alvo, `FUZZTIME`) e alvos `race`, `fuzz`,
      `fuzz-long` no `Makefile`.
- [x] T008 `.github/workflows/ci.yml`: job `race` (`go test -race ./...`, Go estável) e job
      `fuzz` (`FUZZTIME=20s ./scripts/fuzz.sh`), ambos paralelos aos existentes.

## Bloco 4 — documentação e fechamento

- [x] T009 Seção "Race e fuzzing" em `learn/testing` + `pt/aprender/testes`: como fuzzar um
      handler do próprio app com `TestRequest`, e o que o framework já fuzza.
- [x] T010 `CONTRIBUTING.md` e `docs/pt-BR/CONTRIBUTING.md`: comandos novos e a regra do corpus
      commitado.
- [x] T011 `CHANGELOG.md` (0.24.0), `version` em `cmd/trilha/main.go`, ROADMAP (Fase 3, item 15).
- [ ] T012 `make test` verde e `make release VERSION=0.24.0 ISSUES="33"`.
