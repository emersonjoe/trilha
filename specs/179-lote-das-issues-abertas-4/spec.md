# Spec 179 — Lote das issues abertas 4: CSRF mascarado, HTML comprimido, pool do upstream, props de ilha por tipo

**Feature Branch**: `179-lote-das-issues-abertas-4` | **Created**: 2026-10-18 | **Status**: Pronta para release (0.159.0)

- **Issues**: #308, #305, #307, #303 — a fonte do escopo e dos critérios.
- **Versão**: 0.159.0

Forma curta em lote, como as specs 155, 157 e 158: quatro mudanças pequenas, cada uma com teste
próprio; #308 e #305 andam juntas porque a segunda só é segura com a primeira.

## Por quê

- **#308/#305** — O runtime não comprimia página nem JSON: um app migrado do Next.js perdia o
  gzip que tinha e precisava configurar o proxy (e lembrar de excluir o SSE). Comprimir HTML com
  o token CSRF igual em toda resposta, ao lado de texto que o visitante controla, abre o BREACH.
- **#307** — Sem `Transport`, o `Upstream` usava o `http.DefaultTransport`: duas conexões ociosas
  por host para a API que recebe todo o tráfego, e TCP (e TLS) de novo a cada rajada.
- **#303** — O `trilha gen` só tipava as props de uma ilha com struct literal na chamada; uma
  variável compartilhada por três ilhas saía `unknown`.

## Jornada e risco

"Segurança percebida" e "preencher e salvar": pior impacto, um POST legítimo recusado (token
mascarado mal verificado) ou um segredo extraído pelo tamanho da resposta. "Navegar e listar":
página que não abre ou stream que trava (compressão errada). Cobertos por unidade, pelo
`examples/blog` e pelos 44 cenários de navegador (todo formulário deles posta o token mascarado).

## O que muda

- **#308** — `Ctx.CSRFToken()` devolve `base64url(pad || segredo XOR pad)`, com pad novo por
  resposta e o mesmo valor dentro dela; o cookie não muda. A verificação desmascara e compara em
  tempo constante, e aceita também o segredo cru (o que uma página aberta antes do deploy
  posta), que não entrega nada: o HTML não o carrega mais. `TestClient` manda o token mascarado;
  o `PageSnapshot` troca por `{{CSRF}}` todo token que prova o cookie.
- **#305** — `Config.Compress` (vazio = ligado; `Off` ou `TRILHA_COMPRESS=off` desliga): resposta
  de texto (`text/*` menos `text/event-stream`, JSON, problem+json, XML, SVG, manifest) a partir
  de 1 KiB vai em gzip para quem aceita, com `Vary: Accept-Encoding` e ETag fraca (`W/`,
  RFC 9110 §8.8.3). Fora: `HEAD`, `Upgrade`, 204/206/304, `Content-Encoding` já presente (o gzip
  dos estáticos), imagens e binários. `Flush` passa pelo gzip, então `ui.Defer` e streams de HTML
  continuam progressivos. Writers em `sync.Pool`.
- **#307** — `trilha.UpstreamTransport()`: um `*http.Transport` compartilhado (clone do padrão,
  64 conexões ociosas por host, 256 no total, 90 s), usado por todo `Upstream` sem `Transport` e
  exportado para o cliente de API do próprio app. O `examples/local-login` o usa com timeout.
- **#303** — o tipo das props vem da expressão: literal, `&x`, variável (`:=`, `var x T`,
  `var x = …`), parâmetro da função, conversão `T(x)` e chamada a função do app que devolve o
  struct. A nota passa a dizer que formas são lidas.

## Decisões

1. **Mascarar, não randomizar o cookie.** Trocar o segredo por resposta quebraria abas abertas
   e formulários em paralelo; o pad por resposta tem o mesmo efeito contra o BREACH sem estado.
