# Spec 178 — Desempenho: renders, bundle, cache, consultas, imagens

**Feature Branch**: `178-desempenho` | **Created**: 2026-10-18 | **Status**: Pronta para release (0.158.0)

- **Issue**: #309 — a fonte do escopo, da fronteira de aceitação e dos itens que ficaram para depois.
- **Versão**: 0.158.0

Forma curta em vários pacotes: cada achado é pequeno e tem teste próprio, nenhum inventa
convenção em `app/`; spec única porque nasceram da mesma auditoria e dividem fixture,
documentação e release. A compressão (#305), o CSRF mascarado (#308), o poll que não troca o
fragmento igual (#306) e o pool do upstream (#307) saíram em paralelo na spec 177
(`177-entrega-rapida`, 0.157.0); esta spec foi rebaseada sobre ela e não os repete.

## Por quê

Uma auditoria das cinco frentes de desempenho achou trabalho que o framework faz a troco de
nada: alocação por elemento no `h`, um buffer novo e uma cópia do documento por página, o chat
reconstruindo o texto e forçando layout a cada token, um observador do documento por ilha, a
tela de assinaturas lendo a tabela inteira, a busca em memória dobrando todo documento a cada
consulta, o combobox em toda página, 2,6 MB de PNG num capítulo. E três defeitos de correção no
`cache.Do` provados com teste: chave presa para sempre depois de um pânico, valor velho gravado
depois do `Invalidate`, e o cancelamento do líder derrubando quem esperava; mais o `Vary` que
apagava o `Origin` do CORS.

## Jornada e risco

- **Quem usa o app**: toda página (render), as telas de lista (assinaturas, busca), o chat, as
  ilhas e o combobox. Pior impacto: combobox morto, busca que deixa de achar, chat que não rola.
- **Quem escreve o app**: `cache.Do` travando uma rota inteira depois de um pânico; deploy
  deixando CSS velho congelado por um ano (o `immutable` do exemplo e da documentação).
- Navegador: `TestUIChatStreamsWithoutLayoutPerToken`, `TestUIIslandsShareOneObserver`,
  `TestUIComboboxLoadsOnDemand`.

## O que muda

**Cache e requisições**

- `cache.Do`: pânico no `fn` vira erro para quem espera e libera a chave (o pânico segue para o
  chamador); `Invalidate`/`Delete`/`Clear` durante o `fn` impede gravar o que ele leu; quem
  espera para no próprio `ctx` e, se só o `ctx` do líder acabou, pergunta de novo.
  `cache.Once` ganha o mesmo tratamento de pânico.
- `Vary` acumula (`addVary`): `Trilha-Fragment` e `Accept-Encoding` entram ao lado do `Origin`
  do CORS em vez de apagá-lo (200 e 304).
- `examples/blog` e a documentação (en/pt) param de recomendar `StaticCacheControl` imutável: a
  URL com o `?v=` certo já sai imutável; o campo vale para o resto.

**Renders**

- `h`: renderizar uma árvore pronta num buffer reaproveitado não aloca (classe mesclada escrita
  em pedaços, escape direto no writer com as mesmas cinco trocas do `html.EscapeString`).
- `writeHTML`: buffer de um `sync.Pool` (até 1 MiB volta ao pool) e envelope escrito à parte;
  só a resposta com ilha ou script de dev junta o documento numa peça.
- `ui.chat.js`: token entra com `append` e a rolagem acontece uma vez por quadro.
- `ui.island.js`: um `MutationObserver` para todas as ilhas.

**Consultas**

- Receita billing: `Store.PaginaAssinaturas(ctx, Filtro{Estado, Q, Limite, Deslocamento})` — o
  banco filtra (`lower(email) LIKE ? ESCAPE '\'`), conta e corta; a tela usa ele. Migração nova
  `0101_billing_indexes.sql` (estado+data, data, emissão da fatura, chave estrangeira das
  faturas). Contrato `cobrancatest` em memória, SQLite e PostgreSQL.
- `trilha.SearchMemory`: título e corpo dobrados no `Put`; posição de cada id num mapa (o
  `Reindex` deixa de ser quadrático).

**Bundle, imagens, fontes e scripts**

- `ui.combo.js` (novo em `ui.Files`): o comportamento do combobox sai do `ui.js` e é carregado
  por ele na primeira página — ou fragmento — com `[data-ui-combo]`, pela URL versionada que o
  `ui.Head` põe em `data-ui-combo-js`.
- `ui.Avatar`: `loading="lazy"` e `decoding="async"`.
- `examples/cadastro` e `examples/assistente`: `TreeScript`/`ChatScript` vão para a página que
  tem a árvore/o chat.
- Site: as 14 capturas viram WebP (2.614.373 → 455.830 bytes); toda figura leva
  `width`/`height` (lidos do cabeçalho, WebP incluído, sem dependência) e o CSS usa `max-width`.

## Decisões

1. **Índices em migração própria.** Migração aplicada é conferida pela soma (spec 126); editar
   a `0100` impediria todo projeto que já a aplicou de subir.
2. **Método novo no `Store` da receita, não troca de assinatura.** `Assinaturas` continua para
   o relatório e o CSV; a tela usa a página. Projeto com `Store` próprio só sente ao refazer o
   `trilha add billing`, e o compilador diz o método que falta.
3. **Combobox carregado pelo `ui.js`, não por `ComboboxScript(c)`.** O componente não recebe
   `*Ctx`, e pedir um script a mais quebraria em silêncio todo app que já usa o combobox.
   Atributo `data-ui-combo-js` (não `data-ui-combo`, que casaria com a própria tag).
4. **Achado descartado após conferência**: "tabela com tenant sem índice no CRUD" — `ID` nunca
   entra nas colunas e sempre há um campo, então o índice `(tenant_id, coluna)` sempre sai.
   "`Touch` O(n) a cada requisição de API" — já é limitado a uma escrita por minuto por chave.

## Fora de escopo (na issue #309)

Paginação de faturas e da fila do notify; o N+1 do digesto (tarefa diária); fontes do Google no
site; kit `ui` só nas páginas do site que o usam; minificar o `ui.js`; cópias antigas do kit em
`site/public` e `examples/*/public` (o site roda um `ui.js` anterior com o combobox dentro, por
isso não é afetado); contenção do `cache.Get`.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `sync.Pool`, `unicode/utf8`; WebP lido pelo cabeçalho, sem `x/image` |
| III — gerador determinístico | goldens da receita billing regravados e revisados; arquivo novo de migração |
| IV — API pública | `api/current.txt` inalterado; `ui.Files` ganha um nome; `Store` da receita é código gerado |
| VI — teste primeiro | cada item tem teste que reprovava antes (lista em *Tarefas*); cenários novos com caso no `JORNADAS.md` |
| VII — segurança por padrão | `Vary` não perde `Origin`; LIKE escapado e por parâmetro |

## Segurança e privacidade

- **Fronteiras**: navegador → tela de assinaturas (a busca vai ao SQL, sempre por parâmetro, com
  `%`/`_` escapados); `cache` compartilhado entre requisições (o valor lido antes de uma
  invalidação não é servido); respostas com CORS em caches compartilhados (`Vary`).
- **ASVS 5.0 nível 2**: V3.4 (`Vary` correto com CORS), V1.2.4 (consulta parametrizada), V16.5
  (pânico tratado sem travar a chave).
- **Top 10:2025**: A05 Injection (LIKE parametrizado e escapado), A02 Security Misconfiguration
  (`immutable` em arquivo sem versão).
- **Exceções**: nenhuma. Nenhum dado pessoal novo; logs inalterados.

## Tarefas

- [x] T001 Cache e requisições — `cache/flight_test.go` (5 casos travavam ou reprovavam),
  `vary_test.go`, blog com `?v=` errado.
- [x] T002 Renders — `h/alloc_test.go` (277 → 0 alocações), `render_alloc_test.go` (3,9× → <
  0,5×), `TestUIChatStreamsWithoutLayoutPerToken`, `TestUIIslandsShareOneObserver`.
- [x] T003 Consultas — contrato `cobrancatest`, `search_alloc_test.go`, equivalência
  `hasWordPrefix`/`firstMatch`; migração `0101`, goldens, `CtxPackCost` 356 → 366.
- [x] T004 Bundle e mídia — `TestUIComboboxLoadsOnDemand`, testes dos exemplos, `TestMisc`
  (avatar), orçamento e tamanho das figuras do site, `TestBlockImageSize`, `TestWebPSize`.
- [x] T005 Rebase sobre a `main` com a spec 177 (`177-entrega-rapida`): fica a implementação
  dela para #305–#308; as duplicatas daqui saíram.
- [x] T006 Documentação en/pt, CHANGELOG, ROADMAP, versão.
- [ ] T007 Release 0.158.0 (#309).

## Aceitação

- **SC-001** Depois de um `fn` que entra em pânico, o próximo `cache.Do` da mesma chave
  responde; um `Invalidate` durante o `fn` deixa a chave vazia.
- **SC-002** Uma página com CORS e `c.ETag` traz `Vary` com `Origin` e `Trilha-Fragment`, no 200
  e no 304.
- **SC-003** A tela de assinaturas lê uma página do banco; os planos usam índice (SQLite: `SEARCH
  ... USING INDEX billing_subscriptions_state`, sem `TEMP B-TREE`).
- **SC-004** Página sem combobox não baixa `ui.combo.js`; a com combobox baixa a URL versionada
  e escolhe pelo teclado.
- **SC-005** Nenhuma imagem de `site/public/docs` passa de 150 KB e toda figura tem
  `width`/`height`.

## Evidências

| O quê | Antes | Depois |
|---|---|---|
| `BenchmarkPage_Trilha` (`bench/`) | 30.194 B, 500 allocs, ~55 µs | 16.797 B, 303 allocs, ~37 µs |
| `h`: árvore de 4 KB num buffer reaproveitado | 4.912 B, 277 allocs, 18,4 µs | 0 B, 0 allocs, 8,8 µs |
| Página de ~97 KB, bytes alocados por requisição | 373.618 (3,9×) | < 48 KB (< 0,5×) |
| `ui.js` em toda página | 35.927 B | 31.810 B (+ `ui.combo.js` 5.080 B só onde há combobox) |
| Busca em memória, 1.000 docs | 27,2 ms | 0,44 ms |
| `SearchMemory` `Put` de 20.000 docs | 1,18 s | 23 ms |
| Capturas do capítulo da nuvem | 2.614.373 B | 455.830 B |

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | | | |
