# Spec 037 — Inspetor de rotas no `trilha dev`

- **Issue**: [#37](https://github.com/emersonjoe/trilha/issues/37) — a issue é a fonte do escopo.
- **Branch**: `037-inspetor-de-rotas`
- **Versão**: 0.28.0

## Por quê

`trilha routes` responde "que rotas existem". As perguntas que aparecem quando alguma coisa
não responde o esperado são outras: *qual* rota atendeu `/blog/hoje` quando existem
`/blog/{slug}` e `/blog/hoje`, *quais* layouts embrulharam aquela página, *quais* middlewares
rodaram antes do handler. Hoje isso se descobre lendo `trilha_gen.go` — código gerado, que é
o pior lugar para procurar — ou colocando `fmt.Println` no layout para ver se ele executa.

A informação já existe no scanner: cadeia de layouts por rota (de dentro para fora), lista de
middlewares, tipo, métodos e a pasta de origem. Falta um lugar onde ela apareça junto, no
navegador que já está aberto, enquanto o dev server roda.

## O que muda

O `trilha dev` passa a servir uma página em `/_trilha/routes`, ao lado do `/_trilha/events`
que ele já intercepta. Quem serve é o supervisor (`internal/dev`), não o app: a página é
montada a partir de um `scan.Scan` feito na hora, então ela nunca entra no binário e não
existe fora do `trilha dev` — em produção o caminho é um 404 do app, como qualquer outro.

A página mostra, sem CSS externo e sem JavaScript:

- a tabela de rotas em ordem de precedência do `net/http` (a mais específica primeiro), com
  padrão, tipo (`page`/`api`), métodos, pasta de origem, layouts (de fora para dentro,
  incluindo o layout raiz) e middlewares;
- um formulário `GET` com um caminho: `/_trilha/routes?path=/blog/hoje` responde qual padrão
  vence, resolvido pelo mesmo `http.ServeMux` que o app usa — não por uma regra reescrita — e
  os valores dos parâmetros do padrão vencedor;
- o rodapé do app: layout raiz, `not_found.go`, `error.go` e `setup.go`, quando existem.

```text
$ trilha dev
→ http://localhost:3000
  rotas: http://localhost:3000/_trilha/routes
```

O `Server` ganha um campo:

```go
type Server struct {
	Root   string
	Module string // caminho do módulo, para o scan que monta o inspetor
	Addr   string
	// …
}
```

## Fora de escopo

- **Editar ou criar rotas pela página.** Inspetor é leitura; escrever é `trilha generate`.
- **Métricas, tempo de resposta, requisições recentes.** Isto é o mapa estático do app, não um
  profiler; o que é observabilidade já tem `/_trilha/metrics`.
- **Servir o inspetor a partir do app (`package trilha`) com um `if Dev`.** Uma flag pode ser
  ligada por engano em produção; um caminho que não existe no binário, não.
- **Autenticação na página.** Ela só existe no processo do `trilha dev`, que escuta o que o
  `--addr` mandar; quem expõe o dev server na rede já expõe o app inteiro.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — convenção sobre configuração | Nada novo em `app/`; a página lê o que o scanner já produz. |
| II — só biblioteca padrão | `net/http`, `html/template` e `internal/scan`. |
| VI — teste primeiro | Teste do handler antes do handler; teste de 404 no app antes de fechar. |
| VII — segurança por padrão | O inspetor não existe no binário de produção; o caminho `/_trilha/*` já é reservado pelo framework. |

## Tarefas

- [ ] T001 Teste que falha em `internal/dev/inspector_test.go`: a página lista os padrões de
      uma árvore sintética com layouts e middlewares, ordena por precedência, e
      `?path=/blog/hoje` aponta a rota literal em vez de `/blog/{slug}`; caminho sem rota diz
      que ninguém responde; a página escapa o que veio da URL.
- [ ] T002 `internal/dev/inspector.go`: o handler, a resolução por `http.ServeMux` e o
      template; `serveHTTP` intercepta `/_trilha/routes`; `Server.Module`.
- [ ] T003 Teste em `serve_test.go` (raiz): o app responde 404 em `/_trilha/routes` em `Dev` e
      em `Prod` — o inspetor nunca é do app.
- [ ] T004 `cmd/trilha/dev.go` passa `Module` e a linha do endereço no `Run` imprime o link.
- [ ] T005 Documentação nas duas locales: `learn/dev-and-production` e `reference/cli`.
- [ ] T006 `CHANGELOG.md` (0.28.0), `version` em `cmd/trilha/main.go`, item 19 do `ROADMAP.md`.
- [ ] T007 `make test` verde e `make release VERSION=0.28.0 ISSUES="37"`.

## Aceitação

- **SC-001** A página em `/_trilha/routes` lista todas as rotas do projeto com tipo, métodos,
  origem, layouts e middlewares, em ordem de precedência.
- **SC-002** `?path=X` responde com o padrão que atenderia `X` e com os parâmetros extraídos,
  usando o `http.ServeMux`; quando nada casa, diz que nada casa.
- **SC-003** `GET /_trilha/routes` no app (dev ou prod) é 404: o inspetor vive no supervisor.
- **SC-004** A página não carrega nada de fora: sem CSS, sem JS, sem fonte externa.
- **SC-005** `make test` verde, incluindo `TestNoExternalDeps`.
