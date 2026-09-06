# Feature Specification: WebSocket (adaptador, não núcleo) e upload com progresso

**Feature Branch**: `024-websocket-upload` | **Created**: 2026-09-05 | **Status**: Rascunho
**Input**: issue [#24](https://github.com/emersonjoe/trilha/issues/24) (ROADMAP, Fase 1,
item 5). A issue pede duas coisas e uma decisão: WebSocket no núcleo ou adaptador, e upload
com progresso. As duas viram uma spec só porque esbarram no mesmo lugar — o que o Trilha faz
com a conexão e com o corpo da requisição antes do handler ver qualquer coisa.

## O problema

`c.Stream()` (SSE) cobre servidor → cliente, que é a maior parte do "tempo real" de um app
renderizado no servidor. Sobram dois casos, e nos dois o Trilha hoje **atrapalha**:

1. **Bidirecional.** Quem precisa de WebSocket pega uma biblioteca (é o normal em Go) e ela
   falha na primeira linha: `response does not implement http.Hijacker`. O
   `responseWriter` do Trilha embrulha o writer do `net/http` para contar bytes e status, e
   não repassa `Hijack`. Ou seja: o framework manda usar a porta de saída e tranca a porta.

2. **Upload.** `MaxBodyBytes` vale 1 MiB por padrão e é global, aplicado em `wrap` antes do
   handler. Um formulário com um anexo de 5 MB responde 413 e a única saída é subir o limite
   para o app inteiro — inclusive para as rotas que não recebem arquivo, que é exatamente
   onde o limite serve para alguma coisa. Some-se o `ReadTimeout` de 30 s: um upload grande
   em rede ruim morre no meio sem que ninguém tenha feito nada errado. E não há progresso:
   o usuário aperta "Enviar" e olha para uma página parada por 40 segundos.

## A decisão sobre WebSocket

**Fica fora do núcleo. O que entra é a porta de saída funcionando.**

*A favor de implementar a RFC 6455 aqui:* mantém o princípio II verdadeiro também para o
app (nenhuma dependência em lugar nenhum), dá uma API só para os dois sentidos, e o
handshake mais o enquadramento básico cabem em ~400 linhas — como o `examples/blog` desta
spec demonstra em ~90.

*Contra, e é o que vence:* as 400 linhas são a parte fácil. O que dá trabalho é o que gera
bug na casa dos outros — fragmentação e frames de continuação, frames de controle
intercalados com o fluxo de dados, o *close handshake* que precisa ser respondido com
prazo, validação de UTF-8 no frame de texto (a RFC manda derrubar a conexão), mascaramento,
limite de tamanho de mensagem, escrita concorrente, contrapressão, e `permessage-deflate`
(RFC 7692), que todo mundo acaba querendo. Nada disso tem suíte de conformidade rodando num
código escrito à mão — a Autobahn tem mais de 500 casos e existe exatamente porque essa
implementação é mais sutil do que parece.

Some-se o argumento de escopo: WebSocket é **transporte**. Não conversa com roteamento,
layout, formulário, CSRF ou renderização, que é o assunto do Trilha. Seria a única parte do
framework que não usa nenhuma outra parte.

E a assimetria decisiva: um app que precisa de WebSocket adiciona `coder/websocket` ao
**go.mod dele** — o princípio II obriga o framework, não o app. Já um app que não precisa
não consegue remover 400 linhas do framework, mas paga por elas em superfície pública,
compromisso de compatibilidade da 1.0 e tempo de manutenção que sai de outro item do
roadmap.

## A ideia

Duas mudanças pequenas no runtime e um arquivo novo no kit.

```go
// Bidirecional: a resposta agora é http.Hijacker, então qualquer biblioteca funciona.
func WS(c *trilha.Ctx) error {
	conn, _, err := c.Hijack() // prazos de leitura e escrita já removidos
	if err != nil {
		return err
	}
	defer conn.Close()
	return meuWebsocket.Serve(conn) // ou coder/websocket, gorilla, o que for
}

// Upload: o limite é por rota, não do app inteiro. Vai no middleware.go da
// rota porque o CSRF de formulário lê o corpo antes do handler — a decisão
// tem de vir antes de qualquer leitura.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if c.Request().Method == "POST" {
		c.AllowBody(8 << 20) // só esta requisição
		c.NoReadDeadline()   // rede ruim não é erro
	}
	return next()
}
```

No cliente, o progresso é o que o navegador já sabe:

