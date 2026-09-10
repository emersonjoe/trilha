# Spec 096 — a receita `users`, e o `Requires` que ela estreia

- **Issues**: [#116](https://github.com/emersonjoe/trilha/issues/116) e
  [#117](https://github.com/emersonjoe/trilha/issues/117) — as issues são a fonte do escopo.
- **Branch**: `096-receita-users`
- **Versão**: 0.76.0

## Por quê

A #117 lista sete telas do "mês 1" e três já existem como receita. Das quatro que faltam,
**usuários é a que as outras três esperam** — permissões, organizações e perfil são todas sobre a
mesma tabela de gente. A 0.75.0 escreveu essa tabela; falta a tela.

É também a tela que ninguém escreve bem na primeira vez: convite com token que expira, senha que
só a pessoa define, papel que muda, conta que se desativa em vez de se apagar. Cada um desses é
um detalhe que, esquecido, vira incidente.

## O que muda

```
$ trilha add users
  + internal/usuarios/convites.go     convite, papel, ativar, resetar
  + internal/usuarios/convites_test.go
  + app/usuarios/page.go              a tela: lista, convidar, papel, ativar/desativar, resetar
  + app/usuarios/middleware.go        RequireRole("admin")
  + app/convite/token_/page.go        onde a pessoa define a própria senha
```

**O `Requires` estreia aqui**, porque agora existe uma receita que depende de outra:
`users` precisa do `login` — a tabela, a sessão e a tela de entrar. Sem ele, `trilha add users`
recusa e diz o que rodar antes, em vez de escrever cinco arquivos que não compilam.

```
$ trilha add users
  error: this recipe needs `trilha add login` first (internal/usuarios/usuarios.go is not there)
```

O que a tela faz, e o que cada decisão evita:

- **Convidar é criar a linha sem senha.** Quem define a senha é a pessoa, no link do convite —
  um administrador que escolhe a senha de alguém é um administrador que sabe a senha de alguém.
- **O token do convite fica guardado como hash**, com validade de 48 horas e uso único. Um dump
  da memória (ou, amanhã, da tabela) não é um molho de chaves.
- **Desativar, não apagar.** A trilha de auditoria aponta para quem fez o quê, e uma linha
  apagada deixa a trilha falando de um id que não existe mais.
- **Resetar senha é emitir outro convite**, com o mesmo caminho e a mesma validade: um jeito só de
  uma senha nascer.
- **A pasta exige o papel `admin`**, e quem está logado sem ele leva 403 e não um desvio para o
  login — a pessoa é conhecida, só não autorizada.

O link do convite aparece na tela para quem acabou de convidar, porque é o administrador que já
tem o direito de criá-lo. Mandá-lo por e-mail é a linha do `mail` que o `Next` imprime.

## Fora de escopo

- **Store em SQL e migrações.** O padrão do repositório: interface e memória aqui, SQL na receita
  do cookbook.
- **`--tenant`, permissões e perfil.** São as outras três telas da #117, e cada uma é uma receita
  em cima desta tabela — que é o motivo de esta vir primeiro.
- **Mandar o convite por e-mail.** O `mail` existe desde a 0.63.0, mas quem manda é o app: a
  receita não decide o remetente, o assunto nem o idioma de ninguém.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `crypto/rand`, `crypto/sha256`, `auth` e `ui` |
| VI — teste primeiro | a receita escreve dois testes, e o e2e da CI aplica ela num projeto novo |
| VII — segurança por padrão | token com hash, validade e uso único; a pasta exige papel |

## Tarefas

- [x] T001 Teste que falha: `add users` sem o `login` recusa e diz o que rodar antes
- [x] T002 O `Requires`, com a mensagem em en e pt
- [x] T003 A receita: convites, tela, middleware, aceitar o convite, testes
- [x] T004 O e2e da CI aplicando `login` e `users` no mesmo projeto
- [x] T005 Documentação (en + pt)
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.76.0`

## Aceitação

- **SC-001** `trilha add users` sem o `login` recusa, nomeia o que falta e não escreve nada.
- **SC-002** Com o `login`, os dois juntos passam `trilha check` sem uma edição.
- **SC-003** Convidar cria a pessoa inativa e sem senha; o link define a senha e ativa.
- **SC-004** O token vale uma vez, expira, e o que fica guardado é o hash dele.
- **SC-005** Desativar impede a entrada e mantém a linha.
