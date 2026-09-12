---
title: E-mail
description: trilha/mail — o corpo é h.Node, o texto se escreve sozinho, o dev escreve .eml e um teste afirma sobre o que foi enviado sem servidor nenhum.
---

Mandar e-mail são três problemas usando um casaco só: falar com um servidor, montar uma
mensagem válida e não mandar nada de dentro de um teste. O `trilha/mail` responde os três, e o
que vale ler é quais decisões ele tira de você.

## Ou: `trilha add mail`

```bash
trilha add mail --dry-run
```

```text
  + internal/correio/correio.go
  + internal/correio/correio_test.go

--dry-run: nada foi escrito
```

Dois arquivos e nenhuma tela — o `mail` não tem listagem nem rota, então não toca o
`app/setup.go`. O que chega é o `internal/correio/correio.go`: a variável `Mailer` abaixo, já
construída a partir de `mail.FromEnv()`, e um teste que prova isso contra o `mail.Outbox` sem
rede. O que esta página soma é tudo o que a receita não pode decidir por você: a mensagem em
si, o `mail.Layout` e o `mail.Button`, o fluxo de convite, e como trocar de provedor pelo
`Transport`. Rode a receita pelo arquivo que é dono do `Mailer`; escreva as mensagens aqui,
porque uma receita não tem como adivinhar o que o seu app tem para dizer.

## O remetente

```go
// Mailer is the one an app holds: built once, at startup, from the
// environment. New opens no connection and reads no file, so it belongs in a
// var and not behind a sync.Once.
var Mailer = mail.New(mail.FromEnv())
```

O `FromEnv` lê uma variável:

```bash
TRILHA_MAIL_URL='smtp://usuario:senha@smtp.org.br:587?from=Acervo <no-reply@org.br>'
```

A porta 587 é STARTTLS; a 465 (ou `smtps://`) é TLS implícito. Com a variável vazia o
comportamento depende de onde o app está rodando, e é essa diferença que importa: em dev ele
escreve `.eml` em `./mail` e diz onde; em qualquer outro lugar o `Send` devolve
`mail.ErrNotConfigured`.

A assimetria é de propósito. Um app em produção que arquiva convites numa pasta caladinho é um
app cujos usuários nunca são convidados, e ninguém descobre por uma semana.

## Enviando

```go
// SendWelcome is what a handler calls. The message is an h.Node — the same
// nodes the pages are written with — and mail.Layout is what keeps the tables
// and the inline CSS out of here.
func SendWelcome(c *trilha.Ctx, nome, email, link string) error {
	return Mailer.Send(c.Context(), mail.Message{
		To:      []string{email},
		Subject: "Sua conta está pronta",
		Body: mail.Layout("Acervo",
			h.P(h.Textf("Olá, %s.", nome)),
			mail.Button("Definir minha senha", link),
			mail.Muted(h.Text("O link vale por uma hora.")),
		),
	})
}
```

O `mail.Layout` é uma tabela centralizada, com larguras em pixels e toda regra inline, porque o
Outlook renderiza com o Word — sem flexbox, sem grid, sem float — e o Gmail arranca o `<style>`
do cabeçalho. Fazer isso uma vez aqui é o que mantém isso fora da sua aplicação. O `Button` é
um `<a>` com cara de botão: controle de formulário dentro de e-mail não faz nada, e link
degrada para link.

Toda mensagem sai `multipart/alternative`, e **o texto é gerado do mesmo nó**:

```
Acervo

Olá, Ana.

Definir minha senha <https://acervo.org.br/convite/abc123>

O link vale por uma hora.
```

Um link vira `texto <https://…>` em vez de sumir, que é o que faz a mensagem servir para quem
lê num cliente sem HTML — e um ponto a menos de spam. Escreva o `Message.Text` você mesmo só
quando quiser outras palavras ali.

Os cabeçalhos viajam em Q-encoding e o corpo em quoted-printable, porque uma linha de HTML
gerado passa dos 998 octetos que o SMTP aceita, e servidor que quebra a linha por você quebra
no meio de uma URL. O `Bcc` é destinatário do envelope e de cabeçalho nenhum. E os cabeçalhos
que o pacote escreve não podem ser trocados pelo `Message.Headers`: mensagem com dois `From` é
mensagem que um servidor recusa e outro entrega para a pessoa errada.

## O ciclo inteiro: um convite

O `examples/local-login` convida quem ainda não tem conta. O link é uma capacidade com prazo —
`c.Link`, um uso, 48 horas — e o e-mail é o que entrega:

