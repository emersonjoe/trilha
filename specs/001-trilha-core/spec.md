# Feature Specification: Trilha — framework web para Go estilo Next.js

**Feature Branch**: `001-trilha-core`
**Created**: 2026-09-04
**Status**: Draft
**Input**: User description: "Framework web para Go estilo Next.js: roteamento por arquivos em app/, layouts aninhados, rotas de API, middleware, dev server com recarga, build em binário único"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Criar páginas só com arquivos (Priority: P1)

Um desenvolvedor Go cria um novo projeto, escreve `app/page.go` com uma função `Page` e
`app/layout.go` com uma função `Layout`, roda `trilha dev` e abre `http://localhost:3000`.
Ele vê a página renderizada dentro do layout. Cria `app/sobre/page.go` e `/sobre` passa a
responder sem registrar nada. Cria `app/blog/slug_/page.go` e `/blog/ola` responde com
`slug = "ola"`. Cria `app/docs/path__/page.go` e `/docs/a/b/c` responde com `path = "a/b/c"`.

**Why this priority**: É a proposta de valor central: roteamento por convenção, sem tabela
de rotas manual. Sem isso não há framework.

**Independent Test**: Criar o app de exemplo `examples/blog`, rodar `trilha gen` e
`go run .`, e verificar via HTTP que `/`, `/blog`, `/blog/ola` e `/docs/a/b` respondem com
o HTML esperado envolto pelos layouts corretos.

**Acceptance Scenarios**:

1. **Given** um projeto com `app/page.go` exportando `Page`, **When** `GET /`, **Then**
   responde 200 `text/html` com o HTML devolvido por `Page` dentro do `Layout` raiz.
2. **Given** `app/blog/layout.go` e `app/blog/slug_/page.go`, **When** `GET /blog/ola`,
   **Then** a página recebe `c.Param("slug") == "ola"` e é envolvida por `blog/layout.go`
   e depois pelo layout raiz, nessa ordem (de dentro para fora).
3. **Given** `app/docs/path__/page.go`, **When** `GET /docs/a/b/c`, **Then**
   `c.Param("path") == "a/b/c"`.
4. **Given** uma rota inexistente, **When** `GET /nada`, **Then** responde 404 com a
   página de `app/not_found.go` se existir, senão uma 404 padrão do framework.
5. **Given** `Page` devolve um erro qualquer, **When** `GET` nessa rota, **Then** responde
   500 com `app/error.go` (ou página padrão), mostrando o stack apenas em modo dev.
6. **Given** uma rota estática `/blog/novo` e uma dinâmica `/blog/slug_`, **When**
   `GET /blog/novo`, **Then** a estática vence.

---

### User Story 2 - Rotas de API e formulários (Priority: P1)

O desenvolvedor cria `app/api/posts/route.go` exportando `GET` e `POST`. `GET /api/posts`
devolve JSON; `POST /api/posts` lê o corpo JSON, valida e responde 201. Em uma página, ele
exporta `POST` em `page.go` para tratar o envio de um `<form>` e redireciona ao final
(padrão POST-redirect-GET). Métodos não exportados respondem 405.

**Why this priority**: Toda aplicação real precisa de endpoints e formulários; junto com a
US1 forma o MVP utilizável.

**Independent Test**: No app de exemplo, `curl` em `/api/posts` (GET/POST/DELETE) e um
`POST` de formulário em `/blog/novo` verificando o redirecionamento e o cookie CSRF.

**Acceptance Scenarios**:

1. **Given** `route.go` exporta `GET`, **When** `GET /api/posts`, **Then** 200 com
   `Content-Type: application/json` e o corpo produzido por `c.JSON(...)`.
2. **Given** `route.go` exporta só `GET`, **When** `DELETE /api/posts`, **Then** 405 com
   cabeçalho `Allow: GET`.
