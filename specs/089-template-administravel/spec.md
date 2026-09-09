# Spec 089 — o template app administrável, feito das receitas

- **Issue**: [#117](https://github.com/emersonjoe/trilha/issues/117) — a issue é a fonte do escopo.
- **Branch**: `089-template-administravel`
- **Versão**: 0.70.0

## Por quê

O `trilha new --template app` entrega um bom dia 1: login, shell, dashboard, um CRUD. A issue
mede o que toda aplicação interna tem no **mês 1** e o template não — usuários, permissões,
chaves, auditoria, configurações, perfil — e conta as semanas que o iniciante gasta fazendo cada
uma diferente.

A própria issue diz como: *"o template passa a ser o resultado de `trilha add` sobre o esqueleto
atual"*. Isso deixou de ser uma ideia na 0.69.0 — o `trilha add` existe, com três receitas. Esta
spec faz a junção, que é a metade do trabalho com nenhuma duplicação: as telas do template são as
mesmas telas que o `add` escreve, de uma fonte só.

## O que muda

```
$ trilha new minha-app --template app
  …
  + app/admin/middleware.go        RequireRole("admin")
  + app/admin/auditoria/page.go    a receita audit
  + app/admin/chaves/page.go       a receita api-keys
  + app/admin/config/page.go       a receita settings
  + internal/auditoria/store.go
  + internal/config/config.go
  ~ app/setup.go                   as três linhas, marcadas
```

Três coisas, e as três pequenas porque o mecanismo já existe:

- **As receitas ganham um lugar.** `Options.At` diz sob qual pasta as telas caem; o padrão
  continua `app/`, e o template pede `app/admin/`. Os arquivos de `internal/` não se movem — eles
  não são tela.
- **`trilha new --with audit,api-keys,settings`** aplica receitas na criação, em qualquer
  template. É o que o `--template app` passa a fazer por padrão, e é o que qualquer pessoa pode
  pedir para o `--template blog`.
- **`app/admin/` é guardado por papel.** As três telas nomeiam pessoas, emitem credenciais e
  mudam o comportamento da aplicação para todo mundo — deixá-las atrás do mesmo login que a
  listagem de itens seria entregar a chave junto com a porta.

O que **não** muda: o esqueleto continua o que era. Quem quer o template sem as telas de
administração pede `--with ""`.

## Fora de escopo

A issue lista sete telas; esta versão entrega as três que já são receitas, e o caminho para as
outras é escrever mais receitas — não mais template. Ficam para specs próprias, e a issue segue
aberta:

- **`usuarios`** (convidar, papel, ativar/desativar, reset de senha). É a maior, e é uma receita:
  precisa do `mail` (#113) ou do `c.Link` (#106) para o convite, e de uma tabela de usuários que
  hoje o template tem em memória.
- **`permissoes`** com o `ui.PolicyGrid`, **`organizacoes`** com o tenant, e **`perfil`**. As
  três são receitas de meia hora depois que a de usuários existir, porque as quatro compartilham
  a mesma tabela.
- **`--create-admin`** e o store em SQL com `migrations/001_init.sql`. Os dois pertencem à
  receita de usuários: é ela que tem uma tabela de gente para semear e migrar.
- **`--tenant`** no `new`.
- **A doc com captura de cada tela.** As capturas envelhecem sozinhas; a lista do que vem no
  template é texto e está na doc do `cli`.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo; é o mecanismo da 0.69.0 com um prefixo |
| VI — teste primeiro | o e2e do template já roda `gen --check`, `vet` e `test`, agora com as três telas dentro |
| VII — segurança por padrão | `app/admin/` exige papel, e as três telas dizem por quê no próprio arquivo |
| Uma fonte | as telas do template **são** as receitas; não há uma segunda cópia para envelhecer |

## Tarefas

- [x] T001 Teste que falha: uma receita aplicada com `At` cai sob a pasta pedida
- [x] T002 `Options.At` nas receitas, com os arquivos de `internal/` parados no lugar
- [x] T003 `trilha new --with`, e o `--template app` pedindo as três
- [x] T004 `app/admin/middleware.go` no template
- [x] T005 e2e: o template novo passa `gen --check`, `vet`, `test` e `audit`
- [x] T006 Documentação: o que vem no template, en + pt
- [x] T007 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.70.0`

## Aceitação

- **SC-001** `trilha new x --template app` traz as três telas sob `app/admin/`, e o projeto passa
  no `trilha check` sem edição.
- **SC-002** `app/admin/` responde 403 para quem está logado sem o papel, e não 302 para o login:
  a pessoa é conhecida, só não autorizada.
- **SC-003** `--with ""` devolve o template como ele era.
- **SC-004** `--with audit` funciona em `--template blog` também.
- **SC-005** As telas do template são exatamente o que o `trilha add` escreve — uma fonte, sem
  cópia no `templates/`.
