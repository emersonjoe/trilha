# Feature Specification: Receitas de plataforma — billing, notify, admin

**Feature Branch**: `162-receitas-de-plataforma` | **Created**: 2026-09-24 | **Status**: Implemented
**Input**: Plano Tokens 70, spec 162 (`PLANO-TOKENS-70.md` §2) — o agente compõe features
verificadas em vez de gerar do zero: três receitas de nível plataforma, cada uma com telas,
política, auditoria, testes e custo em tokens publicado. Marco M2 (média ≥ 60%).

## Contexto

`trilha add` já tem 21 receitas, cada uma um padrão que o projeto recebe e passa a ser dono
(spec 088 em diante). O que falta são as três features de plataforma que todo produto SaaS
escreve e que um agente hoje gera do zero — cobrança, notificações e backoffice — e o preço
de cada receita em tokens, visível antes de instalar. A escrita é ~35% do custo do agente
(§1.2 do plano); uma receita que chega com teste e tela é escrita que o agente não faz.

### Decisões que vinculam a implementação

1. **Receita gera código do app, não do framework.** Nenhuma linha nova no runtime, no `ui`
   ou em `auth`: as três receitas compõem `auth` (sessão, papéis), `mail` (+ `mail.Outbox`
   nos testes), `task`, `webhook` (`webhook.Verify`, `Hooks.Emit`), `approval`,
   `trilha.Search`, `trilha.Settings`, `trilha.Limiter`, `Ctx.CSV`, `Ctx.Audit` e o kit `ui`.
   A superfície pública (`api/current.txt`) não muda.
2. **`CtxPackCost` é medido, não chutado.** Cada `Recipe` declara o custo estimado (4
   chars/token, `internal/tokbudget`) do `trilha ctx --pack <receita>` num projeto mínimo
   com a receita instalada; `TestRecipeCtxPackCost` instala cada receita e confere o número
   exato — mudou a receita, o teste diz o número novo. É `est.` em toda saída até a régua
   medir (M2).
3. **Composição é dado.** `Recipe.Includes` lista receitas aplicadas antes, na mesma pasta
   (`Recipe.At`, o padrão da receita quando `Options.At` é vazio). O `admin` é isso: `users`,
   `audit`, `approvals` e `search` sob `app/admin/`, mais o `middleware.go` padrão-nega e a
   tela inicial. Nada de copiar quatro telas que já existem e já têm teste.
4. **Caminhos corrigidos em relação ao plano.**
   - O webhook de cobrança é `POST /webhooks/billing` (`app/webhooks/billing/`), e não
     `/billing/webhook`: `/billing/` é a pasta das telas, guardada por papel — o provedor não
     tem sessão —, e endereço de provedor é fixo sob `/webhooks/` (precedente da receita
     `channel-whatsapp`). Não se move com `--at`.
   - A assinatura `timestamp.body` com janela de 5 min é `webhook.Verify` (cabeçalhos
     `X-Webhook-Timestamp`/`X-Webhook-Signature`, `webhook.Tolerance`), não `VerifyHMAC`, que
     assina só o corpo e não tem janela.
   - As tabelas `billing_plans`, `billing_subscriptions`, `billing_invoices` e
     `billing_events` chegam como `migrations/0100_billing.sql`, a convenção do
     `trilha add store` (`store.Migrate` aplica por ordem de nome, uma vez, conferindo o
     hash). O armazenamento que as telas usam é memória atrás dos mesmos métodos, como toda
     receita (`usuarios`, `acesso`, `aprovacoes`): a receita não obriga `DATABASE_URL`.
   - `Settings[T]` é uma seção global, não por usuário: a configuração do notify (limite por
     canal, hora do digesto) é `trilha.Settings`; as preferências por usuário usam a mesma
     struct com tags `form`/`validate` e `c.Bind`, guardadas por `Subject`.
   - O digesto roda uma vez por dia por data (o store lembra o último dia) e tem dois
     gatilhos: o relógio do processo (verifica de hora em hora) e o botão da fila, que o
     dispara como `task` com a data como chave. `task.Run` exige um `*Ctx`, que o relógio
     não tem — por isso o relógio chama o digesto direto.
   - A página de doc das receitas é o cookbook (onde os selos de custo da spec 160 já
     vivem): `cookbook/billing`, `cookbook/notifications`, `cookbook/backoffice` e as
     traduções `receitas/cobranca`, `receitas/notificacoes`, `receitas/backoffice`.
5. **Canais do notify.** `mail` vem ligado; `webhook` e `whatsapp` são ligados por
   `Insert.If` quando as receitas `webhooks`/`channel-whatsapp` existem, em qualquer ordem
   (o par carrega a mesma linha sob a mesma marca). A preferência só oferece canal ligado.
   O canal `webhook` emite o evento `notificacao.enviada` pelo `Hooks` do app; o `whatsapp`
   manda texto pelo cliente da receita, ao telefone da preferência.