3. **Given** `page.go` exporta `POST`, **When** um formulário válido é enviado com token
   CSRF, **Then** o handler roda e responde 303 para a URL indicada em `c.Redirect`.
4. **Given** um `POST` sem token CSRF ou com token inválido, **When** enviado, **Then** 403
   sem executar o handler.
5. **Given** um corpo maior que o limite configurado, **When** enviado, **Then** 413.
6. **Given** o handler devolve `trilha.ErrNotFound`, **When** chamado, **Then** 404 no
   formato da rota (JSON para `route.go`, HTML para `page.go`).

---

### User Story 3 - Middleware por subárvore (Priority: P2)

O desenvolvedor cria `app/middleware.go` exportando `Middleware(c *trilha.Ctx, next
trilha.Next) error` para registrar cada requisição, e `app/admin/middleware.go` que exige um
cookie de sessão e devolve redirecionamento para `/login` quando ausente. Os middlewares
aplicam-se a todas as rotas da sua subárvore, do mais externo para o mais interno.

**Why this priority**: Autenticação, logs e cabeçalhos são transversais; sem isso o
desenvolvedor duplica código em cada handler.

**Independent Test**: No app de exemplo, `GET /admin` sem cookie responde 302 para
`/login`; com cookie responde 200; ambas as requisições aparecem no log do middleware raiz.

**Acceptance Scenarios**:

1. **Given** `app/middleware.go` e `app/admin/middleware.go`, **When** `GET /admin`,
   **Then** executam nesta ordem: raiz → admin → handler; `next()` pode ser omitido para
   curto-circuitar.
2. **Given** o middleware define `c.Set("user", u)`, **When** a página lê `c.Get("user")`,
   **Then** obtém o valor definido.
3. **Given** um middleware devolve erro, **When** a requisição chega, **Then** o handler
   não executa e o erro segue o tratamento padrão (404/redirect/500).

---

### User Story 4 - Servidor de desenvolvimento com recarga (Priority: P2)

O desenvolvedor roda `trilha dev`, edita `app/page.go`, salva, e em menos de 2 segundos o
navegador recarrega mostrando a mudança. Se o código não compila, o terminal mostra o erro
do compilador e o navegador exibe uma tela de erro com a mensagem; ao corrigir, tudo volta
sozinho. Arquivos em `public/` são servidos em `/` e alterações neles também recarregam.

**Why this priority**: É a experiência que define "estilo Next.js"; sem recarga o framework
é só um roteador.

**Independent Test**: Iniciar `trilha dev` no exemplo, alterar um texto de `page.go`, e
medir com um script que faz polling em `/` que o novo texto aparece em < 2 s; introduzir um
erro de sintaxe e verificar que `/` mostra a tela de erro de compilação.

**Acceptance Scenarios**:

1. **Given** `trilha dev` rodando, **When** um arquivo `.go` em `app/` muda, **Then** a CLI
   regenera `trilha_gen.go`, recompila, reinicia o processo e o navegador recarrega.
2. **Given** um erro de compilação, **When** o navegador pede qualquer rota, **Then** vê
   uma página com a saída do compilador; o processo anterior continua morto (sem versão
   antiga "fantasma").
3. **Given** `public/style.css`, **When** `GET /style.css`, **Then** 200 com
   `text/css`; `GET /../go.mod` responde 404.
4. **Given** a página raiz, **When** renderizada em dev, **Then** contém o script de
   live-reload; em produção não contém.

---

### User Story 5 - Build em binário único e scaffold (Priority: P3)

O desenvolvedor roda `trilha build` e recebe `bin/<nome>` com `public/` embutido, que sobe
sem nenhum arquivo ao lado. Para começar do zero, `trilha new meu-app` cria um projeto com
`go.mod`, layout, página inicial, uma rota de API, `public/style.css` e `.gitignore`, que
já roda com `trilha dev`. `trilha routes` imprime a tabela de rotas descobertas.

