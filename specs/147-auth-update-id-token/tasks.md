# Tarefas — Spec 147

- [x] T001 `auth/update_test.go`: `Update` muda o `Extra`, a requisição seguinte com o mesmo
      cookie lê o valor novo, a resposta não traz `Set-Cookie`, o `SessionID` não muda; o handler
      que chamou lê o valor novo em `User(c)`/`Tenant(c)`; sem sessão dá `ErrNoSession`; store que
      falha devolve o erro; sem store (sessão no cookie) o valor novo também persiste
- [x] T002 `auth/idtoken_test.go`: `OnLoginToken` recebe o `Raw` que confere contra o JWKS do
      provedor de teste e `Claims.All["hd"]`; erro dele recusa o login; `OnLogin` roda antes;
      `Login` (sem provedor) não chama `OnLoginToken`; o token não fica na sessão nem no cookie
- [x] T003 `auth/session.go`: `persist(c, u, cookie bool)`, `write` sobre ele, `Auth.Update`
- [x] T004 `auth/tenant.go`: `SwitchTenant` sobre `Update`
- [x] T005 `auth/flow.go`: `IDToken`, `Options.OnLoginToken`, chamada no `Callback`
- [x] T006 `api/current.txt` (`go test -run TestSuperficiePublica -update .`)
- [x] T007 Referência do `auth` em en e pt: `Update` na lista do `Auth`, `OnLoginToken` na tabela
      de `Options`, uma seção para cada caso
- [x] T008 Uso em `examples/`: `Update` no `local-login` (o token que o acervo renovou, com teste
      de integração) e `OnLoginToken` no `sso` (claims que o kit não mapeia, com teste)
- [x] T009 `CHANGELOG.md` (Added), `version` = 0.126.0, `ROADMAP.md`
- [ ] T010 `make test` verde e `make release VERSION=0.126.0 ISSUES="206 208"`
