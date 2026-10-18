# Spec 177 — Desempenho: renders, bundle, requisições e cache, consultas, imagens

**Feature Branch**: `177-desempenho` | **Created**: 2026-10-18 | **Status**: Pronta para release (0.157.0)

- **Issue**: #309 — a fonte do escopo, da fronteira de aceitação e dos itens que ficaram para depois.
- **Versão**: 0.157.0

Forma curta em vários pacotes: cada achado é pequeno e tem teste próprio, nenhum inventa
convenção em `app/` nem muda símbolo público coberto pelo `api/current.txt`; spec única porque
nasceram da mesma auditoria e dividem fixture, documentação e release.

## Por quê

Uma auditoria das cinco frentes de desempenho (renders, bundle, requisições e cache, consultas,
imagens/fontes/scripts) achou trabalho que o framework faz a troco de nada em toda requisição
ou toda visita: nenhum byte comprimido, um buffer novo e uma cópia do documento por página,
alocação por elemento no `h`, fragmento vivo trocado sem ter mudado, a tela de assinaturas
lendo a tabela inteira, a busca em memória dobrando todo documento a cada consulta, 2,6 MB de
PNG num capítulo. E três defeitos de correção no `cache.Do` que a auditoria de cache provou com
teste: chave presa para sempre depois de um pânico, valor velho gravado depois do `Invalidate`,
e o cancelamento do líder derrubando quem esperava.

## Jornada e risco

- **Quem usa o app**: toda página (gzip, render) e as telas de lista (assinaturas, busca,
  fragmento vivo, chat). Pior impacto se quebrar: página que não abre (gzip com corpo ou ETag
  errados), fragmento que para de atualizar, combobox morto, busca que deixa de achar.
- **Quem escreve o app**: `cache.Do` travando uma rota inteira depois de um pânico; deploy
  deixando CSS velho congelado por um ano (o `immutable` do exemplo e da documentação).
- Jornadas críticas cobertas no navegador: "navegar e listar" (`TestUIPollSkipsUnchanged`),
  "preencher e salvar" (`TestUIComboboxLoadsOnDemand`), "interação rica"
  (`TestUIChatStreamsWithoutLayoutPerToken`, `TestUIIslandsShareOneObserver`).

## O que muda

**Requisições e cache**

- Arquivo de texto de `public/` e `Mounts` (`.css .js .mjs .json .map .svg .txt .html .xml
  .csv .md .wasm .webmanifest`, a partir de 1 KiB) vai em gzip para quem aceita
  (`Accept-Encoding` com `q` > 0), com `Vary: Accept-Encoding` nas duas codificações e ETag
  própria (`"<v>-gz"`, RFC 9110 §8.8.3). A cópia é feita uma vez por versão e guardada.
  Sem gzip: `Range`, cliente que recusa, arquivo pequeno ou que não encolhe, imagem. **Página
  não é comprimida** (decisão 1).
- `Vary` acumula: `Trilha-Fragment` entra ao lado do `Origin` do CORS em vez de apagá-lo
  (200 e 304).
- `cache.Do`: pânico no `fn` vira erro para quem espera e libera a chave (o pânico segue para
  o chamador); `Invalidate`/`Delete`/`Clear` durante o `fn` impede gravar o que ele leu; quem
  espera para no próprio `ctx` e, se só o `ctx` do líder acabou, pergunta de novo.
  `cache.Once` ganha o mesmo tratamento de pânico.
- `examples/blog` e a documentação (en/pt) param de recomendar `StaticCacheControl` imutável:
  a URL com o `?v=` certo já sai imutável sem opção; o campo vale para o resto.

**Renders**

- `h`: renderizar uma árvore pronta num buffer reaproveitado não aloca (classe mesclada
  escrita em pedaços, escape direto no writer com as mesmas cinco trocas do
  `html.EscapeString`).
- `writeHTML`: buffer de um `sync.Pool` (até 1 MiB volta ao pool) e envelope escrito à parte;
  só a resposta com ilha ou script de dev junta o documento numa peça.
- `ui.live.js`: resposta igual ao que está na tela não troca o fragmento nem dispara
  `trilha:hydrate` (comparação pela serialização do navegador nos dois lados, e pela última
  resposta).
- `ui.chat.js`: token entra com `append` e a rolagem acontece uma vez por quadro.
- `ui.island.js`: um `MutationObserver` para todas as ilhas.

**Consultas**

- Receita billing: `Store.PaginaAssinaturas(ctx, Filtro{Estado, Q, Limite, Deslocamento})
  ([]Assinatura, int, error)` — o banco filtra (`lower(email) LIKE ? ESCAPE '\'`, com `%` e `_`
  como texto), conta e corta; a tela usa ele. Migração nova `0101_billing_indexes.sql` com os
  índices de estado+data, data, emissão da fatura e a chave estrangeira das faturas (decisão 3).
  Contrato `cobrancatest` cobre página, total, caixa, curinga e estado em memória, SQLite e
  PostgreSQL.
