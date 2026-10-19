# Spec 181 — Limite de tamanho no commit: 5.000 tokens por arquivo, 500 linhas de código

**Feature Branch**: `181-limite-de-tamanho` | **Created**: 2026-10-19 | **Status**: Pronta para release (0.161.0)

- **Issue**: #314 — a fonte do escopo.
- **Branch**: `181-limite-de-tamanho`
- **Versão**: 0.161.0

## Por quê

Num projeto desenvolvido com agentes de IA, os arquivos crescem até o agente não conseguir
lê-los, revisá-los ou mudá-los sem carregar boa parte do projeto no contexto. O `AGENTS.md` pede
arquivos pequenos (spec 179), mas nada barra o commit que quebra a regra; a revisão humana é a
única barreira, e ela chega tarde.

## Jornada e risco

Jornada de quem escreve o app com um agente. Pior impacto: o hook barrar um commit legítimo
(arquivo gerado, lockfile, binário tratado como código) ou deixar passar o que deveria parar.
Coberto por unidade (regras e exceções), por teste contra um repositório git real (índice ×
árvore de trabalho, `.gitattributes`) e por um e2e com `git commit` de verdade.

## O que muda

- **`trilha precommit [--all]`** — lê os arquivos staged do índice (`--all`: todos os
  versionados) e reprova: arquivo acima de 5.000 tokens (bytes/4, o `tokbudget` do framework);
  arquivo de código acima de 500 linhas. Isentos do limite de linhas: testes (`_test.go`,
  `.test.`, `.spec.`, `test_`, pastas `test`, `tests`, `__tests__`, `testdata`, `e2e`), docs,
  CSS e Markdown. Fora de qualquer limite (não escritos por agente): lockfiles, `vendor/`,
  `node_modules/`, binários, `.golden`, e gerados — cabeçalho `Code generated ... DO NOT EDIT.`
  em qualquer sintaxe de comentário, os carimbos do kit, do `AGENTS.md`, da skill e do hook, e
  `linguist-generated`/`linguist-vendored` no `.gitattributes` do índice. A mensagem lista cada
  arquivo com a contagem e o limite e diz como dividir, em inglês ou português (`TRILHA_LANG`).
- **`trilha hooks [--force]`** — grava `.githooks/pre-commit` (versionado, carimbado sob o
  shebang, mesma regra do `AGENTS.md`) e aponta o `core.hooksPath` para `.githooks`; se ele já
  aponta para outra pasta, fica como está e a CLI diz como encadear. O script roda o
  `trilha precommit` do `PATH` ou, sem ele, `go run` da versão do `go.mod`.
- **`trilha agents`** instala o hook; **`trilha new --agents`** grava o script (o repositório
  ainda não existe, então a CLI diz para rodar `trilha hooks` depois do `git init`).
- **`AGENTS.md`** (regra 9 e a tabela de portões) e a **skill `trilha-quality`** (seção 9 e
  "Antes de terminar") dizem os limites e mandam dividir antes de terminar.

## Fora de escopo

Aplicar os limites ao próprio repositório do framework (o pedido é para os projetos que o usam;
vários arquivos do núcleo passam de 500 linhas e dividi-los é uma spec própria); portão no
`trilha check` sobre o projeto inteiro (quebraria projetos existentes sem aviso — o `--all`
mede antes).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `os/exec` chamando o `git`, `regexp`; nenhuma dependência |
| IV — API pública | nada novo nos pacotes cobertos; `internal/precommit` e dois comandos na CLI |
| VI — teste primeiro | `TestCheckLimits`, `TestCheckIgnoresWhatAgentsDoNotWrite`, `TestStaged`, `TestWriteHooks`, `TestEnableHooks`, `TestPrecommitHookStopsTheCommit` |
| VII — segurança por padrão | o hook só lê o índice e escreve na saída; não executa o conteúdo dos arquivos |

## Segurança e privacidade

- **Fronteiras**: o hook roda na máquina de quem faz o commit, com os arquivos do próprio
  repositório; a CLI chama só `git` com argumentos fixos (nomes de arquivo depois de `--`).
- **NIST SSDF 1.1**: PW.7 (revisão: arquivos que cabem numa leitura), PO.3 (ferramenta que
  automatiza a prática). **OWASP**: nada exposto em rede.
- **Exceções**: nenhuma.

## Tarefas

- [x] T001 Testes: `internal/precommit` (limites, exceções, índice × árvore, `.gitattributes`),
  `internal/scaffold/hooks_test.go`, e2e `TestPrecommitHookStopsTheCommit`.
- [x] T002 `internal/precommit`, `internal/scaffold/hooks.go` e `agents/pre-commit.sh`,
  `cmd/trilha/precommit.go` e `i18n_precommit.go`, `trilha agents`/`new --agents`.
- [x] T003 `AGENTS.md` e skill (en/pt), referência da CLI e capítulo de IA (en/pt).
- [x] T004 CHANGELOG, ROADMAP, versão.
- [ ] T005 Release 0.161.0 (#314).

## Aceitação

- **SC-001** Um `git commit` com um arquivo de código de 601 linhas é barrado com a contagem e as
  instruções de divisão; com o arquivo dividido, um `go.sum` grande e um `trilha_gen.go` grande,
  o commit passa.
- **SC-002** O que vale é o índice: um arquivo grande só na árvore de trabalho não barra.
- **SC-003** `trilha hooks` liga o `core.hooksPath` uma vez e não mexe num que aponta para outra
  pasta.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` (`go test -json ./...`) | 1638 | 0 | 2 |
