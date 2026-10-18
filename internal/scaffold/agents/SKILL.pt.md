name: trilha-quality
description: O checklist de qualidade do {{.Name}} (um app Trilha) — desempenho (renders, bundle, requisições e cache, consultas e N+1, imagens, fontes, scripts), padrões de projeto, código limpo, reúso, revisão pelo tamanho e segurança por OWASP e NIST. Use ao escrever qualquer mudança e de novo antes de dizer que terminou.
---

# Checklist de qualidade — {{.Name}}

Passe pelas seções que a mudança toca e depois pelo **Antes de terminar**. Cada item diz o que
fazer e o que procurar; "procure" é `grep -rn` em `app/` e `internal/`.

## 1. Renders

- Um valor de que o layout e a página precisam é calculado uma vez: `cache.Once(c, "user", …)`.
- Mudar um pedaço da tela responde esse pedaço: `c.Fragment()` no servidor,
  `ui.Swap`/`ui.Navigate` no cliente. Recarregar a página inteira para ordenar, filtrar ou
  salvar é bug.
- Rota cujo dado tem versão a declara: `if c.ETag(rev) { return nil, nil }` responde 304 antes
  de renderizar qualquer coisa. Fragmento com `ui.Poll` sem novidade chama `c.PollStop()` ou
  mantém o HTML idêntico — o cliente deixa a resposta idêntica em paz.
- Procure: trabalho no layout que a página também faz; `ui.Poll` com intervalo curto numa rota
  pesada; `h.Raw` montado por concatenação.

## 2. Bundle e carregamento sob demanda

- O `ui.Head(c)` carrega a base do kit. O comportamento de um componente é carregado pela
  página que tem o componente — `ui.TreeScript(c)`, `ui.ChatScript(c)`, `ui.LiveScript(c)`,
  `ui.NavigateScript(c)` —, nunca pelo layout para todas as páginas.
- Trecho lento de uma página é um `ui.Defer(c, id, url)`: a página responde agora, o trecho
  quando estiver pronto.
- Ilha (`c.Island`) é para estado que o servidor não guarda; link, formulário ou fragmento
  custam menos e funcionam sem JavaScript.
- Procure: `Script(c)` no `layout.go`; `<script src>` sem `defer`; pacote npm trazido para o
  que o kit já faz (`trilha ui components`).

## 3. Requisições e cache

- Todo arquivo de `public/` é ligado pelo `c.Asset("/x.css")`: a URL leva o hash do conteúdo e
  fica em cache por um ano, e um deploy nunca deixa ninguém com CSS velho. Deixe o
  `StaticCacheControl` curto.
- Leitura compartilhada entre requisições passa pelo `cache.Do(ctx, cache, cache.Key{Name, TTL,
  Tags}, fn)`; a escrita que a muda chama `Invalidate(tag)`. Nunca guarde dado de um usuário
  sob um nome compartilhado.
- Chamadas independentes a outros serviços rodam em paralelo, cada uma com o contexto da
  requisição.
- Chamada HTTP de saída usa um cliente com timeout e transporte compartilhado, nunca o
  `http.DefaultClient`. O link que a pessoa está para seguir pode ser antecipado com
  `ui.Prefetch()`.
- Procure: `http.DefaultClient`, `http.Get(`, `/style.css` escrito à mão sem `c.Asset`,
  `time.Sleep` num handler.

## 4. Consultas e N+1

- Nenhuma consulta dentro de laço. Dez linhas que precisam cada uma do seu autor são um
  `WHERE id IN (…)` ou um `JOIN`, não dez `SELECT`s. Um teste que conta as chamadas ao store
  prova.
- Toda lista é paginada pelo banco: `LIMIT`/`OFFSET` (o `store.Paginate` da receita store) e
  um `count(*)` com o mesmo `WHERE`. Nunca carregue a tabela para filtrar ou paginar em Go.
- Toda coluna de `WHERE`, `JOIN` ou `ORDER BY` tem índice; confira com `EXPLAIN` (sem varredura
  sequencial, sem ordenação temporária em tabela grande). Índice vai numa migração **nova** —
  migração aplicada é conferida pela soma.
- Nomeie as colunas (sem `SELECT *`); busca é `LIKE` com `store.Like` (escapado) e por
  parâmetro.
- Procure: `for … range` com `QueryContext`/`QueryRowContext`/chamada ao store dentro; `SELECT`
  sem `LIMIT` atrás de uma tela; coluna de `ORDER BY` que não aparece nas migrações.

