# Feature Specification: Versionar assets estáticos (cache busting)

**Feature Branch**: `017-cache-busting` | **Created**: 2026-09-05 | **Status**: Implementada (v0.8.0)
**Input**: "os assets do site não têm versão na URL, então nos dez minutos seguintes a cada
publicação um visitante pode pegar HTML novo com JS antigo. Se quiser, viro isso numa spec
pequena de cache busting no `trilha export`." — "sim faça isso"

## O problema

O site do Trilha é publicado no GitHub Pages, que serve tudo com cache de dez minutos e
não deixa configurar cabeçalho. Como `/site.css` e `/site.js` têm sempre o mesmo endereço,
uma publicação produz uma janela em que o navegador já pegou o HTML novo — que não estava em
cache, ou expirou antes — e continua com o CSS e o JS antigos. O sintoma é uma página
quebrada por alguns minutos depois de cada publicação, e ninguém consegue reproduzir depois.

O mesmo vale para qualquer app Trilha atrás de CDN, e a situação piora quando o app segue o
conselho da própria documentação: `StaticCacheControl = "public, max-age=31536000,
immutable"` num endereço sem versão significa que um visitante recorrente pode ficar **um
ano** com o CSS velho. Hoje o `examples/blog` faz exatamente isso, com um comentário que
promete um versionamento que não existe.

A causa é uma só: o endereço do arquivo não muda quando o conteúdo muda.

## Escopo

A decisão é versionar por **parâmetro de consulta** (`/site.css?v=8f3a1c92`), não por nome
de arquivo com hash:

- O arquivo continua existindo no mesmo caminho, então um HTML antigo em cache continua
  funcionando — ele só carrega a versão nova do CSS, que é o pior caso aceitável. Com nome
  hasheado, o HTML antigo apontaria para um arquivo que a publicação apagou: 404 e página
  sem estilo nenhum.
- O `trilha export` não precisa escrever cópias, e o diretório publicado não acumula
  versões antigas.
- CDNs e navegadores tratam a URL inteira como chave, então `?v=` novo é sempre um objeto
  novo — que é o problema a resolver.

Fora de escopo: renomear arquivos, pipeline de build de assets (minificação, bundling),
`Subresource Integrity`, e versionar o que não está em `Config.Public`.

## Cenários

1. **Publicação do site** — a pessoa altera `site.css` e publica. Um visitante que ainda tem
   o HTML antigo em cache continua vendo a página antiga, coerente; assim que pega o HTML
   novo, ele pede `?v=` novo e recebe o CSS novo, sem passar por um estado quebrado.
2. **App atrás de CDN** — o app usa `c.Asset("/app.js")` no layout e
   `StaticCacheControl` longo. O arquivo pode ser cacheado por um ano com segurança, porque
   a URL muda a cada alteração.
3. **Desenvolvimento** — em `dev` o hash acompanha o arquivo em disco a cada recarga, então
   editar o CSS e atualizar a página basta; nada de `Cmd+Shift+R`.
4. **Arquivo inexistente** — `c.Asset("/nao-existe.css")` devolve o caminho sem versão em vez
   de quebrar a página, e registra um aviso; um erro de digitação não derruba o app.
5. **Exportação estática** — o HTML gerado pelo `trilha export` já sai com as versões, sem
   nenhuma configuração adicional.

## Requisitos funcionais

- **FR-001** `Ctx.Asset(path)` devolve o caminho com o prefixo de `BasePath` e um parâmetro
  `v` derivado do conteúdo do arquivo em `Config.Public`.
- **FR-002** O hash é calculado uma vez por arquivo e guardado; em `Env == Dev` ele é
  recalculado quando o arquivo muda (mtime e tamanho).
- **FR-003** Um pedido cujo `v` corresponde ao hash atual é servido com
  `Cache-Control: public, max-age=31536000, immutable`, sobrepondo `StaticCacheControl`.
  Sem `v`, ou com `v` divergente, vale a regra de hoje.
- **FR-004** Caminho desconhecido, diretório ou caminho inválido devolvem o caminho
  original, sem versão, com aviso no log (uma vez por caminho).
- **FR-005** `ui.Head` e o layout do site usam `Ctx.Asset`; o `examples/blog` deixa de
  prometer versionamento que não faz e passa a fazê-lo.
- **FR-006** O `trilha export` não ganha opção nova: as páginas exportadas herdam as URLs
  versionadas por serem renderizadas pelo mesmo layout.
- **FR-007** Custo por requisição inalterado: nenhuma alocação nova no caminho quente de
  quem não usa `Asset`, e nenhuma leitura de disco por requisição servida.
- **FR-008** `trilha audit` avisa quando `StaticCacheControl` contém `immutable` e nenhum
  arquivo do projeto chama `.Asset(` — a combinação que congela um asset por um ano.

## Critérios de aceitação

- Alterar um arquivo em `public/` muda o `?v=` da próxima renderização.
- Dois processos com o mesmo conteúdo produzem o mesmo `?v=` (o hash é do conteúdo, não da
  hora do build), então o cache não é invalidado por uma publicação que não mudou nada.
- O site continua passando em `TestInternalLinksResolve` e nos testes de base path.
