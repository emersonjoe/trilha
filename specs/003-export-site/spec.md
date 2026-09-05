# Feature Specification: Exportação estática e site de documentação

**Feature Branch**: `003-export-site` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "Exportação estática (trilha export) e site de documentação construído com o próprio Trilha, publicado no GitHub Pages"

## User Scenarios & Testing

### User Story 1 - Exportar um app como site estático (P1)

O desenvolvedor roda `trilha export` e recebe em `out/` um arquivo `index.html` por rota de
página sem parâmetros, os arquivos de `public/` e um `404.html`. Rotas dinâmicas entram
quando o `Setup` lista caminhos com `a.AddExportPath("/blog/ola")`. O resultado sobe em
qualquer hospedagem estática (GitHub Pages, S3, nginx).

**Independent Test**: `trilha export` no `examples/blog` produz `out/index.html`,
`out/blog/index.html`, `out/precos/index.html`, `out/style.css`, `out/404.html` e nada para
`/blog/{slug}` (dinâmica) nem para `/api/posts` (API).

**Acceptance Scenarios**:
1. **Given** rotas de página sem parâmetro, **When** `trilha export`, **Then** cada uma
   vira `<caminho>/index.html` (a raiz vira `index.html`), com o mesmo HTML do servidor em
   produção (sem script de dev).
2. **Given** `a.AddExportPath("/blog/ola-trilha")`, **When** exporta, **Then** existe
   `out/blog/ola-trilha/index.html`.
3. **Given** uma rota que responde erro (404/500) na exportação, **When** exporta, **Then**
   o comando falha citando a rota; `404.html` vem da página not_found.
4. **Given** `TRILHA_BASE_PATH=/trilha`, **When** o app gera links com `c.Base()`,
   **Then** eles saem prefixados (necessário para Pages em subcaminho).

### User Story 2 - Site de documentação (P1)

Um desenvolvedor novo abre a documentação e encontra: página inicial com o que é o Trilha e
blocos "código → resultado" renderizados de verdade; trilha **Aprender** em capítulos
progressivos (início rápido, páginas e rotas, layouts, HTML com `h`, formulários, API,
middleware, dev e produção), cada um com exemplos próprios e um desafio com solução
recolhível; e **Referência** por pacote (`trilha`, `Ctx`, `h`, `tmpl`, erros, CLI,
convenções). Sidebar, sumário da página, anterior/próximo, tema claro/escuro e leitura
confortável no celular.

**Independent Test**: o site (`site/`) sobe com `trilha dev`, todas as páginas respondem
200, cada capítulo de Aprender tem seção "Desafio", a exportação gera o site inteiro e o
deploy no GitHub Pages responde na URL pública; verificação visual no navegador em desktop
e móvel, claro e escuro.

### Edge Cases
- Rota de página com parâmetro e sem caminho listado: ignorada com aviso.
- `public/` ausente: exporta só HTML.
- Diretório de saída existente: limpo antes (só se contiver um marcador `.trilha-export`).
- Página que usa `c.Base()` em dev (sem base): prefixo vazio.

## Requirements
- **FR-001**: `App.Export(dir)` MUST renderizar rotas de página estáticas e caminhos
  adicionais via `App.AddExportPath`, mais `public/` e `404.html`.
- **FR-002**: O arquivo gerado MUST desviar para `Export` quando `TRILHA_EXPORT=<dir>`.
- **FR-003**: `trilha export [-o out]` MUST fazer gen + build + executar com `TRILHA_EXPORT`.
- **FR-004**: `Ctx.Base()` MUST devolver `TRILHA_BASE_PATH` (sem barra final) para links.
- **FR-005**: O site MUST viver em `site/` como app Trilha, conteúdo em Markdown embutido
  convertido por um conversor mínimo próprio (sem dependências), com realce de Go feito
  na exportação (sem JS externo obrigatório).
- **FR-006**: O site MUST ter design e textos originais; nomes de outros frameworks só
  em comparações, sem logotipos ou marcas de terceiros.
- **FR-007**: Workflow do GitHub Actions MUST exportar o site e publicar no Pages a cada
  push em `main`.

## Success Criteria
- **SC-001**: `go test ./...` cobre export (unitário no runtime e integração no exemplo).
- **SC-002**: Site publicado responde 200 em todas as rotas listadas na sidebar.
- **SC-003**: Lighthouse-like: sem JS bloqueante, CSS < 20 KB, funciona sem JS (tema
  segue o sistema quando JS está desligado).