- `trilha.SearchMemory`: o título e o corpo são dobrados no `Put`, e a posição de cada id é
  um mapa (o `Reindex` deixa de ser quadrático).

**Bundle, imagens, fontes e scripts**

- `ui.combo.js` (novo em `ui.Files`): o comportamento do combobox sai do `ui.js` e é
  carregado por ele na primeira página — ou fragmento — que tem `[data-ui-combo]`, pela URL
  versionada que o `ui.Head` põe em `data-ui-combo-js` na própria tag (tag escrita à mão cai
  no arquivo ao lado do `ui.js`).
- `ui.Avatar`: imagem com `loading="lazy"` e `decoding="async"`.
- `examples/cadastro` e `examples/assistente`: `TreeScript`/`ChatScript` saem do layout e vão
  para a página que tem a árvore/o chat.
- Site: as 14 capturas viram WebP (2.614.373 → 455.830 bytes, mesma resolução); toda figura
  leva `width`/`height` (lidos do cabeçalho do arquivo, WebP incluído, sem dependência), e o
  CSS usa `max-width` para não esticar a captura estreita.

## Decisões

1. **Gzip só para arquivo estático.** A página leva o token CSRF ao lado de texto que o
   visitante controla — o formato que o BREACH explora; o token não é mascarado por
   requisição. Comprimir HTML fica para quando houver máscara, com opção explícita.
2. **Cópia gzip por versão, no processo.** O custo é da primeira requisição depois do deploy;
   não há arquivo pré-comprimido para manter. `BestCompression` porque é feito uma vez.
3. **Índices em migração própria.** Migração aplicada é conferida pela soma (spec 126); editar
   a `0100` impediria todo projeto que já a aplicou de subir.
4. **Método novo no `Store` da receita, não troca de assinatura.** `Assinaturas` continua para
   o relatório e o CSV; a tela usa a página. Projeto com `Store` próprio só sente ao refazer o
   `trilha add billing`, e o compilador diz o método que falta.
5. **Combobox carregado pelo `ui.js`, não por `ComboboxScript(c)`.** O componente não recebe
   `*Ctx`, e pedir um script a mais quebraria em silêncio todo app que já usa o combobox.
   Atributo `data-ui-combo-js` (não `data-ui-combo`, que casaria com a própria tag).
6. **Orçamento do `ui.live.js` sobe de 6 para 7 KiB**: a comparação custou ~500 bytes e
   poupa uma troca e uma hidratação por tique.
7. **Achado descartado após conferência**: "tabela com tenant sem índice no CRUD" — `ID` nunca
   entra nas colunas e sempre há um campo, então o índice `(tenant_id, coluna)` sempre sai.
   "`Touch` O(n) a cada requisição de API" — já é limitado a uma escrita por minuto por chave.

## Fora de escopo (na issue #309)

Paginação de faturas e da fila do notify; o N+1 do digesto (tarefa diária); fontes do Google
no site; kit `ui` só nas páginas do site que o usam; minificar o `ui.js`; cópias antigas do kit
em `site/public` e `examples/*/public` (o site roda um `ui.js` anterior com o combobox dentro,
por isso não é afetado); contenção do `cache.Get`; `ui.live.js` não dispara `trilha:swap`
depois da troca (correção, não desempenho).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `compress/gzip`, `sync.Pool`, `unicode/utf8`; WebP lido pelo cabeçalho, sem `x/image` |
| III — gerador determinístico | goldens da receita billing regravados e revisados; arquivo novo de migração |
| IV — API pública | `api/current.txt` inalterado; `ui.Files` ganha um nome; `Store` da receita é código gerado |
| V — dev e prod | gzip nos dois ambientes; `Cache-Control` de dev inalterado |
| VI — teste primeiro | cada item tem teste que reprovava antes (lista em *Tarefas*); cenários novos com caso no `JORNADAS.md` |
| VII — segurança por padrão | HTML fora do gzip (BREACH); `Vary` não perde `Origin`; LIKE escapado e por parâmetro |

## Segurança e privacidade

- **Fronteiras**: navegador → arquivos estáticos (nova codificação, mesma origem e conteúdo);
  navegador → tela de assinaturas (busca agora vai ao SQL, sempre por parâmetro, com `%`/`_`
  escapados); `cache` compartilhado entre requisições (o valor lido antes de uma invalidação
  não é mais servido — ASVS V1.2/V8 de integridade do dado em cache).
- **ASVS 5.0 nível 2**: V3.4 (cabeçalhos: `Vary` correto para caches compartilhados com CORS),
  V1.2.4 (consulta parametrizada), V3.5 (CSRF preservado: página não comprimida), V16.5
  (pânico tratado sem travar o processo).
- **Top 10:2025**: A05 Injection (LIKE parametrizado e escapado), A02 Security Misconfiguration
  (`immutable` em arquivo sem versão), A04 Cryptographic Failures/BREACH (decisão 1).
