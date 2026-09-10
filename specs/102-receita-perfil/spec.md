# Spec 102 — a receita `profile`: a própria conta, e só ela

- **Issues**: [#116](https://github.com/emersonjoe/trilha/issues/116) e
  [#117](https://github.com/emersonjoe/trilha/issues/117) — as issues são a fonte do escopo.
- **Branch**: `102-receita-perfil`
- **Versão**: 0.82.0

## Por quê

Terceira das quatro telas da #117. É a menor e a que mais aparece: toda aplicação com login tem
uma tela onde a pessoa muda o próprio nome e a própria senha, e é a tela onde mais se escreve o
mesmo bug — o formulário manda o `id`, e o servidor confia nele.

## O que muda

```
$ trilha add profile
  + internal/usuarios/perfil.go       trocar o nome, trocar a senha
  + internal/usuarios/perfil_test.go
  + app/perfil/page.go                a tela: nome, e a troca de senha
  + app/perfil/middleware.go          exige sessão
  + perfil_test.go
```

Três decisões, e as três são sobre a mesma coisa — **de quem é a conta que está sendo mudada**:

- **O id vem da sessão, nunca do formulário.** Não existe campo `id` na tela. Um formulário que
  manda o id é um formulário que alguém edita, e aí a tela de perfil vira a tela de perfil dos
  outros.
- **Trocar a senha exige a senha atual**, mesmo com a sessão aberta. Um computador destravado por
  dois minutos não deve virar uma conta perdida — e a senha atual é o que o dono sabe e quem
  passou ali não.
- **Trocar a senha fecha a sessão e manda entrar de novo.** Quem troca a senha está, metade das
  vezes, dizendo que alguém a tinha — e entrar de novo é a prova de que a senha nova é a que ela
  quis. Encerrar as **outras** sessões pede um store que saiba achá-las pelo dono, e o de memória
  não sabe; o arquivo diz isso onde a linha entraria.

## Fora de escopo

- **Trocar o e-mail.** É outro fluxo: sem confirmar no endereço novo, trocar e-mail é trocar de
  dono. Vale quando a receita `mail` existir.
- **Foto, fuso, idioma.** Preferências são do app, não do framework, e cada uma pede um lugar para
  guardar que a receita não conhece.
- **Segundo fator.** É uma receita própria, e não uma linha nesta.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `auth` e `ui`, que já são do repositório |
| VI — teste primeiro | dois testes escritos pela receita, e o e2e da CI aplicando-a |
| VII — segurança por padrão | o id vem da sessão; a troca pede a senha atual e derruba as outras sessões |

## Tarefas

- [x] T001 Teste que falha: `add profile` sem o `login` recusa; com ele escreve os cinco
- [x] T002 A receita: perfil, tela, middleware, testes
- [x] T003 O e2e da CI aplicando `profile` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.82.0`

## Aceitação

- **SC-001** `trilha add profile` sem o `login` recusa e diz o que rodar antes.
- **SC-002** Com o `login`, o projeto passa `trilha check` sem uma edição.
- **SC-003** A tela muda o nome de quem está logado, e não há campo de id em lugar nenhum.
- **SC-004** A troca de senha com a senha atual errada é recusada com a mensagem no campo.
- **SC-005** Depois de trocar a senha, a sessão acaba, a senha nova entra e a antiga não.
