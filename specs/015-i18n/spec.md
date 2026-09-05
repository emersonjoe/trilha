# Feature Specification: Inglês por padrão, português como tradução (i18n)

**Feature Branch**: `015-i18n` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: decisão do usuário ("implemente") sobre a recomendação de publicar a documentação
em inglês por padrão, mantendo a tradução em português do Brasil, com emenda da constituição.

## Por quê

O Trilha é uma biblioteca pública em `github.com/emersonjoe/trilha`, com site em GitHub
Pages, releases e issues abertas a qualquer pessoa. Hoje **todo** material voltado ao público
está em português: site, README, arquivos de comunidade, templates de issue, mensagens da CLI
e o projeto que `trilha new` gera. Isso limita a adoção ao público lusófono e destoa do
próprio código, que já é em inglês por constituição.

A decisão: **inglês é o idioma padrão de tudo que é público; o português do Brasil é
tradução de primeira classe**, publicada junto, nunca depois. O que é interno ao trabalho do
mantenedor (specs, constituição) permanece em português.

## Escopo

| Superfície | Hoje | Depois |
|---|---|---|
| Site (`site/`) | pt-BR em `/aprender`, `/referencia` | inglês em `/`, `/learn`, `/reference`; pt-BR em `/pt`, `/pt/aprender`, `/pt/referencia`; caminhos antigos redirecionam para `/pt/...` |
| Home, 404, header, footer, sidebar, sumário, demos, títulos de callout, `tema.js` | pt-BR | por idioma |
| `README.md` | pt-BR | inglês; `README.pt-BR.md` com o mesmo conteúdo; link cruzado no topo |
| `CONTRIBUTING`, `GOVERNANCE`, `SECURITY`, `SUPPORT`, `CODE_OF_CONDUCT` | pt-BR | inglês na raiz; tradução em `docs/pt-BR/` |
| `.github/ISSUE_TEMPLATE/*`, `PULL_REQUEST_TEMPLATE.md` | pt-BR | inglês (GitHub não localiza formulários) |
| `CHANGELOG.md` | pt-BR | inglês (histórico traduzido, entradas novas em inglês) |
| Mensagens da CLI (`cmd/trilha`) | pt-BR | inglês por padrão; pt-BR quando `TRILHA_LANG` (ou `LC_ALL`/`LC_MESSAGES`/`LANG`) começa com `pt` |
| Projeto gerado por `trilha new` | pt-BR | idioma da CLI; `--lang en|pt` sobrescreve |
| Erros do runtime, do scanner e do gerador (`trilha`, `internal/scan`, `internal/gen`, `internal/scaffold`) | mistos | inglês (são erros de biblioteca, como o resto do Go) |
| `specs/`, `.specify/memory/constitution.md` | pt-BR | pt-BR (idioma de trabalho do mantenedor) |
| `examples/` | pt-BR (textos dos apps) | **fora do escopo**: os apps continuam em português; a documentação diz isso |
| `bench/RESULTS.md`, `THIRD_PARTY_NOTICES.md` | — | fora do escopo (gerado por script / texto legal) |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ler a documentação em inglês (Priority: P1)

Uma pessoa que não lê português chega em `emersonjoe.github.io/trilha` e encontra a home,
os capítulos de "Learn" e a "Reference" em inglês, com as mesmas demos executadas ao vivo.

**Independent Test**: `go test ./site/` renderiza todas as páginas de `/learn` e `/reference`
com status 200, `<html lang="en">`, links internos válidos e as demos presentes.

**Acceptance Scenarios**:

1. **Given** o site exportado, **When** abro `/`, **Then** a home está em inglês e os botões
   levam a `/learn` e `/reference`.
2. **Given** um capítulo em inglês, **When** olho o header, **Then** há um link "Português"
   que leva à **mesma página** em português (não à home).
3. **Given** qualquer página, **When** leio o `<head>`, **Then** há `<link rel="alternate"
   hreflang="en">`, `hreflang="pt-BR"` e `x-default` apontando para a versão em inglês.

### User Story 2 - Continuar lendo em português (Priority: P1)

