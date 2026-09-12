---
title: auth
description: Provider, Options, Auth, User e Store — a API do pacote auth, com os padrões e o que cada campo muda.
---

`import "github.com/emersonjoe/trilha/auth"` — login com a biblioteca padrão, por um provedor
(OpenID Connect) ou contra a tabela de usuários do próprio app. O pacote não registra rota:
expõe manipuladores que o seu `app/` publica.

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

Essas claims são lidas do `id_token` **e do access token**, quando o access token for um JWT
do mesmo emissor — onde os dois trazem a mesma claim, vale a do `id_token`. Isso não é
firula: **o Keycloak põe `realm_access` e `resource_access` só no access token**, então ler
apenas o `id_token` dá um login que funciona com um `Roles` vazio, e todo `RequireRole`
responde 403 sem motivo aparente. Access token opaco — o que a maioria dos provedores emite —
não muda nada; um que não confere não vale papel nenhum.

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
| `Audience string` | — | nomeia o público que este `Auth` atende, para dois deles no mesmo processo |
| `LoginPath string` | `/entrar` | para onde `Require` manda um navegador anônimo |
| `AfterLogin string` | `/` | destino após o retorno, quando não há `next` |
| `AfterLogout string` | `/` | destino após o logout |
| `RoleClaims []string` | — | claims adicionais do `auth.Claims` (o ID token decodificado) de onde ler papéis, além dos padrão |
| `Store Store` | `nil` | persiste a sessão; `nil` = cookie assinado, sem estado |
| `OnLogin func(c, *User) error` | — | roda dentro de `Login` e `Callback`, com a sessão ainda não gravada; o erro dele impede o login |
| `RequireVerifiedEmail bool` | `false` | recusa o login cujo e-mail o provedor não garante |

### O e-mail que o provedor não garantiu

Um provedor que responde com `email` não está dizendo que aquele endereço é desta pessoa.
Isso ele diz numa segunda claim: `email_verified` (OIDC Core §5.1). O Google, e qualquer
provedor onde alguém pode digitar um endereço no perfil, manda `email_verified: false` — a
pessoa escreveu, ninguém conferiu.

Isso importa para quem autoriza por e-mail, que é quase toda ferramenta interna: uma lista de
permitidos, ou o domínio da empresa. O `u.Email` sozinho autoriza o que alguém digitou.

O `User.EmailVerified` carrega a claim, então o `OnLogin` decide. `RequireVerifiedEmail: true`
decide por você, e a recusa acontece **antes** do `OnLogin` — a sua regra nunca vê um endereço
que não devia existir:

```go
auth.New(p, auth.Options{
	RequireVerifiedEmail: true,
	OnLogin: func(c *trilha.Ctx, u *auth.User) error {
		if !permitidos[u.Email] {
			return errors.New("e-mail fora da lista")
		}
		return nil
	},
})
```

Vem desligado porque ligar muda quem entra. Dois detalhes que valem saber:

- a forma em string conta. Alguns provedores mandam `"true"` em vez do booleano do JSON, e as
  duas são lidas como verificado;
- um e-mail que veio do `preferred_username` — a saída para o provedor que não manda `email`
  nenhum — **nunca** é verificado. Um nome de usuário não é um endereço que alguém garantiu.

O `auth.Sessions` não tem provedor nem claim: ali o `EmailVerified` é seu para preencher, e só
a sua aplicação sabe se confirmou o endereço.

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
func (a *Auth) LoginPath() string            // Options.LoginPath, ou o padrão
func (a *Auth) Provider() *Provider          // o provedor configurado, para a tela que o nomeia
func (a *Auth) Sessions(c *trilha.Ctx) ([]User, error) // todas as sessões de quem está logado, esta primeiro
func (a *Auth) LogoutOthers(c *trilha.Ctx) error       // encerra as outras, mantém esta

