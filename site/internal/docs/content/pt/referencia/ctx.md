---
title: Ctx
description: Tudo que uma função de rota pode fazer com o contexto da requisição.
---

`*trilha.Ctx` embrulha a requisição e a resposta. É criado por requisição e não deve ser
usado por outra goroutine depois que o handler devolve.

## Requisição

| Método | Descrição |
|---|---|
| `Request() *http.Request` | a requisição original |
| `SetContext(ctx)` | troca o contexto da requisição: um middleware passa valores a código que só recebe `*http.Request` |
| `SetRequest(*http.Request)` | troca a requisição (URL reescrita, corpo embrulhado) |
| `Context() context.Context` | contexto da requisição (cancelamento) |
| `Param(nome) string` | parâmetro de rota (`slug_` → `"slug"`) |
| `Pattern() string` | o gabarito da rota que casou (`/blog/{slug}`), a forma agregável do caminho; `""` para o que o fallback respondeu (estático, 404, redirecionamento de barra) |
| `Query(nome) string` | primeiro valor do parâmetro de query |
| `Form(nome) string` | campo do formulário (faz o parse sob demanda, com limite de tamanho) |
| `FormErr() error` | erro do parse do formulário: 400 inválido, 413 grande demais |
| `BindJSON(&v) error` | decodifica o corpo JSON; campos desconhecidos são erro (400); 413 acima do limite |
| `Cookie(nome) (*http.Cookie, error)` | cookie da requisição |
| `Accepts(ofertas...) string` | a oferta que o cliente prefere (`Accept`, ranqueado por `q`), ou `""`; cabeçalho ausente ou `*/*` fica com a primeira oferta |
| `RequestID() string` | `X-Request-ID` recebido ou um id gerado |
| `Env() trilha.Env` | `trilha.Dev` ou `trilha.Prod` |
| `Base() string` | prefixo de URL (`TRILHA_BASE_PATH`), sem barra final |
| `App() *trilha.App` | a aplicação |
| `Fragment() string` | id que o cliente quer trocar (cabeçalho `Trilha-Fragment`), ou `""` numa navegação normal ([Interatividade](/pt/aprender/interatividade)) |

## Resposta

| Método | Descrição |
|---|---|
| `JSON(code, v) error` | escreve JSON com `Content-Type` correto |
| `Text(code, s) error` | escreve texto simples |
| `HTML(code, node) error` | escreve um nó como documento inteiro, sem layouts |
| `Redirect(url) error` | devolve o erro de redirecionamento 303 (use com `return`) |
| `Status(code)` | status que a próxima renderização de página vai usar |
| `Header(k, v)` | define um cabeçalho de resposta |
| `SetCookie(*http.Cookie)` | adiciona `Set-Cookie` |
| `Flash(tipo, texto)` | guarda um aviso para a requisição seguinte, num cookie assinado: a notícia que o redirect comeria. O `ui.Flashes(c)` mostra. Numa resposta de fragmento ele vai no cabeçalho `Trilha-Flash`, e quem mostra é o `ui.js`. Sem `TRILHA_SECRET` nada é escrito e o app avisa uma vez no log |
| `Flashes() []Flash` | os avisos deixados pela requisição anterior mais os que esta ainda não mandou; ler é gastar, e ler duas vezes dá a mesma lista |
| `Render(code, node) error` | escreve a página **com os layouts da rota** (como o GET): para um `POST` devolver o formulário com erros (422); num fragmento, sem os layouts |
| `Stream() *Stream` | resposta em Server-Sent Events: `Send(evento, dados)`, `JSON(evento, v)`, `Comment(s)`, `Flush()`, `Done()`; desliga o *write timeout* ([IA e agentes](/pt/aprender/ia-e-agentes)) |
| `Writer() http.ResponseWriter` | acesso direto (downloads longos, WebSocket) |
| `Written() bool` | se a resposta já começou |

## Cache HTTP

| Método | Descrição |
|---|---|
| `ETag(tag) bool` | escreve `ETag` (com aspas, se faltarem) e diz se a requisição já a tinha |
| `LastModified(t) bool` | escreve `Last-Modified` e diz se a cópia está em dia |
| `CacheControl(v)` | escreve `Cache-Control` como veio |

