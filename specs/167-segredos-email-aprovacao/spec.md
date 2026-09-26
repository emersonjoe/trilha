# Feature Specification: Segredos em arquivo, dependência opcional, e-mail assíncrono e aprovação por link

**Feature Branch**: `167-segredos-email-aprovacao` | **Created**: 2026-09-25 | **Status**: Entregue (0.148.0)
**Input**: issues [#283](https://github.com/emersonjoe/trilha/issues/283),
[#284](https://github.com/emersonjoe/trilha/issues/284),
[#285](https://github.com/emersonjoe/trilha/issues/285) e
[#286](https://github.com/emersonjoe/trilha/issues/286), todas medidas no Trilha Work. As
issues são a fonte do escopo; aqui ficam as decisões e as diferenças em relação à proposta.
Pedido do mantenedor na sessão: "corrija as issues do github e depois veja isso". O "isso" é a
medição do bench que parou no lado baseline, e a correção entra na mesma versão.

## O que muda

1. **#283 — segredos em arquivo.** `ConfigFromEnv` lê `TRILHA_SECRET_FILE` e
   `TRILHA_SECRET_PREVIOUS_FILE`. `mail.FromEnv` lê `TRILHA_MAIL_URL_FILE` e/ou
   `TRILHA_MAIL_PASSWORD_FILE`, e neste caso a URL vai sem senha. O arquivo é lido uma vez, no
   boot, com `TrimSpace`. Pânico no boot quando:
   - as duas formas estão definidas;
   - o arquivo não abre;
   - há arquivo de senha sem URL ou sem usuário.

   O `trilha audit` lê o segredo pelo `_FILE` e avisa quando há senha em `TRILHA_MAIL_URL` fora
   de dev. O `trilha dev` não injeta segredo efêmero quando existe `TRILHA_SECRET_FILE`.
2. **#284 — `trilha.Lookup[T](b) (T, bool)`.** O `Use` passa a ser escrito sobre o `Lookup`. Um
   valor de outro tipo sob a chave de T continua sendo pânico, porque é bug no `Setup` e não
   dependência ausente.
3. **#286 — `mail.Options.Tasks` e `Backoff`.** O `Send` monta a mensagem na hora, de modo que
   um erro de endereço ou de cabeçalho continua voltando no `Send`. Depois enfileira
   `mail.TaskName` (`"mail.send"`) no runner e retorna.
   - Nova tentativa, por padrão depois de 5 s, 30 s e 2 min, para erro temporário: 4xx,
     `DeadlineExceeded`, `*net.OpError` ou erro com `Temporary() true`.
   - Nenhuma nova tentativa para 5xx nem para o resto.
   - O `Timeout` passa a valer por tentativa e ganhou a nota sobre antispam no envio.
   - Como a tarefa não carrega payload, a mensagem fica num mapa em memória pela chave da tarefa.
     Um reinício a perde, e a tarefa fica como interrompida. Isso está documentado.
4. **#285 — link de decisão no `approval`.**
   - `q.Link(ctx, id, LinkOptions{To, TTL})` devolve um token de 32 bytes em base64url. O store
     guarda só o SHA-256.
   - `q.ByLink(c, token)` mostra e não decide. `q.DecideByLink(c, token, estado, motivo)`
     consome o link com `TakeLink` antes de decidir, então é de uso único.
   - A decisão fica com `By = "link:<To>"`, também no audit. As duas funções põem
     `Cache-Control: no-store`, `Referrer-Policy: no-referrer` e `X-Robots-Tag: noindex`.
   - Toda falha é o mesmo `ErrLinkInvalid`, um 404.
   - O relógio esquece os links vencidos (`PurgeLinks`). O `LinkStore` é interface opcional do
     `Store`: o `Memory()` a implementa, e um store sem ela guarda os links em memória.
   - **Diferença em relação à proposta:** a issue pedia `q.LinkHandler(render)` montado com
     `a.Mount`. O Trilha não tem `Mount`, porque as rotas são arquivos. Por isso a página é do
     app (`app/aprovar/{token}/page.go`), e o CSRF do POST é o da rota de página, que o framework
     já confere. Escrever o handler fora da árvore `app/` criaria uma segunda forma de rota.
5. **Bench.** O `Workspace` passa a levar `bench/agent/apps` e `bench/agent/baseline`, que são
   as fixtures lidas pelo `Build` com `AppDir` e pelo `BuildBaseline`. O resto de `bench/`
   continua fora. Antes, a medição da série parava no primeiro baseline com
   `lstat …/bench/agent/baseline/comments: no such file or directory`. Os cenários s5–s8 do lado
   trilha teriam parado do mesmo jeito.

## Constitution Check

- **II:** só biblioteca padrão. O `mail` passa a importar `task`, que importa a raiz; a raiz não
  importa `mail`, então não há ciclo. `TestNoExternalDeps` verde.
- **IV:** só adições públicas. São elas:
  - `trilha.Lookup`;
  - `mail.Options.Tasks`, `mail.Options.Backoff` e `mail.TaskName`;
  - no `approval`: `Link`, `ByLink`, `DecideByLink`, `LinkOptions`, `Link` (tipo), `LinkStore`
    e `ErrLinkInvalid`.

  O `Store` do `approval` não ganhou método, para não quebrar implementações de terceiros.
- **VI:** teste primeiro, por bloco:
  - `TestSecretFromFile`, `TestFromEnvLeSegredosDeArquivo` e `TestAuditoriaDeEmail`;
  - `TestLookupOptional`;
  - `TestSendAssincrono*` e `TestTemporario`;
  - `TestDecidirPorLink` e `TestLinkExpiraEEsquecido`;
  - `TestWorkspaceCarriesTheFixtures`.

## Segurança e privacidade (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras de confiança**:
  - plataforma → processo: o segredo vem do ambiente ou de arquivo;
  - app → servidor SMTP: o envio sai do caminho da requisição;
  - quem recebeu o e-mail → página do link: sem sessão, e o token é a credencial.
- **Controles**:
  - **V13 configuração:** segredo fora do ambiente, que aparece em `docker inspect` e em
    `/proc/<pid>/environ`. Config ambígua falha no boot. O audit aponta senha no ambiente.
  - **V7 logs:** uma linha por tentativa, com assunto e destinatários, nunca o corpo.
  - **V6/V7 tokens:** 256 bits de `crypto/rand`, guardados como hash (um vazamento da tabela não
    gera links válidos), uso único, TTL e resposta idêntica para toda falha.
  - **V3/V14 página:** `no-store`, `no-referrer` e `noindex`.
  - **V4 CSRF:** GET não muda estado, e o POST passa pela checagem da rota de página. A
    pré-visualização de link do cliente de e-mail não decide nada.
  - **V2 disponibilidade:** o formulário não fica preso no SMTP.
  - Referências no OWASP Top 10:2025: A02, A04, A07 e A09.
- **Exceções registradas**:
  - A mensagem pendente e os links vivem em memória por padrão. Um reinício perde e-mails ainda
    não entregues, e a tarefa fica como interrompida. A caixa de saída durável é trabalho do
    provedor.
  - Consultar o link pelo hash num map não é comparação em tempo constante do segredo, mas o
    tempo da consulta não revela nada sobre um token que ninguém tem.

## Tarefas

- [x] **T01** — #283: `envOrFile`/`fromFile`, audit e `trilha dev`. *Aceite:* testes de arquivo
  e de pânico.
- [x] **T02** — #284: `Lookup`, `Use` sobre ele e a documentação do `Provide`. *Aceite:*
  `TestLookupOptional`.
- [x] **T03** — #286: `mail/async.go`. *Aceite:* o `Send` volta antes do servidor; 4xx e timeout
  tentam de novo; 5xx não; as esperas acabam.
- [x] **T04** — #285: `approval/link.go` e `LinkStore` na memória. *Aceite:* GET não decide;
  POST sem CSRF é recusado; uso único; TTL; headers.
- [x] **T05** — Bench: fixtures no workspace. *Aceite:* teste sintético, e os 8 cenários da
  série montam os dois lados a partir do workspace do repositório real (conferido à mão).
- [x] **T06** — Referência en/pt (app, mail, approval, segurança, receita docker), `make api`,
  CHANGELOG 0.148.0 e ROADMAP. *Aceite:* `make test` verde.
