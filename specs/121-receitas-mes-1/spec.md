# Spec 121 — As três telas do mês 1: tenant, permissions e profile inteiras

- **Issue**: [#154](https://github.com/emersonjoe/trilha/issues/154) — a issue é a fonte do escopo.
- **Branch**: `121-receitas-mes-1`
- **Versão**: 0.100.0

## Por quê

A issue foi escrita na 0.78, quando só `users` existia. Desde então `permissions` (spec 101),
`profile` (102) e `tenant` (108) chegaram — cada uma com a metade que cabia numa spec curta. O
que a issue mede no Acervo e ainda não está em nenhuma delas: criar e desativar organização e
configurá-la; criar e apagar papel, ver quem o tem e o que **eu** posso; ver as sessões abertas,
encerrar as outras, e trocar o e-mail sem trocar de dono. E o template `app` continua sem as três.

## O que muda

**Núcleo — `auth`.** O `Store` de sessão só sabe `Save/Load/Delete`; listar as sessões de uma
pessoa é o que "sessões abertas" e "troca de senha derruba as outras" precisam.

- `auth.SessionLister` (interface opcional do `Store`): `Sessions(subject) []*User`. O
  `MemoryStore` implementa.
- `Auth.Sessions(c) ([]User, error)`: as sessões de quem está logado, a atual primeiro; sem um
  store que liste, `ErrNoSessionList`.
- `Auth.LogoutOthers(c) error`: apaga as outras sessões da mesma pessoa e audita
  `auth.logout_others`. Sem `Store`, o mesmo erro — a sessão no cookie não tem "as outras".

**Motor de receitas — `Insert.If`.** Uma linha do `setup.go` que só entra quando um arquivo
existe. As duas receitas envolvidas carregam a mesma linha com o mesmo marcador, cada uma
condicionada ao arquivo da outra, então a ligação acontece em qualquer ordem e uma vez só.

**`tenant`** ganha: `Criar(nome)`, `Ativar/Desativar(id)` (organização inativa não recebe
`SwitchTenant`), `Membros(org)`; na tela, o formulário de criar, o botão de ativar/desativar com
`ui.Confirm`, a contagem de membros, e a configuração da organização atual —
`store.Config(org)` é um `trilha.Settings[Configuracao]` por organização, ligado ao `Store.Configs`
sob demanda, desenhado por `ui.SettingsForm`. Tudo por `_action` no mesmo `POST`.

**`permissions`** ganha: `acesso.Papeis()` (a lista viva), `Criar(nome)` (nasce como `leitor`),
`Apagar(nome)` (recusado enquanto alguém o tem — a tabela de `login`); na tela, "quem tem este
papel" por papel e a seção "O que eu posso" com o nível de quem está logado por módulo. Com
`users` presente, `usuarios.Papeis = acesso.Papeis` no `setup.go` (`Insert.If`): o select de
papel passa a ser a matriz. `usuarios.Papeis` vira função por isso.

**`profile`** ganha: a lista de sessões abertas com "encerrar as outras"; a troca de senha
encerra as outras; a troca de e-mail em dois passos — `c.Link("email", …, Uses: 1)` enviado
por `usuarios.EnviarConfirmacao`, e `{{.At}}perfil/email/[token]` faz o `Claim` e só então
`TrocarEmail`. Sem quem envie, o pedido é recusado com a mensagem dizendo o que ligar; a
receita `mail` ganha `correio.Confirmacao(ctx, para, link)` e o `Insert.If` liga as duas.

**Template `app`** passa a incluir `users`, `permissions`, `profile` e `tenant`; `profile` e
`tenant` ficam em `app/` (são de qualquer logado), o resto em `app/admin/`.

## Fora de escopo

- **Idioma e fuso por pessoa.** A sessão é montada pelo `Entrar` da receita `login`, e o
  `Locale` é do app; é uma peça do núcleo (sessão que carrega preferência) antes de ser uma
  linha do perfil. Ficou fora na 102 pelo mesmo motivo.
- **Renomear papel.** É apagar e criar mais uma cascata na tabela de pessoas; apagar recusa
  enquanto alguém o tem, e isso já protege o que renomear protegeria.
- **Isolamento por tenant em `audit`/`api-keys`/`users`.** As stores dessas receitas não têm
  coluna de tenant; a spec 108 decidiu que o WHERE é do app. Não é uma tela a mais.
- **`generate page`** não existe; o `generate crud` já herda o `middleware.go` da pasta.
- **`RequireTenant` no `app/middleware.go`.** Receita só edita `setup.go`; o `Next` diz onde pôr.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo |
| VI — teste primeiro | `TestSessions*` no `auth`, `TestInsertIf` nas receitas, testes escritos por cada receita, `TestAddE2E` e o e2e do template |
| VII — segurança por padrão | e-mail só muda após o `Claim`; senha nova derruba as outras sessões; apagar papel em uso é recusado; organização inativa não recebe troca |
| Docs EN/PT no mesmo commit | referência `auth`, `cli`; receitas |

## Tarefas

- [x] T001 Testes que falham: `auth.Sessions`/`LogoutOthers`; `Insert.If`
- [x] T002 Núcleo: `SessionLister`, `MemoryStore.Sessions`, `Auth.Sessions`, `Auth.LogoutOthers`; `Insert.If`
- [x] T003 Receita `tenant`: criar, ativar/desativar, membros, configuração por organização
- [x] T004 Receita `permissions`: papéis, quem tem, o que eu posso; `users.Papeis` como função + ligação
- [x] T005 Receita `profile`: sessões, senha derruba, e-mail por link; `mail.Confirmacao` + ligação
- [x] T006 Template `app` com as quatro; `recipeDir`; e2e
- [x] T007 `api/current.txt`, docs EN + PT (`auth`, `cli`, recipes)
- [x] T008 `CHANGELOG.md`, `ROADMAP.md` (89 fechado), `version`, `make test`, `scripts/release.sh 0.100.0 --issues 154`

## Aceitação

- **SC-001** Perfil: com duas sessões da mesma pessoa, "encerrar as outras" e a troca de senha
  deixam só a atual; a outra recebe 401 na próxima requisição.
- **SC-002** Perfil: `POST` com e-mail novo não muda nada; abrir o link muda; abrir de novo dá 404.
- **SC-003** Permissões: criar papel aparece na grade e no select de `users`; apagar papel com
  alguém nele é recusado com a mensagem; "o que eu posso" mostra o nível do logado.
- **SC-004** Organizações: criar, desativar (troca recusada), reativar; a configuração salva de
  A não aparece em B.
- **SC-005** `trilha new --template app` sem `--with` escreve as oito receitas e `trilha check`
  fica verde; `TestAddE2E` continua verde em qualquer ordem.
