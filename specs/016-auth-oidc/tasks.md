# Tasks: 016-auth-oidc

- [ ] T001 Provedor OIDC falso em `httptest` (descoberta, JWKS, autorização, token) e os
      testes de fluxo feliz e de recusa: `state`, `nonce`, `exp`, assinatura, `aud`,
      `alg: none`, chave rodada
- [ ] T002 `auth/jwt.go` + `auth/jwks.go`: JWS compacto, RSA e ECDSA, cache e rotação
- [ ] T003 `auth/provider.go`: descoberta com cache, `OIDC`, `EntraID`, `Keycloak`
- [ ] T004 `auth/flow.go`: PKCE, `state`, `nonce`, `Start`, `Callback`, `Logout`
- [ ] T005 `auth/session.go`: cookie assinado, prazos, rotação, `Store` opcional
- [ ] T006 `auth/middleware.go` + `auth/roles.go`: `Require`, `RequireRole`, `User`, papéis
- [ ] T007 `examples/sso`: área protegida, papel exigido, logout, testes de integração
- [ ] T008 `trilha audit`: segredo do cliente em variável, `redirect_uri` em HTTPS
- [ ] T009 Documentação: capítulo, referência, README, CHANGELOG, versão 0.7.0; fechar #40
