# Spec 149 — Range para corpo que não busca posição, e SVG servido neutralizado

- **Issues**: [#201](https://github.com/emersonjoe/trilha/issues/201) e
  [#210](https://github.com/emersonjoe/trilha/issues/210) — as issues são a fonte do escopo;
  aponte para elas, não as reescreva aqui.
- **Branch**: `feat/inline-range-svg`
- **Versão**: 0.128.0

## Por quê

Os dois pedidos chegam do mesmo lugar: um app que serve binário que **não é dele** —
o corpo de outro serviço, o logotipo que um cliente subiu — e que, para servi-lo, precisa
abandonar `c.Inline`/`c.Attachment` e escrever a resposta à mão.

`send` entrega o corpo ao `http.ServeContent` só quando ele é um `io.ReadSeeker`. O corpo de
uma resposta HTTP nunca busca posição, então o caso que o `Config.Upstreams` existe para
servir — um frontend Trilha na frente de um backend que já existe — cai no `io.Copy` e no 200.
Sem 206, o `<audio>` do Safari no iOS não posiciona a reprodução: "pular para o instante X"
vira baixar o arquivo inteiro (#201).

`c.Inline` recusa `image/svg+xml`, e a recusa está certa — para o navegador um SVG é documento
com script, e servir script da própria origem é XSS armazenado com outro nome. O que falta é a
saída: hoje a recusa é um erro de programação sem nome, que a tela não tem como distinguir de
um bug, e quem precisa do logotipo em SVG sai do envelope e escreve a resposta à mão (#210).

Nos dois casos, sair do envelope custa as três coisas que ele dava de graça: o nome saneado, o
`nosniff` e a relaxação de enquadramento. Trocar uma proteção por perder três é o oposto do que
a recusa queria.

## O que muda

Uma porta só, com opções, em vez de quatro métodos novos (`InlineRange`, `AttachmentRange`,
`InlineNeutralized`, …):

```go
// SendOpts é o que a forma de três argumentos não sabe dizer.
type SendOpts struct {
	Inline           bool
	Size             int64
	ContentRange     string
	NeutralizeScript bool
}

func (c *Ctx) Send(name string, body io.Reader, ctype string, o SendOpts) error

var ErrCannotInline = errors.New("trilha: media type may not be sent inline")
```

`c.Inline(name, body, ctype)` é `c.Send(name, body, ctype, SendOpts{Inline: true})` e
`c.Attachment(...)` é `SendOpts{}`: mesma função por dentro, mesma resposta, e as duas
continuam sendo o que quase todo mundo escreve.

### 1. `Size`: o Range que o kit corta do stream (#201)

`Size` é o comprimento total de um corpo que não busca posição — o `Content-Length` que o
outro serviço respondeu. Com ele:

| Requisição | Resposta |
|---|---|
| sem `Range` | 200 com `Content-Length: Size` e **`Accept-Ranges: bytes`** — é esta resposta que ensina o tamanho ao navegador e o autoriza a pedir pedaços |
| `Range: bytes=10-19` | 206, `Content-Range: bytes 10-19/Size`, `Content-Length: 10`, e só aqueles dez bytes |
| `Range: bytes=900-` | 206 até o último byte |
| `Range: bytes=-10` | 206 com os dez últimos |
| `Range` ilegível ou fora do arquivo | 416 com `Content-Range: bytes */Size`, sem corpo do arquivo |
| `Range` com mais de um intervalo | 200 com o arquivo inteiro — resposta legal para qualquer `Range`, e `multipart/byteranges` não paga num stream |
| `HEAD` | os mesmos cabeçalhos, nenhum byte lido do corpo |

O corte é `io.CopyN(io.Discard, body, first)` e depois um `io.LimitReader`: o prefixo é
descartado à medida que chega, nunca acumulado. É a diferença que a issue nomeia — ler o
arquivo inteiro para a memória para ganhar um `bytes.Reader` custaria o áudio inteiro na
memória do contêiner a cada pedido de 64 KB.

**Sem `Size` nada muda**: um `io.Reader` sem tamanho declarado continua saindo inteiro, com
200 e sem `Accept-Ranges`. O kit não pode inventar o `/total` do `Content-Range`, e prometer
`Range` sem saber o tamanho é pior do que não prometer.

### 2. `ContentRange`: o corpo que já é o pedaço (#201)

Quando quem chama consegue repassar o `Range` ao outro serviço, o certo é repassar: o backend
responde 206 e o prefixo não viaja duas vezes. O corpo que volta **já é** o pedaço, e é só
disso que o kit precisa saber:

```go
res, _ := http.DefaultClient.Do(req) // com o Range da requisição repassado
return c.Send(a.Nome, res.Body, res.Header.Get("Content-Type"), trilha.SendOpts{
	Inline:       true,
	ContentRange: res.Header.Get("Content-Range"), // "bytes 10-19/4096"
})
```

O valor é conferido (`bytes primeiro-último/total`, com `*` aceito no total, `primeiro` ≤
`último`), vira o `Content-Range` da resposta, o status vira 206, o `Content-Length` sai
calculado do intervalo e nada é descartado nem cortado. Valor torto é erro de programação, não
uma resposta torta: um `Content-Range` que quem chama montou errado seria um arquivo corrompido
no navegador, e um cabeçalho montado a partir de resposta de terceiro é por onde entra uma
segunda linha de cabeçalho.

`Size` e `ContentRange` juntos são erro: dizem duas coisas diferentes sobre o mesmo corpo.

### 3. `NeutralizeScript`: o SVG servido com a política que desliga o script (#210)

`NeutralizeScript: true` faz `Send` aceitar inline o que `inlineNever` recusa —
`image/svg+xml`, `image/svg`, `text/html`, `application/xhtml+xml` — e **impõe** na resposta:

```
Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'self'
```

Nada de script, nada de buscar nada, estilo embutido (é o que faz um SVG ser um desenho), e
enquadrável pela própria origem, que é a razão de um inline existir. A política é do kit e não
de cada app: quem está portando uma tela não deveria ter de descobrir qual CSP é a certa, e três
apps escrevendo a sua dariam três políticas diferentes, uma delas errada.

Limites, de propósito:

- a opção abre **só** a porta do script. `application/zip` inline continua recusado com ela: a
  lista do que um visor mostra não tem nada a ver com neutralizar script;
- ela não vale só para os tipos de `inlineNever` — o cabeçalho sai para qualquer tipo enviado
  com ela. Uma rota que serve o que a pessoa subiu (PDF, imagem, SVG) escreve uma chamada só; o
  kit recusar um PDF porque quem chamou foi cauteloso seria o kit punindo a cautela;
- o cabeçalho é escrito nesta resposta mesmo quando o app tem `Security.CSP` próprio. A
  neutralização é a condição para o documento sair; uma política mais frouxa por baixo dela
  seria a condição não valendo.

### 4. `ErrCannotInline`: a recusa que a tela sabe tratar (#210)

Sem a opção, a recusa continua — mas agora com nome:

```go
if err := c.Inline(a.Nome, corpo, a.Tipo); errors.Is(err, trilha.ErrCannotInline) {
	return c.Render(http.StatusOK, semVisor(a)) // a tela desenha o seu fallback
}
```

A mensagem embrulhada diz o tipo e as duas saídas (mandar como anexo, ou `NeutralizeScript`).
`trilha.CanInline(ctype)` continua respondendo a mesma pergunta antes da chamada.

## Constitution Check

| Princípio | Como esta spec o respeita |
|---|---|
| I. Convenção sobre configuração | Nenhuma convenção nova em `app/`: são métodos do `Ctx`. As rotas novas de `examples/blog` usam as convenções que já existem (`route.go`, `page.go`) |
| II. Só biblioteca padrão | `io`, `net/http`, `strconv`, `strings`. Nenhuma dependência nova; `TestNoExternalDeps` continua verde |
| III. Geração explícita | `examples/blog/trilha_gen.go` regerado pela CLI e commitado |
| IV. Contrato pequeno e estável | Três símbolos novos (`Send`, `SendOpts`, `ErrCannotInline`), nenhuma assinatura alterada, nada removido; `api/current.txt` atualizado e cada símbolo com doc comment e uso em `examples/` |
| V. Dev < 2 s, produção num binário | Não toca o servidor de desenvolvimento nem o build |
| VI. Teste primeiro | `send_test.go` (corte, 416, passagem do `Content-Range`, SVG neutralizado, recusa tratável) e `examples/blog/midia_test.go` escritos antes da implementação |
| VII. Segurança por padrão | O caminho novo **mantém** o envelope: nome saneado, `nosniff`, enquadramento relaxado só nesta resposta. A recusa de SVG continua sendo a resposta padrão; abri-la exige escrever `NeutralizeScript: true`, e o que sai com ela sai sob `default-src 'none'`. `Content-Range` de terceiro é conferido antes de virar cabeçalho |

## Tarefas

Em [tasks.md](tasks.md); o plano e as decisões em [plan.md](plan.md).

## Aceitação

- `Send` com `Size` e `Range: bytes=10-19` sobre um corpo que não busca posição responde 206,
  `Content-Range: bytes 10-19/1000` e exatamente os dez bytes daquele trecho;
- o mesmo corpo sem `Range` responde 200 com `Accept-Ranges: bytes` e `Content-Length`;
- `Range` fora do arquivo responde 416 com `Content-Range: bytes */1000`;
- `ContentRange` de um corpo já parcial responde 206 sem cortar nada, e um valor torto é erro;
- SVG com `NeutralizeScript` responde 200, `Content-Disposition: inline`, `nosniff` e a CSP
  acima; sem a opção, `errors.Is(err, trilha.ErrCannotInline)` é verdadeiro e nada foi escrito;
- `/midia/audio` e `/midia/marca` do `examples/blog` provam os dois caminhos ponta a ponta;
- `make test` verde, `api/current.txt` com as três linhas novas, referência do `Ctx` em inglês
  e em português no mesmo commit.
