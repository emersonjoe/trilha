---
title: E-mail
description: Uma interface que os handlers chamam, SMTP atrás dela em produção, o log em dev, um corpo vindo de template e cabeçalhos que se recusam a ser injetados.
---

Mandar e-mail são três problemas vestindo um casaco só: falar com um servidor, montar uma
mensagem válida e não mandar nada a partir de um teste. Só o primeiro é sobre SMTP.

## A costura

```go
// Mailer is what the handlers call. They never learn which one they got,
// which is the whole point: the test and the dev server do not send mail.
type Mailer interface {
	Send(ctx context.Context, to []string, subject, body string) error
}
```

Uma interface com um método, definida onde é usada. Os handlers nunca ficam sabendo qual
implementação receberam, e é esse o ponto inteiro: um teste que cadastra um usuário não pode
mandar e-mail para um endereço de verdade.

```go
// SetupMailer makes the choice once, at startup. Production without an
// address configured fails to start, which is better than a sign-up that
// silently sends nothing.
func SetupMailer(a *trilha.App) error {
	if a.Env() == trilha.Dev {
		trilha.Provide[Mailer](a, LogMailer{Log: a.Logger()})
		return nil
	}
	addr, from := os.Getenv("SMTP_ADDR"), os.Getenv("SMTP_FROM")
	if addr == "" || from == "" {
		return errors.New("SMTP_ADDR and SMTP_FROM are required outside dev")
	}
	host, _, _ := strings.Cut(addr, ":")
	trilha.Provide[Mailer](a, SMTPMailer{
		Addr: addr,
		From: from,
		Auth: smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASSWORD"), host),
	})
	return nil
}
```

```go
// SendWelcome is the other end of the seam: a handler asks for the interface,
// never for the implementation behind it. The type argument is what Provide
// filed the value under, which is why it is written out here — LogMailer and
// SMTPMailer are two answers to the same question.
func SendWelcome(c *trilha.Ctx, name, email, link string) error {
	return Welcome(c.Context(), trilha.Use[Mailer](c), name, email, link)
}
```

`Provide` guarda o mailer sob `Mailer`, a interface, e não sob a struct que por acaso está
atrás dela hoje — é para isso que serve o argumento de tipo. Um handler que pede `Mailer`
recebe o log em dev e o SMTP em produção, e nunca fica sabendo da diferença.

Produção sem endereço configurado se recusa a subir. Isso é de propósito: um cadastro que
silenciosamente não manda nada é descoberto por um cliente, e um processo que não sobe é
descoberto pelo deploy.

## Enviando

```go
// SMTPMailer sends through a real server.
type SMTPMailer struct {
	Addr string // "smtp.example.com:587"
	From string
	Auth smtp.Auth
}
```

```go
// Send hands the message to the server. smtp.SendMail takes no context, so
// the deadline is honoured here: when the request gives up, the handler
// returns and the goroutine finishes on its own.
func (m SMTPMailer) Send(ctx context.Context, to []string, subject, body string) error {
	msg, err := Message(m.From, to, subject, body)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(m.Addr, m.Auth, m.From, to, msg) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

`smtp.SendMail` não recebe contexto, e um servidor de e-mail que para de responder seguraria a
requisição até o timeout de escrita. O `select` devolve o prazo ao handler; a goroutine termina
sozinha.

:::nota
A porta 587 com `PlainAuth` quer dizer STARTTLS, e o `net/smtp` recusa autenticação em texto
numa conexão que não está cifrada — essa recusa é um recurso. A porta 465 é TLS implícito, que
o `net/smtp` não faz sozinho: conecte com `tls.Dial` e use `smtp.NewClient`.
:::

## A mensagem

```go
// Message assembles the bytes of RFC 5322. A newline inside a header is how
// a form field becomes a second Bcc:, so anything that came from outside is
// refused rather than escaped.
func Message(from string, to []string, subject, body string) ([]byte, error) {
	for _, v := range append([]string{from, subject}, to...) {
		if strings.ContainsAny(v, "\r\n") {
			return nil, errors.New("cookbook: header injection")
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String()), nil
}
```

O laço lá em cima é a única checagem de segurança deste arquivo e a que costuma faltar. Uma
quebra de linha dentro de um cabeçalho é como um campo "nome" de formulário vira um segundo
`Bcc:` — seu servidor, a lista de outra pessoa. Recusar é o certo; escapar é um chute.

O corpo vem de `text/template`, não de `html/template`:

```go
// welcome is text/template, not html/template: what is being escaped here
// is nothing, and HTML escaping in a plain-text mail turns an apostrophe
// into &#39;.
var welcome = template.Must(template.New("welcome").Parse(
	`Hello, {{.Name}}.

Your account is ready. Set your password here:
{{.URL}}

This link is good for one hour.
`))
```

```go
// Welcome renders the body and sends it.
func Welcome(ctx context.Context, m Mailer, name, email, link string) error {
	var b strings.Builder
	if err := welcome.Execute(&b, struct{ Name, URL string }{name, link}); err != nil {
		return err
	}
	return m.Send(ctx, []string{email}, "Welcome", b.String())
}
```

Escape de HTML num e-mail de texto puro transforma um apóstrofo em `&#39;` na caixa de entrada
de alguém. Se você manda um e-mail multipart com HTML, aí o `html/template` é o certo para
aquela parte — e a parte de texto vai junto, porque muito cliente mostra ela.

## Em dev

```go
// LogMailer is the implementation for dev and tests: it writes the message
// to the log. Nobody's inbox learns about your fixtures.
type LogMailer struct{ Log *slog.Logger }

```

A mensagem inteira no log, link incluído, que é o que você de fato precisa quando está testando
uma redefinição de senha pela quinta vez.

:::dica
Duas outras implementações se pagam: uma que junta as mensagens numa fatia, para os testes
conferirem, e uma que escreve arquivos `.eml` num diretório para você abrir num cliente de
e-mail.
:::