- **Exceções**: nenhuma. Nenhum dado pessoal novo; logs inalterados.

## Tarefas

- [x] T001 Requisições e cache — `cache/flight_test.go` (5 casos travavam ou reprovavam),
  `compress_test.go` (`TestStaticGzip`, `TestVaryKeepsWhatWasThere` reprovavam),
  `blog_test` com `?v=` errado; `cache/typed.go`, `cache/cache.go`, `compress.go`,
  `static.go`, `render.go`, `httpcache.go`, blog, docs en/pt.
- [x] T002 Renders — `h/alloc_test.go` (277 → 0 alocações), `render_alloc_test.go`
  (3,9× → < 0,5× o tamanho da página); cenários `TestUIPollSkipsUnchanged`,
  `TestUIChatStreamsWithoutLayoutPerToken`, `TestUIIslandsShareOneObserver` reprovavam;
  `h/node.go`, `render.go`, `ui.live.js`, `ui.chat.js`, `ui.island.js` e cópias.
- [x] T003 Consultas — contrato `cobrancatest` (não compilava sem o método),
  `search_alloc_test.go` (alocação por documento), equivalência `hasWordPrefix`/`firstMatch`;
  receita billing, migração `0101`, goldens, `search.go`, `CtxPackCost` 356 → 366.
- [x] T004 Bundle e mídia — `TestUIComboboxLoadsOnDemand` reprovava; `ui.combo.js`, `ui.Head`,
  `ui.Files`; testes dos exemplos (`/voz`, `/ficha`) reprovavam; `TestMisc` (avatar);
  `TestDocumentationImagesFitTheBudget`/`TestDocumentationFiguresHaveTheirSize` (37 falhas),
  `TestBlockImageSize`, `TestWebPSize`.
- [x] T005 Documentação en/pt (dev e produção, referência de app, referência do ui, receita de
  cobrança, CLI), CHANGELOG, ROADMAP, versão.
- [ ] T006 `scripts/release.sh 0.157.0 --issues "309"`.

## Aceitação

- **SC-001** `/ui.css` com `Accept-Encoding: gzip` volta em gzip, descompacta nos mesmos bytes,
  tem ETag própria e revalida com 304; sem o cabeçalho, com `Range` ou `q=0`, volta como está.
- **SC-002** Uma página nunca sai comprimida.
- **SC-003** Depois de um `fn` que entra em pânico, o próximo `cache.Do` da mesma chave
  responde; um `Invalidate` durante o `fn` deixa a chave vazia.
- **SC-004** O fragmento vivo que respondeu o mesmo HTML é o mesmo elemento depois de três
  tiques, sem `trilha:hydrate`.
- **SC-005** A tela de assinaturas lê uma página do banco; os planos de execução usam índice
  (SQLite: `SEARCH ... USING INDEX billing_subscriptions_state`, sem `TEMP B-TREE`).
- **SC-006** Página sem combobox não baixa `ui.combo.js`; a com combobox baixa a URL
  versionada e escolhe pelo teclado.
- **SC-007** Nenhuma imagem de `site/public/docs` passa de 150 KB e toda figura tem
  `width`/`height`.

## Evidências

Medições (Go 1.24.7, linux/amd64, 4 núcleos):

| O quê | Antes | Depois |
|---|---|---|
| `BenchmarkPage_Trilha` (`bench/`) | 30.194 B, 500 allocs, ~55 µs | 16.797 B, 303 allocs, ~37 µs |
| `h`: árvore de 4 KB num buffer reaproveitado | 4.912 B, 277 allocs, 18,4 µs | 0 B, 0 allocs, 8,8 µs |
| Página de ~97 KB, bytes alocados por requisição | 373.618 (3,9×) | < 48 KB (< 0,5×) |
| Primeira visita, kit (`ui.css`+`ui.js`+`ui.theme.css`) | 82.392 B | 21.288 B (gzip) |
| `ui.js` em toda página | 35.927 B | 31.810 B (+ `ui.combo.js` 5.080 B só onde há combobox) |
| Busca em memória, 1.000 docs | 27,2 ms | 0,44 ms |
| `SearchMemory` `Put` de 20.000 docs | 1,18 s | 23 ms |
| Capturas do capítulo da nuvem | 2.614.373 B | 455.830 B |

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` (`go test -json ./...`) | | | |
| Navegador (Chromium), cenários + módulo | `UITEST_REQUIRED=1 make test-ui` | | | |
| SQL (SQLite e PostgreSQL) | `SQLTEST_POSTGRES=1 make test-sql` | | | |

Ambiente desta execução: o Chromium do container (revisão 1194) servido no lugar da 1234 que o
driver pede, porque `cdn.playwright.dev` está fora da política de rede; Firefox e WebKit não
estão instalados, então os três motores ficam para a CI. O PostgreSQL rodou com `initdb` e
`pg_ctl` como o usuário `postgres` (o container roda como root, que o `initdb` recusa).
