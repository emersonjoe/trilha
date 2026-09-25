# Tasks: Receitas de plataforma — billing, notify, admin

- [x] **T01** — Metadados de receita em `recipes.go`: `CtxPackCost` (medido por
  `TestRecipeCtxPackCost` em `internal/ctx`), `At` e `Includes`; `trilha add --list` e
  `--json` com o custo `est.`; o `add` termina com o custo do pack. *Aceite:*
  `trilha add --list` mostra custo `est.` por receita (`go test ./internal/ctx/ ./cmd/trilha/ -run 'Cost|AddList'`).
- [x] **T02** — Billing: migração `0100_billing.sql` + modelos + estados com transições
  fechadas. *Aceite:* `TestBillingStates`.
- [x] **T03** — Billing: webhook HMAC `timestamp.body` (`webhook.Verify`) + idempotência por
  `event_id` + janela de 5 min, em `/webhooks/billing` (caminho corrigido, ver spec).
  *Aceite:* `TestBillingWebhookValid`, `TestBillingWebhookUnsigned`,
  `TestBillingWebhookExpired`, `TestBillingIdempotentEvent`.
- [x] **T04** — Billing: dunning de três tentativas por `task` + `mail`. *Aceite:*
  `TestBillingDunningCycle` (com `mail.Outbox`).
- [x] **T05** — Billing: telas (assinaturas com filtro, planos, faturas) + papéis
  `billing:admin`/`billing:reader` + CSV + auditoria; regra do `trilha audit` para a conexão
  do webhook. *Aceite:* `TestBillingCSVAdminOnly`, `TestBillingInstall` (golden dos
  arquivos), `trilha audit` verde com a regra nova no e2e. Goldens de DOM: spec 164 T02.
- [x] **T06** — Notify: canais (mail; webhook e whatsapp por `Insert.If`) + preferências +
  horário silencioso + digesto + limite por canal + fila com reenvio. *Aceite:*
  `TestNotifyInstall`, `TestNotifyPreferences`, `TestNotifyQuietHours`, `TestNotifyDigest`,
  `TestNotifyRateLimit`, `TestNotifyOutboxReplay`.
- [x] **T07** — Admin: `app/admin/` com `middleware.go` padrão-nega + tela inicial + as
  quatro telas compostas (`Includes`) + auditoria das decisões de usuário. *Aceite:*
  `TestAdminInstall`, `TestAdminDefaultDeny`, `TestAdminAuditTrail`, `TestAdminApprovalFlow`.
- [x] **T08** — Páginas de cookbook bilíngues das três receitas (comando, o que instala,
  custo, estender) com blocos Go vindos do código das receitas; códigos
  `E_BILLING_WEBHOOK_UNSIGNED`/`E_NOTIFY_RATE` no catálogo. *Aceite:* testes de conteúdo do
  site + `TestCookbookSnippetsAreReal` + `TestErrorCatalogComplete`.
- [x] **T09** — Régua: nota de fechamento. Prompts congelados e série histórica intocados;
  a re-medição de S5/S6 com `trilha add billing/notify/admin` é do mantenedor com o executor
  da régua. *Aceite:* tasks.md.
- [x] **T10** — CHANGELOG + ROADMAP + suíte. *Aceite:* `make test` verde.