```go
h.Form(h.Method("post"), h.Action("/anexos"), h.EncType("multipart/form-data"),
	ui.UploadTo("lista"),          // envia por XHR e troca #lista com a resposta
	trilha.CSRFInput(c),
	ui.Input(h.Type("file"), h.Name("arquivo")),
	ui.UploadBar(),                // <progress> que a barra atualiza
	ui.Submit(h.Text("Enviar")),
)
```

`ui.UploadScript(c)` carrega `ui.upload.js` — arquivo à parte, como o `ui.nav.js` da 0.14.0.
Sem JavaScript o formulário envia normalmente e a rota responde a página inteira.

## Requisitos

- **FR-001** A resposta do Trilha implementa `http.Hijacker`, delegando pelo
  `http.ResponseController` (que segue a cadeia de `Unwrap`).
- **FR-002** `Ctx.Hijack()` remove os prazos de leitura e escrita da conexão antes de
  devolvê-la, e marca a requisição como sequestrada: o log registra 101 e o framework não
  tenta mais escrever nada nela (nem página de erro, nem cabeçalho).
- **FR-003** `Ctx.AllowBody(n int64)` troca o limite de corpo **desta** requisição, e vale
  chamada de `middleware.go` (antes do CSRF ler o formulário) ou do handler. Vale
  para `FormErr`, `Bind*` e leitura direta de `Request().Body`. Chamada depois do corpo
  começar a ser lido, vale a partir dali (documentado, não é erro).
- **FR-004** `Ctx.NoReadDeadline()` remove o prazo de leitura desta requisição, simétrico ao
  `NoWriteDeadline` que já existe.
- **FR-005** Estourar o limite continua respondendo 413 com a mensagem atual; o `413` não
  vira 400 nem 500 por causa do `AllowBody`.
- **FR-006** `ui.UploadTo(id)`, `ui.UploadBar()` e `ui.UploadScript(c)` no kit; o
  comportamento vive em `public/ui.upload.js`, que o `ui.Head` **não** carrega.
- **FR-007** O envio por XHR manda o cabeçalho `Trilha-Fragment: id` e troca `#id` com
  `window.ui.swap`, igual ao fragmento da 018 — mesma rota, mesma resposta.
- **FR-008** Progresso: `<progress>` dentro do formulário recebe `value`/`max` a cada evento
  de `xhr.upload.progress`, e o evento `trilha:upload` (`detail.loaded`, `detail.total`,
  `detail.form`) permite ao app mostrar do jeito dele.
- **FR-009** Recuo: 5xx, erro de rede ou resposta sem o id fazem o formulário enviar de
  verdade. O atributo de upload não colide com `data-trilha-target` — são atributos
  diferentes, então o handler de fragmento do `ui.js` não dispara junto.
- **FR-010** `ui.upload.js` entra em `ui.Files` e é gravado por `trilha ui`, com teto de
  4 KiB no teste.

## Fora de escopo

- **WebSocket no núcleo.** Decidido acima. O `examples/blog/internal/ws` é do exemplo, não
  do framework: só frames de texto, sem fragmentação, sem deflate — o mínimo que prova que a
  porta abriu, com o aviso escrito de que produção pede biblioteca.
- **`Ctx.File(name)` com limite de tamanho e tipo.** É a issue
  [#28](https://github.com/emersonjoe/trilha/issues/28), Fase 2. Esta spec entrega o corpo
  chegando inteiro e o progresso; a validação do arquivo é lá.
- **Progresso do lado do servidor** (bytes recebidos por handler). Com proxy no caminho o
  número do navegador já é o único honesto do ponto de vista do usuário, e um segundo canal
  só para contar bytes paga caro por pouco.
- **Retomar upload interrompido, upload em pedaços, direto para S3.** Cada um é um protocolo
  próprio e nenhum é problema do framework.
- **Limite por rota declarado em `route.go`.** `AllowBody` na primeira linha do handler diz a
  mesma coisa sem inventar convenção nova no `app/`.

## Aceitação

- **SC-001** Uma rota que chama `c.Hijack()` responde a um handshake WebSocket real e devolve
  um frame de texto mascarado — verificado por teste que fala o protocolo na mão.
- **SC-002** Sem `AllowBody`, um corpo de 2 MiB responde 413; com `c.AllowBody(4<<20)`, o
  mesmo corpo passa e o handler lê o arquivo inteiro.
- **SC-003** O `examples/blog` recebe um anexo maior que o limite global e devolve o
  fragmento da lista; o mesmo formulário, enviado sem JavaScript, devolve a página inteira.
- **SC-004** `ui.upload.js` está em `ui.Files`, cabe em 4 KiB e o `ui.js` não fala de upload.
- **SC-005** `trilha ui --js-only` grava os três `.js`.
