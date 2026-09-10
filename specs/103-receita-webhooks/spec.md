# Spec 103 — a receita `webhooks`: o que o app avisa para fora

- **Issue**: [#116](https://github.com/emersonjoe/trilha/issues/116) — a issue é a fonte do escopo.
- **Branch**: `103-receita-webhooks`
- **Versão**: 0.83.0

## Por quê

O módulo `webhook` existe desde a 0.65.0, com entrega assinada, retry, backoff e o
`ui.WebhooksPanel`. O `examples/blog` usa. E é exatamente o caso que a issue descreve: quem tem um
projeto e precisa disso hoje só descobre o módulo se já souber que ele existe — e, sabendo, ainda
copia seis arquivos de um exemplo trocando o módulo à mão.

## O que muda

```
$ trilha add webhooks
  + internal/avisos/avisos.go        os eventos deste app, e o Hooks que os entrega
  + internal/avisos/avisos_test.go
  + app/webhooks/page.go             registrar, ver entregas, reenviar, testar
  + webhooks_test.go
  ~ app/setup.go                     avisos.Setup(a)
```

O que a receita escreve, e por que cada linha:

- **A lista de eventos é fechada, num arquivo.** Um erro de digitação num `Emit` vira erro na hora
  de escrever, e não um evento que ninguém assina — que, visto de fora, é idêntico a um parceiro
  que não está ouvindo.
- **O `Env` vem do app.** `http://` só existe em dev; endereço de rede privada é recusado sempre.
  É o padrão do módulo, e a receita o escreve explícito para ninguém "consertar" isso depois.
- **A tela é uma rota com um GET e um POST**, e o POST inteiro é `hooks.Handle(c)`: registrar,
  revogar, reenviar e testar são um formulário só.
- **Guardar a pasta é do app, e a receita diz isso duas vezes** — no comentário do arquivo e na
  linha final do comando. Ela não escreve o middleware porque não sabe como este projeto
  autentica: escrever um que importa a receita `login` obrigaria quem usa OIDC a instalá-la.

## Fora de escopo

- **O lado que recebe.** `webhook.Verify` é uma função e já está na doc do módulo, com a ordem que
  se erra (responder 2xx antes de fazer o trabalho) — é receita de página, não de arquivo.
- **Store em SQL.** Como em todas as receitas: memória aqui, tabela na receita do cookbook.
- **`Emit` no lugar certo.** Onde o app avisa é decisão do app; o `Next` diz a linha e onde ela
  costuma ir.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o módulo `webhook` e o kit, que já são do repositório |
| VI — teste primeiro | a receita escreve um teste, e o e2e da CI aplica ela num projeto novo |
| VII — segurança por padrão | `Env` do app, endereço privado recusado, segredo mostrado uma vez |

## Tarefas

- [x] T001 Teste que falha: `add webhooks` escreve os quatro e liga o `Provide`
- [x] T002 A receita: eventos, tela, middleware, teste
- [x] T003 O e2e da CI aplicando `webhooks` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.83.0`

## Aceitação

- **SC-001** `trilha add webhooks` num projeto novo passa `trilha check` sem uma edição.
- **SC-002** A tela responde e oferece a lista fechada de eventos deste app.
- **SC-003** Um endereço de rede privada é recusado no registro.
- **SC-004** O comando diz, no fim, que a pasta precisa ser guardada.
