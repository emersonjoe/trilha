# Spec 105 — a receita `mail`: o lugar de onde este app manda e-mail

- **Issue**: [#116](https://github.com/emersonjoe/trilha/issues/116) — a issue é a fonte do escopo.
- **Branch**: `105-receita-mail`
- **Versão**: 0.85.0

## Por quê

O módulo `mail` existe desde a 0.63.0, com layout, botão, texto alternativo, SMTP e o modo de
dev que escreve `.eml` numa pasta. O que falta é o mesmo de sempre: o arquivo onde as mensagens
de um app moram, com um nome por mensagem, em vez de um `mail.Send` montado no meio de um handler.

## O que muda

```
$ trilha add mail
  + internal/correio/correio.go       o mailer, e uma mensagem com nome
  + internal/correio/correio_test.go  o Outbox, que é a história de teste inteira do módulo
```

Três decisões que a receita escreve junto:

- **Uma função por mensagem, com nome.** O handler diz "manda o convite", não "monta um
  multipart". É o que faz a mensagem ser revisável por quem não escreve Go.
- **O mailer é variável de pacote**, e é de propósito: é o que deixa um teste pôr um `Outbox` no
  lugar e conferir o que foi mandado — sem container, sem rede, sem SMTP falso.
- **Em dev não sai e-mail nenhum**, e isso é dito. Sem `TRILHA_MAIL_URL` as mensagens viram `.eml`
  em `./mail`; em produção, `Send` devolve `ErrNotConfigured` em vez de fingir que enviou, e o
  `trilha audit` já avisa quando o código manda e-mail e a variável está vazia.

## Fora de escopo

- **Fila e retry de envio.** Se o envio precisa sobreviver a uma queda, ele é uma tarefa — e a
  receita `tasks` existe. O `Next` diz a linha.
- **Templates em arquivo.** O corpo é `h.Node`, o mesmo que as páginas usam: uma segunda linguagem
  de template seria uma segunda coisa para aprender e para escapar errado.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o módulo `mail`, que já é do repositório |
| VI — teste primeiro | a receita escreve o teste com `Outbox`, e o e2e da CI aplica ela |
| VII — segurança por padrão | nada é enviado sem configuração explícita; o dev escreve em disco |

## Tarefas

- [x] T001 Teste que falha: `add mail` escreve os dois arquivos
- [x] T002 A receita: mailer, mensagem com nome, teste com Outbox
- [x] T003 O e2e da CI aplicando `mail` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.85.0`

## Aceitação

- **SC-001** `trilha add mail` num projeto novo passa `trilha check` sem uma edição.
- **SC-002** O teste que vem junto prova o assunto, o destinatário e o link do corpo.
- **SC-003** Sem configuração, `Send` não finge que enviou.