func Sessions(o Options) *Auth               // o mesmo tipo, sem provedor
func (a *Auth) Login(c *trilha.Ctx, u *User) error // sessão para quem o app autenticou
func (a *Auth) RequireFunc(pred func(*User, *trilha.Ctx) bool) trilha.MiddlewareFunc
```

`Require` responde **302** para o login quando a requisição é uma navegação (Accept com
`text/html`, fora de `/api/`) e **401** caso contrário. `RequireRole` responde **403** para
quem está autenticado sem o papel. Basta **um** dos papéis listados; a comparação ignora
maiúsculas.

### Dois públicos no mesmo processo

A área interna e o portal são rotas da mesma árvore, com um middleware cada — o desenho que
este framework incentiva. Dê a cada `Auth` o seu **`Audience`**:

```go
var (
	Portal  = auth.Sessions(auth.Options{Audience: "portal", CookieName: "portal_session", Store: lojaPortal})
	Interno = auth.Sessions(auth.Options{Audience: "interno", CookieName: "interno_session", Store: lojaInterna})
)
```

Cookie próprio e `Store` próprio **não** bastam para separá-los. Sem audience, todo `Auth` do
binário guarda o usuário da requisição no mesmo slot do `Ctx`: uma chamada de
`Interno.User(c)` a partir do lado do portal — um helper, um layout, um handler que pegou a
instância errada — responde com a pessoa que o outro lado acabou de deixar entrar, e a página
é desenhada com a identidade errada. Nada falha e nada é registrado.

Nomear o público separa os dois nos dois lugares que decidem:

| | Com `Audience` |
|---|---|
| o slot do `Ctx` | um por instância: o `Interno.User(c)` não lê o que o `Portal` gravou |
| a sessão | carrega o público (`User.Audience`), e o `Session` recusa a de outro — o mesmo nome de cookie, ou um `Store` compartilhado, deixa de ser porta |

Uma aplicação com um `Auth` só — a esmagadora maioria — não vê nada disso: sem `Audience`, o
slot e a sessão são exatamente os de hoje. Ligar a opção num app que já está no ar encerra as
sessões abertas uma vez, porque elas foram gravadas sem público.

O `auth.Tenant(c)` continua respondendo pela sessão por onde a requisição passou, e o
`Keys.User(c)` não muda: uma chave de API não é sessão e não tem público, então um `Auth` com
`Audience` não responde com o chamador de uma chave.

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
	Extra map[string]string // o que o OIDC não tem claim para dizer (token da API, tenant)
}

func (u *User) HasRole(role string) bool
```

## A matriz de permissões

Uma lista de papéis responde "esta pessoa é admin". O que uma aplicação pergunta de verdade é
"esta pessoa pode editar documentos", e a resposta é uma matriz: papel × módulo × nível.

```go
var Policy = auth.Policy{
	Modules: []string{"docs", "processos", "rh"},
	Levels:  auth.Levels{"ver", "editar", "administrar"},   // ordenados: administrar ⊇ editar ⊇ ver
	Roles: map[string]auth.Grants{
		"admin":    auth.All("administrar"),
		"analista": {"docs": "editar", "processos": "ver"},
		"leitor":   auth.All("ver"),
	},
}
```

A ordem de `Levels` é o significado inteiro: `administrar` cobre `editar`, que cobre `ver`,
então uma célula guarda um valor em vez de três booleanos. Módulo que o papel não nomeia é
módulo que ele não alcança — a ausência é negação, nunca herança — e `Default` é o que um
usuário logado sem papel conhecido pode fazer, que é nada até você dizer o contrário. Papel
apagado tem que perder acesso, não herdar o de outro.

| Símbolo | O que faz |
|---|---|
| `Policy.Can(u, módulo, nível) bool` | este usuário pode isto |
| `Policy.Level(u, módulo) string` | o que ele tem, ou `""` |
| `Policy.LevelOf(papel, módulo) string` | o nível de um **papel**, para a célula da grade, onde ainda não há usuário |
| `Policy.RolesSorted()`, `Policy.ModuleNames()`, `Policy.LevelNames()` | os nomes em ordem estável — papéis ordenados, módulos como declarados, níveis do mais fraco ao mais forte — que é o que faz uma grade desenhar igual duas vezes |
| `(*Auth) RequirePolicy(p, módulo, nível)` | o guarda |
| `auth.All(nível) Grants` | o mesmo nível em todo módulo |
| `auth.BindPolicy(c, p)` | lê de volta o que a grade postou |
| `auth.PolicyStore`, `auth.PolicyFrom` | matriz que se edita |
| `ui.PolicyGrid(p, opts)` | a tela que edita |

