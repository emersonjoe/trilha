# Spec 124 — A sessão remota e o e-mail que o provedor não garantiu

- **Issues**: [#164](https://github.com/emersonjoe/trilha/issues/164),
  [#165](https://github.com/emersonjoe/trilha/issues/165) — as issues são a fonte do escopo.
- **Branch**: `claude/relaxed-keller-v1pwuf`
- **Versão**: 0.103.0

## Por quê

As duas issues são de uma migração real (o Farol, `emersonjoe/farol#9`, trocando `go-oidc` por
`trilha/auth`) e as duas dizem a mesma coisa por dois lados: o `auth` foi escrito para a sessão
que cabe num cookie e para o provedor em que se confia inteiro. Quem sai desses dois casos
perde alguma coisa, e em silêncio.

**A claim que não chega (#164).** O `Callback` monta o `User` com `Subject`, `Email`, `Name` e
`Roles`. O `email_verified` fica no `claims.All` e não passa dali: nem no `User`, nem por outro
caminho da API pública. Quem autoriza por **lista de permitidos por e-mail** — o caso do Farol,
e o caso de quase toda ferramenta interna — confere `u.Email` contra a lista e aceita. Mas o
Google, e qualquer provedor OIDC, emite `email` com `email_verified: false` quando a pessoa
digitou aquele endereço e não provou que é dela. É a diferença entre *"o provedor diz que este
e-mail é dele"* e *"o provedor diz que ele digitou este e-mail"*, e a OIDC Core §5.1 define a
claim exatamente para isso. Hoje o Farol lê a claim à mão no próprio callback; migrar para o
Trilha custaria essa verificação.

**O `Store` sem contexto (#165).** A interface tem três métodos e nenhum recebe
`context.Context`. Para o `MemoryStore` isso é indiferente. Para uma sessão em Postgres — a
tabela `sessoes` que o Farol já tem — não é: não dá para honrar o prazo da requisição, cancelar
a consulta quando o cliente desliga, nem levar o trace junto. A implementação fica reduzida a
`context.Background()` com um tempo inventado, **em toda requisição autenticada**.

E o `Load` devolve `(*User, bool)` sem `error`, o que é pior que o contexto: um banco fora do ar
vira "não existe sessão", e a pessoa é mandada para o login. O incidente aparece como uma onda
de gente relogando, sem um log, sem um 503, sem nada que diga que o banco caiu.

## O que muda

**`User.EmailVerified` e `Claims.EmailVerified` (#164).** O `Callback` preenche os dois a partir
da claim, aceitando o booleano JSON e a string `"true"` — provedores mandam das duas formas, e
um app que recusa e-mail não verificado não pode depender de qual.

Um detalhe que decide o resultado: o `Callback` cai para `preferred_username` quando não há
`email`. Nesse caminho o `EmailVerified` continua **falso**, porque um nome de usuário não é um
e-mail que alguém garantiu.

```go
Sessions/New(p, auth.Options{
	// Recusa o login quando o provedor manda um e-mail que ele mesmo não
	// garante. Desligado por padrão: ligar muda quem entra.
	RequireVerifiedEmail: true,
	OnLogin: func(c *trilha.Ctx, u *auth.User) error {
		if !permitidos[u.Email] {
			return errors.New("e-mail fora da lista")
		}
		return nil
	},
})
```

Com a opção ligada, o `Callback` recusa (`fail`, 401 e o motivo no log) **antes do `OnLogin`**:
a função do app não chega a ver um e-mail que não devia existir. Desligada, o `OnLogin` recebe
`u.EmailVerified` e decide sozinho.

O login local (`auth.Sessions`) não tem provedor e não tem claim: o campo é do app, que é quem
sabe se confirmou o endereço. Documentado no `Login`.

**`StoreContext` (#165)**, interface opcional ao lado da `Store`, no mesmo padrão que o
`SessionLister` deste pacote já usa (e que a stdlib usa em `io.WriterTo` e `http.Flusher`):

```go
type StoreContext interface {
	SaveContext(ctx context.Context, id string, u *User, ttl time.Duration) error
	LoadContext(ctx context.Context, id string) (*User, error) // ErrNoSession quando não há
	DeleteContext(ctx context.Context, id string) error
}
```

O fluxo faz type assertion em cada acesso: implementou, usa a versão com `c.Context()`; não
implementou, usa a de hoje. Nenhum `Store` existente quebra, e o `MemoryStore` continua como
está — para uma sessão que vive dentro do processo, contexto é cerimônia.

E o erro passa a ter dois destinos. `Session` devolve `ErrNoSession` quando não há sessão e o
erro do store quando o store falhou; os guardas (`Require`, `RequireRole`, `RequireFunc`,
`RequirePolicy`) separam os dois:

| O que aconteceu | O que a pessoa recebe |
|---|---|
| não está logada | o login, como hoje |
| o store falhou | **503**, e uma linha de `Error` no log com o erro |

Mandar alguém para o login porque o banco caiu transforma um incidente em um laço de login e
some com ele de todo log — que é o comportamento de hoje.

## Fora de escopo

- **`SessionLister` com contexto.** Ele tem o mesmo problema, mas as duas operações que o usam
  (`Sessions`, `LogoutOthers`) são tela de conta e não requisição autenticada, que é o custo que
  a #165 mede. A issue nomeia três métodos; ampliar aqui seria escopo que ela não pediu. Vale
  issue própria.
- **Trocar a interface `Store`.** Quebraria todo `Store` da v0.x, incluindo o `MemoryStore` em
  produção. A alternativa está registrada na própria issue.
- **`MemoryStore` recebendo contexto**, e qualquer mudança no formato do cookie — sessão antiga
  carrega com `EmailVerified: false`, que é o valor seguro.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `context`, `errors`, `net/http`. O exemplo de `Store` em SQL na doc é `database/sql` sem driver: mostra a forma, não traz dependência. |
| III — coerente com Go | Interface opcional com type assertion é o padrão da stdlib e o que o `SessionLister` deste mesmo pacote já faz. `ErrNoSession` como sentinela de `errors.Is`. |
| VI — teste primeiro | Provedor falso emitindo `email_verified: false`; store falso que implementa as duas interfaces e registra qual foi chamada; `LoadContext` falhando vira 503 e não redirect; o contexto que chega é o da requisição, e cancelá-lo cancela a consulta. |
| VII — segurança por padrão | `RequireVerifiedEmail` desligado por padrão porque ligar muda quem entra — mas o campo chega ao `OnLogin` de qualquer jeito, então quem já confere a lista passa a poder conferir a verificação sem opção nenhuma. O `preferred_username` nunca conta como verificado. |

## Tarefas

- [x] T001 Teste que falha: `email_verified: false` chega ao `OnLogin` como `false`; com a opção,
      o login é recusado e o `OnLogin` não roda; a string `"true"` vale
- [x] T002 `Claims.EmailVerified`, `User.EmailVerified`, `Options.RequireVerifiedEmail`
- [x] T003 Teste que falha: store com `StoreContext` recebe o contexto da requisição; store que
      falha vira 503 e não redirect; store antigo continua funcionando
- [x] T004 `StoreContext`, os acessos do fluxo por ele, `Session` distinguindo os dois erros
- [x] T005 `examples/sso` liga `RequireVerifiedEmail`; rota e teste de integração
- [x] T006 Documentação en + pt (referência de auth), `api/current.txt`
- [x] T007 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, `ROADMAP.md`
- [x] T008 `make test` verde e `scripts/release.sh 0.103.0 --issues "164 165"`

## Aceitação

- **SC-001** Com `RequireVerifiedEmail: true`, um id_token com `email_verified: false` é recusado
  com 401, o `OnLogin` não roda e o log diz o motivo; com `true` (booleano ou string) o login vale.
- **SC-002** Sem a opção, o `OnLogin` recebe `u.EmailVerified` com o valor da claim, e o e-mail
  vindo de `preferred_username` chega como não verificado.
- **SC-003** Um `Store` que implementa `StoreContext` recebe o `context.Context` da requisição em
  `Save`, `Load` e `Delete`, e o cancelamento da requisição chega nele.
- **SC-004** Um `LoadContext` que devolve erro diferente de `ErrNoSession` produz 503 com log, e
  não um redirect para o login. Um `Store` que só implementa a interface antiga se comporta
  exatamente como hoje.
- **SC-005** `api/current.txt` registra `StoreContext`, os dois `EmailVerified` e
  `RequireVerifiedEmail`, e nada mais.
