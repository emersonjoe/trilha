# Spec 101 — a receita `permissions`: a matriz, e quem pode mexer nela

- **Issues**: [#116](https://github.com/emersonjoe/trilha/issues/116) e
  [#117](https://github.com/emersonjoe/trilha/issues/117) — as issues são a fonte do escopo.
- **Branch**: `101-receita-permissions`
- **Versão**: 0.81.0

## Por quê

Das quatro telas que faltavam na #117, a de usuários saiu na 0.76.0 e as outras três esperavam por
ela. A de permissões é a próxima, e é a que muda mais coisa no app com menos código: quando existe
uma matriz, o `if papel == "admin"` espalhado por dezoito arquivos deixa de ser escrito.

A `auth.Policy` e o `ui.PolicyGrid` existem desde a Fase 5. O que não existe é o caminho de quem
tem um projeto e não sabe que existem — e é isso que uma receita é.

## O que muda

```
$ trilha add permissions
  + internal/acesso/acesso.go        a matriz como dado, e as três funções que a leem
  + internal/acesso/acesso_test.go
  + app/permissoes/page.go           o grid, e o POST que salva
  + app/permissoes/middleware.go     guardado pelo próprio módulo que administra
  + permissoes_test.go
```

A matriz é **dado declarado num arquivo**, não uma tabela: módulos, níveis e o que cada papel tem
em cada módulo. Três funções a leem — `Exige` para o middleware, `Pode` para a página que decide
se desenha um botão, e `Salvar` para a tela que a edita.

Duas decisões que a receita escreve junto, porque são as que se erra:

- **A tela que edita a matriz é guardada pela própria matriz** — `Exige("usuarios",
  "administrar")` — e não por um papel escrito na mão. Uma tela de permissões atrás de um
  `if papel == "admin"` é uma matriz com uma exceção do lado de fora.
- **Esconder botão é cosmético.** O `Pode` existe para a página não desenhar o que a pessoa não
  pode fazer, e o comentário no arquivo diz, onde ele é usado, que quem recusa é o middleware.

Depende da receita `login`, como a `users`: é a sessão que diz qual papel a pessoa tem.

## Fora de escopo

- **Guardar a matriz num banco.** Ela vive em memória, como todo store destas receitas, com o
  comentário dizendo onde entra a tabela. A troca é `auth.PolicyStore`, e é do app.
- **Permissão por registro** (esta pessoa neste documento). É outra coisa, e não é uma matriz.
- **`organizacoes` e `perfil`.** Continuam na #117.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `auth` e `ui`, que já são do repositório |
| VI — teste primeiro | a receita escreve dois testes, e o e2e da CI aplica ela num projeto novo |
| VII — segurança por padrão | a tela que edita a matriz é guardada pela matriz |

## Tarefas

- [x] T001 Teste que falha: `add permissions` sem o `login` recusa; com ele escreve os cinco
- [x] T002 A receita: matriz, tela, middleware, testes
- [x] T003 O e2e da CI aplicando `permissions` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.81.0`

## Aceitação

- **SC-001** `trilha add permissions` sem o `login` recusa e diz o que rodar antes.
- **SC-002** Com o `login`, o projeto passa `trilha check` sem uma edição.
- **SC-003** A tela responde 403 para quem entrou sem o nível, e não desvio para o login.
- **SC-004** O que o grid posta muda a matriz, e o próximo pedido já obedece.
- **SC-005** A mudança da matriz entra na trilha de auditoria.
