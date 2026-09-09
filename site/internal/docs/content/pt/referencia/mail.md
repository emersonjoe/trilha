---
title: mail
description: Mailer, Options, Message, Layout, Transport, SMTP, Dir e Outbox — a API do pacote mail, com os padrões e o que cada campo muda.
---

`import "github.com/emersonjoe/trilha/mail"` — o punhado de mensagens que uma aplicação interna
manda de verdade: o convite, o link, o aviso de que o fluxo terminou. O corpo é `h.Node`, a
mesma linguagem das páginas, e toda mensagem sai `multipart/alternative` com o texto gerado do
mesmo nó.

## O remetente

```go
func New(o Options) *Mailer                                  // não faz rede, não lê arquivo
func FromEnv() Options                                       // a partir do TRILHA_MAIL_URL
func (m *Mailer) Send(ctx context.Context, msg Message) error
```

O `New` mora numa var de pacote, junto do resto do setup do app. O `Send` bloqueia até o
servidor aceitar ou o prazo vencer, e pode ser chamado de várias goroutines.

| `Options` | Padrão | O que faz |
|---|---|---|
| `From string` | — | o remetente, `"no-reply@org.br"` ou `"Acervo <no-reply@org.br>"` |
| `Transport Transport` | do ambiente | para onde vão as mensagens; `nil` em produção é `ErrNotConfigured` |
| `Timeout time.Duration` | 10 s | limita a entrega inteira, como prazo no context |
| `Logger *slog.Logger` | `slog.Default()` | uma linha por mensagem, com assunto, destinatários e quanto demorou |

## O ambiente

```bash
TRILHA_MAIL_URL='smtp://usuario:senha@smtp.org.br:587?from=Acervo <no-reply@org.br>'
TRILHA_MAIL_URL='smtps://usuario:senha@smtp.org.br'  # TLS implícito, porta 465
TRILHA_MAIL_FROM='Acervo <no-reply@org.br>'          # ou o from= acima
TRILHA_MAIL_DIR=mail                                 # só em dev, padrão ./mail
```

Vazio, o comportamento depende do `TRILHA_ENV`: em **dev** escreve `.eml` em `./mail` e diz
onde, na saída de erro; em **qualquer outro lugar** não há transporte e o `Send` devolve
`ErrNotConfigured`. Um app em produção que arquiva convites numa pasta caladinho é um app cujos
usuários nunca são convidados.

URL que ele não consegue ler é panic no boot, e não um remetente fazendo outra coisa. Para
autenticar em canal aberto, `?insecure_auth=1` — escrito por extenso, porque uma flag que
desliga uma checagem de cifra tem de ser legível no arquivo que a define.

## Message

```go
type Message struct {
	To, Cc, Bcc []string
	From        string            // sobrepõe o Options.From
	ReplyTo     string
	Subject     string
	Body        h.Node            // o HTML
	Text        string            // gerado do Body quando vazio
	Headers     map[string]string // os que esta struct não tem campo para
}
```

Todo endereço é analisado antes de qualquer envio, então um endereço errado falha aqui, com o
endereço na mensagem, e não como um 501 três saltos adiante. O `Bcc` é destinatário do envelope
e de cabeçalho nenhum. O `Headers` não troca os cabeçalhos que o pacote escreve — `From`, `To`,
`Cc`, `Bcc`, `Subject`, `Date`, `Message-ID`, `MIME-Version`, `Content-Type`,
`Content-Transfer-Encoding` —, porque mensagem com dois `From` é mensagem que um servidor
recusa e outro entrega para a pessoa errada.

Assunto vai em Q-encoding e corpo em quoted-printable: uma linha de HTML gerado passa dos 998
octetos que o SMTP aceita, e servidor que quebra por você quebra dentro de uma URL.

## O layout

```go
func Layout(marca string, corpo ...h.Node) h.Node
func Button(texto, url string) h.Node
func Muted(filhos ...h.Node) h.Node
func PlainText(n h.Node) string
```

