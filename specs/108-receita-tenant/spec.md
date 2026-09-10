# Spec 108 — a receita `tenant`: a coluna que não pode faltar

- **Issues**: [#116](https://github.com/emersonjoe/trilha/issues/116) e
  [#117](https://github.com/emersonjoe/trilha/issues/117) — as issues são a fonte do escopo.
- **Branch**: `108-receita-tenant`
- **Versão**: 0.88.0

## Por quê

Uma coluna é a forma mais comum de multi-tenant, e esquecer essa coluna numa consulta é o bug mais
comum de multi-tenant: o relatório que mostra as linhas de outro cliente, descoberto pelo cliente.

O `auth.Tenant`, o `RequireTenant` e o `SwitchTenant` existem desde a Fase 6. Falta o que a #117
chama de tela de organizações: escolher em qual se está, trocar, e a linha que diz onde a coluna
entra.

## O que muda

```
$ trilha add tenant
  + internal/organizacoes/organizacoes.go     as organizações, e de quem é cada uma
  + internal/organizacoes/organizacoes_test.go
  + app/organizacoes/page.go                  escolher e trocar
  + app/organizacoes/middleware.go            exige sessão (e não exige tenant: é aqui que se escolhe)
  + organizacoes_test.go
```

Três decisões, e as três são a mesma frase dita de jeitos diferentes — **quem confere se a pessoa
pode entrar naquela organização é o app**:

- **A tela de escolher não pode exigir tenant.** É a tela onde ele é escolhido; guardá-la com
  `RequireTenant` é o laço que a #118 descreve para o login.
- **`SwitchTenant` não confere pertencimento**, e diz isso na doc do módulo. A receita escreve a
  checagem contra a tabela de organizações **antes** da troca, porque a versão sem checagem é uma
  URL que troca a organização de qualquer um para qualquer uma.
- **A troca é auditada dos dois lados** — de onde e para onde. Uma investigação que começa em
  "essa pessoa viu as linhas erradas" começa perguntando quando ela trocou.

E o arquivo diz, onde a consulta seria escrita, que a coluna vem do `auth.Tenant(c)` e que a
cláusula é do app: uma cláusula que o framework gerasse seria uma cláusula que ninguém lê na
revisão.

## Fora de escopo

- **Convidar alguém para uma organização.** É a receita `users` com uma coluna a mais, e ela já
  existe; juntar as duas é trabalho de quem tem as duas.
- **Um schema por tenant, ou um banco por tenant.** São outras formas, e a coluna é a comum.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `auth`, que já é do repositório |
| VI — teste primeiro | a receita escreve dois testes, e o e2e da CI aplica ela |
| VII — segurança por padrão | pertencimento conferido pelo app antes da troca, e troca auditada |

## Tarefas

- [x] T001 Teste que falha: `add tenant` escreve os cinco e liga o `Provide`
- [x] T002 A receita: organizações, tela, middleware, testes
- [x] T003 O e2e da CI aplicando `tenant` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.88.0`

## Aceitação

- **SC-001** `trilha add tenant` num projeto novo passa `trilha check` sem uma edição.
- **SC-002** Trocar para uma organização de que a pessoa participa muda o `auth.Tenant`.
- **SC-003** Trocar para uma de que ela não participa é recusado.
- **SC-004** A tela de escolher responde para quem está logado sem organização nenhuma.