**Why this priority**: Fecha o ciclo criar→desenvolver→publicar, mas o desenvolvedor
consegue fazer tudo à mão com `go build` se ainda não existir.

**Independent Test**: `trilha new x && cd x && trilha build && ./bin/x` em um diretório
vazio; `curl localhost:3000/` e `curl localhost:3000/style.css` respondem 200; mover o
binário para outro diretório e repetir.

**Acceptance Scenarios**:

1. **Given** um projeto válido, **When** `trilha build`, **Then** existe um binário e
   `trilha_gen.go` está atualizado.
2. **Given** um diretório vazio, **When** `trilha new app`, **Then** `cd app && trilha
   dev` sobe sem edições.
3. **Given** um projeto, **When** `trilha routes`, **Then** imprime uma linha por rota com
   método, padrão e arquivo de origem, ordenadas por padrão.

---

### Edge Cases

- `page.go` e `route.go` no mesmo diretório: erro de geração com mensagem apontando os
  dois arquivos (uma pasta responde página ou API, nunca os dois).
- `page.go` sem função `Page` exportada, ou com assinatura errada: erro de geração citando
  arquivo, linha e a assinatura esperada.
- Dois segmentos dinâmicos irmãos (`a_` e `b_` na mesma pasta): erro de geração (ambíguo).
- `slug__` (catch-all) que não é folha: erro de geração.
- Nome de pasta que gera nome de pacote inválido (ex.: `1abc`, `go-mod`): o gerador usa
  alias de import seguro; o nome do pacote declarado no arquivo prevalece.
- Requisição com barra final (`/blog/`) em rota registrada sem barra: redireciona 301
  para `/blog` (exceto `/`).
- `HEAD` em rota com `GET`: respondido automaticamente pelo `net/http`.
- Layout raiz ausente: o framework envolve com um `<html>` mínimo e avisa em dev.
- `h.Raw` recebendo texto de usuário: responsabilidade do desenvolvedor; documentado como
  única porta sem escape.
- Processo filho do `dev` que morre sozinho (panic no `Setup`): CLI mostra a saída e fica
  aguardando a próxima alteração, sem sair.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema MUST mapear a árvore `app/` em rotas HTTP: cada `page.go` → rota de
  página, cada `route.go` → rota de API, com o caminho derivado das pastas.
- **FR-002**: O sistema MUST suportar segmento dinâmico (`nome_` → `{nome}`) e catch-all
  (`nome__` → `{nome...}`), expondo o valor por `c.Param("nome")`.
- **FR-003**: O sistema MUST aplicar `layout.go` de cada pasta ancestral, do mais interno
  ao mais externo, recebendo o conteúdo filho já renderizado como nó.
- **FR-004**: O sistema MUST aplicar `middleware.go` de cada pasta ancestral, do mais
  externo ao mais interno, com capacidade de curto-circuito e de propagar valores por `Ctx`.
- **FR-005**: Rotas de API MUST expor os métodos pelas funções exportadas `GET`, `POST`,
  `PUT`, `PATCH`, `DELETE`; métodos ausentes respondem 405 com `Allow`.
- **FR-006**: Páginas MAY exportar `POST`, `PUT`, `PATCH`, `DELETE` para tratar
  formulários; o framework MUST validar token CSRF nesses métodos.
- **FR-007**: `Ctx` MUST oferecer: `Param`, `Query`, `Form`, `BindJSON`, `JSON`, `HTML`,
  `Text`, `Redirect`, `Status`, `Header`, `Cookie`/`SetCookie`, `Set`/`Get`, `Request`,
  `Writer`, `Title`/`SetTitle`, `CSRFToken`, `Env` (dev/prod).
- **FR-008**: O sistema MUST tratar `trilha.ErrNotFound` como 404 (usando `not_found.go` se
  existir), `*trilha.RedirectError` como redirecionamento e qualquer outro erro como 500
  (usando `error.go` se existir), com stack apenas em dev.
