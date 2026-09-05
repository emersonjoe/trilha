# Trilha Constitution

Trilha é um framework web para Go no estilo Next.js: roteamento por arquivos, layouts
aninhados, rotas de API, middleware, servidor de desenvolvimento com recarga automática e
um único binário de produção. Este documento fixa os princípios que toda feature deve obedecer.

## Core Principles

### I. Convenção sobre configuração (NON-NEGOTIABLE)
A estrutura de pastas em `app/` é a única fonte de verdade das rotas: `page.go` responde uma
página, `route.go` responde uma API, `layout.go` envolve os descendentes, `middleware.go`
intercepta a subárvore, `not_found.go` e `error.go` tratam falhas. Nenhuma rota é registrada
manualmente pelo usuário. Segmentos dinâmicos usam nome de pasta válido em import path do Go:
`slug_` (um segmento) e `slug__` (catch-all). Toda convenção nova precisa de exemplo em
`examples/` e de teste que a exercite antes de ser documentada.

### II. Só biblioteca padrão em runtime
O módulo `trilha` (runtime e CLI) depende apenas da biblioteca padrão do Go (`net/http`,
`html/template`, `go/ast`, `os/exec`...). O roteador é o `http.ServeMux` do Go 1.22+ com
patterns por método e `{param}`/`{rest...}`. HTML é gerado por um DSL de funções Go tipadas
(`h.Div`, `h.Text`) com escape por padrão; `h.Raw` é a única porta para HTML sem escape e
deve ser gritante no código. Dependências de terceiros só entram em `examples/` ou como
adaptadores opcionais em módulos separados, nunca no núcleo.

### III. Geração de código explícita, sem mágica em runtime
A descoberta de rotas acontece em tempo de build: a CLI varre `app/`, analisa os arquivos com
`go/ast` e gera um único arquivo `trilha_gen.go` (`package main`) que importa cada pacote de
rota e registra os handlers com tipos verificados pelo compilador. Nada de `reflect` para
descobrir handlers, nada de plugins carregados em runtime. O arquivo gerado é determinístico
(mesma entrada → bytes idênticos), tem cabeçalho `// Code generated ... DO NOT EDIT.` e é
commitado para que `go build ./...` funcione sem a CLI.

### IV. Contrato de handler pequeno e estável
Toda função de rota recebe um único `*trilha.Ctx` e devolve `error`. Páginas devolvem
`(h.Node, error)`; layouts recebem `(c *trilha.Ctx, children h.Node)`; rotas de API exportam
`GET`, `POST`, `PUT`, `PATCH`, `DELETE` com assinatura `func(c *trilha.Ctx) error`.
Erros são valores: `trilha.ErrNotFound` cai na página 404, `trilha.Redirect(...)` devolve
um redirecionamento, qualquer outro erro cai na página de erro (com stack em dev, opaco em
produção). Nenhum handler chama `panic` para controle de fluxo; `recover` existe só na borda.

### V. Desenvolvimento com recarga em < 2 s, produção em um binário
`trilha dev` observa `app/`, `public/` e `*.go`, regenera, recompila e reinicia o processo;
o navegador recarrega sozinho via SSE sem proxy. Um ciclo editar→ver deve ficar abaixo de
2 s em um app de exemplo. `trilha build` produz um único binário estático com `public/`
embutido via `embed`; o binário não depende de arquivos externos nem da CLI. Sem `dev` e
`prod` divergirem em comportamento além de: stack traces, live-reload e cache de estáticos.

### VI. Teste primeiro no núcleo, exemplo como teste de integração
Scanner de rotas, gerador, DSL de HTML e o pipeline de renderização têm testes unitários
escritos antes da implementação (`go test`, sem framework externo), incluindo casos de
escape de HTML, precedência de rotas e determinismo do gerador (golden files). O app em
`examples/blog` é executado por testes de integração com `httptest` cobrindo cada convenção.
Nenhuma feature é "pronta" sem `go vet ./...` e `go test ./...` verdes na raiz.

### VII. Segurança por padrão
Todo texto passa por escape contextual; cabeçalhos `X-Content-Type-Options: nosniff`,
`Referrer-Policy` e `X-Frame-Options` saem por padrão; arquivos estáticos nunca servem fora
de `public/` (sem path traversal); corpo de requisição tem limite configurável (1 MiB
padrão); formulários têm proteção CSRF por token com cookie `SameSite=Lax`; erros em
produção não vazam caminho de arquivo nem stack. Logs são estruturados (`log/slog`) e nunca
registram corpo de requisição nem cookies.

## Estilo e idioma

Código, identificadores e comentários do framework em inglês (é uma biblioteca pública);
documentação, specs e mensagens da CLI em português do Brasil, com README bilíngue quando
houver. API pública pequena: cada símbolo exportado precisa de doc comment e de um uso em
`examples/`. Compatível com as duas últimas versões estáveis do Go.

## Fluxo de trabalho

Toda mudança começa por uma spec em `specs/NNN-nome/` (spec-kit): spec → plan → tasks →
implement. O `Constitution Check` do plano deve listar cada princípio e como a feature o
respeita; violações precisam de justificativa em "Complexity Tracking". Commits pequenos
por tarefa; `gofmt` e `go vet` limpos; mensagens de commit sem trailer de coautoria.

## Governance

Esta constituição prevalece sobre qualquer outra prática do repositório. Emendas exigem:
registro da mudança neste arquivo com nova versão semântica, atualização dos templates em
`.specify/templates/` que dependam do princípio alterado, e migração dos exemplos afetados.
Revisões de código verificam aderência aos princípios I–VII antes de aprovar.

**Version**: 1.0.0 | **Ratified**: 2026-09-04 | **Last Amended**: 2026-09-04
