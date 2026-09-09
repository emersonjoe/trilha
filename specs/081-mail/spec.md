# Spec 081 — módulo trilha/mail

- **Issue**: [#113](https://github.com/emersonjoe/trilha/issues/113) — a issue é a fonte do escopo.
- **Branch**: `081-mail`
- **Versão**: 0.63.0

## Por quê

Todo app interno manda e-mail: convite de usuário, o link do `c.Link` (#106), aviso de que o
fluxo terminou. O framework não diz nada sobre isso, então o que se escreve é `net/smtp`
direto no manipulador — e é ali que o iniciante trava, em perguntas que não são sobre o
produto dele: porta 587 ou 465, STARTTLS ou TLS implícito, PLAIN ou LOGIN, como escrever o
corpo sem voltar a fazer `<table>` de 2003, como testar sem mandar e-mail de verdade para
uma pessoa de verdade, e como não deixar o request esperando o servidor SMTP.

A última é a que mais dói e a que menos aparece: `smtp.SendMail` sem prazo pendura o
manipulador até o TCP desistir, o que em produção é uma página que fica girando.

## O que muda

Módulo opcional `trilha/mail`, com o corpo em `h.Node` — a mesma linguagem das páginas.

```go
// app/setup.go
var Mail = mail.New(mail.FromEnv())

// TRILHA_MAIL_URL=smtp://usuario:senha@smtp.org.br:587?from=Acervo+<no-reply@org.br>
// vazio + TRILHA_ENV=dev  → escreve .eml em ./mail/ e loga o caminho
// vazio + prod            → Send devolve mail.ErrNotConfigured
```

```go
err := Mail.Send(c.Context(), mail.Message{
	To:      []string{u.Email},
	Subject: "Você foi convidado",
	Body:    mail.Layout("Acervo", h.P(h.Text("Clique para entrar:")), mail.Button("Entrar", link)),
})
```

| Símbolo | O que é |
|---|---|
| `New(Options) *Mailer`, `FromEnv() Options` | o remetente, configurado por ambiente |
| `Message{To, Cc, Bcc, From, ReplyTo, Subject, Body, Text, Headers}` | `Body` é `h.Node`; `Text` só se você quiser escrever o alternativo à mão |
| `(*Mailer).Send(ctx, Message) error` | monta, codifica e entrega |
| `Layout(marca string, corpo ...h.Node) h.Node`, `Button(texto, url)` | o e-mail transacional pronto, CSS inline |
| `Transport`, `SMTP`, `Dir`, `Outbox` | para onde vai: servidor, pasta, memória |
| `ErrNotConfigured` | prod sem servidor: erro, não silêncio |
| `PlainText(h.Node) string` | o alternativo, gerado |

**Todo e-mail sai `multipart/alternative`**, com o texto gerado do próprio `h.Node`: quem lê
em cliente sem HTML — e todo filtro de spam — recebe algo legível, e um link vira
`texto <https://…>` em vez de sumir. Cabeçalhos vão em `Q-encoding` e o corpo em
quoted-printable, porque uma linha de HTML gerado passa dos 998 octetos que o SMTP aceita.

**Prazo de 10 s em tudo**, e ele é do `context`: o manipulador que desiste desliga a conexão.

Teste sem contêiner e sem servidor: `Outbox` guarda em memória e devolve o que foi enviado,
já separado em texto e HTML.

```go
box := &mail.Outbox{}
m := mail.New(mail.Options{From: "Acervo <no-reply@org.br>", Transport: box})
// … roda o convite …
if len(box.Messages()) != 1 || !strings.Contains(box.Messages()[0].HTML, link) { … }
```

O `trilha audit` passa a avisar quando o código manda e-mail e `TRILHA_MAIL_URL` está vazia —
o modo dev é bom para desenvolver e péssimo como descoberta de produção.

## Fora de escopo

- **Envio assíncrono.** A issue o quer via `task` (#111), que não existe ainda. `Send` é
  síncrono; quando o `task` chegar, ele entra como `Options.Tasks` sem mudar a assinatura.
- **Fila persistente, retentativa, DKIM.** É trabalho do servidor SMTP e do provedor, não de
  um módulo de 700 linhas.
- **Provedores HTTP** (SendGrid, SES, Resend). `Transport` é uma interface de um método; a
  receita mostra como escrever um, e é honesto deixar isso fora do zero-dependências.
- **`mail.Sent(t)` como a issue pediu.** Um pacote que não é de teste não pode importar
  `testing`: quem importa registra as flags de teste em todo binário do projeto. O `Outbox`
  faz a mesma coisa sem esse preço.
- **Recebimento, IMAP, anexos.** Anexo entra quando alguém precisar; hoje ninguém precisa.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net/smtp`, `mime`, `mime/multipart`, `mime/quotedprintable`, `net/mail`, `crypto/tls` |
| VI — teste primeiro | servidor SMTP falso em `net.Listen`, com TLS de verdade gerado no teste |
| VII — segurança por padrão | TLS obrigatório para autenticar (a senha nunca sai em claro), prazo em tudo, prod sem servidor é erro |
| Convenção nova | uso em `examples/local-login` (convite por e-mail com `c.Link`) + teste de integração |

## Tarefas

- [x] T001 Teste que falha: `Outbox` recebe um `multipart/alternative` com texto e HTML
- [x] T002 `Message`, montagem, `PlainText`, `Layout`, `Button`
- [x] T003 Teste do SMTP falso: STARTTLS negociado, AUTH só sob TLS, prazo respeitado
- [x] T004 `SMTP`, `Dir`, `FromEnv`, `ErrNotConfigured`
- [x] T005 Convite por e-mail em `examples/local-login` + teste de integração
- [x] T006 Aviso do `trilha audit`
- [x] T007 Receita en + pt, referência en + pt
- [x] T008 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.63.0 --issues "113"`

## Aceitação

- **SC-001** Um `Send` produz uma mensagem que um cliente sem HTML lê inteira, com os links.
- **SC-002** Autenticação nunca acontece fora de TLS, nem quando o servidor oferece.
- **SC-003** Sem `TRILHA_MAIL_URL`: em dev escreve `.eml` e loga o caminho; em prod devolve
  `ErrNotConfigured` sem fingir que enviou.
- **SC-004** Um teste de aplicação afirma sobre o e-mail enviado sem rede e sem contêiner.
- **SC-005** `TestNoExternalDeps` continua verde.