- **FR-009**: O DSL `h` MUST escapar texto e valores de atributo por padrão, oferecer os
  elementos HTML5 comuns, `Fragment`, `If`, `Map`, `Raw`, `Doctype`, e renderizar em
  streaming para `io.Writer`.
- **FR-010**: A CLI MUST gerar `trilha_gen.go` determinístico a partir de `app/`, com
  imports explícitos e registro tipado; e falhar com mensagem clara em qualquer convenção
  violada (ver Edge Cases).
- **FR-011**: `trilha dev` MUST observar mudanças, regenerar, recompilar, reiniciar e
  disparar recarga no navegador via SSE, e exibir erros de compilação no navegador.
- **FR-012**: `trilha build` MUST produzir um binário único com `public/` embutido;
  `trilha new` MUST criar um projeto funcional; `trilha routes` MUST listar as rotas.
- **FR-013**: Arquivos em `public/` MUST ser servidos na raiz com proteção contra path
  traversal e `Cache-Control` adequado (curto em dev, longo em prod).
- **FR-014**: Respostas MUST incluir por padrão `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`; o corpo de
  requisição MUST ter limite (1 MiB padrão, configurável).
- **FR-015**: O pacote raiz `app` MAY exportar `Setup(a *trilha.App) error` para inicializar
  recursos antes de servir; falha em `Setup` aborta a subida com a mensagem no terminal.
- **FR-016**: Logs MUST usar `log/slog` estruturado: método, caminho, status, duração e
  id de requisição; nunca corpo nem cookies.

### Key Entities

- **Route**: padrão de caminho (`/blog/{slug}`), tipo (page | api), arquivo de origem,
  pacote Go, conjunto de métodos exportados, cadeia de layouts e cadeia de middlewares.
- **Layout**: função que envolve um nó filho; pertence a uma pasta e vale para a subárvore.
- **Middleware**: função que recebe `Ctx` e `Next`; pertence a uma pasta e vale para a
  subárvore.
- **Ctx**: envelope da requisição/resposta com parâmetros, valores compartilhados, título,
  token CSRF e ambiente.
- **Node**: valor renderizável do DSL `h` (elemento, texto, fragmento, raw).
- **App**: configuração em runtime (porta, ambiente, limite de corpo, FS embutido, logger).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um projeto novo (`trilha new`) chega ao navegador com `trilha dev` em menos
  de 60 segundos, sem editar arquivo algum.
- **SC-002**: Ciclo editar→salvar→ver no navegador abaixo de 2 s no app de exemplo (medido
  por script de polling) em máquina de desenvolvimento.
- **SC-003**: 100% das convenções de arquivo (page, route, layout, middleware, not_found,
  error, `nome_`, `nome__`, public, Setup) são cobertas por teste de integração no exemplo.
- **SC-004**: O runtime e a CLI não têm nenhuma dependência fora da biblioteca padrão
  (`go list -deps` sem módulos externos).
- **SC-005**: Rodar o gerador duas vezes sobre a mesma árvore produz bytes idênticos (golden
  test); `go vet ./...` e `go test ./...` passam na raiz do repositório.
- **SC-006**: Toda violação de convenção listada em Edge Cases produz erro com arquivo e
  motivo, verificado por teste de tabela.

## Assumptions

- Go 1.22+ (padrões com método e `{param}` no `http.ServeMux`); desenvolvimento em 1.25.
- Renderização é server-side; não há hidratação nem componente cliente. Interatividade
  cliente fica a cargo de JS em `public/` (ou htmx) e está fora do escopo v1.
- Sem geração estática (`export`), sem streaming/suspense, sem grupos de rota `(grupo)`,
  sem rotas paralelas/interceptadas na v1.
- Templates `html/template` não são o mecanismo principal; o DSL `h` é. Um adaptador para
  `html/template` pode vir depois.
- Um app é um único binário; escala horizontal fica por conta do operador.
- Módulo publicado como `github.com/emersonjoe/trilha`.