### O guarda

```go
// app/docs/middleware.go
var ver = acesso.Auth.RequirePolicy(acesso.Policy, "docs", "ver")

func Middleware(c *trilha.Ctx, next trilha.Next) error { return ver(c, next) }
```

Duas linhas e não uma: o `middleware.go` precisa exportar uma **função** com aquela assinatura,
e uma var do tipo certo não é uma. O `MiddlewarePOST` guarda um método só, então uma pasta pode
ser legível por um nível e gravável por outro.

Anônimo vai para o login. Usuário logado que não pode recebe **403** — ele é conhecido, só não
autorizado, e mandá-lo ao login viraria laço — e a mensagem diz o que faltou: `needs editar on
docs`. É essa frase que a pessoa repete para quem administra a aplicação; um "forbidden" seco
transforma um conserto de dois minutos numa thread de suporte.

### Esconder botão não é regra

```go
if acesso.Policy.Can(acesso.Auth.User(c), "docs", "editar") { … }
```

Correto e cosmético. A regra que vale é o middleware, porque botão escondido continua sendo um
endereço que dá para digitar.

O `trilha audit` avisa quando a política declara um módulo e nenhuma rota o exige: a matriz diz
que a área está protegida, e se ninguém pede, a proteção é uma frase num arquivo.

### Matriz que se edita

`Policy` é dado no código, que é onde ele mora quando só um deploy o muda. Aplicação cujos
administradores inventam papéis guarda os papéis numa tabela:

```go
type PolicyStore interface {
	Load(ctx context.Context) (map[string]Grants, error)
	Save(ctx context.Context, roles map[string]Grants) error
}

policy, err := auth.PolicyFrom(ctx, store, padroes)   // store vazio mantém os padrões
```

O `PolicyFrom` é um retrato de propósito: `Policy` é um valor, e um valor que mudasse debaixo
de uma requisição deixaria uma requisição responder duas vezes — liberado no middleware,
negado no botão. Chame de novo depois de salvar.

A tela vem pronta:

```go
ui.PolicyGrid(policy, ui.PolicyGridOpts{
	Action: "/admin/permissoes",
	CSRF:   trilha.CSRFInput(c),
	Labels: map[string]string{"docs": "Documentos"},
})
```

Uma linha por papel, uma coluna por módulo, um select de níveis por célula, campos chamados
`grant.<papel>.<módulo>` — que é o que o `auth.BindPolicy` lê do outro lado. Sem JavaScript: é
um formulário, ele posta, o handler salva e redireciona. Célula que nomeia módulo ou nível que
a política não declara é descartada em silêncio, porque responder 400 só diria a quem forjou
qual nome tentar em seguida.

Guarde essa tela com o módulo que administra a aplicação. Ela é a tela mais valiosa que existe.

### O que isto não expressa

Regra sobre um registro — o dono de um documento, a linha de um inquilino — continua sendo
`RequireFunc`, escrito à mão. Uma política que alcançasse o dado precisaria do dado, e aí seria
uma consulta, não uma declaração.

## Sessão sem OIDC

Um app cujos usuários são uma tabela sua — e-mail, hash de senha, papel — monta o mesmo
`*Auth` com `Sessions` e diz sozinho quem é a pessoa:

```go
sessoes := auth.Sessions(auth.Options{Store: store, LoginPath: "/entrar"})

// app/entrar/page.go
func POST(c *trilha.Ctx) error {
	u, err := usuarios.Verify(c.Form("email"), c.Form("senha"))
	if err != nil {
		return c.Render(422, formulario(c, "e-mail ou senha inválidos"))
	}
	return sessoes.Login(c, &auth.User{Subject: u.ID, Email: u.Email,
		Roles: []string{u.Papel}, Extra: map[string]string{"api_token": u.JWT}})
}
```