2. **Aceitar o token cru sem prazo.** Não enfraquece: quem tem o segredo cru já poderia
   mascará-lo; o que importa é que ele não sai mais no HTML.
3. **Compressão ligada nos dois ambientes**, como o gzip dos estáticos (princípio V).
4. **Decidir com 1 KiB guardado**, não pelo `Content-Length`: o render não o declara; o corpo
   menor que isso sai cru.
5. **Resolução de tipo pelo AST, sem `go/types`** (como a spec 176): cobre as formas do caso
   real; o que não é lido continua com nota.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `compress/gzip`, `crypto/rand`, `crypto/subtle`, `go/ast` |
| IV — API pública | `Config.Compress` e `UpstreamTransport()` novos, com doc comment e uso em `examples/`; `api/current.txt` regravado; nada removido |
| V — dev e prod | compressão igual nos dois |
| VI — teste primeiro | `TestCSRFTokenMaskedPerResponse`, `TestDynamicResponsesCompressed` e irmãos, `TestUpstreamKeepsConnectionsUnderLoad` (56 conexões com o transporte padrão), `TestPropsTypedByTheExpression` reprovavam antes |
| VII — segurança por padrão | token mascarado e compressão por padrão, juntos |

## Segurança e privacidade

- **Fronteiras**: navegador → página (token CSRF no HTML; compressão do corpo); app → API do
  upstream (pool de conexões, mesma origem fixa, nenhum dado novo).
- **ASVS 5.0 nível 2**: V3.5 (CSRF: token por resposta, verificação em tempo constante), V3.4
  (cabeçalhos: `Vary` e ETag corretos para caches compartilhados), V12/V13 (configuração segura
  do transporte de saída: timeout no cliente do exemplo).
- **Top 10:2025**: A01 (CSRF), A04 Cryptographic Failures (BREACH: segredo fora do corpo
  comprimido), A05 Security Misconfiguration (compressão de SSE desligada por tipo).
- **NIST SSDF**: PW.5/PW.7 (prática segura e revisão: o par #308/#305 entra no mesmo release);
  SECURITY-MODEL ganha a linha do BREACH.
- **Exceções**: nenhuma.

## Tarefas

- [x] T001 Testes que reprovavam: `csrf_mask_test.go`, `compress_dynamic_test.go`,
  `upstream_pool_test.go`, `TestPropsTypedByTheExpression`, `TestRespostasComprimidas` (blog).
- [x] T002 `csrf.go`, `testing.go`, `snapshot.go`, `compress.go`, `trilha.go`, `upstream.go`,
  `upstream_pool.go`, `internal/islands`; testes antigos que comparavam o token com o cookie
  passam a conferir que ele o prova; o teste gerado da receita publiclookup compara retratos.
- [x] T003 Documentação en/pt (dev e produção, referência de app, ctx, upstreams),
  SECURITY-MODEL en/pt, `api/current.txt`, golden do `islands.d.ts`.
- [x] T004 CHANGELOG, ROADMAP, versão.
- [ ] T005 Suíte, CI completa e `scripts/release.sh 0.159.0 --issues "308 305 307 303"`.

## Aceitação

- **SC-001** Duas respostas da mesma sessão trazem tokens diferentes, ambos aceitos; um
  adulterado ou de outra sessão é 403; o token cru antigo passa.
- **SC-002** Página e JSON de mais de 1 KiB voltam em gzip para quem aceita, descompactam no
  mesmo corpo, com `Vary` e ETag fraca que revalida em 304; SSE, imagem, corpo curto, `HEAD` e
  `Compress: Off` voltam crus; o que foi enviado com `Flush` chega antes do fim.
- **SC-003** Três rajadas de 20 requisições ao upstream abrem no máximo 30 conexões.
- **SC-004** As seis formas de props do `testdata/islands/app/jogo` saem tipadas, sem nota.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | | | |
| Navegador (Chromium local) | `UITEST_REQUIRED=1 make test-ui` | 48 | 0 | 0 |
