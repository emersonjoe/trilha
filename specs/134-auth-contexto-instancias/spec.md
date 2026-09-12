# Spec 134 — O prazo que a lista de sessões não tinha e o slot que dois públicos dividiam

- **Issues**: [#176](https://github.com/emersonjoe/trilha/issues/176),
  [#180](https://github.com/emersonjoe/trilha/issues/180) — as issues são a fonte do escopo.
- **Branch**: `feat/auth-contexto-instancias`
- **Versão**: 0.113.0

## Por quê

As duas issues são do mesmo pacote e do mesmo tipo de erro: o `auth` responde com uma
afirmação tranquila onde deveria responder com um erro ou com um `nil`. Nas duas, nada quebra,
nada é registrado, e o que sobra na tela é falso.

**A lista de sessões sem prazo e sem erro (#176).** A [#165](https://github.com/emersonjoe/trilha/issues/165)
deu contexto e erro ao `Store` (`StoreContext`, 0.103.0) e deixou o `SessionLister` como estava:
`Sessions(subject string) []*User`, sem `context.Context` e sem `error`. Para o `MemoryStore`
tanto faz. Para o mesmo store em Postgres que a #165 destravou, a consulta não honra o prazo da
requisição nem é cancelada quando o navegador desliga — e um banco fora do ar devolve slice
vazia, que as duas operações traduzem de formas diferentes e as duas erradas: o `Sessions(c)`
desenha *"nenhuma outra sessão"*, que é exatamente o que o `ErrNoSessionList` existe para não
deixar acontecer, e o `LogoutOthers(c)` responde **sucesso sem ter encerrado nada**, de modo que
quem acabou de trocar a senha acredita que derrubou as outras sessões.

**O slot de contexto compartilhado (#180).** `ctxKey = "auth.user"` é constante de pacote: todo
`auth.Auth` do binário grava e lê o mesmo lugar do `Ctx`. Uma aplicação com dois públicos no
mesmo processo — área interna e área externa como rotas da mesma árvore, com middlewares
diferentes — é o desenho que o kit incentiva, e cookie próprio com `Store` próprio **não bastam**
para separá-los: basta um handler do lado externo chamar `internoAuth.User(c)`, ou um layout
fazê-lo por baixo, para receber o usuário que o outro lado acabou de gravar. O efeito, descrito
na issue a partir de um caso em produção, é uma tela montada com a identidade errada — o menu e
o nome da organização de quem não é a pessoa. Silencioso, e a favor do erro.

## O que muda

### 1. `SessionListerContext`, ao lado do `StoreContext` (#176)

Interface opcional nova, aditiva, no mesmo padrão da #165 — nenhum `Store` existente quebra e o
`MemoryStore` continua com o `SessionLister` de hoje:

```go
type SessionListerContext interface {
	SessionsContext(ctx context.Context, subject string) ([]*User, error)
}
```

`Sessions` e `LogoutOthers` passam por um único ponto que escolhe a interface:

| O que o `Store` implementa | O que as duas operações fazem |
|---|---|
| `SessionListerContext` | `SessionsContext(c.Context(), sub)`; o erro do store sobe |
| só `SessionLister` | `Sessions(sub)`, exatamente como hoje |
| nenhuma das duas (ou sem `Store`) | `ErrNoSessionList`, como hoje |

O erro do store vira o erro de `Sessions(c)` e de `LogoutOthers(c)` em vez de lista vazia e de
`nil`. Quem chama já trata erro nas duas (`ErrNoSessionList`), então não há caminho novo na
aplicação: há um valor que deixa de ser mentira.

### 2. `Options.Audience`: o slot de contexto e a sessão passam a ter dono (#180)

```go
Sessions(auth.Options{
	Audience:   "portal",            // o público que este Auth atende
	CookieName: "portal_session",
	Store:      portalStore,
})
```

Três coisas passam a depender dele, e nenhuma aparece para quem tem um `Auth` só:

1. **O slot é da instância.** `a.ctxKey()` é `"auth.user"` quando não há `Audience` e
   `"auth.user." + Audience` quando há. `User`, `Optional`, `Require` e o `remember` usam o slot
   da instância; quem não nomeia o público continua exatamente no slot de hoje.
2. **A sessão carrega o público.** `User.Audience` (`aud` no JSON) é gravado pelo `Auth` que
   escreve a sessão — no login e em cada renovação — e `Session` recusa (`ErrNoSession`) uma
   sessão cujo `aud` não é o seu. É o que protege o caso que o slot sozinho não protege: dois
   `Auth` no mesmo processo com o mesmo nome de cookie, ou o mesmo `Store` compartilhado.
3. **`User(c)` recusa o que não é seu.** Antes de devolver o que achou no `Ctx`, confere o
   `aud`; o que é de outro público vira `nil`, que é o resultado que a guarda sabe tratar.

O `auth.Tenant(c)` continua respondendo pela sessão da requisição: o `remember` grava também no
slot compartilhado, que é o único lugar onde uma função sem instância pode procurar.

## Fora de escopo

- **`trilha check` apontando dois `Auth` com o mesmo `Audience` no mesmo binário**, sugerido no
  fim da #180. É uma análise estática no `cmd/trilha`, não no `auth`, e fecha o assunto na
  origem para quem esquece de nomear o segundo público — mas não é o que impede o vazamento,
  que é o que esta spec entrega. Fica registrado para uma issue própria.
- **Dar contexto e erro ao `PolicyStore` e ao `KeyStore`.** É a mesma lacuna da #176 em outras
  duas interfaces; nenhuma das duas está no escopo destas issues.

## Decisões

1. **Aditivo, não quebra.** `SessionListerContext` ao lado de `SessionLister`, `Audience` como
   campo novo de `Options` com o zero valendo o comportamento de hoje. Nada sai do
   `api/current.txt`, então não há ciclo de `Deprecated:` a cumprir (princípio IV).
2. **O carimbo na sessão, e não só o slot.** A issue chama a conferência de `aud` de
   "complemento provavelmente mais útil dos dois", e é. O slot separado resolve a leitura
   cruzada dentro da requisição; o carimbo resolve a sessão cruzada — o cookie do portal
   apresentado ao `Auth` interno, que hoje é aceito quando os dois usam o nome de cookie padrão
   e a mesma chave de assinatura do `App`.
3. **`Audience` é campo do `User` e não uma chave no `Extra`.** Pelo mesmo motivo do `Tenant`:
   o que o framework confere em toda requisição tem que estar no mesmo lugar em toda aplicação.
   O contorno de `Extra["aud"]` é o que cada app escreveria sozinho, e é justamente o que a issue
   recusa.
4. **Ligar o `Audience` num app existente encerra as sessões abertas.** Elas foram escritas sem
   `aud` e o `Auth` que agora tem público não as reconhece: uma volta ao login, uma vez. O
   contrário — aceitar sessão sem dono — seria manter a porta que a issue veio fechar. Está no
   `CHANGELOG` como mudança de comportamento de quem adota a opção.
5. **A chave de API continua no slot compartilhado.** `Keys` não é um `Auth` e não tem público;
   um `Auth` com `Audience` não enxerga mais o chamador por chave no `User(c)`, o que é o
   resultado certo — a chave não é sessão daquele público — e `Keys.User(c)` e `auth.Tenant(c)`
   continuam iguais.

## Constitution Check

- **I. Convenção sobre configuração** — nenhuma convenção nova em `app/`; nada muda no scanner
  nem no gerador.
- **II. Só biblioteca padrão** — `context` e o que o pacote já importa. `TestNoExternalDeps`
  segue verde.
- **III. Geração explícita** — o gerador não é tocado; `examples/blog/trilha_gen.go` não muda.
- **IV. Contrato pequeno e estável** — três símbolos novos (`SessionListerContext`,
  `Options.Audience`, `User.Audience`), nenhum removido; `api/current.txt` regravado por
  `make api` e o diff é a revisão.
- **V. Dev < 2 s, produção em um binário** — sem efeito.
- **VI. Teste primeiro** — os testes que falham entram antes: contexto e erro nas duas operações
  de lista, e a travessia de dois `Auth` no mesmo mux.
- **VII. Segurança por padrão** — é a razão da spec: o `LogoutOthers` que mentia sobre ter
  encerrado sessões e o usuário de um público entregue ao outro passam a ser, respectivamente,
  erro e `nil`.

## Aceitação

- **SC-001** Um `Store` que implementa `SessionListerContext` recebe o `context.Context` da
  requisição em `Sessions(c)` e em `LogoutOthers(c)`, e o cancelamento chega nele.
- **SC-002** O erro do store vira erro em `Sessions(c)` (e não lista vazia) e em
  `LogoutOthers(c)` (e não `nil`).
- **SC-003** Um `Store` que só implementa `SessionLister` — `MemoryStore` inclusive — se comporta
  exatamente como hoje, e sem `Store` as duas continuam devolvendo `ErrNoSessionList`.
- **SC-004** Dois `Auth` com `Audience` diferentes no mesmo mux: com a sessão aberta em um, o
  `User(c)` do outro devolve `nil` — inclusive quando os dois compartilham nome de cookie e
  `Store`.
- **SC-005** Um `Auth` sem `Audience` — a esmagadora maioria — não vê diferença: mesmo slot,
  mesma sessão, `auth.Tenant(c)` e `Keys.User(c)` inalterados.
- **SC-006** `api/current.txt` registra `SessionListerContext`, `Options.Audience` e
  `User.Audience`; a referência de `auth` em inglês e em português documenta os dois contratos.