`true` quer dizer que o `304` já foi escrito: devolva `nil, nil` e não escreva mais nada. Só `GET` e
`HEAD` respondem `304`; nos outros métodos os cabeçalhos são escritos e a resposta é sempre
`false`. Etiqueta vazia ou data zerada não escrevem nada. Quando os dois são declarados, quem
decide é o `If-None-Match` e a data fica como metadado, como pede a RFC 9110. Os arquivos em
`static/` já vêm com ETag: a impressão digital do conteúdo que vai no `?v=`.

## Entre página e layout

| Método | Descrição |
|---|---|
| `SetTitle(s)` / `Title() string` | título da página, lido pelos layouts |
| `Set(chave, v)` / `Get(chave) any` | valores por requisição (middleware → página → layout) |

## Ilhas

```go
func (c *Ctx) Island(src string, props any, children ...h.Node) h.Node
```

Renderiza `<div data-trilha-island="…" data-trilha-props="…">` com os filhos como conteúdo
de origem, vindo do servidor. `src` é um módulo em `public/` (endereçado pelo `Asset`, então
leva o hash do conteúdo) cuja **exportação padrão** é a função de montagem, chamada uma vez
com `(el, props, island)`. `props` é qualquer coisa que o `encoding/json` serialize, ou `nil`; viaja
como atributo escapado e volta pelo `JSON.parse`, então é dado, nunca marcação. Props que
não serializam avisam uma vez e deixam o conteúdo de origem em paz. O carregador é um único
script inline com o nonce da requisição, emitido junto da primeira ilha da resposta
([Interatividade](/pt/aprender/interatividade)).

### O caminho de volta: o objeto `island`

O terceiro argumento é o canal para o servidor. Ele existe para a ilha não ter que
redescobrir o token, os cabeçalhos e o formato de erro que o resto do framework já combinou:

| Membro | O que faz |
|---|---|
| `island.csrf()` | o token desta resposta — o mesmo que o `CSRFInput` põe em todo formulário |
| `island.signal` | um `AbortSignal`, abortado quando o elemento sai da página |
| `island.get(url)` | lê JSON |
| `island.post(url, dados)` | manda JSON com o token junto e devolve o que a rota respondeu |
| `island.send(metodo, url, dados)` | o mesmo, para `PUT`, `PATCH` e `DELETE` |
| `island.swap(url, id)` | troca um fragmento, como um link com alvo faz |

Uma rota que responde `422` volta como `IslandInvalid`, e o `.fields` dela é o mesmo objeto
que o formulário mostraria; qualquer outra falha é um `IslandError` com `.status` e
`.detail`. Um `Trilha-Location` na resposta é seguido como navegação, então
POST → redirect → GET também funciona de dentro de uma ilha.

O cookie de dupla submissão é `HttpOnly`, então o token chega à ilha escrito no elemento,
como `data-trilha-csrf`. É ele que o `island.post` manda no `X-CSRF-Token`.

Um `route.go` é API, e API não confere o token — o cliente dela leva um bearer, não um
cookie. A ilha é a exceção, porque o cliente dela é a página:

```go
// app/blog/novo/rascunho/middleware.go
func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error {
	return trilha.RequireCSRF(c, next)
}
```

A rota continua sendo API, então os erros dela continuam em problem+json — que é o que a
ilha consegue ler.

### Os tipos que o módulo enxerga

O script que a página carrega para tudo isso é o `trilha.IslandRuntime` (`/ui.island.js`),
gravado em `public/` pelo `trilha ui` junto com o resto do kit; o `trilha check` avisa quando um
projeto monta uma ilha sem ele.

O `trilha gen` grava `public/islands.d.ts` a partir das chamadas `c.Island` que acha em
`app/`: uma interface por struct de props, mais o objeto `island` e a assinatura da função de
montagem. Aponte o módulo para lá e o editor confere os dois lados da fronteira:

```js
/** @type {import("/islands.d.ts").IslandMount<"/editor.js">} */
export default function (el, props, island) { … }
```

Props escritas como literal de mapa ou como variável não têm nome onde pendurar um tipo: a
ilha continua declarada, com tipo `unknown`, e o `trilha gen` avisa. O `gen --check` compara
o arquivo como compara o `trilha_gen.go`, e o app que perdeu a última ilha perde o arquivo.

## Conexão longa e corpo grande

| Método | Descrição |
|---|---|
| `AllowBody(n int64)` | limite de corpo **desta** requisição, no lugar do `Config.MaxBodyBytes` |
| `NoReadDeadline() error` | tira o prazo de leitura desta requisição (upload lento não é erro) |
| `NoWriteDeadline() error` | tira o prazo de escrita (download longo, SSE) |
| `Hijack() (net.Conn, *bufio.ReadWriter, error)` | assume a conexão: prazos removidos, e o Trilha não escreve mais nada nela |

