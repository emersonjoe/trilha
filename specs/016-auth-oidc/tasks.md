# Tasks: 016-auth-oidc

- [x] T001 Provedor OIDC falso em `httptest` (descoberta, JWKS, autorização, token) e os
      testes de fluxo feliz e de recusa: `state`, `nonce`, `exp`, assinatura, `aud`,
      `alg: none`, chave rodada
- [x] T002 `auth/jwt.go` + `auth/jwks.go`: JWS compacto, RSA e ECDSA, cache e rotação
- [x] T003 `auth/provider.go`: descoberta com cache, `OIDC`, `EntraID`, `Keycloak`
- [x] T004 `auth/flow.go`: PKCE, `state`, `nonce`, `Start`, `Callback`, `Logout`
- [x] T005 `auth/session.go`: cookie assinado, prazos, rotação, `Store` opcional
- [x] T006 `auth/middleware.go` + `auth/roles.go`: `Require`, `RequireRole`, `User`, papéis
- [x] T007 `examples/sso`: área protegida, papel exigido, logout, testes de integração
- [x] T008 `trilha audit`: segredo do cliente em variável, `redirect_uri` em HTTPS
- [x] T009 Documentação: capítulo, referência, README, CHANGELOG, versão 0.7.0; fechar #40

Todas as tarefas concluídas na v0.7.0. Onde a implementação decidiu diferente do plano:

- A recusa por falta de configuração no `examples/sso` não usa 503 em página HTML: uma
  resposta 5xx vira a tela genérica de erro, que não explica nada. O navegador volta para a
  home, que diz o que falta; só a chamada de API recebe 503.
- `Options` ganhou `IdleOff` (quiosques e formulários longos) e `AfterLogout`, que o plano
  não previa.
- `Auth.Optional()` entrou para páginas que só trocam a saudação — sem ela, todo uso de
  `User` fora de uma subárvore protegida releria o cookie.
