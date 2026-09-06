# Spec 044 — AGENTS.md no scaffold e llms.txt no site

- **Issue**: [#46](https://github.com/emersonjoe/trilha/issues/46) — a issue é a fonte do escopo.
- **Branch**: `044-agents-md`
- **Versão**: 0.36.0

## Por quê

Hoje o agente que abre um projeto Trilha pela primeira vez não tem onde ler as convenções. Ele
descobre por tentativa: cria um `handler.go` que ninguém varre, edita o `trilha_gen.go` (que a
próxima geração sobrescreve), instala uma dependência que o `TestNoExternalDeps` do usuário
reprova. A régua da spec 043 mostra o preço disso: a mediana do cenário `comments` gastou 39
rodadas e 2,2 M de tokens de cache, boa parte procurando a convenção que uma página de texto
teria contado.

Do lado do site é o mesmo problema com outro público: o agente que precisa consultar a
documentação hoje baixa HTML com navegação, sidebar e script de analytics, e paga por tudo
isso. O padrão de fato para isso é um `/llms.txt` — índice curto em Markdown — e um
`/llms-full.txt` com o conteúdo inteiro em texto puro.

A issue pede as duas coisas e é explícita sobre o default: **suporte a IA é opt-in**. Um
projeto novo não ganha `AGENTS.md` sem pedir, porque o arquivo é uma escolha do time, não uma
convenção do framework.

## O que muda

**1. `trilha agents` — comando novo.** Escreve `AGENTS.md` e `CLAUDE.md` na raiz do projeto.

```bash
trilha agents [--force] [--lang en|pt]
```

`AGENTS.md` é gerado pelo framework e carimbado como os arquivos do kit `ui`: a primeira linha
é `<!-- trilha agents <hash> -->`. Uma versão intocada é atualizada em silêncio na próxima
rodada; uma versão editada localmente só é sobrescrita com `--force`, e sem ele o comando sai
com erro dizendo qual arquivo foi mexido. `CLAUDE.md` é um ponteiro de três linhas para o
`AGENTS.md` e só é criado — nunca sobrescrito, porque a partir da primeira linha que o time
acrescenta ele é do projeto.

Conteúdo do `AGENTS.md`, nas duas línguas:

- as convenções em três linhas (pasta = rota, `page.go`/`route.go`/`layout.go`/`middleware.go`,
  `slug_` é parâmetro);
- os comandos (`trilha dev`, `gen`, `generate`, `audit`, `build`, `export`, `openapi`, `routes`,
  `ui`, `version` e `make test`) e o que cada um verifica;
- o que **não** fazer: editar `trilha_gen.go`, acrescentar dependência, pôr segredo no código;
- onde estão as receitas e a referência.

**2. `trilha new --agents`.** Mesmo conteúdo, na criação do projeto. Sem a flag, `trilha new`
não escreve nenhum dos dois arquivos — o default é sem IA.

**3. `/llms.txt` e `/llms-full.txt` no site**, nas duas locales (`/llms.txt`, `/pt/llms.txt`,
`/llms-full.txt`, `/pt/llms-full.txt`). O curto é um índice: título do site, um parágrafo de
apresentação e um item por página com título, link e descrição. O longo é o Markdown de todas
as páginas concatenado, com os blocos de código intactos.

**4. `App.Export` grava caminho com extensão como arquivo.** Hoje todo caminho vira
`<caminho>/index.html`, então `/llms.txt` cairia em `llms.txt/index.html`. Passa a valer a mesma
regra que a varredura já usa para pasta com ponto (spec 008): se o último segmento do caminho
tem ponto, o corpo é gravado nesse arquivo.

```go
a.AddExportPath("/llms.txt")   // out/llms.txt, não out/llms.txt/index.html
```

## Fora de escopo

- **Servidor MCP** (`trilha mcp`) e `trilha ctx --json`: issues #47 e #50, cada uma com sua spec.
- **Publicar o `AGENTS.md` do próprio repositório**: o `CLAUDE.md` da raiz já cumpre esse papel
  e tem regras de contribuição que não valem para um projeto de usuário.
- **`llms.txt` no `trilha export` de um app de usuário**: o gerador é do site, que conhece o
  índice das páginas. Um app qualquer não tem esse índice.
- **Traduzir o `AGENTS.md` depois de escrito**: rodar `trilha agents --lang pt --force` regrava.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | textos embutidos com `embed`; nenhum import novo fora da stdlib |
| III — gerador determinístico | o carimbo é o hash do próprio corpo; mesma entrada, mesmo arquivo |
| VI — teste primeiro | e2e da CLI e teste do site antes do código, em cada bloco |
| VII — segurança por padrão | `AGENTS.md` diz para não pôr segredo no código e aponta `trilha audit` |
| Público em duas línguas | `AGENTS.md`/`CLAUDE.md` em `en` e `pt`; `/llms.txt` nas duas locales |

## Tarefas

Uma rodada de `make test` por bloco.

### Bloco 1 — carimbo genérico e escrita dos arquivos (SC-001, SC-002)

- [x] T001 Teste que falha em `internal/scaffold/agents_test.go`: `WriteAgents` cria os dois
      arquivos; segunda rodada mantém; corpo intocado com carimbo velho é atualizado; corpo
      editado devolve `ErrAgentsModified` e só cede com `force`; `CLAUDE.md` existente nunca é
      tocado; língua desconhecida é erro.
- [x] T002 `internal/scaffold/agents.go` + `agents/*.md`; carimbo generalizado em `ui.go`
      (CSS `/* … */` e Markdown `<!-- … -->` pelo mesmo par de funções).

### Bloco 2 — CLI (SC-003, SC-006)

- [x] T003 Teste que falha em `cmd/trilha/e2e_test.go`: `trilha new` **não** cria `AGENTS.md`;
      `trilha new --agents` cria os dois; `trilha agents` cria num projeto existente e a segunda
      rodada não falha; depois de editar, falha sem `--force` e passa com ele.
- [x] T004 Teste que falha em `cmd/trilha/agents_test.go`: todo comando citado no `AGENTS.md`
      das duas línguas existe no `usage`, e todo comando do `usage` é citado — o arquivo não
      pode envelhecer sem a suíte reclamar.
- [x] T005 `cmd/trilha/agents.go`, flag `-agents` em `new.go`, despacho em `main.go`, mensagens
      e linha do `usage` em `i18n.go`.

### Bloco 3 — llms.txt (SC-004, SC-005)

- [x] T006 Teste que falha em `export_test.go`: caminho cujo último segmento tem ponto vira o
      arquivo, e não `<caminho>/index.html`.
- [x] T007 `export.go`: a regra do ponto no `Export`.
- [x] T008 Teste que falha em `site/internal/docs/llms_test.go` e `site/site_test.go`: as quatro
      rotas respondem `text/plain`; o índice tem uma linha por página da locale, com o link
      prefixado pelo base path; o longo tem o corpo de todas as páginas e os blocos de código
      inteiros; nenhum link cruza locale; as quatro entram no `ExportPaths`.
- [x] T009 `site/internal/docs/llms.go`, `site/app/llms.txt/route.go` e irmãs,
      `AddExportPath` no `site/app/setup.go`.

### Bloco 4 — documentação e fechamento

- [x] T010 `learn/ai-and-agents` + `pt/aprender/ia-e-agentes`: seção sobre `trilha agents`, o
      que o `AGENTS.md` conta e o `/llms.txt` do site; `reference/cli` + `pt/referencia/cli`: o
      comando, as flags e a tabela de quem é dono de cada arquivo; `learn/dev-and-production` +
      `pt/aprender/dev-e-producao` e `reference/app` + `pt/referencia/app`: a regra do ponto no
      export. (`reference/ai` é o pacote `ai`, não a documentação para agentes: a seção ficou
      onde o leitor já procura o assunto.)
- [x] T011 `CHANGELOG.md` (0.36.0) com a régua antes/depois, `version` em `cmd/trilha/main.go`,
      item 24 do `ROADMAP.md`.
- [ ] T012 `make bench-agent` com o `AGENTS.md` no fixture, `make test` verde e
      `scripts/release.sh 0.36.0 --issues "46"`.

## Aceitação

- **SC-001** `trilha new x` não deixa `AGENTS.md` nem `CLAUDE.md` no projeto.
- **SC-002** `trilha new x --agents` deixa os dois; rodar `trilha agents` de novo não falha e não
  perde edição local sem `--force`.
- **SC-003** Todo comando citado no `AGENTS.md` está no `usage` da CLI, e vice-versa, nas duas
  línguas — verificado por teste.
- **SC-004** `GET /llms.txt`, `/llms-full.txt`, `/pt/llms.txt` e `/pt/llms-full.txt` respondem
  `200 text/plain; charset=utf-8`, e o índice lista todas as páginas da locale.
- **SC-005** `trilha export` do site grava `out/llms.txt` como arquivo de texto.
- **SC-006** `AGENTS.md` sai em inglês por padrão e em pt-BR com `TRILHA_LANG=pt` ou `--lang pt`.
- **SC-007** `bench/agent/RESULTS.md` traz a medição depois, ao lado da linha de base da 0.33.0.