O limite padrão é do app; a exceção é da rota. Levante no `middleware.go` da rota, não no
handler — o CSRF de formulário lê o corpo antes do handler rodar, então a decisão tem de vir
antes:

```go
// app/anexos/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if c.Request().Method == "POST" {
		c.AllowBody(8 << 20) // só esta requisição; o resto do app segue no limite do app
		c.NoReadDeadline()
	}
	return next()
}
```

Estourar o limite continua sendo 413 com a mensagem de sempre, pelo `FormErr`, pelos `Bind*`
ou na leitura direta do `Request().Body`.

### WebSocket

O Trilha não tem WebSocket próprio, e isso é decisão. O protocolo é transporte: não encosta
em rota, em layout nem em render. O que ele exige — frames de fragmentação e continuação,
frame de controle no meio de uma mensagem, aperto de mão de fechamento com prazo, validação
de UTF-8, máscara, limite de tamanho, escrita concorrente, contrapressão,
`permessage-deflate` — são algumas centenas de linhas que a suíte Autobahn cobra em mais de
500 casos. A assimetria decide: o seu app pode pôr `coder/websocket` no go.mod **dele** (o
princípio II obriga o framework, não o app), mas não consegue tirar essas linhas do
framework.

O que faltava era a porta, e ela é o `Hijack`:

```go
func WS(c *trilha.Ctx) error {
	conn, _, err := c.Hijack() // prazos de leitura e escrita já removidos
	if err != nil {
		return err
	}
	defer conn.Close()
	return meuWebsocket.Serve(conn) // coder/websocket, gorilla, o que você escolher
}
```

Depois do `Hijack` a conexão é sua: o framework não escreve cabeçalho, página de erro nem
corpo nela, e o log de acesso registra 101.

## Segurança

| Método | Descrição |
|---|---|
| `CSRFToken() string` | token da requisição; cria o cookie na primeira chamada |
| `trilha.CSRFInput(c) h.Node` | `<input type="hidden" name="_csrf">` para formulários |
| `trilha.CSRFTokenFrom(r) string` | o mesmo token, para quem só recebe o `*http.Request` (`html/template`, `templ`, um handler seu); `""` fora de uma requisição da Trilha |
| `trilha.NonceFrom(r) string` | o nonce da CSP daquela requisição, mesmo motivo e mesma regra ([Segurança](/pt/referencia/seguranca)) |

O token é verificado automaticamente em `POST`, `PUT`, `PATCH` e `DELETE` de `page.go`
(e de `route.go` se `Config.CSRFForAPI` estiver ligado), pelo campo `_csrf` ou pelo
cabeçalho `X-CSRF-Token`.

## Bind