## 5. Imagens, fontes e scripts

- Imagem diz o tamanho: `h.Img(h.Src(…), h.Alt(…), h.Width(w), h.Height(ht),
  h.Attr("loading", "lazy"), h.Attr("decoding", "async"))`. A primeira imagem da página (acima
  da dobra) não é lazy.
- Foto é WebP (ou AVIF), ícone é SVG (`ui.Icon`), e nada é maior que o tamanho em que aparece.
- Fontes: a pilha do sistema primeiro; fonte web é woff2 servido daqui com
  `font-display: swap` e `preload` do peso que aparece acima da dobra — nada de CSS de fonte
  de terceiros em toda página.
- Script é `defer` (ou módulo); script inline leva `trilha.NonceAttr(c)`. Nada de `@import`
  encadeado no CSS.

## 6. Padrões de projeto (GoF), onde tiram código

- **Strategy**: uma interface, backends trocáveis — o `Store` das receitas com implementação em
  memória e em SQL, provadas pelo mesmo contrato.
- **Decorator**: comportamento transversal embrulha o handler — `middleware.go`, não um `if`
  copiado em toda rota.
- **Adapter**: uma API de fora é embrulhada uma vez, em `internal/`, no formato de que o app
  precisa.
- **Factory**: `New…` devolve um valor pronto; nada de struct montado pela metade no chamador.
- **Observer**: eventos e `ui.Live`/SSE em vez de perguntar de novo o que pode ser avisado.
- Padrão que acrescenta um tipo e não poupa nada não é usado. Go prefere composição e
  interfaces pequenas, declaradas por quem consome.

## 7. Código limpo

- Uma função faz uma coisa e cabe na tela; o nome diz a intenção (`faturasPendentes`, não
  `dados2`).
- Erro é valor: devolva embrulhado com contexto (`fmt.Errorf("fatura %s: %w", id, err)`),
  nunca engolido, nunca `panic` para controle de fluxo.
- Nada de código morto, de código comentado, de opção que é sempre igual.
- O comentário diz por que algo é assim; o código já diz o quê.
- `gofmt` e `go vet` limpos (o `trilha check` roda os dois).

## 8. Reúso

- Antes de escrever, procure: `trilha ui components --json` (telas), `trilha add` (receitas),
  `trilha ui describe <Nome>` e `grep -rn` em `internal/` por uma função que já faz isso.
- A segunda cópia de um bloco vira uma função, no pacote que os dois chamadores já importam.
- Tipos e regras compartilhados moram em `internal/<domínio>`; o `app/` só liga a rota a eles.

## 9. Revisão pelo tamanho e pela leitura

- Releia o diff inteiro como quem vai revisá-lo. Para cada linha: dá para tirar? Dá para
  encurtar e continuar claro?
- Tire o que a mudança não precisava: auxiliar usado uma vez, opção que ninguém liga, log que
  sobrou da depuração.
- Ganha a menor mudança legível; uma linha esperta que precisa de comentário para ser entendida
  perde para duas simples.

## 10. Segurança (OWASP ASVS 5.0 nível 2, Top 10:2025, NIST SSDF 1.1)

- **Entrada**: `c.Bind` num struct com tags `validate`; a checagem que falha devolve o
  formulário com `FieldErrors`, nunca um 500. Os limites de tamanho continuam ligados.
- **Saída**: HTML passa pelo `h` (escapado); `h.Raw` nunca recebe o que alguém digitou.
- **Acesso**: autorização mora no `middleware.go` (política, papel, tenant) — esconder um link
  é cosmético. Negar por padrão.
- **Sessão e CSRF**: use os do framework; uma escrita numa `route.go` chamada pelo navegador
  manda o token CSRF.
- **SQL**: só parâmetros, nunca entrada concatenada; `LIKE` escapado.
- **Segredos**: do ambiente, nunca no código, no log ou no HTML.
- **Cadeia de suprimentos (SSDF)**: nenhuma dependência nova sem necessidade; o `trilha check`
  roda o `trilha audit`.
- Registre na mudança o que cruza uma fronteira de confiança e como é conferido.

## Antes de terminar

1. O `trilha check` passa.
2. Cada seção acima que a mudança toca foi conferida.
3. Um teste prova a mudança; bug corrigido tem teste que falhava antes da correção.
4. O diff foi relido pelo tamanho (seção 9).