Quem já usava o site em português continua com todo o conteúdo, agora sob `/pt`, e os links
antigos (issues, README de versões anteriores, `SECURITY.md`, specs) não quebram.

**Independent Test**: `GET /aprender/seguranca` responde redirecionamento permanente para
`/pt/aprender/seguranca`; no export estático o mesmo caminho vira um `index.html` com
`<meta http-equiv="refresh">` e `<link rel="canonical">` para o destino.

**Acceptance Scenarios**:

1. **Given** o link antigo `/referencia/ctx`, **When** acesso, **Then** chego em
   `/pt/referencia/ctx` com o conteúdo de antes.
2. **Given** a página `/pt/aprender/formularios`, **When** clico em "English", **Then** abro
   `/learn/forms`.
3. **Given** o export, **When** conto as páginas, **Then** cada página em inglês tem a sua
   correspondente em português e vice-versa (teste de sincronia: mesmas seções, mesmo
   número de páginas, mesmas diretivas `@demo`).

### User Story 3 - Usar a CLI e o projeto novo no meu idioma (Priority: P2)

Ao rodar `trilha new agenda` em um terminal com `LANG=en_US.UTF-8`, as mensagens e a página
inicial gerada saem em inglês; com `LANG=pt_BR.UTF-8` ou `TRILHA_LANG=pt`, em português.

**Independent Test**: e2e da CLI com `TRILHA_LANG=pt` e `TRILHA_LANG=en` compara a saída de
`trilha help` e o `app/page.go` gerado.

**Acceptance Scenarios**:

1. **Given** `TRILHA_LANG` vazio e `LANG=C`, **When** `trilha help`, **Then** o uso sai em
   inglês.
2. **Given** `TRILHA_LANG=pt`, **When** `trilha audit`, **Then** títulos e dicas saem em
   português.
3. **Given** `trilha new x --lang pt` com `LANG=en_US`, **Then** `app/page.go` tem os textos
   em português e `h.Lang("pt-BR")`.

### User Story 4 - Contribuir sabendo a regra (Priority: P2)

Quem abre um PR encontra `CONTRIBUTING.md` em inglês dizendo que toda mudança visível ao
público vem com a tradução no mesmo PR, e o teste do site falha se uma página ficar sem par.

**Acceptance Scenarios**:

1. **Given** um capítulo novo só em `content/en/`, **When** `go test ./site/`, **Then** o
   teste de sincronia falha apontando a página que falta em `content/pt/`.

### Edge Cases

- `/pt` sem barra final e `/pt/` devem servir a home em português; o export grava
  `pt/index.html`.
- Um slug em inglês acessado sob `/pt/aprender/<slug-en>` é 404 (não há adivinhação).
- `TRILHA_LANG=pt_BR.UTF-8`, `pt-BR`, `pt` e `PT` são todos português; qualquer outro valor
  (inclusive `C`, `POSIX`, vazio) é inglês.
- A página 404 do site é única (`404.html`): em inglês, com uma linha em português e links
  para as duas homes.
- O exemplo de formulário simulado em `tema.js` escolhe as mensagens pelo `lang` do `<html>`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O conteúdo do site vive em `site/internal/docs/content/<locale>/<section>/`
  com `en` (`learn`, `reference`) e `pt` (`aprender`, `referencia`); as seções e slugs são
  paralelos por posição, o que dá a tradução de qualquer página sem tabela manual.
- **FR-002**: Rotas: `/`, `/learn`, `/learn/{slug}`, `/reference`, `/reference/{slug}`,
  `/pt`, `/pt/aprender`, `/pt/aprender/{slug}`, `/pt/referencia`, `/pt/referencia/{slug}`.
  Os caminhos antigos `/aprender[/{slug}]` e `/referencia[/{slug}]` respondem 301 para
  `/pt/...`.
- **FR-003**: `trilha export` grava, para caminhos exportados que respondem 3xx, um
  `index.html` de redirecionamento (`meta refresh` 0 s, `link rel=canonical`, link
  clicável), em vez de falhar. Caminhos exportados que respondem 3xx para fora do site
  continuam sendo erro.
- **FR-004**: Todo texto de interface do site (header, footer, sidebar, sumário, vizinhos,
  rótulos de demo, títulos de callout, `tema.js`, nota de estatísticas, 404) sai no idioma
  da página; o `<html lang>` é `en` ou `pt-BR`.
- **FR-005**: Cada página emite `hreflang` para as duas versões e `x-default` para o inglês;
  o header tem o link para a tradução da página atual.
- **FR-006**: As demos (`site/internal/demos`) existem nos dois idiomas com fonte e
  resultado equivalentes; `@demo nome` no Markdown resolve pela locale da página. Callouts
  aceitam os nomes em inglês (`tip`, `warning`, `note`, `challenge`, `solution`) além dos
  atuais.
- **FR-007**: Teste de sincronia entre locales: mesmas seções, mesma contagem de páginas,
  mesmo conjunto de diretivas `@demo` por página correspondente.
- **FR-008**: `README.md` em inglês e `README.pt-BR.md` em português, ambos com link para o
  outro na primeira linha; os links para o site apontam para o idioma correspondente.
- **FR-009**: `CONTRIBUTING.md`, `GOVERNANCE.md`, `SECURITY.md`, `SUPPORT.md`,
  `CODE_OF_CONDUCT.md` em inglês na raiz, com tradução em `docs/pt-BR/<mesmo nome>` e link
  cruzado. Templates de issue/PR em inglês.
- **FR-010**: A CLI escolhe o idioma por `TRILHA_LANG`, depois `LC_ALL`, `LC_MESSAGES`,
  `LANG` (prefixo `pt`, sem distinção de caixa, = português; senão inglês). Todas as
  mensagens de `cmd/trilha` (uso, flags, progresso, avisos, `audit`) passam por uma tabela
  única de textos.
- **FR-011**: `trilha new` aceita `--lang en|pt` (padrão: idioma da CLI); os templates do
  scaffold geram textos e `h.Lang` no idioma escolhido.
- **FR-012**: Mensagens de erro e log do runtime (`trilha`), do scanner, do gerador e do
  scaffold ficam em inglês; a página de erro de compilação do dev server e a página 404
  simples também.
- **FR-013**: `CHANGELOG.md` em inglês, com o histórico traduzido e a entrada desta versão.
- **FR-014**: A constituição é emendada (v1.2.0) para registrar a regra de idioma; o
  `CONTRIBUTING` e o `GOVERNANCE` refletem a mesma regra.

### Key Entities

- **Locale** (site): código (`en`, `pt`), prefixo de URL (``, `/pt`), `lang` HTML, nome
  exibido, seções com título e slugs.
- **Page** (site): locale, seção, slug, posição; `Path()` inclui o prefixo.
- **Tabela de textos da CLI**: chave → {en, pt}.

## Success Criteria *(mandatory)*

- **SC-001**: `go test ./...` e `make test` verdes; `go test ./site/` cobre as duas locales,
  redirecionamentos antigos, `hreflang`, switcher e sincronia.
- **SC-002**: O site exportado (`trilha export --base /trilha`) contém `index.html` para
  cada página das duas locales e stubs de redirecionamento para os caminhos antigos.
- **SC-003**: Nenhuma string em português restante em `cmd/trilha`, `internal/`, e na raiz
  do módulo fora da tabela de textos (`grep` por acentos nos `.go` não-teste, excluindo
  `site/`, `examples/` e a tabela).
- **SC-004**: Um leitor que só lê inglês consegue seguir todos os capítulos de "Learn" sem
  encontrar texto em português fora dos exemplos citados (`examples/`), que são declarados
  como em português.
- **SC-005**: Release `v0.6.0` publicado com notas em inglês.

## Assumptions

- O prefixo `/pt` (não `/pt-BR`) porque `pt-BR` não é nome válido de pacote Go para a pasta
  de rota e `pt` é o código de idioma primário; o `hreflang` continua `pt-BR`.
- Exemplos (`examples/`) ficam em português: são apps completos com domínio próprio; traduzir
  dobraria o código sem valor didático. A documentação em inglês avisa ao citá-los.
- Não há detecção de idioma pelo `Accept-Language` no site: é estático (GitHub Pages) e a
  escolha explícita por URL é mais previsível e indexável.
