# Tasks: O portão que ensina

- [x] **T01** — `internal/checkerr` com `Doc` + catálogo completo (`ErrorGuides`, códigos do
  scanner, códigos das ferramentas e famílias) + `TestErrorCatalogComplete` varrendo
  `E_[A-Z_]+` do código-fonte. *Aceite:* `go test ./internal/checkerr/`.
- [x] **T02** — Orquestrador: cada portão do `check` carrega `code/hint/doc` do catálogo;
  govulncheck como portão de segurança quando presente. *Aceite:*
  `go test ./cmd/trilha/ -run Check`.
- [x] **T03** — `--json` com o esquema do plano + exit codes 0/1/2; e2e atualizado.
  *Aceite:* `go test ./cmd/trilha/ -run 'Check|E2E'`.
- [x] **T04** — Mapeadores gofmt/vet/test/vuln/api com fixtures de saída real.
  *Aceite:* `go test ./cmd/trilha/ -run Mapper`.
- [x] **T05** — `/docs/errors` e `/docs/errors/<code>` geradas do catálogo (busca + âncoras),
  nas duas línguas. *Aceite:* `go test ./site/ -run Errors`.
- [x] **T06** — MCP `get_error(code)` + referência bilíngue. *Aceite:*
  `go test ./site/internal/docsmcp/`.
- [x] **T07** — Régua S8: nota de fechamento. A régua v2 (spec 159) já mede
  rodadas-até-verde — a coluna `rounds` da série publicada em `/custos` — e os prompts são
  congelados, então nada aqui os reabre. O que esta spec mudou para o S8 é a qualidade do
  sinal: `trilha check` agora imprime código + conserto + link por falha (e `check --json`
  dá o mesmo para máquina), e o AGENTS.md v2 já aponta o portão. A re-medição do S8 é do
  mantenedor, junto da primeira medição da série.
  *Aceite:* tasks.md.
- [ ] **T08** — CHANGELOG/ROADMAP + suítes. *Aceite:* `make test` verde.
