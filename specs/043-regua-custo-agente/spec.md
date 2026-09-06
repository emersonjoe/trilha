# Spec 043 — Régua: custo por feature de um agente

- **Issue**: #45 — a issue é a fonte do escopo (quatro cenários, três execuções, mediana,
  Trilha antes × Trilha depois); esta spec só fixa o contrato do harness.
- **Branch**: `043-regua-custo-agente`
- **Versão**: 0.33.0 (a 0.32.0 é da spec 042, na outra sessão)

## Por quê

A Fase 5 promete que uma ferramenta de IA gasta menos token para entregar uma feature num
projeto Trilha. Sem número isso é opinião: cada item seguinte (#46 a #50) precisa provar que
baixou o custo, e "baixou" só existe contra uma medição anterior feita do mesmo jeito.

Hoje não há como medir. O `bench/` mede o custo por requisição do framework em processo;
nada mede o custo de *usar* o framework — abrir arquivos para descobrir o que existe,
errar uma assinatura, rodar cinco verificações separadas.

## O que muda

Um programa em `bench/agent/` (pacote `main` dentro do módulo `bench`, que já é separado do
runtime) que:

1. monta uma cópia de `examples/blog` ou `examples/sso` num diretório temporário, com
   `go.mod` próprio e `replace` para o repositório, e a CLI `trilha` compilada no `PATH`;
2. roda um agente de linha de comando (`claude -p`, por padrão) com uma frase de tarefa, cwd
   na cópia, sem MCP, sem plugins, sem memória do usuário (`--setting-sources project`, para
   que só o que estiver **no projeto** — o futuro `AGENTS.md` da #46 — conte);
3. copia um teste escondido para a cópia e roda `go vet ./...` e `go test ./...`: passou ou
   não passou;
4. lê do JSON que o agente devolve os tokens de entrada (novos e lidos do cache), os de
   saída, as rodadas, o tempo, o custo e o modelo;
5. grava `bench/agent/results.json` e renderiza `bench/agent/RESULTS.md` com a mediana de
   três execuções por cenário.

```
make bench-agent            # 4 cenários × 3 execuções; exige `claude auth login` antes
make bench-agent-dry        # monta os cenários e prova que o teste escondido falha sem o agente
cd bench && go run ./agent -scenario comments -runs 1 -keep
```

Os cenários, em uma frase cada (o prompt exato está em `scenario.go`, é parte do contrato e
não muda sem mudar a versão):

| Nome | Fixture | Tarefa | Teste escondido prova |
|---|---|---|---|
| `comments` | blog | `POST`/`GET /api/posts/{id}/comments` com `Bind`, validação, 404 | 201 com o comentário, 422 no inválido, 404 no post inexistente, `GET` lista |
| `contact-form` | blog | página `/contato` no layout raiz com formulário do kit `ui` | 200 com `<form` dentro do layout, `POST` válido não é erro, inválido é 422 |
| `cognito` | sso (só Keycloak) | trocar o provedor para Cognito lendo `SSO_REGION`, `SSO_USER_POOL_ID`, `SSO_LOGOUT_DOMAIN` | `Configure()` monta o emissor do Cognito e o domínio de logout; `auth.Keycloak(` some da fonte |
| `pagination` | blog | `/blog` com 5 posts por página, `?page=N`, anterior/próxima | páginas 1 e 3 listam 5 e 4 posts; página 2 aponta para 1 e 3 |

O CI **não** roda o agente. O que roda em `go test ./...` do módulo `bench` é o harness
sem agente: leitura do JSON, mediana, tabela, e a prova de que cada teste escondido **falha**
na fixture intocada (um teste que passa antes do agente não mede nada).

## Fora de escopo

- Números contra outro framework: decisão da spec 011; a comparação é Trilha antes × depois.
- Outros agentes além de um comando que devolva o mesmo JSON (`-agent` troca o binário; o
  formato do resultado é o do Claude Code). Adaptar outro formato é uma spec quando houver
  um segundo agente com formato estável.
- Rodar no CI: precisa de conta e gasta dinheiro; o `RESULTS.md` é regravado à mão, como o
  do `bench/`.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o harness usa `os/exec`, `encoding/json`, `testing`; nada entra no runtime nem na CLI |
| VI — teste primeiro | os testes do harness e a prova de que as fixtures falham vêm antes do harness rodar de verdade |

## Tarefas

- [x] T001 Teste que falha em `bench/agent/agent_test.go`: `ParseResult` lê o JSON do
      `claude -p` (sucesso e erro de autenticação); `Median` de três e de dois; `Render` monta
      a tabela com a mediana e `n/3` passou.
- [x] T002 `bench/agent/claude.go`, `report.go`: leitura do resultado, mediana, tabela.
- [x] T003 Teste que falha: para cada cenário, `Build` monta a fixture, `go vet` passa nela e
      `Verify` **falha** sem o agente.
- [x] T004 `bench/agent/scenario.go`, `fixture.go`, `main.go`: os quatro cenários, a cópia com
      `go.mod`+`replace`, a CLI no `PATH`, o teste escondido, o comando.
- [x] T005 `Makefile` (`bench-agent`, `bench-agent-dry`), `bench/agent/RESULTS.md` com a
      metodologia e a tabela.
- [x] T006 Docs: seção "Custo por feature para um agente" em `reference/performance` e
      `referencia/desempenho`.
- [x] T007 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`.
- [ ] T008 Primeira medição (exige `claude auth login` na máquina): `make bench-agent`, commit
      de `results.json` + `RESULTS.md`, item 23 do `ROADMAP.md`, `make release VERSION=0.33.0 ISSUES="45"`.

## Aceitação

- **SC-001** `cd bench && go test ./...` verde sem conta nenhuma e sem `claude` no `PATH`.
- **SC-002** `make bench-agent-dry` mostra os quatro cenários com `verify: FAIL (expected)`.
- **SC-003** `RESULTS.md` traz metodologia, versão do Trilha, agente e modelo, e a tabela com
  três execuções por cenário — preenchida na T008.
- **SC-004** Nenhum arquivo fora de `bench/`, `Makefile`, docs, `CHANGELOG.md`,
  `cmd/trilha/main.go` (versão) e `ROADMAP.md` muda.
