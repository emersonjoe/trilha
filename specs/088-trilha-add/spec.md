# Spec 088 — trilha add: escrever a receita no projeto

- **Issue**: [#116](https://github.com/emersonjoe/trilha/issues/116) — a issue é a fonte do escopo.
- **Branch**: `088-trilha-add`
- **Versão**: 0.69.0

## Por quê

Metade das coisas desta fase são **padrões de app**, não primitivos: uma tabela de auditoria,
uma tela de chaves, uma seção de configurações. No núcleo elas ficam rígidas; como documentação
viram trabalho de copiar. O `examples/local-login` já é uma receita que funciona — só que o
iniciante precisa saber que ela existe, clonar o repositório e ajustar caminhos à mão.

O que ele quer é o comando que **escreve os arquivos no projeto dele**, legíveis, que ele passa
a possuir.

A diferença para o `generate` (0.68.0) é a direção: o `generate crud` escreve a partir do
**código do usuário** — um struct vira tela; o `add` escreve a partir de uma **receita do
Trilha**. Os dois terminam em `gen` e deixam o `trilha check` verde.

## O que muda

```
$ trilha add
  audit        the trail of who did what, and the screen that reads it
  api-keys     keys for the API: issue, revoke, and the middleware that requires one
  settings     a settings section somebody edits on a screen instead of in the environment

$ trilha add audit
  + internal/auditoria/store.go        the sink and its memory
  + app/auditoria/page.go              the screen, with ui.AuditTable
  ~ app/setup.go (one line added)      Config.Audit, and the store behind it
  ✓ trilha_gen.go (1 rota nova)
  Doc: https://trilha.dev/reference/observability
```

Regras, e cada uma é uma coisa que dá errado quando falta:

- **`add` nunca sobrescreve.** Arquivo que existe é pulado com aviso, e o comando segue: quem
  roda `add` uma segunda vez está acrescentando, não recomeçando.
- **A edição do `setup.go` é marcada.** A linha entra depois de um `// trilha:add <receita>`, e
  a marca é o que faz a segunda execução reconhecer que já está lá em vez de duplicar.
- **`--dry-run` mostra e não escreve.** É o que se roda antes de deixar um comando mexer num
  projeto que já tem código.
- **Não há porta de "requisitos".** A spec previa uma, e ao escrever as três receitas nenhuma
  precisou de nada: o campo teria consumidor nenhum, e campo sem consumidor é promessa que
  ninguém pediu — a mesma regra que segurou o `LookupEnum` até a 0.66.0. Ele chega com a
  primeira receita que de fato dependa de algo, e é lá que será escrito e testado.
- **`--list --json`** para o `mcp` e para o agente do editor.

E a regra que decide se isto envelhece bem: **cada receita é aplicada num projeto novo e passa
pelo `trilha check`, no CI**. Uma receita que quebrou em silêncio é pior que nenhuma receita,
porque quem a rodou já está com o código dela dentro do projeto.

## Fora de escopo

- **As outras receitas** que a issue lista — `login`, `share-link`, `webhooks`, `mail`, `blob`,
  `tasks`, `permissions`, `tenant`. Esta versão traz o mecanismo e três receitas; cada uma das
  outras é meia hora depois de o mecanismo existir, e nenhuma delas ensina nada de novo sobre
  ele. O `login` em particular é a maior e a que mais depende de escolhas do projeto.
- **Receita como módulo Go próprio, com `go.mod` e `replace`**, como a issue propõe. O objetivo
  ali é "o CI compila a receita"; aplicá-la num projeto de verdade e rodar `trilha check` prova
  a mesma coisa com mais fidelidade — é no projeto que ela precisa funcionar — e sem quatro
  módulos a mais no repositório.
- **A página de doc gerada da receita.** Cada receita aponta para a página que já existe; gerar
  uma por receita vale quando houver dez, não três.
- **O cenário do `bench/agent`** ("acrescente chaves de API"). Ele mede o que esta spec entrega,
  então vem depois dela.
- **Editar o `layout.go`** para pôr o item no menu. Um `ui.Shell` tem formas demais para uma
  inserção marcada acertar; o comando diz qual link acrescentar, e a pessoa acrescenta.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `text/template`, `go/format` |
| VI — teste primeiro | e2e que aplica cada receita num projeto novo e roda `trilha check` |
| VII — segurança por padrão | as receitas usam o que o framework já protege: `trilha.Secret` na chave, `Require` na tela |

## Tarefas

- [x] T001 Teste que falha: `add audit` num projeto novo escreve os arquivos e liga o sink
- [x] T002 O mecanismo: registro, escrita, pular existente, inserção marcada
- [x] T003 As receitas `audit`, `settings` e `api-keys`
- [x] T004 `trilha add`, `--list --json`, `--dry-run`
- [x] T005 e2e: cada receita num projeto novo, `trilha check` verde
- [x] T006 AGENTS.md e a referência do `cli` en + pt
- [x] T007 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.69.0`

## Aceitação

- **SC-001** `trilha add` lista as receitas com uma linha cada; `--list --json` devolve o mesmo
  em JSON.
- **SC-002** `trilha add <receita>` num projeto novo escreve os arquivos, edita o `setup.go` uma
  vez, e o `trilha check` fica verde sem edição.
- **SC-003** Rodar de novo não duplica nem sobrescreve: os arquivos são pulados com aviso e a
  linha marcada não entra duas vezes.
- **SC-004** `--dry-run` mostra o que faria e não escreve nada.
- **SC-005** Três receitas no mesmo projeto convivem: um bloco de imports, três marcas, e o
  `trilha check` verde depois de todas.