`Login` redireciona para o `next` que pediram, ou para o `AfterLogin`. `Start` e `Callback`
respondem erro claro num `Auth` sem provedor; `Logout` limpa a sessão e aterrissa, já que
não há nada acima deste app para encerrar.

`User.Extra` é o que esta sessão carrega e uma claim não sabe dizer — o token que um
[upstream](/pt/referencia/upstreams) injeta, o tenant. Ele viaja por onde o resto da sessão
viaja: deixe-o pequeno e nunca ponha uma senha ali.

### Senhas

```go
func HashPBKDF2(senha string) (string, error)   // pbkdf2_sha256$600000$sal$hash
func CheckPBKDF2(codificado, senha string) bool // tempo constante; false para hash ilegível
func PBKDF2(senha, sal []byte, iter, tamanho int) []byte
```

O `CheckPBKDF2` lê duas grafias, decididas pelo prefixo, porque a tabela que ele precisa manter
funcionando nem sempre é a do Django:

| Escrito por | Forma |
|---|---|
| Django, passlib | `pbkdf2_sha256$<iterações>$<sal>$<digest base64>` |
| `hashlib`, à mão | `pbkdf2$<iterações>$<sal hex>$<digest hex>` |

A segunda existe porque o `hashlib.pbkdf2_hmac` devolve bytes e formato nenhum, então uma app
que não usa Django nem passlib escolhe um, e o que se escolhe é hex. Ali o sal é hex que foi
decodificado para bytes antes da derivação, e é por isso que lê-lo como texto dá a resposta
errada para a senha certa. Só SHA-256; `pbkdf2_sha512` é `false`.

O `HashPBKDF2` continua gravando a primeira: uma grafia para escrever, duas para ler. Assim a
tabela de usuários que já existe continua valendo sem migração de senha — o número de iterações
viaja dentro de cada hash, então aumentar `DefaultPBKDF2Iterations` não tranca ninguém do lado
de fora.
`bcrypt` e `argon2` fazem isso melhor e nenhum dos dois está na biblioteca padrão, que é a
razão inteira de este estar aqui.

### Autorização que os papéis não expressam

Uma matriz de módulo e nível, um tenant, o dono de um registro: todos a mesma forma, e
nenhum deles uma lista de nomes de papel.

```go
// app/painel/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return sessoes.RequireFunc(func(u *auth.User, c *trilha.Ctx) bool {
		return u.Extra["tenant"] == c.Param("tenant")
	})(c, next)
}
```

O anônimo nunca chega ao predicado: vai para o login antes, como no `Require`. Esconder o
item do menu é cosmético — a regra é o middleware.

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

### Um store que está em outro lugar

```go
type StoreContext interface {
	SaveContext(ctx context.Context, id string, u *User, ttl time.Duration) error
	LoadContext(ctx context.Context, id string) (*User, error) // ErrNoSession quando não há
	DeleteContext(ctx context.Context, id string) error
}
```

O `Store` não tem contexto e o `Load` dele não tem erro, o que serve para um mapa dentro deste
processo e não serve para uma tabela no Postgres. Sem contexto o store não consegue honrar o
prazo da requisição, cancelar a consulta quando o navegador desliga, nem levar o trace junto —
ele fica reduzido a `context.Background()` com um tempo inventado, **em toda requisição
autenticada**. Sem erro, um banco fora do ar é indistinguível de uma sessão que não existe:
todo mundo é mandado para o login e nenhum log diz por quê.

Implemente também o `StoreContext` e o fluxo passa a usá-lo — a mesma forma de interface
opcional do `SessionLister` acima, então nada escrito contra o `Store` quebra:

```go
func (s *PGStore) LoadContext(ctx context.Context, id string) (*auth.User, error) {
	var blob []byte
	err := s.db.QueryRowContext(ctx, "select u from sessoes where id = $1", id).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, auth.ErrNoSession // não existe essa sessão
	}
	if err != nil {
		return nil, err // o banco falhou: 503, e não a tela de login
	}
	var u auth.User
	return &u, json.Unmarshal(blob, &u)
}
```

