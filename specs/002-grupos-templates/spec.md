# Feature Specification: Grupos de rota, adaptador html/template e recarga de estáticos

**Feature Branch**: `002-grupos-templates`
**Created**: 2026-09-05
**Status**: Draft
**Input**: User description: "Grupos de rota, adaptador html/template e recarga de public/ sem recompilar"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Grupos de rota (Priority: P1)

O desenvolvedor quer que `/precos` e `/sobre` compartilhem um layout de marketing e que
`/painel` e `/config` compartilhem um layout de app, sem que "marketing" ou "app" apareçam
na URL. Ele cria `app/marketing-/layout.go`, `app/marketing-/precos/page.go`,
`app/marketing-/sobre/page.go` e `app/app-/layout.go`, `app/app-/painel/page.go`. As URLs
continuam `/precos`, `/sobre`, `/painel`; cada uma recebe o layout do grupo e depois o raiz.

**Why this priority**: É o recurso do Next.js mais pedido depois de layouts aninhados; sem
ele o desenvolvedor duplica layout ou inventa um segmento de URL artificial.

**Independent Test**: No exemplo, `GET /precos` responde dentro de `<section class="marketing">`
e `GET /painel` dentro de `<section class="app">`; `GET /marketing-/precos` responde 404.

**Acceptance Scenarios**:

1. **Given** `app/marketing-/layout.go` e `app/marketing-/precos/page.go`, **When**
   `GET /precos`, **Then** 200 com o layout do grupo envolvendo a página, dentro do raiz.
2. **Given** um grupo, **When** `GET /marketing-/precos`, **Then** 404 (o nome do grupo
   nunca é segmento de URL).
3. **Given** dois grupos com a mesma rota filha (`a-/x/page.go` e `b-/x/page.go`), **When**
   `trilha gen`, **Then** erro `E_DUPLICATE_ROUTE` citando as duas pastas.
4. **Given** `app/marketing-/middleware.go`, **When** `GET /precos`, **Then** o middleware
   do grupo roda depois do raiz e antes da página.
5. **Given** `app/marketing-/page.go`, **When** `trilha gen`, **Then** a página responde
   `/` se não houver `app/page.go`, e erro `E_DUPLICATE_ROUTE` se houver.
6. **Given** grupo dentro de grupo (`a-/b-/x/page.go`), **When** `GET /x`, **Then** 200 com
   os dois layouts, do mais interno ao mais externo.

---

### User Story 2 - Páginas com html/template (Priority: P2)

Um desenvolvedor que prefere templates a código escreve `app/relatorio/page.go` que carrega
`app/relatorio/relatorio.html` (via `embed`) e devolve `tmpl.Node(t, "relatorio", dados)`.
O resultado entra no pipeline normal: layouts, título, escape de contexto do próprio
`html/template`.

**Why this priority**: Amplia o público do framework sem tocar no núcleo; templates são o
caminho conhecido da maioria dos devs Go.

**Independent Test**: Página do exemplo renderizada por template aparece dentro do layout
raiz; um valor com `<script>` sai escapado; erro de template vira 500.

**Acceptance Scenarios**:

1. **Given** um `*template.Template` válido, **When** `tmpl.Node(t, name, data)` é
   devolvido por `Page`, **Then** o HTML do template aparece dentro dos layouts.
2. **Given** dados com `<b>`, **When** renderizados por `{{.Nome}}`, **Then** saem escapados.
3. **Given** um nome de template inexistente, **When** renderizado, **Then** a página
   responde 500 e o erro cita o nome.
4. **Given** `tmpl.Must(fs, "*.html")`, **When** chamado no `init` do pacote, **Then**
   devolve o template parseado ou entra em pânico na subida (nunca no request).

---

### User Story 3 - Estáticos recarregam sem recompilar (Priority: P3)

Ao editar `public/style.css` com `trilha dev` rodando, o navegador recarrega em menos de
0,5 s, sem regenerar nem recompilar o binário (o app já lê `public/` do disco em dev).
Mudanças em `.go` continuam recompilando.

**Why this priority**: Iterar CSS com rebuild de ~1 s é irritante e desnecessário.

**Independent Test**: Script de medição: editar `style.css` e medir até o novo conteúdo
responder em `/style.css`; o log do dev não mostra `↻ ... ✓ pronto` para essa mudança.

**Acceptance Scenarios**:

1. **Given** `trilha dev` rodando, **When** só arquivos em `public/` mudam, **Then** a CLI
   emite `reload` no SSE sem rebuild e loga `↻ public/style.css (estático)`.
2. **Given** uma mudança em `public/` e outra em `.go` no mesmo lote, **When** detectadas,
   **Then** faz o rebuild completo uma vez.
3. **Given** `public/` é criado pela primeira vez, **When** detectado, **Then** faz rebuild
   (o `embed` do arquivo gerado precisa existir).

---

### Edge Cases

- Grupo com nome que vira alias inválido (`1a-`): alias seguro como já ocorre.
- Grupo dinâmico (`slug_-`): erro `E_BAD_SEGMENT` — grupo não pode ser dinâmico.
- Grupo vazio (sem rotas abaixo): ignorado silenciosamente.
- `route.go` dentro de grupo: funciona igual (API sem layouts, com middlewares do grupo).
- Template que escreve parcialmente antes de falhar: o pipeline renderiza em buffer, então
  a resposta é um 500 limpo.
- Remoção de `public/` inteira em dev: rebuild (o embed deixa de existir).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Pasta com sufixo `-` MUST ser grupo de rota: não contribui segmento, mas seus
  `layout.go` e `middleware.go` aplicam-se à subárvore.
- **FR-002**: O scanner MUST detectar rotas duplicadas (mesmo padrão vindo de pastas
  diferentes) e falhar com `E_DUPLICATE_ROUTE` citando ambas.
- **FR-003**: `trilha routes` MUST mostrar a pasta de origem real (com o grupo) ao lado do
  padrão sem o grupo.
- **FR-004**: Pacote `tmpl` MUST oferecer `Node(t *template.Template, name string, data any) h.Node`
  e `Must(fsys fs.FS, patterns ...string) *template.Template`.
- **FR-005**: Erros de template MUST surgir como erro de render (500), nunca como saída
  parcial.
- **FR-006**: `trilha dev` MUST distinguir lotes só de `public/` e responder com `reload`
  sem rebuild, exceto quando `public/` passa a existir ou deixa de existir.
- **FR-007**: A ordem de layouts/middlewares com grupos MUST seguir a árvore de pastas
  (grupo conta como nível), igual às pastas normais.

### Key Entities

- **Group**: pasta `nome-`; nó da árvore sem segmento de URL.
- **tmpl.Node**: nó do DSL que executa um template nomeado em um buffer.

## Success Criteria *(mandatory)*

- **SC-001**: Exemplo tem ao menos dois grupos com layouts distintos, cobertos por teste de
  integração, e `trilha routes` os lista sem o nome do grupo no padrão.
- **SC-002**: Edição de CSS chega ao navegador em < 0,5 s no script de medição, sem
  linha de rebuild no log.
- **SC-003**: `go test ./...` e `go vet ./...` verdes; goldens atualizados; zero deps.

## Assumptions

- Grupos não podem ser dinâmicos nem catch-all; combinar `-` com `_` é erro.
- O adaptador usa `html/template` (nunca `text/template`) para manter escape contextual.
