# Plano: 016-auth-oidc

## Superfície pública (pacote `auth`)

```go
func OIDC(issuer, clientID, clientSecret, redirectURL string) *Provider
func EntraID(tenant, clientID, clientSecret, redirectURL string) *Provider
func Keycloak(baseURL, realm, clientID, clientSecret, redirectURL string) *Provider

type Options struct {
    Scopes        []string      // padrão: openid profile email
    Absolute      time.Duration // 8h
    Idle          time.Duration // 30m
    CookieName    string        // "trilha_session"
    LoginPath     string        // "/entrar"
    AfterLogin    string        // "/"
    RoleClaims    []string      // extras, além dos do provedor
    Store         Store         // opcional (revogação, sessão grande)
}

func New(p *Provider, o Options) *Auth
func (a *Auth) Start(c *trilha.Ctx) error     // GET /entrar
func (a *Auth) Callback(c *trilha.Ctx) error  // GET /entrar/retorno
func (a *Auth) Logout(c *trilha.Ctx) error    // POST /sair
func (a *Auth) Require() trilha.MiddlewareFunc
func (a *Auth) RequireRole(roles ...string) trilha.MiddlewareFunc
func (a *Auth) User(c *trilha.Ctx) *User      // nil quando anônimo

type User struct {
    Subject, Email, Name string
    Roles                []string
    IssuedAt, ExpiresAt  time.Time
    Claims               map[string]any
}
func (u *User) HasRole(r string) bool
```

## Arquivos

| Arquivo | Papel |
|---|---|
| `auth/provider.go` | `Provider`, descoberta (`/.well-known/openid-configuration`) com cache, atalhos Entra ID e Keycloak |
| `auth/jwks.go` | busca e cache do JWKS, rotação com limite de frequência, RSA e ECDSA |
| `auth/jwt.go` | decodificação e validação de JWS compacto (`RS256`, `RS384`, `RS512`, `ES256`, `ES384`) |
| `auth/flow.go` | PKCE, `state`, `nonce`, `Start`, `Callback`, troca de código, `Logout` |
| `auth/session.go` | cookie de sessão assinado, prazos absoluto e de inatividade, rotação, `Store` |
| `auth/middleware.go` | `Require`, `RequireRole`, `User` |
| `auth/roles.go` | extração de papéis por provedor (Entra: `roles`/`groups`; Keycloak: `realm_access`/`resource_access`) |
| `auth/*_test.go` | provedor OIDC falso em `httptest`: fluxo feliz e sete caminhos de ataque |
| `examples/sso/` | app com área protegida, papel exigido e logout |
| `site/.../aprender/autenticacao.md`, `.../referencia/auth.md` | documentação |

## Decisões

1. **Sessão no cookie, assinada com `TRILHA_SECRET`.** Sem banco, sem Redis, sem estado
   entre réplicas. O que vai dentro é pequeno: `sub`, `email`, `name`, papéis e prazos —
   não o `id_token` inteiro, e nunca o `refresh_token`.
2. **Sem `refresh_token` no cookie.** Guardá-lo no navegador transforma um vazamento de
   cookie em acesso persistente. Quem precisar de sessão longa liga um `Store`.
3. **PKCE sempre**, mesmo com segredo de cliente: é o que protege contra interceptação do
   código no retorno, e o Entra ID já exige em app público.
4. **Tolerância de relógio de 60 s** nos prazos do token: relógio de servidor desalinhado é
   a causa mais comum de "login que não funciona só em produção".
5. **Descoberta com cache e prazo.** Um provedor fora do ar não pode virar um `GET` a cada
   login; o documento e o JWKS ficam em memória com validade de 1 h e rebusca limitada.
6. **Rotas vêm de arquivos.** O pacote não registra nada: o app escreve três `route.go` de
   três linhas. Isso mantém o princípio I e deixa o caminho das URLs com quem escreve o app.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I | nenhuma rota registrada pelo pacote; o app expõe os handlers em `app/` |
| II | só biblioteca padrão (`crypto/*`, `encoding/*`, `net/http`) |
| III | nada de reflexão para descobrir handler; o gerador não muda |
| IV | os handlers têm a assinatura normal `func(*trilha.Ctx) error` |
| V | sem serviço externo em dev: o exemplo roda contra um provedor falso embutido |
| VI | teste antes: provedor falso, fluxo feliz e sete recusas |
| VII | segredo fora do log, PKCE, cookies restritos, sem redirecionamento aberto, evento de segurança em toda falha |

## Complexity Tracking

Validar JWS à mão é a parte delicada. Mitigação: lista de algoritmos permitida (nunca lida
do token sem conferir), `kid` obrigatório, teste para `alg: none` e para troca de algoritmo
assimétrico por simétrico — as duas falhas clássicas de biblioteca de JWT.
