# Spec 147 — gravar numa sessão viva e ver o id_token no login

- **Issues**: [#206](https://github.com/emersonjoe/trilha/issues/206) e
  [#208](https://github.com/emersonjoe/trilha/issues/208) — as issues são a fonte do escopo;
  aponte para elas, não as reescreva aqui.
- **Branch**: `feat/auth-update-id-token`
- **Versão**: 0.126.0

## Por quê

Os dois pedidos são o mesmo buraco visto de dois lados: o `auth` sabe escrever uma sessão
**no login** e mais nada. Depois disso, o que está guardado só muda derrubando a sessão (
`Login` rotaciona o `SessionID` e redireciona) ou passando pelo único campo que tem uma porta
de escrita (`SwitchTenant`, que desiste quando o tenant não muda). E o que o provedor assinou
— o `id_token` e as claims — é verificado dentro do `Callback`, usado para preencher cinco
campos do `*User` e descartado antes de qualquer código da app ver.

O efeito no app real é o mesmo nos dois casos: a app deixa de cachear e vai buscar o valor a
cada requisição, ou reimplementa o fluxo. A issue #206 registra a primeira (uma ida à API
interna por página, para ler a marca da organização que um admin acabou de editar); a #208
registra a segunda (o `auth.OIDC` inteiro trocado por "o backend faz o Google e este app adota
a sessão dele", porque repassar o `id_token` ao backend que tem a mesma `client_id` é a única
forma de pedir a sessão sem inventar um segredo compartilhado).

## O que muda

### 1. `Update`: gravar na sessão que está aberta

```go
func (a *Auth) Update(c *trilha.Ctx, fn func(*User)) error
```

Lê a sessão corrente **do store** (ou do cookie, quando não há store), aplica `fn`, grava e
deixa o valor novo no contexto da requisição. Não rotaciona o `SessionID`, não redireciona, não
audita.

Contrato, ponto por ponto:

| Detalhe | Decisão |
|---|---|
| De onde vem o `*User` | `a.Session(c)`, que relê o store — não o `*User` que já está no contexto, que pode ter sido lido no começo da requisição |
| `SessionID` | preservado; `fn` que o altere é ignorada nessa parte (ver abaixo) |
| `Audience` | reestampado pelo caminho de escrita, como em todo login |
| `Set-Cookie` | **com store, nenhum**: o cookie carrega só o identificador, e o identificador não mudou. Sem store a sessão *é* o cookie, então ele é reescrito — com o mesmo prazo e sem identificador novo |
| Contexto da requisição | `a.remember(c, u)`, para que `a.User(c)`, `auth.Tenant(c)` e o ator da trilha abaixo deste ponto leiam o valor novo |
| Sem sessão | devolve o erro de `Session` (`ErrNoSession` ou o erro do store), sem redirecionar: quem chama está no meio de um POST e decide |
| `Extra` | garantido não-nil antes de `fn`, para que uma sessão criada sem ele não exija um `make(map[string]string)` dentro do callback. Vazio continua saindo do JSON pelo `omitempty` |
| `fn` nil | erro; uma chamada que não muda nada é bug de quem chamou, não um no-op silencioso |
| Auditoria | nenhuma linha. `SwitchTenant` audita porque o framework sabe o que mudou; aqui só a app sabe, e `c.Audit` é dela |

O `SessionID` é preservado de propósito e não por descuido: o identificador é o nome da sessão
no store, e trocá-lo dentro de um `Update` deixaria a linha antiga órfã e o cookie do navegador
apontando para o vazio — logout no meio da requisição, que é exatamente o que a #206 não quer.
Rotacionar o identificador continua sendo `Login`, que é onde rotacionar tem significado.

`SwitchTenant` passa a ser `Update` com uma linha de trilha: mesmo caminho de escrita, mesmo
`remember`, e o campo deixa de ter uma porta de escrita própria. O comportamento visível dele
não muda — inclusive a saída antecipada quando o tenant já é aquele, que continua sendo a
resposta certa (mover para onde a sessão já está não é mover, e uma linha de trilha dizendo "de
acme para acme" é ruído no arquivo que uma investigação lê). O que a issue #206 pedia dele —
ser veículo para mutar o `Extra` — deixa de ser necessário: isso é o `Update`.

### 2. `OnLoginToken`: o `id_token` verificado e as claims, sem guardar nada

```go
type IDToken struct {
	Raw    string   // o id_token compacto, como o provedor assinou
	Claims *Claims  // as claims já verificadas (assinatura, iss, aud, nonce, exp)
}

// em Options:
OnLoginToken func(c *trilha.Ctx, u *User, t *IDToken) error
```

Roda dentro do `Callback`, **depois** do `OnLogin` e antes de a sessão ser escrita — então um
erro dele para o login do mesmo jeito. `Login` (o caminho sem provedor) não o chama: ali não
existe `id_token`, e chamá-lo com `Raw` vazio seria convidar a app a tratar "não houve token"
como "o token é isto".

Por que um callback novo e não `User.Extra["id_token"]` com um `KeepIDToken: true`: o `Extra`
viaja onde a sessão viaja — cookie assinado ou store — então guardar o token ali põe uma
credencial de terceiro no cookie do navegador, com o prazo da sessão e não o do token. O
callback entrega o token **durante** a requisição do callback, que é a janela em que o caso de
uso da #208 acontece: a app repassa o `Raw` ao seu backend, recebe a sessão da aplicação e
guarda *essa* no `u.Extra`. O token do provedor não persiste em lugar nenhum, e é a `#208` que
diz que essa é a forma melhor ("ela não põe credencial em lugar nenhum").

Os dois callbacks convivem porque respondem perguntas diferentes: `OnLogin` é o que roda nos
dois logins (senha e OIDC) e fica igual ao que é hoje; `OnLoginToken` é o que só o OIDC tem. Um
app com as duas portas de entrada escreve a regra comum em um e a parte do token no outro.

O `AccessToken` fica fora: não é uma afirmação de identidade assinada para esta app, é a
credencial do provedor para o `userinfo`, e nenhuma das duas issues pede. Os papéis
(`Roles`, `RoleClaims`, `rolesFromAccess`) não mudam.

## Fora de escopo

- `Save(c, *User)` como segunda porta de escrita: a #206 registra que a forma de callback é a
  que evita a corrida de ler-modificar-gravar com um store remoto, e duas portas para a mesma
  escrita é a que alguém usa errado.
- Um lock por sessão no `Store`: `Update` relê antes de aplicar `fn`, o que fecha a janela do
  read-modify-write da própria app, mas não a de dois `Update` concorrentes. Um store que
  precise disso implementa `StoreContext` e resolve na transação; a interface não ganha método.
- `KeepIDToken`/refresh token na sessão: guardar credencial de terceiro na sessão é outra
  decisão, com outro prazo de validade, e não é a que estas issues pedem.
- Renovar o `id_token` (refresh token): o kit nunca guardou refresh token e continua não
  guardando.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo importado; `Update` e `IDToken` são stdlib |
| III — geração explícita | não toca scanner nem gerador |
| IV — API pública pequena e vigiada | dois símbolos novos (`Auth.Update`, `IDToken`) e um campo de `Options`, todos com doc comment, em `api/current.txt` e na referência; nada removido nem renomeado, `OnLogin` com a mesma assinatura |
| VI — teste primeiro | `auth/update_test.go` e `auth/idtoken_test.go` entram falhando: `Update` muda o `Extra` e a requisição seguinte (mesmo cookie, sem `Set-Cookie` novo) lê o valor novo; `OnLoginToken` vê o `Raw` que o provedor de teste assinou e uma claim que o kit não mapeia |
| VII — segurança por padrão | `Update` não rotaciona identificador (quem rotaciona é o login) e por isso não substitui `Login`; o `id_token` não entra na sessão nem em log nenhum — o callback o vê e ele morre no fim da requisição; `Update` recusa sessão de outro `Audience` porque passa por `Session` |
| Idioma | código e doc comments em inglês; referência do `auth` em `/reference/auth` e `/pt/referencia/auth` no mesmo commit |

## Aceitação

- `Update` grava no `Extra`, a requisição seguinte com o mesmo cookie lê o valor novo, a
  resposta do `Update` não traz `Set-Cookie` (com store) e o `SessionID` é o mesmo antes e
  depois;
- o handler que chamou `Update` lê o valor novo em `a.User(c)` e `auth.Tenant(c)` no resto da
  requisição;
- `Update` sem sessão devolve `ErrNoSession`; com store quebrado devolve o erro do store;
- `OnLoginToken` recebe o `Raw` que verifica contra o JWKS do provedor de teste e
  `Claims.All["hd"]` (uma claim que o kit não mapeia); um erro dele recusa o login;
- `OnLogin` continua rodando nos dois logins, antes do `OnLoginToken`;
- `make test` verde, `api/current.txt` atualizado, `CHANGELOG` e `ROADMAP` fechados na 0.126.0.
