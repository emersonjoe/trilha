# Spec 038 — Cookbook, checklist de produção e guia de migração

- **Issue**: [#38](https://github.com/emersonjoe/trilha/issues/38) — a issue é a fonte do escopo.
- **Branch**: `038-cookbook`
- **Versão**: 0.29.0

## Por quê

O site ensina o framework ("Aprender") e descreve cada símbolo ("Referência"). Quem vai
colocar um app no ar procura uma terceira coisa: a receita pronta do que todo app faz e o
framework não decide por você — abrir o banco, guardar sessão, receber upload, paginar, mandar
e-mail, rodar tarefa periódica, empacotar em Docker. Hoje a resposta está espalhada entre um
parágrafo do capítulo de dados, um exemplo e o código do `examples/blog`.

Faltam também dois documentos que ninguém escreve na hora certa: a lista do que conferir antes
de publicar, e o caminho de quem já tem um `net/http` e quer trazer para cá sem reescrever
tudo de uma vez.

O risco de um cookbook é conhecido: receita que não compila. Um trecho errado na documentação
custa mais caro que a ausência dele, porque quem copia confia.

## O que muda

Uma terceira seção no site, em pé de igualdade com "Aprender" e "Referência":

| Página | O que responde |
|---|---|
| índice | o que é o cookbook e de onde vem o código |
| `database` | `database/sql` com Postgres e SQLite, pool, migração, sqlc, repositório no `setup.go` |
| `sessions` | sessão em cookie assinado, login, usuário atual no middleware, flash |
| `uploads` | `multipart`, limite de corpo, validação de tipo, gravação e entrega |
| `pagination` | página e cursor, `LIMIT`/`OFFSET`, o rodapé de páginas em `h` |
| `email` | `net/smtp`, remetente injetado em `Values`, corpo com `tmpl`, log em dev |
| `scheduled-tasks` | tarefa periódica iniciada no `setup.go` e encerrada no `Shutdown` |
| `docker` | `Dockerfile` distroless, variáveis, sonda de saúde, `compose` |
| `production-checklist` | o que conferir antes de publicar, em ordem |
| `migration` | `net/http` puro → Trilha, rota a rota; e o que muda entre versões menores |

As páginas ficam em `content/en/cookbook/` e `content/pt/receitas/`, servidas em `/cookbook/…`
e `/pt/receitas/…`, com o mesmo paralelismo de slugs que o site já exige entre locales.

O código das receitas mora em `examples/cookbook/`, um pacote Go que compila com o resto do
repositório (`go vet ./...` e `go test ./...` passam por ele) e só usa a biblioteca padrão. A
documentação cita esses arquivos, e um teste do site garante que cada bloco ```go` das páginas
do cookbook aparece **literalmente** em algum `.go` do repositório: receita que não compila
não chega à página, porque o bloco deixaria de casar com a fonte.

## Fora de escopo

- **Driver de banco de verdade nas receitas.** O pacote usa `database/sql` sem registrar
  driver: importar `pgx` ou `sqlite` traria dependência externa ao repositório. A linha do
  import do driver aparece na página como texto, não como código verificado.
- **Uma quarta seção "Deploy" por provedor.** O `docker` cobre o que é do framework; o resto é
  documentação de quem hospeda.
- **Guia de migração de outros frameworks** (Echo, Gin, Fiber). O caminho descrito é o de
  `net/http`, que é o denominador comum deles.
- **Traduzir o `examples/cookbook`.** Código e comentários em inglês, como o resto do repositório.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | O pacote das receitas importa `database/sql`, `net/smtp`, `mime/multipart` e o próprio framework. |
| III — documentação é parte da entrega | As dez páginas saem nas duas línguas no mesmo commit. |
| VI — teste primeiro | O teste que casa bloco com fonte vem antes das páginas. |
| VII — segurança por padrão | Upload valida tipo e tamanho; sessão usa cookie assinado; o checklist é a lista do `trilha audit` mais o que ele não vê. |

## Aceitação

- **SC-001** Cada uma das dez páginas existe nas duas locales, com o mesmo slug na posição
  correspondente (`TestLocalesInSync`).
- **SC-002** Todo bloco ```go` das páginas do cookbook aparece literalmente em um `.go` do
  repositório; um trecho inventado quebra o teste.
- **SC-003** `examples/cookbook` compila no `go vet ./...` e não importa nada fora da
  biblioteca padrão e do próprio framework (`TestNoExternalDeps` continua verde).
- **SC-004** As páginas respondem, entram no export e aparecem na navegação das duas locales
  (`TestEveryPageResponds`, `TestExportPathsCoverEveryPage`).
- **SC-005** O checklist de produção cita `trilha audit` e cada item tem como ser conferido.
