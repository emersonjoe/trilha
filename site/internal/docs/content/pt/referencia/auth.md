---
title: auth
description: Provider, Options, Auth, User e Store — a API do pacote auth, com os padrões e o que cada campo muda.
---

`import "github.com/emersonjoe/trilha/auth"` — login OpenID Connect com a biblioteca padrão.
O pacote não registra rota: expõe manipuladores que o seu `app/` publica.

## Provedores

```go
func OIDC(issuer, clientID, clientSecret, redirectURL string) *Provider
func EntraID(tenant, clientID, clientSecret, redirectURL string) *Provider
func Keycloak(baseURL, realm, clientID, clientSecret, redirectURL string) *Provider
func Cognito(region, userPoolID, clientID, clientSecret, redirectURL string) *Provider
func Clerk(frontendAPI, clientID, clientSecret, redirectURL string) *Provider
```

| Construtor | Emissor resultante | Papéis lidos de |
|---|---|---|
| `OIDC` | o que você passar | `roles`, `groups` |
| `EntraID` | `https://login.microsoftonline.com/<tenant>/v2.0` | `roles`, `groups`, `wids` |
| `Keycloak` | `<baseURL>/realms/<realm>` | `realm_access.roles`, `resource_access[clientID].roles` |
| `Cognito` | `https://cognito-idp.<region>.amazonaws.com/<userPoolID>` | `cognito:groups` |
| `Clerk` | a Frontend API URL, normalizada (`https://<slug>.clerk.accounts.dev`) | `roles`, `groups` — o `id_token` do Clerk traz a organização (`org_id`), não o papel nela; uma claim configurada entra em `Options.RoleClaims` |

`Provider.LogoutDomain` existe por causa do Cognito: aponte-o para o domínio de managed login
(`<prefixo>.auth.<região>.amazoncognito.com`, ou o seu próprio) e o `Logout` redireciona
para `/logout?client_id=…&logout_uri=…` lá; a URL de retorno precisa estar nas *Allowed
sign-out URLs* do app client. Vazio, o `Logout` apaga a sessão local, diz isso no log e não
finge que federou. Os outros provedores ignoram o campo. O Clerk também não publica
`end_session_endpoint`, e não tem endereço equivalente: lá o `Logout` é sempre local, e o log
diz que a sessão do Clerk ficou de pé.

`Provider.HTTPClient` troca o cliente HTTP (padrão: 10 s de prazo). A descoberta é feita no
primeiro uso e vale por uma hora; um emissor divergente entre a configuração e o documento
é erro, não aviso.

## Options

| Campo | Padrão | O que faz |
|---|---|---|
| `Scopes []string` | `openid profile email` | escopos pedidos ao provedor |
| `Absolute time.Duration` | 8 h | prazo máximo da sessão, contado do login |
| `Idle time.Duration` | 30 min | encerra sessão parada; `IdleOff: true` desliga |
| `CookieName string` | `trilha_session` | nome do cookie de sessão |
| `LoginPath string` | `/entrar` | para onde `Require` manda um navegador anônimo |
| `AfterLogin string` | `/` | destino após o retorno, quando não há `next` |
| `AfterLogout string` | `/` | destino após o logout |
| `RoleClaims []string` | — | claims adicionais de onde ler papéis |
| `Store Store` | `nil` | persiste a sessão; `nil` = cookie assinado, sem estado |

## Auth

```go
func New(p *Provider, o Options) *Auth      // não faz rede
func (a *Auth) Start(c *trilha.Ctx) error   // → provedor (PKCE, state, nonce)
func (a *Auth) Callback(c *trilha.Ctx) error // valida o retorno e cria a sessão
func (a *Auth) Logout(c *trilha.Ctx) error   // apaga a sessão; RP-Initiated Logout quando existe
func (a *Auth) Require() trilha.MiddlewareFunc
func (a *Auth) RequireRole(roles ...string) trilha.MiddlewareFunc
func (a *Auth) Optional() trilha.MiddlewareFunc
func (a *Auth) User(c *trilha.Ctx) *User     // nil quando anônimo
func (a *Auth) Session(c *trilha.Ctx) (*User, error)
```

`Require` responde **302** para o login quando a requisição é uma navegação (Accept com
`text/html`, fora de `/api/`) e **401** caso contrário. `RequireRole` responde **403** para
quem está autenticado sem o papel. Basta **um** dos papéis listados; a comparação ignora
maiúsculas.

## User

```go
type User struct {
	Subject   string    // sub: o identificador estável
	Email     string    // email, ou preferred_username quando não há
	Name      string
	Roles     []string
	IssuedAt  time.Time // momento do login
	ExpiresAt time.Time
	Seen      time.Time // última atividade (janela de ociosidade)
	SessionID string    // muda a cada login
}

func (u *User) HasRole(role string) bool
```

## Store

```go
type Store interface {
	Save(id string, u *User, ttl time.Duration) error
	Load(id string) (*User, bool)
	Delete(id string) error
}

func NewMemoryStore() *MemoryStore
```

Com um `Store` o cookie carrega apenas o identificador e o logout tem efeito imediato para
todo mundo. `MemoryStore` vale para um processo só: réplicas não compartilham, e um
reinício derruba todas as sessões. Para várias réplicas, implemente a interface sobre o seu
banco ou cache.

## Cookies

| Cookie | Validade | Conteúdo |
|---|---|---|
| `trilha_oidc_state` | 10 min | `state` do pedido em curso |
| `trilha_oidc_nonce` | 10 min | `nonce` do pedido em curso |
| `trilha_oidc_verifier` | 10 min | verificador PKCE |
| `trilha_oidc_next` | 10 min | destino após o login (só caminho relativo) |
| `trilha_session` | `Absolute` | a sessão (ou o id dela, com `Store`) |

Todos são assinados (exigem `TRILHA_SECRET`), `HttpOnly`, `SameSite=Lax` e `Secure` sob
HTTPS. Os quatro do fluxo são apagados no retorno, dê certo ou não.

## Algoritmos aceitos

`RS256`, `RS384`, `RS512`, `ES256`, `ES384`. A lista é fixa: o `alg` do token não escolhe
nada. Chaves RSA com módulo menor que 2048 bits são ignoradas no JWKS, o `kid` é
obrigatório, e a tolerância de relógio é de 60 segundos.

## Auditoria

`trilha audit` verifica, quando o projeto importa `trilha/auth`: segredo do cliente escrito
no código (crítico) e `redirect_uri` em `http://` fora de `localhost` (crítico).
