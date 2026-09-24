# Implementation Plan: Receitas de plataforma — billing, notify, admin

**Branch**: `162-receitas-de-plataforma` | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)

## Summary

Três receitas novas em `internal/recipes` (`billing.go`, `notify.go`, `admin.go`) que geram
código do app compondo pacotes existentes, mais três campos em `Recipe` (`CtxPackCost`, `At`,
`Includes`) que dão preço e composição a todas as receitas. Nenhum símbolo público novo,
nenhuma dependência nova.

## Constitution Check

- [x] I — nenhuma convenção nova em `app/`; as receitas usam `page.go`, `route.go`,
  `middleware.go` e `kind.go` como já existem.
- [x] II — só stdlib; o módulo raiz não ganha `require`.
- [x] III — o gerador não muda; o golden dos arquivos da receita é determinístico.
- [x] IV — superfície pública inalterada (`internal/` fora de `api/current.txt`).
- [x] VI — testes nomeados do plano no projeto gerado, e2e aplicando as receitas com
  `trilha check`, golden dos arquivos gerados, custo medido por teste.
- [x] VII — fronteiras e controles na spec; padrão-nega por papel; webhook assinado com
  janela e idempotência; segredo só na conexão selada; trilha sem conteúdo sensível.
- [x] Idioma — código em inglês; identificadores do app gerado em pt (convenção das
  receitas); páginas bilíngues no mesmo commit.

## Design

```
internal/recipes/recipes.go       # Recipe.CtxPackCost, .At, .Includes; Add aplica Includes
internal/recipes/billing.go       # receita billing (Needs login, connections)
internal/recipes/notify.go        # receita notify (Needs login) + links webhook/whatsapp
internal/recipes/admin.go         # receita admin (At app/admin/, Includes users audit approvals search)
internal/recipes/users.go         # POST audita papel/ativo/reset/convite
internal/recipes/webhooks.go      # carrega o link notify-webhooks
internal/recipes/whatsapp.go      # carrega o link notify-whatsapp
internal/recipes/golden_test.go   # TestBillingInstall/TestNotifyInstall/TestAdminInstall
internal/recipes/testdata/<r>/    # golden dos arquivos gerados
internal/ctx/recipes.go           # RecipeInfoOf; receita sem marca detectada pelos arquivos
internal/ctx/cost_test.go         # TestRecipeCtxPackCost
cmd/trilha/add.go                 # custo na listagem, no JSON e no fim do add
cmd/trilha/audit.go               # regra: billing sem a conexão do webhook
cmd/trilha/e2e_test.go            # TestAddPlatformE2E
internal/checkerr/catalog.go      # E_BILLING_WEBHOOK_UNSIGNED, E_NOTIFY_RATE
site/internal/docs/content/{en,pt}/cookbook|receitas/  # três páginas por língua
```

App gerado pelo `billing`:

```
internal/billing/billing.go       # estados, transições, Plano/Assinatura/Fatura, Store, Setup
internal/billing/webhook.go       # Receber: Verify + janela + idempotência + transição + Audit
internal/billing/dunning.go       # lembrete por e-mail como task; Mailer substituível
internal/billing/billing_test.go  # TestBillingStates
app/webhooks/billing/{route,kind}.go
{{.At}}billing/{page,middleware}.go, planos/, faturas/, faturas/csv/
migrations/0100_billing.sql
billing_test.go                   # webhook (válido, sem assinatura, expirado, idempotente),
                                  # dunning, CSV só admin
```

App gerado pelo `notify`: `internal/notificar/{notificar,canais,notificar_test}.go`,
`{{.At}}notificacoes/` (preferências, sessão) e `{{.At}}notificacoes/fila/` (admin),
`notificacoes_test.go`.

App gerado pelo `admin`: as quatro receitas incluídas sob `app/admin/`,
`app/admin/{middleware,page}.go`, `admin_test.go`.

Blocos: T01 (metadados + custo) → T02–T05 (billing) → T06 (notify) → T07 (admin) → T08
(páginas + catálogo) → T09 (régua: nota) → T10 (fechamento).
