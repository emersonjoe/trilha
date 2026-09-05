# Exemplo: sso (Entra ID e Keycloak)

Login corporativo por OpenID Connect, com a biblioteca padrão e mais nada.

O que ensina:

- **O fluxo em três rotas**: `app/entrar` começa, `app/entrar/retorno` termina, `app/sair`
  encerra. Cada uma tem duas linhas — `auth` faz PKCE, `state`, `nonce`, troca do código e
  validação do `id_token`.
- **Proteger uma subárvore**: `app/painel/middleware.go` exige sessão; a página abaixo dela
  já pode ler `sso.User(c)` sem checar nada.
- **Exigir papel**: `app/painel/relatorio/middleware.go` pede o papel de administrador, e
  quem está logado sem ele recebe 403 (401 mandaria de volta ao login, num laço).
- **API e navegador**: `app/api/eu` responde 401 em JSON; `/painel` redireciona para
  `/entrar?next=/painel`.
- **Nenhum segredo no código**: tudo vem do ambiente, e sem configuração o app sobe e
  explica o que falta.

## Rodar

Com Keycloak (um contêiner basta):

```bash
docker run -p 8080:8080 -e KC_BOOTSTRAP_ADMIN_USERNAME=admin -e KC_BOOTSTRAP_ADMIN_PASSWORD=admin quay.io/keycloak/keycloak:26.0 start-dev
```

Crie um realm, um cliente confidencial com `http://localhost:3000/entrar/retorno` como
redirecionamento e um papel de realm chamado `admin`. Depois:

```bash
cd examples/sso
export SSO_PROVIDER=keycloak SSO_URL=http://localhost:8080 SSO_REALM=trilha
export SSO_CLIENT_ID=exemplo SSO_CLIENT_SECRET=... SSO_REDIRECT_URL=http://localhost:3000/entrar/retorno
export TRILHA_SECRET=$(openssl rand -hex 32)
trilha dev
```

Com Entra ID, troque as três primeiras variáveis:

```bash
export SSO_PROVIDER=entra SSO_TENANT=<id-do-diretório>
```

Com o Amazon Cognito:

```bash
export SSO_PROVIDER=cognito SSO_REGION=us-east-1 SSO_USER_POOL_ID=us-east-1_ABC123
export SSO_LOGOUT_DOMAIN=exemplo.auth.us-east-1.amazoncognito.com
```

`SSO_LOGOUT_DOMAIN` é opcional e é o domínio de managed login: o Cognito não publica
`end_session_endpoint`, então sem ele o `/sair` apaga só a sessão local. A URL de retorno
do logout precisa estar nas *Allowed sign-out URLs* do app client.

Variáveis aceitas: `SSO_PROVIDER` (`entra` | `keycloak` | `cognito` | vazio para
`SSO_ISSUER`), `SSO_TENANT`, `SSO_URL`, `SSO_REALM`, `SSO_REGION`, `SSO_USER_POOL_ID`,
`SSO_LOGOUT_DOMAIN`, `SSO_ISSUER`, `SSO_CLIENT_ID`, `SSO_CLIENT_SECRET`,
`SSO_REDIRECT_URL`, `SSO_ADMIN_ROLE` (padrão `admin`), `SSO_ROLE_CLAIMS`.

Teste: `go test ./examples/sso/`.