O `Layout` é uma tabela centralizada, larguras em pixels, toda regra inline — o Outlook
renderiza com o Word, que não conhece flexbox, grid nem float, e o Gmail arranca o `<style>` do
cabeçalho. Fazer isso uma vez aqui é o que mantém isso fora da aplicação. O rodapé diz por que
a mensagem chegou, que é a linha que impede uma mensagem legítima de ser denunciada como spam;
o idioma dele vem do `TRILHA_LANG` (e depois `LC_ALL`, `LC_MESSAGES`, `LANG`), e o app que
quiser as próprias palavras escreve o próprio layout — são vinte linhas.

O `Button` é um `<a>` com cara de botão: controle de formulário em e-mail não faz nada, e link
degrada para link nos clientes que recusam o estilo.

O `PlainText` é o que preenche o `Message.Text`. Bloco vira quebra de linha, lista vira uma
linha por item, `<style>` e `<script>` somem, e link vira `texto <https://…>` — URL que
desaparece joga a mensagem inteira fora. As linhas não são quebradas, porque linha quebrada é
URL quebrada; o quoted-printable já cuida do limite de tamanho.

## Transportes

```go
type Transport interface {
	Deliver(ctx context.Context, from string, to []string, raw []byte) error
}
```

Um método, para que a aplicação que envia por um provedor HTTP escreva o dela em vez de o
framework carregar um driver para cada um. O `raw` é a mensagem completa; `from` e `to` são o
envelope, que nem sempre é o que os cabeçalhos dizem.

### SMTP

```go
type SMTP struct {
	Addr              string      // host:porta
	User, Pass        string      // vazio é sem autenticação
	ImplicitTLS       bool        // a porta 465 já implica
	TLS               *tls.Config // para CA privada — não é lugar de desligar verificação
	AllowInsecureAuth bool
}
```

A porta 465 (ou o `ImplicitTLS`) já nasce cifrada; o resto negocia STARTTLS quando o servidor
oferece. **A autenticação nunca acontece em canal aberto** a menos que o `AllowInsecureAuth`
esteja ligado: servidor que oferece `PLAIN` sem cifra está mal configurado, não é convite. O
`PLAIN` é preferido, e o `LOGIN` entra quando é só o que o servidor anuncia — é o caso do
Office 365 e de alguns appliances, e ele não está na biblioteca padrão.

O prazo vem do context e é posto na conexão, que é o que faz o manipulador que desistiu
desligar. O `smtp.SendMail` não tem prazo nenhum, e esse é o defeito invisível que isto
substitui: um servidor lento segura o manipulador até o TCP perceber, minutos depois.

### Dir e Outbox

```go
type Dir string        // escreve .eml; é o padrão em desenvolvimento
type Outbox struct{}   // guarda em memória, para testes

func (o *Outbox) Messages() []Sent
func (o *Outbox) Last() Sent
func (o *Outbox) Reset()

type Sent struct {
	From    string
	To      []string
	Subject string
	Text    string
	HTML    string
	Raw     []byte
	At      time.Time
}
```

O `Dir` escreve `.eml` — o formato que todo cliente abre com dois cliques, então o que você
confere é a mensagem de verdade, e não uma renderização feita para a conferência.

O `Sent` já vem desmontado, então o teste afirma sobre o link dentro do corpo e não sobre
quoted-printable. O `Last` devolve um `Sent` zerado quando nada foi enviado, para o teste que
esqueceu de checar a contagem falhar no assunto vazio em vez de estourar.

O `Outbox` mora aqui, num pacote que não é de teste, de propósito: ele é o que o `mail.Sent(t)`
teria sido, e pacote que não é de teste não pode importar `testing` — quem importa registra as
flags de teste em todo binário do projeto.

## Erros

O `ErrNotConfigured` é produção sem servidor. É erro, e não um arquivo escrito em algum lugar,
porque aplicação que reporta sucesso para uma mensagem que nunca saiu é aplicação que ninguém
depura até um cliente perguntar.

O `trilha audit` avisa quando o código manda e-mail e o `TRILHA_MAIL_URL` está vazio.

## O que não está aqui

Envio assíncrono, fila persistente, retentativa, DKIM, anexo e IMAP. O primeiro chega com o
módulo de tarefas; o resto é trabalho do servidor de e-mail e do provedor, não de um módulo de
setecentas linhas.