`Bind(v any) error` preenche uma struct a partir do formulário (ou do JSON, quando o
`Content-Type` é `application/json`). Campos casam pela tag `form:"nome"`, depois pela `json:"nome"`, depois pelo nome do campo — um
struct que veio de uma API tem tag json e não tem form, e um formulário que postava `nome` para um
campo que o binder chamava de `Nome` fazia o `required` disparar sobre um valor que alguém digitou;
tipos: `string`, `[]string`, `bool` (`on`/`true`/`1`), `int`, `int64`, `float64`
(vírgula ou ponto), `time.Time` (`2006-01-02` ou `2006-01-02T15:04`) e ponteiros (nil quando
ausente). Struct aninhada é achatada, com a tag como prefixo (`Cobranca Endereco
`+"`form:\"cob_\"`"+` lê `cob_cep`…). Valores que não convertem viram `FieldErrors`
(mensagem `trilha.BindInvalid`, ajustável) depois de todos os campos serem tentados. A tag
`validate:"..."` de cada campo é aplicada logo em seguida, na mesma passada: veja
[Validação](/pt/referencia/validacao).

## File

`File(campo string, regras FileRules) (*Upload, error)` lê um arquivo do formulário
multipart e só o devolve se ele passar pelas regras.

| Símbolo | Papel |
|---|---|
| `FileRules.MaxSize int64` | limite deste arquivo, à parte do `Config.MaxBodyBytes`; 0 deixa o limite do corpo trabalhar |
| `FileRules.Accept []string` | tipos aceitos, comparados com o tipo **detectado**: `"image/png"`, `"image/*"`, `"*/*"`; vazio aceita qualquer um |
| `FileRules.Optional bool` | campo ausente devolve `(nil, nil)` em vez de erro |
| `FileRules.MaxFiles int` | teto de quantos arquivos o `Files` aceita numa requisição; 0 é sem teto |
| `Upload.Name` | nome sanitizado: sem diretório, sem separador, sem caractere de controle, no máximo 100 caracteres, nunca vazio |
| `Upload.MIME` / `Upload.Ext` | tipo detectado nos primeiros 512 bytes, e a extensão correspondente |
| `Upload.Size` / `Upload.File` | tamanho em bytes, e o arquivo posicionado no começo |
| `up.Save(dir) (string, error)` | grava dentro de `dir` (modo 0600) com um nome livre e devolve o caminho |
| `up.Close() error` | fecha o arquivo |

Regra que falha vira `FieldErrors` no nome do campo, como no `Bind`; qualquer outra coisa
(corpo quebrado, disco cheio) volta como está. As mensagens saem do `ValidationMessages`
(`required`, `filemax`, `filetype`, `filecount`) — veja [Validação](/pt/referencia/validacao).

`Files(campo string, regras FileRules) ([]*Upload, error)` lê todos os arquivos que o campo
carrega, na ordem em que o navegador mandou, aplicando as mesmas regras a cada um. Um campo de
arquivo vazio não é um arquivo: ele sai antes da contagem, então `Optional` continua querendo
dizer "ninguém escolheu nada". O arquivo que falha nomeia a própria posição — `files[2]`, não
`files` — então um formulário com uma linha por arquivo põe a mensagem na linha certa; os que
passaram voltam na fatia, e fechá-los é com quem chamou. Acima do `MaxFiles` a requisição
inteira é recusada no nome do campo, antes de um byte ser lido.

## Rascunhos

`Draft(nome string) *Draft` é um formulário em andamento, guardado de uma requisição para a
outra. É a resposta à única pergunta difícil de um formulário em passos: onde mora o passo 1
enquanto a pessoa está no passo 2.

| Símbolo | Papel |
|---|---|
| `c.Draft(nome)` | nomeia o formulário — não a pessoa; o rascunho viaja no cookie dela |
| `d.Save(v any, ttl time.Duration) error` | escreve o rascunho e começa o prazo |
| `d.Load(v any) error` | preenche v, ou `ErrNoDraft` — nunca salvo, terminado, vencido, mexido, ou de outro navegador |
| `d.Clear()` | o rascunho virou registro e deixa de existir |
| `Config.Drafts DraftStore` | onde fica um rascunho acima de 2 KB de JSON; nil é só o cookie |

Abaixo de 2 KB de JSON o rascunho é um **cookie assinado**: nada para configurar, nada para
limpar, e vence sozinho. Acima disso ele precisa do `Config.Drafts` — três métodos sobre o que a
app já roda — e, sem ele, o `Save` devolve um erro citando esse campo em vez de mandar um cookie
que o navegador descartaria sem avisar. (O limite é 2 KB e não os 3 KB que a issue propunha
porque o que vai no cookie é o base64 do rascunho mais o prazo e a assinatura, e 3 KB de JSON
passam dos 4 KB do navegador.)

`ErrNoDraft` é uma resposta, não uma falha: é o que manda a pessoa de volta ao passo 1. Rascunho
escrito por uma versão anterior da struct responde igual — o campo mudou de nome entre dois
deploys, e recomeçar é melhor que um 500 no meio do formulário de alguém.

Rascunho é assinado, então não dá para editá-lo à mão, e **não é secreto**: o que está num cookie
viaja para o navegador e pode ser lido lá. Preço ou o nome de outra pessoa ficam atrás do
`Config.Drafts`, com só a chave no cookie. Assinar precisa do `TRILHA_SECRET`; sem ele, o `Save`
avisa em vez de falhar calado.

O `ui.Steps` desenha o indicador — veja [Formulário em passos](/pt/receitas/formulario-em-passos)
para o fluxo inteiro.

## Planilhas

`CSV(name string, rows any) error` escreve uma fatia — ou um canal de recebimento — como um
arquivo que a pessoa abre, e `BindCSV(r io.Reader, dst any, rules ...CSVRules) (CSVResult, error)`
lê um de volta dizendo qual célula está errada.

| Símbolo | Papel |
|---|---|
| `c.CSV(name string, rows any) error` | download com BOM UTF-8, o separador do locale e linhas em CRLF |
| `csv:"Cabeçalho"` | o cabeçalho da coluna; sem tag é o nome do campo, e `csv:"-"` deixa o campo de fora |
| `BindCSV(r io.Reader, dst any, rules ...CSVRules)` | lê o arquivo num `*[]T`, validando cada linha com as tags `validate` dela |
| `CSVRules.MaxRows int` | teto do arquivo; padrão 100 mil, e acima dele um erro em vez de um corte |
| `CSVRules.Separator rune` | força o delimitador; zero detecta pelo cabeçalho |
| `CSVResult.Rows int` | quantas linhas de dados foram lidas, fora as em branco |
| `CSVResult.Errors []CSVError` | `{Line, Column, Message}` na ordem do arquivo; o cabeçalho é a linha 1 |
| `CSVResult.Warnings []string` | o que foi estranho e não fatal — cabeçalho que nenhum campo reivindica |
| `res.OK() bool` | sem erros: todas as linhas podem ser usadas |

O `Config.Locale` decide o separador (`,` em en, `;` em pt-BR), a data (`2006-01-02 15:04`
contra `02/01/2006 15:04`), a marca decimal e a palavra do booleano (`yes`/`no`, `sim`/`não`);
o `Config.TimeZone` decide em que dia um horário cai. O BOM não é opcional nem configurável:
sem ele o Excel lê todo acento como caractere estranho, e é a primeira coisa que qualquer um
percebe.

Um canal é o que uma exportação de duzentas mil linhas quer — as linhas são escritas conforme
chegam e o prazo de escrita é retirado —, mas encerrar o produtor é com você: faça `select` no
`c.Context().Done()`, ou um navegador que fechou a conexão deixa uma goroutine para trás.
(`iter.Seq` não é aceito: este módulo compila no Go 1.22.)

Na volta, o separador e o BOM são detectados, o cabeçalho casa pela tag em qualquer ordem
ignorando maiúsculas e espaços em volta, linha em branco é pulada, e a data é lida em
`dd/mm/aaaa` além do ISO. Só as linhas que passam entram na fatia, então depois do `res.OK()`
ela é o arquivo inteiro. Coluna obrigatória que falta no cabeçalho é uma mensagem na linha 1,
não a mesma mensagem em toda linha; cabeçalho que nenhum campo reivindica é aviso, porque
planilha ganha coluna o tempo todo. O `ui.CSVErrors` desenha a lista — veja
[Planilhas (CSV)](/pt/receitas/planilhas) para a ida e a volta inteiras.

Junto com o `ui.Flashes(c)` — a linha de resumo, e a tabela do que corrigir embaixo — é a
resposta que uma tela de importação dá sem a pessoa abrir a planilha para achar uma data errada
no olho.

@demo ui-avisos-csv

## Mandando um arquivo

O `File` recebe; estes cinco mandam. O nome, o tipo e os dois cabeçalhos que impedem um
download de virar uma página da sua origem são tudo o que há.

| Símbolo | Papel |
|---|---|
| `Attachment(nome string, corpo io.Reader, tipo string) error` | download: `Content-Disposition: attachment` |
| `Inline(nome string, corpo io.Reader, tipo string) error` | o navegador abre ali mesmo (um PDF num `<iframe>`, uma imagem) |
| `AttachmentFile(caminho, tipo string) error` | abre o arquivo, manda e fecha; ausente é 404, diretório é erro |
| `InlineFile(caminho, tipo string) error` | o mesmo, aberto no visor |
| `Pipe(res *http.Response) error` | entrega ao navegador a resposta de outro serviço e fecha o corpo dela |

O nome passa pela mesma função que saneia o de um upload: um caminho nunca vira nome de
arquivo, e nada dentro dele acrescenta uma segunda linha ao cabeçalho. Ele sai duas vezes —
`filename*=UTF-8''` percent-encoded para quem lê a RFC 5987 e um `filename` ASCII entre aspas
para quem não lê — porque nome com acento quebra em metade dos navegadores quando sai uma vez.

Um `tipo` vazio é detectado nos primeiros 512 bytes, como o `File` fareja um upload, com a
extensão podendo afinar dentro da mesma família (`text/plain` para `text/csv`) e nunca
contrariar. O `X-Content-Type-Options: nosniff` sai sempre: um download que o navegador pode
reinterpretar é um download que vira página desta origem.

O `Inline` só aceita o que um visor mostra — `application/pdf`, `image/*` (SVG não),
`audio/*`, `video/*`, `text/plain`, `text/csv`. HTML, SVG e XML voltam como erro de
programação, não como resposta: são documentos com script, servidos da sua própria origem. O
`trilha.CanInline(ctype)` responde a mesma pergunta, para uma tela oferecer o "ver" só onde há
o que ver.

**A resposta é que diz que pode ser enquadrada por uma página desta origem**, e é essa a metade
que todo mundo erra. O endurecimento padrão manda `X-Frame-Options: DENY` e
`frame-ancestors 'none'` em toda resposta; um documento que carrega isso não aparece no lugar,
não importa o que a página em volta declare. Acrescentar `frame-src` na página que enquadra não
muda nada — quem recusa é a resposta enquadrada, e o console diz isso. O `Inline` afrouxa esses
dois cabeçalhos naquela resposta só, e deixa em paz o app que escreveu o próprio
`Security.CSP`.

Quem desenha é o [`ui.Preview`](/pt/referencia/ui): a barra, o quadro, o cartão para o tipo que
ninguém mostra, e um `<img>` no lugar do quadro quando é imagem.

Um `corpo` que é `io.ReadSeeker` — um `bytes.Reader`, um `os.File` — sai pelo
`http.ServeContent`, então `Range`, `If-Range`, `304` e `HEAD` vêm de graça e o
`Accept-Ranges: bytes` é prometido. Qualquer outro é copiado em stream e não promete nada.
Todo envio desliga o write deadline: um arquivo de 50 MB numa linha ruim não é um handler
lento.

O `Pipe` copia o status e uma lista fechada de cabeçalhos — `Content-Type`,
`Content-Disposition`, `Content-Length`, `Content-Encoding`, `Content-Range`, `Accept-Ranges`,
`Cache-Control`, `ETag`, `Last-Modified`, `Expires`, `Vary`, `Age` — e mais nada. O
`Set-Cookie`, em especial, não viaja: o corpo de outro serviço não senta na sessão deste. Para
um prefixo inteiro encaminhado a outro serviço, veja [Upstreams](/pt/referencia/upstreams); o
`Pipe` é a resposta que você mesmo foi buscar.

## Links públicos

`c.Link(nome, opts)` monta uma URL assinada que funciona sem sessão, e `c.Claim(nome)` é o que a
rota do outro lado chama.

| Símbolo | Papel |
|---|---|
| `c.Link(nome, LinkOpts{...})` | a URL; `nome` é o fim, e link de um fim não abre outro |
| `LinkOpts.Data` | viaja dentro do token: **assinado, não secreto** |
| `LinkOpts.TTL` | zero é uma hora; negativo é erro, não padrão |
| `LinkOpts.Uses` | zero é ilimitado e não precisa de estado nenhum |
| `c.Claim(nome)` | confere assinatura, fim, prazo e usos restantes |
| `link.Consume()` | gasta um uso — depois do trabalho, nunca antes |
| `Config.Links` | um `trilha.LinkStore` que conta os usos dos links com limite — um `INCR` comparado com o limite, que é o que faz o Redis ser o natural; nil conta no processo, o que é honesto sobre uma réplica e é dito uma vez no log |
| `trilha.ErrNoLink` | o que o `Claim` responde para um token inválido, de outro link, vencido ou esgotado — um erro para os quatro casos, de propósito |

**Toda forma de um link falhar responde o mesmo 404.** Assinatura errada, fim errado, vencido, já
gasto: dizer a um estranho qual das quatro aconteceu é dizer o quão perto ele está. Token errado
também custa ao endereço um ponto de um orçamento pequeno, porque adivinhar token em URL é força
bruta.

Esse orçamento **se recompõe, e não é um bloqueio**, o que é uma diferença deliberada em relação
à issue que pediu isto: uma hora de bloqueio por endereço transforma uma pessoa desastrada atrás
do NAT de um escritório numa queda para todo mundo atrás dele — e a propriedade que importa,
adivinhar ficar inviável, é a mesma nos dois casos.

**Com `Uses: 0` não há estado.** Nem linha, nem consulta, nem limpeza: conferir é checar uma
assinatura. É o caso do código de verificação impresso num documento, e é por isso que o fluxo
comum não precisa de tabela.

**Ler não é gastar.** O `Claim` confere que sobrou uso; só o `Consume` tira um. Senão um
recarregar queimaria o link de quem ainda está preenchendo, e um erro de validação custaria o
convite.

Veja [Link público](/pt/receitas/link-publico) para o fluxo inteiro.
