# Tasks: O portão que ensina

- [ ] **T01** — `internal/checkerr` com `Doc` + catálogo completo (`ErrorGuides`, códigos do
  scanner, códigos das ferramentas e famílias) + `TestErrorCatalogComplete` varrendo
  `E_[A-Z_]+` do código-fonte. *Aceite:* `go test ./internal/checkerr/`.
- [ ] **T02** — Orquestrador: cada portão do `check` carrega `code/hint/doc` do catálogo;
  govulncheck como portão de segurança quando presente. *Aceite:*
  `go test ./cmd/trilha/ -run Check`.
- [ ] **T03** — `--json` com o esquema do plano + exit codes 0/1/2; e2e atualizado.
  *Aceite:* `go test ./cmd/trilha/ -run 'Check|E2E'`.
- [ ] **T04** — Mapeadores gofmt/vet/test/vuln/api com fixtures de saída real.
  *Aceite:* `go test ./cmd/trilha/ -run Mapper`.
- [ ] **T05** — `/docs/errors` e `/docs/errors/<code>` geradas do catálogo (busca + âncoras),
  nas duas línguas. *Aceite:* `go test ./site/ -run Errors`.
- [ ] **T06** — MCP `get_error(code)` + referência bilíngue. *Aceite:*
  `go test ./site/internal/docsmcp/`.
- [ ] **T07** — Régua S8: nota de fechamento (rodadas-até-verde já são a coluna `rounds` da
  série; `check --json` é o sinal do agente, documentado). *Aceite:* tasks.md.
- [ ] **T08** — CHANGELOG/ROADMAP + suítes. *Aceite:* `make test` verde.
