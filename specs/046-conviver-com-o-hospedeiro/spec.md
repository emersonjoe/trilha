# Spec 046 — Conviver com o hospedeiro: cabeçalhos, nonce, nomes do CSRF e dependências

- **Issues**: [#52](https://github.com/emersonjoe/trilha/issues/52),
  [#54](https://github.com/emersonjoe/trilha/issues/54),
  [#55](https://github.com/emersonjoe/trilha/issues/55) — a issue é a fonte do escopo.
- **Branch**: `046-conviver-com-o-hospedeiro`
- **Versão**: 0.35.0

## Por quê

A 0.32.0 resolveu o *como entrar* num binário que já existe: o arquivo gerado assume o pacote
da pasta e o hospedeiro monta `NewApp().Handler()`. As três issues desta spec são o *como
conviver depois de entrar*, e as três vieram da mesma migração real (o CRM do Farol).

O app embutido responde no meio de um site que já tem dono. Hoje ele chega escrevendo por
cima: `applySecurity` roda em toda requisição e `Set` sobrescreve a política do hospedeiro, de
modo que o visitante recebe uma CSP nas telas do módulo e outra nas telas ao lado, no mesmo
domínio. Desligar isso custa seis campos com `Off`, um a um, e ainda assim o
`X-Content-Type-Options` sai — coincidiu de o valor ser o mesmo, o que é sorte, não desenho.
Pior: `c.Nonce()` **sorteia** o nonce, sem como receber o que o hospedeiro já sorteou e já
publicou na CSP dele. `trilha.NonceAttr(c)` devolve, nesse arranjo, um nonce que não está na
política vigente — o `<script>` inline é bloqueado no navegador e o servidor não vê erro
nenhum. A ferramenta continua oferecida e a resposta dela está errada.

O CSRF tem o mesmo formato de problema, num lugar mais estreito: `CSRFField` é a constante
`_csrf`, que é o nome mais comum que existe. Na mesma página saem o formulário de sair, do
hospedeiro, e o formulário do módulo, os dois com um campo `_csrf` de valores diferentes.
Funciona — cada um posta para o seu handler — mas quem lê a página não tem como saber qual é
qual, e o teste de integração do Farol teve que ir buscar o token no cookie porque o primeiro
`_csrf` do HTML é o do hospedeiro. No dia em que um formulário for postado para o lado errado,
a falha é "invalid CSRF token", sem que o nome denuncie a troca.

A terceira é a que mais se paga por arquivo. A única porta para as dependências do app é
`Values() map[string]any`, então cada página começa com uma asserção de tipo sobre uma string:
`c.App().Values()["farol"].(*Deps)`. Chave errada devolve `nil` em silêncio, e o app morre no
primeiro uso do pool, longe da causa. Todo app embutido reescreve o mesmo pacotinho `dep` para
não repetir a asserção — e o exemplo do blog, que é a única receita visível de injeção, ensina
estado global de pacote, que só funciona com um app por processo. Uma suíte que sobe um
servidor por teste — que é a suíte de qualquer app sério — faz um teste ver o pool do outro.

## O que muda

### 1. Os cabeçalhos podem ser de quem está na frente (#52)

`Security` ganha um campo, e ele diz a intenção inteira numa linha:

```go
func Setup(a *trilha.App) error {
	a.Security().Delegated = true // quem responde por estes cabeçalhos está na frente
	return nil
}
```

Com `Delegated`, `applySecurity` não escreve **nada** — nem CSP, nem HSTS, nem
`X-Content-Type-Options`. É a diferença entre "não quero esta política" (que já existe, campo
a campo, com `Off`) e "não sou eu que respondo por estas respostas".

O nonce passa a poder vir de fora:

```go
cfg.Security.Nonce = func(r *http.Request) string { return farol.NonceDe(r) }
```

Quando `Nonce` está definida, `c.Nonce()` devolve o que ela devolver, para aquela requisição —
inclusive vazio, que quer dizer "o hospedeiro não tem nonce aqui". Nesse caso `NonceAttr(c)`
não renderiza atributo nenhum, em vez de escrever `nonce=""`, que seria a mesma mentira de
hoje com outra roupa. Sem `Nonce` definida, nada muda: o framework sorteia como sempre.

### 2. Os nomes do CSRF são configuráveis (#54)

```go
type CSRF struct {
	Cookie string // padrão CSRFCookie ("trilha_csrf")
	Field  string // padrão CSRFField ("_csrf")
	Header string // padrão CSRFHeader ("X-CSRF-Token")
}
```

`Config.CSRF` é preenchida com os padrões em `New`, então `a.Config().CSRF.Cookie` é sempre um
nome concreto — é dele que `CSRFInput`, `CSRFToken`, a validação, o cliente de teste e o CORS
passam a ler. As três constantes continuam existindo e continuam valendo o que valiam: são o
padrão, não mais o nome fixo.

```go
func Config(cfg *trilha.Config) error {
	cfg.CSRF.Field = "_farol_trilha_csrf"
	cfg.CSRF.Cookie = "farol_trilha_csrf"
	return nil
}
```

### 3. Dependências com tipo (#55)

```go
// no Setup, uma vez
trilha.Provide(a, &Deps{Pool: pool, Cfg: cfg})

// em qualquer página
d := trilha.Use[*Deps](c)
```

A chave é o tipo (`reflect.TypeOf` do `T`), então não há string para digitar errado. `Use` de
um tipo que ninguém proveu entra em pânico com a frase que diz o conserto — "trilha: nothing
provided for *farol.Deps; call trilha.Provide(a, …) in Setup" — em vez de devolver o zero e
adiar a falha para o primeiro uso. `Values()` continua público e continua servindo para o
resto: `Provide` guarda no mesmo mapa.

O `examples/blog` deixa de ensinar global de pacote: o `posts` vira um `*posts.Store` criado
no `Setup` e recuperado com `Use`, que é o que faz uma suíte com um app por teste ser
possível.

## Fora de escopo

- **`Config.Embedded` como chave-mestra** (desligar cabeçalhos, sessão e CSRF de uma vez): as
  três decisões são independentes — dá para delegar cabeçalho e manter o CSRF da Trilha — e
  uma bandeira que faz três coisas é a que ninguém sabe explicar depois.
- **Nonce da Trilha chegando a quem não usa o `h`** ([#44](https://github.com/emersonjoe/trilha/issues/44)):
  é o caminho contrário, e tem issue própria.
- **Injeção por parâmetro do handler** (`func Page(c *trilha.Ctx, d *Deps)`): quebraria o
  contrato pequeno e estável do princípio IV.
- **Escopo por requisição no `Provide`**: o mapa é do processo, preenchido no `Setup`. Valor
  por requisição já tem lugar, que é `c.Set`/`c.Get`.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — convenção sobre configuração | as três são configuração de borda, não convenção nova em `app/`; o padrão continua sendo o de hoje e quem não embute não escreve nada |
| II — só biblioteca padrão | `reflect` e `crypto/rand`, ambos da padrão |
| IV — contrato de handler estável | `Provide`/`Use` são funções de pacote sobre o `Ctx` que já existe; nenhuma assinatura de handler muda |
| VI — teste primeiro | cada item entra por teste que falha: cabeçalho ausente com `Delegated`, nonce do hospedeiro no atributo, nome de campo trocado no HTML e no cookie, pânico do `Use` sem `Provide` |
| VII — segurança por padrão | o zero valor de `Delegated` é `false` e o de `CSRF` são os nomes de hoje: só delega quem escreveu que delega. `Delegated` é registrado no log de subida, para que "sem cabeçalho" nunca seja acidente silencioso |

## Aceitação

- **SC-001** Com `Delegated`, uma resposta do app não traz nenhum dos sete cabeçalhos, e os
  que o hospedeiro pôs antes chegam intactos ao cliente.
- **SC-002** Com `Security.Nonce` definida, `c.Nonce()` e `NonceAttr` usam o valor do
  hospedeiro; devolvendo vazio, `NonceAttr` não renderiza atributo.
- **SC-003** Com `cfg.CSRF` renomeado, o input do formulário sai com o nome novo, o cookie sai
  com o nome novo, o cabeçalho novo é aceito e o antigo não é.
- **SC-004** `trilha.Use[*T]` devolve o que `Provide` guardou e entra em pânico com o nome do
  tipo quando ninguém proveu.
- **SC-005** `examples/blog` não tem mais estado de pacote em `internal/posts`, e a suíte sobe
  dois apps no mesmo processo sem um ver o outro.
