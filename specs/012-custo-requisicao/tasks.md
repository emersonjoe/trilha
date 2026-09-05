# Tasks: 012-custo-requisicao
- [ ] T001 Profile de CPU e memória do cenário JSON; registrar o top-10 na spec (dados, não leitura)
- [ ] T002 CSP precompilada em `applyConfig` (com e sem nonce) + testes de segurança existentes verdes
- [ ] T003 Nonce só em rotas de página; teste: API sem `nonce-`, página com nonce novo a cada requisição
- [ ] T004 Id de requisição sem `crypto/rand` + doc comment; `values` alocado no primeiro `Set`
- [ ] T005 `logRequest` guardado por `Enabled` e `slog.Duration`; teste com handler de nível Error
- [ ] T006 (opcional) `sync.Pool` de `Ctx`/`responseWriter` com `-race`; descartar se não render
- [ ] T007 `make bench-results`, tabela da página de desempenho, CHANGELOG, merge e tag
