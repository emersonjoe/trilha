# Spec 146 — O nome do método, a resposta HTTP e o campo anulável no `trilha client`

- **Issues**: [#205](https://github.com/emersonjoe/trilha/issues/205) (o nome da tag é cortado
  como sufixo e no singular), [#207](https://github.com/emersonjoe/trilha/issues/207) (o cliente
  gerado não dá acesso à resposta HTTP) e
  [#209](https://github.com/emersonjoe/trilha/issues/209) (campo `required` **e** anulável sai
  `string` com `validate:"required"`) — as issues são a fonte do escopo, com reprodução e
  medição; aqui fica só a decisão.
- **Branch**: `feat/client-tag-resposta-anulavel`
- **Versão**: 0.125.0
- **Número**: a 145 é a spec do protocolo `trilha-spec`, que fechou a 0.124.0; esta é a 146.

## Por quê

As três issues vêm do mesmo lugar — um app real chamando uma API real pelo cliente gerado — e
descrevem três traduções em que o gerador afirma, sem que nada avise, algo que o documento não
diz.

**O nome.** A spec 128 cortou do nome do método a parte que só repete a tag, para que
`list_documents` na tag `documents` fosse `Documents().List()`. A spec 131 (#169) já tinha
descoberto que o corte no início podia deixar um resto minúsculo (`ConfigurarRegra` → método
não exportado) e passou a exigir fronteira de palavra. Sobraram as outras duas posições do
mesmo corte, em que o resto fica maiúsculo: o **sufixo** (`obterResumoDosIdiomas` na tag
`idiomas` → `ObterResumoDos()`, uma preposição sem objeto) e o **singular** da tag dentro do
nome (`responderCard` na tag `cards` → `Responder()`, que não diz o que se responde). O código
compila, o método é exportado, e o que se perde é o significado — então nenhum `go build`
avisa e a única barreira é alguém reparar lendo o `client.go`. Enquanto isso o tipo do corpo
continua `ResponderCardPedido`, carregando a palavra que sumiu do método.

**A resposta.** O cliente devolve `(corpo, error)` e mais nada. `do()` tem o `*http.Response`
com o corpo aberto, mas é privado, e `call()` decodifica e fecha. Para uma API autenticada por
**cookie** isso deixa o login sem saída: a credencial vem no `Set-Cookie`, e a operação que a
produz é exatamente a que não dá acesso a ele. `WithClient` com um `http.CookieJar` não serve —
o jar é do processo, então um servidor que atende muita gente manda a credencial de uma pessoa
na chamada da seguinte, o que é vazamento entre sessões e não inconveniente — e `WithHeader` só
alcança a saída. O contorno é um `http.RoundTripper` que lê um coletor carregado pelo
`context`: umas 40 linhas que todo app com API de cookie reescreve. Os primos do caso são os de
sempre: o `ETag` e o `Location` de um `POST` que cria, o `Link:` de uma paginação.

**O campo anulável.** Um campo que é ao mesmo tempo `required` (sempre presente) e anulável
(`"type": ["string", "null"]`, ou `nullable: true`) sai `string` com `validate:"required"`. Os
dois lados da tradução perdem informação: o tipo apaga o nulo — `null` decodifica para `""`, e
quem escreve o app não distingue "não configurou" de "configurou vazio", o que é inócuo numa
cor e some silenciosamente num `["integer","null"]` cujo zero significa algo — e a tag mente,
porque `required` sobre uma string quer dizer "não vazio", não "presente": a struct afirma que
uma resposta perfeitamente legítima, `"primaria": null`, é inválida. Não morde hoje porque o
cliente decodifica e não valida; morde no dia em que alguém rodar o validador sobre a resposta,
que é a coisa mais natural do mundo de se fazer quando as structs já vêm com as tags prontas —
e o erro vai apontar para o servidor, que está certo.

## O que muda

### 1. O corte da tag no nome do método só acontece no começo, e só da tag inteira

O `operationId` continua perdendo a cauda que só repete caminho e método (o
`list_documents_api_documents_get` da FastAPI). O que sobra perde a tag **apenas** quando ela é
prefixo exato do nome e o resto começa uma palavra sua. Fora daí, o nome fica inteiro:

| `operationId` | tag | antes | agora |
|---|---|---|---|
| `config_reset` | `config` | `Reset` | `Reset` (prefixo exato, na fronteira: cai) |
| `configurar_regra` | `config` | `ConfigurarRegra` | `ConfigurarRegra` (spec 131) |
| `obter_resumo_dos_idiomas` | `idiomas` | `ObterResumoDos` | `ObterResumoDosIdiomas` |
| `responder_card` | `cards` | `Responder` | `ResponderCard` |
| `listar_regras` | `regras` | `Listar` | `ListarRegras` |
| `list_documents` | `documents` | `List` | `ListDocuments` |
| `upsert_user` | `users` | `Upsert` | `UpsertUser` |

`Idiomas().ObterResumoDosIdiomas()` e `Documents().ListDocuments()` são repetitivos ao lado do
grupo, e são certos: o nome que o documento escreveu chega inteiro em quem lê a chamada. Na
dúvida o gerador preserva, porque um nome repetitivo se lê e um nome amputado manda abrir o
`client.go`.

E o corte que sobra, o do prefixo, passa a ser **uma linha do relatório**, como já são o nome
que o gerador inventa e a operação sem `operationId`:

```
  PUT /api/config/reset: the name drops the tag prefix — operationId config_reset is Reset
```

### 2. `WithResponse`: a resposta HTTP de cada chamada, sem tocar na assinatura das operações

Uma `Option` do cliente, simétrica ao `WithHeader` que já existe, chamada uma vez por resposta
— inclusive nas que viram `*Error`:

```go
// no cliente gerado
func WithResponse(fn func(context.Context, *http.Response)) Option
```

```go
// no app: o login lê o Set-Cookie da resposta que produz a credencial
type colheita struct{ cookies []*http.Cookie }
type colheitaKey struct{}

var acervo = api.New(base, api.WithResponse(func(ctx context.Context, r *http.Response) {
	if c, ok := ctx.Value(colheitaKey{}).(*colheita); ok {
		c.cookies = r.Cookies()
	}
}))

func entrar(c *trilha.Ctx, email, senha string) error {
	var colhido colheita
	ctx := context.WithValue(c.Context(), colheitaKey{}, &colhido)
	if _, err := acervo.Sessoes().Login(ctx, api.Credenciais{...}); err != nil {
		return err
	}
	for _, ck := range colhido.cookies { // a credencial desta requisição, e de mais nenhuma
		c.SetCookie(ck)
	}
	return nil
}
```

O gancho recebe o `context` da chamada, e é isso que o torna seguro num servidor: o coletor
mora no `context`, então cada requisição enche o seu — ao contrário de um `CookieJar`, que é do
processo. Recebe a resposta pelo **status e pelos cabeçalhos**; o corpo é da operação, que
decodifica, então ler o corpo ali é tirá-lo de quem chamou. O que falta ao corpo o retorno da
operação já dá.

A escolha entre as três formas que a issue propõe: `WithResponse` (a preferida dela) não muda
assinatura nenhuma, não gera uma linha a mais por operação e vale igual para a operação que
devolve JSON, a que não devolve conteúdo e a que devolve bytes. Um método `<Op>Raw` por
operação dobraria o arquivo gerado para resolver o mesmo caso, e expor `Do` público obrigaria
quem só quer um cabeçalho a montar caminho e query à mão.

### 3. Anulável é ponteiro, e anulável não é `required`

Quando o schema de um campo ou de um parâmetro de query admite nulo — a lista do 3.1
(`"type": ["string","null"]`), a flag do 3.0 (`nullable: true`) ou o `anyOf` do Pydantic, que
já virava ponteiro:

- **o tipo Go é ponteiro**, para que `null` tenha como ser dito: `*string`, `*int64`,
  `*Vinculo`. Fatia, mapa e `json.RawMessage` já têm `nil` e ficam como estão — `*[]string` não
  é um tipo que alguém queira escrever;
- **a tag `validate` não leva `required`**, mesmo quando o documento diz `required`. As demais
  regras (`min`, `max`, `email`, `url`, `oneof`) continuam;
- a tag `json` não muda: `required` continua sem `omitempty`, então o campo viaja como `null`
  em vez de desaparecer.

```jsonc
"primaria": { "type": ["string", "null"], "description": "Cor primária, ou nula." }
// required: ["primaria"]
```

```go
// Cor primária, ou nula.
Primaria *string `json:"primaria"`
```

`required` some da tag porque nenhuma das duas leituras dele é verdade aqui: para o validador
do Trilha e para o `go-playground`, `required` sobre um ponteiro nulo falha — e `"primaria":
null` é uma resposta legítima. Dizer menos é a única opção honesta; quem quiser afirmar
presença tem o `required` do documento, que continua no documento.

Isto vale para o campo de struct e para o parâmetro de query (que já sabe mandar o valor de
trás do ponteiro, spec 130). O campo de `multipart/form-data` fica de fora: não existe `null`
num formulário, então não há o que expressar — o valor ausente é o campo que não viaja.

## Fora de escopo

- **Aviso do `trilha check`** sobre o corte do nome: o relatório do `trilha client` já é onde o
  gerador conta o que inventou, e é o comando que escreve o arquivo. Uma segunda voz dizendo o
  mesmo é ruído.
- **`Do`/`DoRaw` público** e variante `…WithResponse` por operação: `WithResponse` cobre os
  casos medidos da issue #207 sem crescer o arquivo gerado nem a superfície pública.
- **`lowerFirst` e o corte UTF-8 do primeiro byte** (#200): está na PR #211, de fora.
- **Ponteiro para fatia e mapa anuláveis**: `nil` já diz nulo nos dois.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `WithResponse` é `func(context.Context, *http.Response)`; o cliente gerado continua importando só a biblioteca padrão e `TestNoExternalDeps` segue verde. |
| III — geração determinística | Nenhuma decisão nova depende de mapa: o corte do nome é função do `operationId` e da tag, o ponteiro é função do schema. Goldens (`internal/client/api/client.go`) regravados e commitados. |
| IV — contrato pequeno e estável | A superfície pública do repositório não muda (o pacote gerado não é coberto pelo `api/current.txt`). Muda o **arquivo gerado**: `WithResponse` é adição; nome de método e tipo de campo anulável mudam no arquivo de quem regenerar, e o `CHANGELOG` diz isso com o conserto. |
| VI — teste primeiro | Um teste por contrato em `internal/client`, escrito antes: sufixo, singular, `Set-Cookie` na resposta (contra `httptest`, pelo cliente golden), e `required` + anulável. |
| VII — segurança por padrão | `WithResponse` existe para não haver credencial compartilhada: o coletor vem do `context` da chamada, não do processo, e a doc do gancho diz isso. O corpo não é oferecido para leitura, então nada vaza por um gancho que consome a resposta de outra pessoa. |

## Aceitação

- **SC-001** `obter_resumo_dos_idiomas` na tag `idiomas` gera `ObterResumoDosIdiomas`;
  `responder_card` na tag `cards` gera `ResponderCard`; `config_reset` na tag `config` continua
  `Reset` e vira linha do relatório. Teste em `internal/client`.
- **SC-002** Um servidor que responde `Set-Cookie` no login é lido por um app com
  `WithResponse` sem `CookieJar`, com o corpo da operação chegando decodificado do mesmo jeito;
  a resposta que virou `*Error` também passa pelo gancho.
- **SC-003** Campo `required` com `"type": ["string","null"]` sai `*string` sem
  `validate:"required"`; com `nullable: true`, igual; `["integer","null"]` sai `*int64`. Fatia
  anulável continua fatia.
- **SC-004** `make test` verde, `examples/` regenerados e compilando, documentação nas duas
  locales no mesmo commit.
