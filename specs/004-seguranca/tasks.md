# Tasks: Segurança por padrão
- [x] T001 Testes: cabeçalhos/CSP/nonce/HSTS/proxy (`security_test.go`), rate limit (`ratelimit_test.go`), cookies assinados (`signed_test.go`), eventos (`events_test.go`)
- [x] T002 Implementar `security.go`, `proxy.go`, `ratelimit.go`, `signed.go`, `events.go`; timeouts em `trilha.go`; nonce no script de dev
- [x] T003 Exemplo: login com `SetSigned`, middleware admin com `Signed`; `TRILHA_SECRET` no dev (chave efêmera) e no e2e
- [x] T004 `cmd/trilha/audit.go` + teste
- [x] T005 Docs: capítulo Aprender "Segurança", Referência "Segurança", SECURITY.md, CHANGELOG; CI govulncheck
- [x] T006 `make test` verde; verificar cabeçalhos no site publicado após deploy