Os dois erros têm dois destinos:

| O que aconteceu | O que a pessoa recebe |
|---|---|
| não está logada | o login, como sempre |
| o store falhou | **503**, e uma linha de `Error` no log com a causa |

O `Optional()` continua deixando passar como anônimo quando o store está fora do ar — é para
isso que ele existe —, mas registra a falha, para que um incidente não se leia como todo mundo
tendo deslogado ao mesmo tempo.

### Sessões por dono

```go
type SessionLister interface {
	Sessions(subject string) []*User // as sessões vivas de um subject
}

var ErrNoSessionList error
```

Um store que também implementa `SessionLister` dá à tela da conta duas coisas:
`Sessions(c)` — todas as sessões de quem está logado, a atual primeiro, cada uma com
`IssuedAt`, `Seen` e `SessionID` — e `LogoutOthers(c)`, que apaga todas as sessões dessa
pessoa menos esta e escreve `auth.logout_others` na auditoria com a contagem. `MemoryStore`
implementa. Um store que não implementa recebe `ErrNoSessionList` das duas, e é isso que a
tela mostra em vez de uma lista vazia: lista vazia diria "em nenhum outro lugar", e isso não
se saberia.

Um store que mora em outro lugar implementa a versão com contexto, a mesma forma de interface
opcional do `StoreContext`:

```go
type SessionListerContext interface {
	SessionsContext(ctx context.Context, subject string) ([]*User, error)
}
```

A consulta passa a honrar o prazo da requisição e a ser cancelada quando o navegador desliga
e — a parte que decide — um banco fora do ar vira **erro** nas duas operações, em vez de uma
slice vazia. Sem isso as duas mentem de formas diferentes: a tela da conta desenha "nenhuma
outra sessão" por cima do incidente, e o `LogoutOthers` responde sucesso sem ter encerrado
nada, para quem acabou de trocar a senha justamente para encerrá-las. Quem chama já trata erro
nas duas (`ErrNoSessionList`), então não há caminho novo na aplicação: há um valor que deixa
de ser mentira.

Troca de senha é onde `LogoutOthers` mora: a senha muda porque alguém pode ter a antiga, e
esse alguém pode estar logado agora mesmo.

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

## Chaves de API

O `auth.Sessions` é cookie e o OIDC é bearer de terceiro. O `auth.APIKeys` é o terceiro caso: a
chave que **esta** aplicação emite para quem a chama. É um monte de regra que o iniciante não
sabe — guardar só o hash, mostrar o segredo uma vez, ter um identificador para achar a chave sem
abrir o hash, escopo por rota, limite por chave e não por endereço, revogação que vale já,
registro de uso — e a primeira versão sempre guarda a chave em claro.

```go
var Chaves = auth.APIKeys(auth.KeyOptions{
	Store:     chaves.NewStore(db),        // nil guarda em memória
	Prefix:    "ak",                       // as chaves saem "ak_<identificador>_<segredo>"
	Scopes:    []string{"docs:ler", "docs:escrever"},
	RateLimit: trilha.RateLimit{RPS: 10, Burst: 30},
})

// app/api/v1/middleware.go
var exige = Chaves.Require("docs:ler")
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
```

| Símbolo | Papel |
|---|---|
| `APIKeys(KeyOptions)` | o conjunto de chaves da aplicação |
| `Issue(c, nome, escopos, ttl)` | cria uma e devolve o segredo — a única vez que ele existe |
| `Require(escopos...)` | o middleware: bearer, hash, revogação, vencimento, escopo, limite |
| `Revoke(c, id)` / `All()` | encerra uma agora (`auth.ErrUnknownKey` para um id que ninguém emitiu); lista para a tela |
| `User(c)` | quem chamou, como `auth.User` com os escopos de papéis |
| `KeyStore` / `MemoryKeyStore()` | cinco métodos sobre o que a app roda; memória para teste |
| `ui.SecretOnce(c, segredo)` | o cartão que mostra uma vez, com a frase que precisa estar lá |
| `ui.APIKeysTable(c, []ui.APIKeyRow, ui.APIKeysOpts{...})` | a lista, com o identificador e nunca a chave |

