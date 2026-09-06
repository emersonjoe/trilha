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

A superfície pública é escrita e vigiada. O `API.md` diz quais pacotes têm garantia —
`trilha`, `h`, `ui`, `tmpl`, `ai`, `ai/mcp`, `auth`, `cache` — e o que fica de fora
(`internal/`, saída da CLI, HTML do `ui`, arquivo gerado); o `api/current.txt` lista os
símbolos exportados desses pacotes e um teste falha quando a lista muda, para que a remoção
apareça na revisão em vez de aparecer na compilação de quem usa. Símbolo coberto não some sem
ciclo: marca `Deprecated:` no doc comment dizendo o substituto, linha em `Deprecated` no
`CHANGELOG.md` e pelo menos uma versão menor de convivência — exceto correção de segurança que
não dê para fazer compatível, anunciada como tal.

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

## Estilo, idioma e interface

O kit `ui` (pacote `ui` + `public/ui.theme.css`, `ui.css`, `ui.js`) é a interface padrão dos
projetos gerados e dos exemplos: componentes tipados sobre CSS prefixado, copiados para o
projeto e customizáveis, com contrato de tema compatível com o shadcn/ui (MIT, com aviso em
`THIRD_PARTY_NOTICES.md`). O kit não adiciona dependências nem obriga JavaScript; toda
página deve continuar utilizável sem `ui.js`.


Código, identificadores, comentários e mensagens de erro e de log do framework (runtime,
scanner, gerador, scaffold) em inglês: é uma biblioteca pública e o ecossistema Go é em
inglês. **Tudo que é voltado ao público é em inglês por padrão, com tradução em português
do Brasil publicada no mesmo commit**: o site de documentação (inglês em `/`, português em
`/pt`, com `hreflang` e link para a tradução de cada página), `README.md` + `README.pt-BR.md`,
os arquivos de comunidade (`CONTRIBUTING`, `GOVERNANCE`, `SECURITY`, `SUPPORT`,
`CODE_OF_CONDUCT`, traduzidos em `docs/pt-BR/`), as mensagens da CLI (idioma por
`TRILHA_LANG`/`LANG`, inglês quando indefinido) e o projeto gerado por `trilha new`
(`--lang`). `CHANGELOG.md` e templates de issue/PR só em inglês. Uma página, string ou
arquivo público sem a tradução correspondente é bug: o teste do site confere que as duas
locales têm as mesmas páginas e as mesmas demos. As specs em `specs/` e esta constituição
ficam em português do Brasil, idioma de trabalho do mantenedor. Os apps em `examples/` são
em português (domínio próprio) e a documentação avisa isso ao citá-los.
API pública pequena: cada símbolo exportado precisa de doc comment e de um uso em
`examples/`. Compatível com as duas últimas versões estáveis do Go.

## Fluxo de trabalho

Toda mudança começa por uma spec em `specs/NNN-nome/` (spec-kit): spec → plan → tasks →
implement. O `Constitution Check` do plano deve listar cada princípio e como a feature o
respeita; violações precisam de justificativa em "Complexity Tracking". Commits pequenos
por tarefa; `gofmt` e `go vet` limpos; mensagens de commit sem trailer de coautoria.

Mudança pequena — um pacote, sem convenção nova em `app/`, sem quebra de API pública, plano
que cabe em uma tela — pode usar a **spec curta**: um único `spec.md` a partir de
`.specify/templates/spec-curta-template.md`, com por quê, contrato, `Constitution Check` dos
princípios tocados, tarefas e aceitação. Mesma numeração, mesmo branch, mesma release; o que
muda é que plano e tarefas param de repetir a spec em três arquivos. Na dúvida, forma completa.

A **issue é a fonte do escopo**: a spec aponta para ela em vez de recopiar a lista de
implementação, e um fato verificado fora do repositório (o comportamento de um provedor, um
detalhe de RFC) é registrado na issue na primeira vez que alguém o levanta, para que ninguém
vá conferir de novo.

**Um dono da `main` por vez.** Duas frentes simultâneas no mesmo repositório só com divisão de
arquivos combinada antes e número de spec reservado; sem isso o custo do encontro — rebase,
renumeração de versão, conflito de tradução — supera o ganho do paralelismo.

Fechar uma spec é `scripts/release.sh X.Y.Z --issues "NN"` (ou `make release`): testar, fundir
por fast-forward, marcar a tag anotada, publicar a release com as notas do `CHANGELOG.md` e
fechar as issues. O ritual é sempre o mesmo, então é script, não passo a passo escrito à mão.
O branch entra **rebasado** — merge commit é recusado antes de qualquer escrita, porque o
ruleset da `main` também recusa —, e o script não troca de branch: a fusão é um `push` para o
remoto, então a `main` pode estar checada no worktree da outra sessão sem travar a release.

## Governance

Esta constituição prevalece sobre qualquer outra prática do repositório. Emendas exigem:
registro da mudança neste arquivo com nova versão semântica, atualização dos templates em
`.specify/templates/` que dependam do princípio alterado, e migração dos exemplos afetados.
Revisões de código verificam aderência aos princípios I–VII antes de aprovar.

**Version**: 1.4.1 | **Ratified**: 2026-09-04 | **Last Amended**: 2026-09-06 (a release não troca de branch e o branch entra rebasado)