6. **Goldens de DOM das telas** são da spec 164 (T02: "usados nos testes das receitas da spec
   162"); esta spec entrega os testes de comportamento e o golden dos arquivos gerados.
7. **T09 (régua)**: os prompts são congelados e a série só ganha medições novas. A spec
   entrega as receitas e o preço estimado; a re-medição dos cenários (S5/S6 com
   `trilha add`) é do mantenedor, com o executor da régua — como na spec 161.

## User Scenarios & Testing

### US1 - Instalar cobrança verificada (P1)

`trilha add billing` (depois de `login` e `connections`) escreve planos, assinaturas com a
máquina de estados `trial → active → past_due → canceled`, faturas, o intake de webhook
assinado e idempotente, o dunning de três tentativas por e-mail e as telas atrás dos papéis
`billing:admin`/`billing:reader`. `trilha check` fica verde sem edição.

**Independent Test**: e2e `TestAddPlatformE2E` + os testes do projeto gerado.

**Acceptance Scenarios**:

1. **Given** uma assinatura `active`, **When** chega `invoice.payment_failed` assinado,
   **Then** ela vai para `past_due`, a primeira cobrança sai por e-mail e a trilha registra.
2. **Given** um evento sem assinatura, com assinatura errada ou com timestamp de 6 min atrás,
   **When** chega em `/webhooks/billing`, **Then** 401 com `E_BILLING_WEBHOOK_UNSIGNED` e
   nada muda.
3. **Given** o mesmo `event_id` duas vezes, **Then** o segundo responde 200 e não aplica.
4. **Given** três falhas seguidas, **Then** três e-mails e a assinatura `canceled`.
5. **Given** um `billing:reader`, **When** pede o CSV de faturas, **Then** 403; o
   `billing:admin` recebe o arquivo.

### US2 - Notificar respeitando a pessoa (P1)

`trilha add notify` escreve o notificador com canais, preferências por usuário (canal,
digesto diário, horário silencioso), limite por canal e a fila de saída com reenvio manual.

**Acceptance Scenarios**:

1. **Given** horário silencioso 22–7 e agora 23h, **When** o app notifica, **Then** nada sai
   e a notificação fica `retida` até a janela acabar.
2. **Given** preferência de digesto, **When** o digesto roda, **Then** um e-mail agrupa as
   pendentes e o segundo digesto do mesmo dia não manda nada.
3. **Given** o limite do canal estourado, **Then** a notificação fica `limitada` com
   `E_NOTIFY_RATE` e aparece na fila; o admin reenvia pela tela.

### US3 - Backoffice padrão-nega (P1)

`trilha add admin` monta `app/admin/` com o `middleware.go` que só deixa passar `admin`, a
tela inicial e as quatro telas (usuários e papéis, trilha, aprovações, busca). Toda decisão
(trocar papel, desativar, aprovar) vai para a trilha com ator e alvo.

**Acceptance Scenarios**:

1. **Given** anônimo, `billing:reader` e `admin`, **When** cada um abre cada tela de
   `/admin`, **Then** login, 403 e 200, respectivamente (tabela no teste).
2. **Given** o admin troca o papel de alguém, **Then** `usuario.papel` aparece em
   `/admin/auditoria` com o ator e o alvo.

### US4 - Preço antes de instalar (P2)

`trilha add --list` mostra o custo `est.` do `ctx --pack` de cada receita (e o JSON o traz em
`ctx_pack_tokens`); `trilha add <receita>` termina com o mesmo número; as páginas de
cookbook das três receitas dizem o comando, o que instala, o custo e como estender.

## Edge Cases

- Conexão `billing-webhook` ausente: 401 com o mesmo código (não diz ao estranho o que falta)
  e `trilha audit` aponta a conexão.
- Evento de tipo desconhecido, assinado: 200 e registrado como visto, sem transição — o
  provedor não deve re-tentar o que o app escolheu ignorar.
- Transição inválida (ex.: `canceled → active`): o evento é registrado, a transição recusada
  e logada com o código; a resposta é 200 pelo mesmo motivo.
- Preferência com canal não ligado ou hora fora de 0–23: 422 com `FieldErrors`.
- `admin` num projeto que já tem `users` em `app/usuarios/`: os arquivos de `internal/` são
  pulados (já existem) e as telas passam a existir também em `/admin/usuarios`; o `Next` diz
  para apagar a pasta antiga se quiser uma porta só.

## Requirements

### Security and privacy impact (NIST SSDF 1.1 · OWASP ASVS 5.0 N2 · Top 10:2025)

- **Ativos**: segredo HMAC do provedor de cobrança; estado financeiro (assinaturas, faturas);
  preferências e endereços de notificação; papéis dos usuários; a trilha de auditoria.
- **Fronteiras de confiança**:
  1. provedor → `/webhooks/billing` (não autenticado por sessão; autenticado por HMAC de
     `timestamp.body`, janela de 5 min, `event_id` idempotente). Rota `KindAPI` (sem CSRF,
     porque o cliente não carrega token deste site; a assinatura faz esse papel).
  2. navegador → telas `/billing`, `/notificacoes`, `/admin` (sessão + papel por
     `middleware.go`, padrão-nega; POST com CSRF, que o runtime já exige).
  3. app → e-mail/webhook/whatsapp de saída (limite por canal, horário silencioso).
- **Impacto**: nenhum no framework (zero linha de runtime). No app gerado: superfície nova de
  entrada (webhook) e de saída (notificações), ambas cobertas pelos testes abaixo.
- **Controles ASVS 5.0 N2 aplicados**: V2 (validação: tags `validate` + `FieldErrors` em
  planos e preferências); V4/V8 (autorização: papéis por pasta, padrão-nega testado por
  tabela; CSV só `billing:admin`); V7 (sessão: `sessao.Flow`); V13/V14 (segredos: HMAC só via
  conexão selada — `trilha.Secret`, mascarado em `%v`/JSON/log; nada em variável de ambiente
  no handler); V16 (log e auditoria: toda escrita financeira e toda decisão de admin chama
  `c.Audit` com ator e alvo; corpo de notificação nunca vai para o log — só id, canal e
  estado); V4.3 (webhook: assinatura em tempo constante via `hmac.Equal`, janela e
  idempotência contra replay).
- **Top 10:2025**: A01 (controle de acesso) — tabela de acesso por papel; A02 (falha
  criptográfica) — HMAC-SHA256 do pacote `webhook`; A04 (design inseguro) — máquina de
  estados com transições fechadas; A07 (autenticação) — reuso de `auth`; A09 (log) — trilha
  sem conteúdo sensível; A10 (SSRF) — o canal webhook usa o `Hooks` do app, que já recusa
  rede privada.
- **Códigos novos no catálogo** (`internal/checkerr`, página em `/docs/errors`):
  `E_BILLING_WEBHOOK_UNSIGNED` e `E_NOTIFY_RATE`. **Regra nova no `trilha audit`**: projeto
  com a receita `billing` e sem a conexão do webhook declarada no código (`billing-webhook`)
  é avisado; regra de CSRF/CSP por tela fica com a spec 164 (testes ui-a-ui).
- **SSDF**: PO.1 (requisitos de segurança nesta seção), PW.1 (design com fronteiras), PW.7/
  PW.8 (testes nomeados por controle, e2e na CI), RV.1 (portão `trilha check` no e2e).
- **Segredos**: só o HMAC do provedor, na conexão selada. **Exceções**: nenhuma.

### Functional Requirements

- **FR-001**: `Recipe` MUST ter `CtxPackCost` (tokens est.), `At` (pasta padrão) e
  `Includes` (receitas compostas antes, na mesma pasta); `trilha add --list` e
  `--list --json` MUST mostrar o custo; `trilha add` MUST terminar com o custo do pack.
- **FR-002**: `TestRecipeCtxPackCost` MUST conferir o custo declarado de toda receita contra o
  pack medido.
- **FR-003**: `billing` MUST instalar migração, modelos, estados com transições fechadas,
  webhook assinado com janela e idempotência, dunning por `task`+`mail`, telas com papéis e
  CSV só para admin, e auditoria de toda escrita financeira.
- **FR-004**: `notify` MUST instalar canais (mail ligado; webhook/whatsapp por link),
  preferências por usuário validadas, horário silencioso no envio e no digesto, digesto
  diário, limite por canal com `E_NOTIFY_RATE` e fila com reenvio manual.
- **FR-005**: `admin` MUST compor `users`, `audit`, `approvals` e `search` sob `app/admin/`
  com `middleware.go` padrão-nega; troca de papel, ativação e reset de usuário MUST ser
  auditados.
- **FR-006**: Cada receita MUST ter golden dos arquivos gerados em
  `internal/recipes/testdata/` (`make golden` regrava).
- **FR-007**: Páginas de cookbook bilíngues das três receitas MUST dizer comando, o que
  instala, custo e como estender, com blocos Go vindos do código das receitas.

### Key Entities

- **Assinatura** (estado, plano, tentativas de cobrança), **Fatura**, **Plano**, **Evento**
  (id do provedor, visto uma vez).
- **Notificacao** (destinatário, canal, estado: enviada/retida/digesto/limitada/falhou),
  **Preferencias** (canal, digesto, silêncio, telefone).

## Success Criteria

- **SC-001**: `go test ./internal/recipes/ ./internal/ctx/ ./cmd/trilha/` verdes, incluindo o
  e2e que aplica `login connections webhooks channel-whatsapp billing notify admin` num
  projeto novo, roda `trilha check` e exige `PASS` de cada teste nomeado no plano.
- **SC-002**: `make test` verde com as páginas bilíngues, o catálogo de erros completo e a
  superfície pública inalterada.