**Só o hash é guardado**, com pimenta do `trilha.Pepper` — HMAC-SHA256 sob uma chave derivada do
segredo da app. Uma tabela de digests roubada não é uma lista que alguém ataca offline, e sem
segredo o `Issue` recusa em vez de gravar um hash sem chave que pareceria ter funcionado.

**O identificador é a metade que identifica sem abrir o hash.** É o que a tela mostra e o que a
requisição usa para achar a chave; o segredo é comparado em tempo constante, e revogação e
vencimento são conferidos **depois** dessa comparação — responder mais rápido para uma chave
revogada do que para uma errada conta qual das duas aconteceu.

**Chave é ator.** O `Require` põe um `auth.User` na requisição com os escopos como papéis e
`via: "api_key"`, então `c.Audit`, o log e a política enxergam alguém em vez de um buraco.

**O limite é por chave**, não por endereço: um cliente atrás de uma chave é um orçamento, faça o
IP dele o que fizer. **O uso é gravado uma vez por minuto** — gravar a cada requisição
transforma um endpoint de leitura numa escrita por chamada, e "esta chave ainda é usada?" não
precisa do segundo.

**Escopo que a aplicação nunca declarou é pânico na hora de montar a rota.** A alternativa é uma
rota que não guarda nada por causa de um erro de digitação, e ninguém descobrir até ler num
relatório de incidente.

### Quem está usando esta chave, onde, e quando parou

É a primeira pergunta depois de entregar uma chave a um parceiro, e não é uma pergunta de
segurança. A chave já sabe *quem* chamou e quando foi vista pela última vez; o que o
`KeyOptions.Usage` acrescenta é a forma do uso.

```go
var Chaves = auth.APIKeys(auth.KeyOptions{
	Usage:     auth.UsageMemory(),          // ou uma tabela atrás dos mesmos três métodos
	UsageKeep: 400 * 24 * time.Hour,        // retenção, varrida no mesmo laço
})

func main() { … ; if err := Chaves.Setup(a); err != nil { … } ; … }
```

```go
rel, _ := Chaves.Usage(c.Context(), auth.UsageQuery{Key: id, Since: time.Now().AddDate(0, 0, -30)})
rel.Total, rel.Errors, rel.Last
rel.ByRoute   // []UsageRoute{Method, Route, Count, Errors, Last}, a mais chamada primeiro
rel.ByDay     // []UsageDay, da mais antiga para a mais nova — o que um gráfico desenha
rel.ByKey     // []UsageKeyTotal, quando a consulta não era sobre uma chave só
```

O `Usage` é um `auth.UsageStore`, três métodos sobre o que a aplicação já roda: o
`Add(ctx, []auth.UsageRow)` recebe o lote que uma gravação produziu — uma linha por chave, rota
e dia, com o último status —, o `Query` responde o `auth.UsageReport` de cima e o `Prune`
descarta o que passou do `UsageKeep`. O `auth.UsageMemory()` é essa interface sobre um mapa, que
é o store certo para um processo e para um teste.

**Contar não pode custar a requisição.** O `Require` incrementa um balde em memória — uma escrita
de mapa sob um mutex, nada que possa travar numa rede — e os baldes vão para o store em lote. O
`Setup` grava no relógio e no desligamento; sem ele, o `Flush(ctx)` é seu para chamar. Um store que
falha na gravação recebe as contagens de volta no buffer, porque perdê-las porque o banco estava
reiniciando é exatamente a falha que este desenho evita.

**A rota é o padrão.** `/documentos/{id}`, nunca `/documentos/8f2c…`: um contador por caminho
concreto é um contador com uma linha por requisição. Uma requisição que o fallback respondeu não
tem padrão, e não é contada — o endereço ali é entrada do usuário, e quem pedir endereços que não
existem em quantidade suficiente estaria escrevendo linhas.

