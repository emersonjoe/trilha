# Spec 158 — o terceiro lote das issues abertas

- **Issues**: [#266](https://github.com/emersonjoe/trilha/issues/266),
  [#267](https://github.com/emersonjoe/trilha/issues/267),
  [#268](https://github.com/emersonjoe/trilha/issues/268),
  [#269](https://github.com/emersonjoe/trilha/issues/269),
  [#270](https://github.com/emersonjoe/trilha/issues/270),
  [#271](https://github.com/emersonjoe/trilha/issues/271),
  [#272](https://github.com/emersonjoe/trilha/issues/272),
  [#274](https://github.com/emersonjoe/trilha/issues/274) — cada issue é a fonte do próprio
  escopo (problema, resultado esperado e aceitação estão lá); aqui fica só a decisão.
- **Branch**: `158-lote-das-issues-abertas-3`
- **Versão**: 0.137.0

## Por quê

Sete issues abertas no mesmo dia, todas da mesma origem: o programa CPSI Toledo (Acervo,
Protocolo, Intérprete, Rastro) precisa de quatro coisas que todo aplicativo público escreve à
mão e ninguém acerta na primeira vez — receber o webhook de um terceiro sem reler o corpo nem
comparar em tempo variável, a consulta por código sem conta resistente a enumeração, o
formulário que sobrevive à rede caindo e o atendimento por voz em cinco idiomas — mais três
fundações que o núcleo já quase tinha: o locale por requisição em vez de um por processo, a
unidade organizacional dentro da organização, e os spans que o `traceparent` já carregava sem
ninguém os exportar.

Como nas specs 155 e 157, uma release fecha o conjunto: são sete frentes que se tocam (a
receita `public-lookup` usa `c.T`; a `channel-whatsapp` usa `VerifyHMAC`; a `pwa-offline`
usa a convenção nova do scanner), e publicar sete versões deixaria quem lê o CHANGELOG sem o
fio que as liga.

## O que muda

**Webhook de terceiros (#266).** `webhook.VerifyHMAC(r, webhook.HMACOpts{Header, Prefix,
Secret, MaxBody, Hash})` lê o corpo uma vez, limita, compara em tempo constante e devolve o
mesmo `ErrSignature` para todo jeito de falhar. A receita `channel-whatsapp` escreve a rota
de verificação (`hub.challenge`), a recepção que normaliza o payload da Cloud API em
`whatsapp.Message` e deduplica por id, e o cliente de envio (texto e template) com o token e
o segredo em `Connections`. A janela de 24 h está no doc comment do cliente, não em produção.

**Consulta pública por código (#267).** `trilha.CheckDigit(base)` e
`trilha.HasCheckDigit(code)` (ISO 7064 MOD 97-10, alfanumérico) rejeitam o código errado antes
do banco. A receita `public-lookup` escreve o formulário (código + fator), o `POST` que
resolve por `consulta.Buscar`, o limite por IP e por código, a comparação constante do fator
com resposta idêntica para "não existe" e "fator errado", a auditoria com código mascarado e
a linha do tempo só com os eventos `Visivel`.

**PWA offline (#268).** Convenção nova do scanner: `var Offline = true` na pasta da rota;
`App.OfflineRoutes()` lista os gabaritos. `trilha.OfflineForm(c)` marca o form e gera a
`Idempotency-Key`; `trilha.Idempotent(c, ttl)` diz ao handler se é reenvio (e audita com o
carimbo do cliente, `trilha.QueuedAt`); `Config.Idempotency` troca a memória por um store.
`ui.OfflineScript(c)` + `ui.Outbox(c)` fazem o outbox em IndexedDB e o reenvio em ordem. A
receita `pwa-offline` escreve o `sw.js` (casca + rotas offline, network-first, versão pelo
hash de `Asset`, nada de área privada de outro público) e a tela de exemplo.

**Voz (#269).** `ui.Recorder(c, ui.RecorderOpts{…})` grava com `MediaRecorder` e envia pelo
mesmo XHR com progresso do `Dropzone`, com fallback a `<input type=file capture>` quando o
navegador nega. `ai.Client.Transcribe` e `ai.Client.Speak` falam o protocolo OpenAI
(`/audio/transcriptions`, `/audio/speech`) com limite de tamanho e o mesmo `*ai.Error`. O
`examples/assistente` ganha a rota de voz e a `Permissions-Policy` com `microphone=(self)`.

**Locale por requisição (#270).** `Config.Locales` (o primeiro é o padrão) liga a negociação:
preferência da sessão (`Config.LocaleOf`, que o `auth` preenche com `User.Locale`) >
`?lang` (lembrado em cookie) > `Accept-Language` > padrão. `trilha.LoadCatalog` lê
`i18n/<locale>.json` embutidos; `c.T(key, args...)` com plural simples e cadeia de fallback;
chave ausente cai no padrão e nunca na tela. O layout do scaffold escreve
`h.Lang(c.Locale())`. `trilha i18n extract|missing` e o passo `i18n` do `trilha check`.

**Unidade organizacional (#271).** `User.Units` na sessão; `auth.Unit(c)`, `auth.Units(c)`,
`auth.UnitWithin`. O escopo vai no valor do grant (`auth.Grant("edit", auth.ScopeUnitTree)` =
`"edit/tree"`), então `Grants` continua legível e `BindPolicy` continua funcionando.
`Policy.CanIn`, `Policy.ScopeOf`, `Policy.UnitScoped`; `sso.Policy(p, módulo, nível).In(c,
unit)` na rota que recebe a unidade do recurso, gravando a unidade em `Actor.Unit` para a
auditoria. `ListParams.Unit`. `ui.PolicyGrid` ganha a coluna de escopo; a receita `tenant`
ganha a árvore de unidades com `ui.Tree`; `trilha audit` aponta rota de módulo com escopo por
unidade que nunca chama `.In`.

**OpenTelemetry (#272).** `Config.OnRequest` é a costura genérica e sem dependência: um hook
por requisição que vê o gabarito, o status e pode trocar o contexto. O módulo `otel/` (go.mod
próprio, como `bench/`) instala o exportador OTLP/HTTP: um span por requisição com
`http.route`, status e o id do tenant, nunca o caminho concreto nem PII; `otel.Transport`
propaga o contexto no `Upstream` e no cliente `ai`. O núcleo continua com zero dependências.

## Fora de escopo

- Fila durável, UI de conversa e triagem por IA no canal WhatsApp (#266): produto.
- Identidade federada (gov.br) na consulta pública (#267): provedor OIDC depois.
- Sincronização servidor → cliente e *background sync* obrigatório (#268).
- `SpeechRecognition` do navegador e tradução como recurso próprio (#269).
- Tradução automática e pluralização CLDR completa (#270).
- Sincronizar a árvore de unidades com AD/LDAP (#271).
- Métricas e logs por OTel; instrumentação de SQL (#272).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — convenção sobre configuração | `var Offline = true` segue o padrão de `var Kind`/`var CORS`: teste no scanner, rota no `examples/blog`, golden do gerador |
| II — só biblioteca padrão | o SDK OTel fica em `otel/`, módulo separado; `TestNoExternalDeps` intocado. Receitas escrevem `net/http` puro |
| III — geração explícita | o gerador emite `Offline: true`; determinístico, golden regravado |
| IV — contrato de handler | nenhum handler novo; `Requirement.In` devolve `error`, `Idempotent` devolve `(bool, error)` |
| VI — teste primeiro | cada issue tem teste que falhava antes: `TestLocaleNegotiation`, fixtures da Cloud API, enumeração → 429, coletor em memória recebendo o span |
| VII — segurança por padrão | comparação constante (`VerifyHMAC`, fator da consulta), corpo limitado, `ErrSignature` único, código mascarado na auditoria, `microphone=(self)` explícito, o service worker nunca cacheia `Set-Cookie`/`no-store` |

## Segurança e privacidade

- Ativos e fronteiras: o webhook de um terceiro (assinatura HMAC, corpo limitado); a consulta
  pública sem sessão (enumeração, timing); o outbox no navegador (IndexedDB só com campos de
  texto, nunca arquivo); o áudio do cidadão enviado ao provedor de IA; o span exportado.
- ASVS 5.0 L2: V2 (validação: dígito verificador, limites de tamanho), V3 (sessão: `Locale`
  e `Units` viajam assinados), V4 (controle de acesso: `CanIn` nega irmã, herda pelo caminho),
  V7 (auditoria: unidade e código mascarado), V9 (comunicação: HMAC em tempo constante),
  V14 (configuração: `Permissions-Policy` explícita).
- OWASP Top 10:2025: A01 (acesso quebrado — escopo por unidade), A04 (design inseguro —
  resposta idêntica na consulta), A07 (autenticação — `hub.verify_token` selado), A09
  (log e monitoramento — spans e auditoria).
- Segredos e dados pessoais: tokens em `Connections` selados; o span leva id do tenant e
  nunca e-mail; o áudio não é gravado no servidor; a chave de idempotência é aleatória.

## Tarefas

- [x] T001 `webhook.VerifyHMAC` + receita `channel-whatsapp` (fixtures, dedup, 24 h)
- [x] T002 `trilha.CheckDigit`/`HasCheckDigit` + receita `public-lookup`
- [x] T003 Scanner `var Offline`, `OfflineForm`/`Idempotent`/`QueuedAt`, `ui.Outbox`,
      `ui.offline.js`, receita `pwa-offline`
- [x] T004 `ui.Recorder` + `ui.recorder.js`, `ai.Transcribe`/`Speak`, rota de voz no
      `examples/assistente`
- [x] T005 `Config.Locales`/`LocaleOf`, negociação, `Catalog`/`c.T`, scaffold com
      `h.Lang(c.Locale())`, `trilha i18n extract|missing`, passo `i18n` do `check`
- [x] T006 `User.Units`, escopo no grant, `CanIn`/`In`, `Actor.Unit`, `ListParams.Unit`,
      coluna de escopo no `PolicyGrid`, árvore na receita `tenant`, aviso do `trilha audit`
- [x] T007 `Config.OnRequest` + módulo `otel/` com coletor em memória, exemplo e CI
- [x] T008 Docs nas duas locales, `make api`, `make golden`, `trilha ui` nos exemplos e no site
- [x] T009 `CHANGELOG.md`, `version`, `ROADMAP.md`
- [x] T010 #274, achada ao fechar a #267: as dezessete chaves só em inglês faziam três
      receitas escreverem `<no value>` na tela em pt. Traduzidas, com o inglês de reserva e
      um teste que percorre toda receita nos dois idiomas
- [ ] T011 `make test` verde e `make release VERSION=0.137.0 ISSUES="266 267 268 269 270 271 272 274"`

## Aceitação

- **SC-001** Cada issue da lista tem um teste que falhava antes e passa depois, nomeado no
  CHANGELOG.
- **SC-002** `trilha add pwa pwa-offline`, `trilha add connections channel-whatsapp` e
  `trilha add public-lookup` escrevem, e `trilha check` do projeto fica verde com todas as
  receitas juntas.
- **SC-003** `make test` verde na raiz; `cd otel && go test ./...` verde; `TestNoExternalDeps`
  intocado.