```go
// Convite is the message somebody receives before they have an account: the
// only thing in it is the link, and the only thing the link needs to say is
// who invited them and until when it works.
func Convite(ctx context.Context, para, nome, quemConvidou, link string) error {
	return Mailer.Send(ctx, mail.Message{
		To:      []string{para},
		Subject: "Você foi convidado para o " + Marca,
		Body: mail.Layout(Marca,
			h.P(h.Textf("%s convidou você para o %s.", quemConvidou, Marca)),
			mail.Button("Criar minha senha", link),
			mail.Muted(h.Text("O convite vale por 48 horas e só pode ser usado uma vez. "+
				"Se o botão não funcionar, cole este endereço no navegador: "+link)),
		),
	})
}
```

Duas coisas desse exemplo valem copiar. A página que aceita mora em pasta própria, fora da que
convida: um middleware guarda a pasta dele **e tudo que está abaixo**, então uma página de
aceite embaixo da tela de convidar exigiria a sessão que a pessoa convidada ainda não tem. E o
endereço da mensagem é absoluto, montado a partir do `TRILHA_BASE_URL` — e-mail não tem página
atual para ser relativo a.

## Testando

O `mail.Outbox` guarda em memória, já desmontado. O teste afirma sobre o link dentro do corpo,
não sobre quoted-printable:

```go
// caixa põe um Outbox no lugar do remetente do app, que é como um aplicativo
// testa e-mail: sem rede, sem contêiner, sem servidor SMTP falso.
func caixa(t *testing.T) *mail.Outbox {
	t.Helper()
	box := &mail.Outbox{}
	antes := correio.Mailer
	correio.Mailer = mail.New(mail.Options{From: "Acervo <no-reply@exemplo.com>", Transport: box})
	t.Cleanup(func() { correio.Mailer = antes })
	return box
}
```

```go
	// O link vive no texto tanto quanto no HTML — é o que faz a mensagem
	// funcionar num cliente que não mostra HTML.
	link := extraiURL(t, msg.Text, "/convite/")
	if !strings.Contains(msg.HTML, link) {
		t.Fatal("o link do texto não é o mesmo do HTML")
	}
```

:::note
É `Outbox` e não `mail.Sent(t)` porque um pacote que não é de teste não pode importar
`testing`: quem importa registra as flags de teste em todo binário do projeto, e `-test.v` num
servidor web é uma surpresa desagradável.
:::

## Outro provedor

O `Transport` tem um método, e é esse o ponto de extensão. Um app que envia por uma API HTTP
escreve isto, em vez de o framework carregar um driver para cada provedor:

```go
// Deliver implements mail.Transport.
func (r Resend) Deliver(ctx context.Context, from string, to []string, raw []byte) error {
	body, err := json.Marshal(map[string]any{"from": from, "to": to, "raw": string(raw)})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.Key)
	req.Header.Set("Content-Type", "application/json")
	cli := r.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		detalhe, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("resend: %s: %s", res.Status, detalhe)
	}
	return nil
}
```

```go
// SetupMailer picks the transport at startup: the provider when its key is
// there, the environment's answer otherwise.
func SetupMailer() *mail.Mailer {
	o := mail.FromEnv()
	if key := os.Getenv("RESEND_API_KEY"); key != "" {
		o.Transport = Resend{Key: key}
	}
	return mail.New(o)
}
```

A mensagem chega pronta — cabeçalhos, multipart, codificação — então o que sobra é a chamada
HTTP.

## O que está testado

Os testes de unidade montam a mensagem e a leem de volta com `net/mail` e `mime/multipart`,
então o que se afirma é o que um cliente leria, e não o que este pacote quis escrever.

O cliente SMTP é exercitado contra **um servidor de verdade, num socket de verdade, com TLS de
verdade**, porque é a única forma de provar o que interessa: que o STARTTLS foi mesmo
negociado, que a senha não saiu antes dele, e que o `AUTH LOGIN` funciona com os servidores que
só falam isso. Um transporte falso provaria que o pacote chama os próprios métodos.

:::warning
O `SMTP.AllowInsecureAuth` manda a senha por uma conexão que nunca foi cifrada. Ele vem
desligado e continua desligado até alguém digitar o campo: servidor que oferece `PLAIN` em
canal aberto está mal configurado — não é um convite.
:::

## O que a auditoria diz

O `trilha audit` avisa quando o código manda e-mail e o `TRILHA_MAIL_URL` está vazio. O modo
dev é ótimo para desenvolver e uma péssima descoberta em produção.