**Sem `Usage`, nada muda**, byte por byte. O `Keys.Usage` numa aplicação assim responde
`ErrNoUsage` em vez de um relatório vazio: não ter dado e não contar são respostas diferentes.

**Chave revogada mantém o histórico.** A pergunta "quem estava usando isto?" chega depois da
revogação, não antes.

```go
ociosas, _ := Chaves.Idle(c.Context(), time.Now().AddDate(0, 0, -90))
```

O `Idle` são as chaves sem nenhuma chamada registrada desde aquele momento. A issue que pediu isto
queria no `trilha audit`; ficou aqui, porque aquele comando lê código na máquina de alguém e os
contadores estão em produção — uma verificação que não os enxerga é uma verificação que diz sempre
que está tudo bem.

As telas são o [`ui.APIUsage`](/pt/referencia/ui) e a coluna de chamadas do `ui.APIKeysTable`, e o
`trilha add api-keys` escreve as duas. O painel recebe a forma do próprio kit — um
`ui.APIUsageData{Total, Errors, Last, Days []ui.APIUsageDay, Routes []ui.APIUsageRoute}` com um
`ui.APIUsageOpts{Days, Limit, Empty}` — e não o relatório deste pacote: um componente do kit que
importasse `auth` arrastaria autenticação para toda aplicação que desenha um botão. Copiar cinco
campos é o preço disso, e é o preço certo.

## Multi-tenant por coluna

Uma coluna é a forma mais comum de multi-tenant, e esquecer essa coluna numa consulta é o bug
mais comum de multi-tenant: o relatório que mostra as linhas de outra organização, descoberto
por um cliente.

**O framework carrega o valor e aponta a consulta que esqueceu. A consulta é sua.** Não há ORM
aqui, e um `WHERE` gerado por este pacote seria um `WHERE` que ninguém consegue ler numa
revisão — o oposto do que um filtro de tenant precisa.

```go
// no login, ou no OnLogin
u.Tenant = row.TenantID

// num repositório
rows, err := db.QueryContext(c, `SELECT … FROM documentos WHERE tenant_id = $1`, auth.Tenant(c))
```

| Símbolo | Papel |
|---|---|
| `auth.User.Tenant` | um campo próprio, ao lado de Roles; viaja onde a sessão viaja |
| `auth.Tenant(c)` | a organização da sessão atual, ou "" |
| `sso.RequireTenant()` | recusa sessão sem organização escolhida |
| `Options.ChooseTenantPath` | para onde o navegador vai escolher; vazio responde 403 |
| `sso.SwitchTenant(c, id)` | troca a sessão e registra os dois lados |

É **campo e não mais uma entrada no `Extra`** porque tudo o que o framework faz com ele precisa
encontrá-lo no mesmo lugar em toda aplicação: ele entra na trilha como `actor.tenant` e no
registro de acesso como `tenant` — que é o primeiro filtro de qualquer pergunta do suporte, e o
campo que diz se "viram as linhas erradas" é sobre uma consulta ou sobre alguém ter trocado de
organização.

O `RequireTenant` trata "entrou e não escolheu" pelo que é: um administrador que ainda não
escolheu, ou um primeiro login. O navegador é redirecionado; o resto leva 403, porque um
redirecionamento para uma tela não é resposta que uma API use.

**Conferir se alguém pode entrar numa organização é da aplicação.** O `SwitchTenant` troca a
sessão e audita; ele não sabe o que é um vínculo, e fingir que sabe seria uma checagem que parece
garantia e não é.

### A consulta que esqueceu

O `trilha audit` conta, por tabela, quantas consultas a filtram por tenant — e nomeia as que não:

```
warn  consultas que talvez estejam sem o filtro de tenant
      internal/docs/repo.go:88: documents é filtrada por tenant em 7 de 8 consultas, e não nesta
```

É heurística de texto e diz isso: sem parser de SQL, sem veredito, um lugar para olhar. Consulta
que está certa de propósito — relatório global, listagem de admin — merece um comentário dizendo
isso, tanto para a próxima pessoa quanto para a ferramenta.
