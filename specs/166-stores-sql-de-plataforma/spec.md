# Feature Specification: Stores SQL de billing e notify

**Feature Branch**: `166-stores-sql-de-plataforma` | **Created**: 2026-09-25 | **Status**: Entregue (0.147.0)
**Input**: sugestão de issue do fechamento do Plano Tokens 70 ("stores SQL para billing e
notify"), pedida pelo mantenedor. Decisões dele, na sessão: **interface + duas implementações**
(e não cache em memória com persistência) e **módulo de teste próprio** com driver real.

## Contexto

As receitas `billing` e `notify` (spec 162) guardavam as linhas em memória, com a migração da
cobrança escrita "para o dia em que o `trilha add store` chegar". Um restart perdia assinaturas,
faturas, fila e preferências; duas réplicas tinham dois estados.

## O que muda

1. **`cobranca.Store` vira interface** (`Planos`, `SalvarPlano`, `ApagarPlano`, `Assinaturas`,
   `Assinatura`, `Assinar`, `Faturas`, `Visto`, `Esquecer`, `Pagou`, `Falhou`, `Cancelar`), todos
   com `context.Context` e `error`. `NovoStore()` é a memória (`*Memoria`); `NovoSQL` (novo
   `internal/cobranca/sql.go`) usa só `database/sql`. A regra da transição (`aplicar`) é uma só
   para as duas.
2. **`notificar.Store`** (novo): preferências e fila (`Preferencias`, `SalvarPreferencias`,
   `Guardar`, `Atualizar`, `Uma`, `Fila`, `Esperando`, `Dia`). O `Notificador` passa a ser as
   regras sobre um `Store`; `Preferencias`, `SalvarPreferencias` e `Fila` ganham `ctx` e `error`.
   Nova migração `migrations/0110_notify.sql`.
3. **A ligação** é uma linha em `app/setup.go` (`trilha:link billing-store`,
   `trilha:link notify-store`) que as receitas carregam dos dois lados, cada uma condicionada ao
   arquivo da outra (`Insert.If`): vale em qualquer ordem. Ela aponta `cobranca.Banco` /
   `notificar.Banco` para `store.DB` e `store.D.Arg`; o `Setup` usa o SQL quando `Banco` existe.
   `Banco` é função porque `store.Setup` pode rodar depois.
4. **Contrato**: `cobrancatest.Contrato` e `notificartest.Contrato` (pacotes próprios, para o
   `testing` não entrar no binário) rodam na memória nos testes do pacote e sobre o que o app
   ligou em `TestBillingStoreContract` / `TestNotifyStoreContractOnTheApp`. Os ids levam um
   prefixo da execução: o contrato não perturba nem é perturbado por um banco com outras linhas.
5. **Webhook com banco**: falha do store (não da máquina) responde 500 e **esquece** o id do
   evento (`Esquecer`), para a nova tentativa do provedor valer; recusa da máquina continua 200.
   Mudança de estado no SQL é otimista (`UPDATE … WHERE state = ? AND attempts = ?`), com até
   três releituras: duas réplicas no mesmo evento não se sobrescrevem.
6. **Migração 0100 corrigida**: sem chave estrangeira de `plan_id` (o evento chega com o plano do
   provedor, que pode não existir localmente; apagar plano não pode levar o histórico) e a coluna
   `interval` vira `period`. Permitido porque as versões 0.140–0.146 nunca foram publicadas
   (última tag `v0.139.0`): nenhum projeto aplicou a 0100.

## Constitution Check

- II — só biblioteca padrão: as receitas geram `database/sql` puro; os drivers (modernc SQLite,
  pgx) só no módulo `sqltest/` e no projeto de quem escolhe. `TestNoExternalDeps` verde.
- IV — API pública do framework: nenhuma mudança. O que muda é código **gerado** pelas receitas
  (assinaturas do `Store` e do `Notificador`); projetos já gerados não são tocados.
- VI — teste primeiro: contratos + `sqltest` em SQLite (duas ordens) e PostgreSQL.
- VII — segurança: toda consulta com placeholder do dialeto; nada de entrada no texto SQL.

## Segurança e privacidade (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras**: aplicação → banco (DSN no ambiente, redigido nos erros pelo store); provedor →
  webhook (inalterado: HMAC, janela, idempotência — agora durável).
- **Controles**: V5/V1.2 injeção — placeholders sempre, identificadores fixos no código
  (Top 10 A03/A05:2025); V7 logs — a fila guarda o corpo da notificação, nunca em log;
  V8 dados — o corpo da notificação passa a persistir no banco (mesmo conteúdo que a fila em
  memória já mostrava ao admin).
- **Exceção registrada**: o limite por canal continua por processo (protege o canal de um laço,
  não a pessoa de uma frota de réplicas).

## Tarefas

- [x] **T01** — `cobranca.Store` interface + `Memoria` + `SQL` + `Esquecer`; webhook 500 em
  falha do store. *Aceite:* `TestBillingStates` (contrato na memória).
- [x] **T02** — `notificar.Store` + `Memoria` + `SQL` + migração 0110. *Aceite:*
  `TestNotifyStoreContract`.
- [x] **T03** — Ligações `billing-store` e `notify-store` nas duas ordens. *Aceite:* `sqltest`
  confere o marcador uma vez em cada ordem.
- [x] **T04** — Módulo `sqltest/` (SQLite e cluster PostgreSQL temporário), `make test-sql`,
  job `sql` na CI (opcional até o M3). *Aceite:* `SQLTEST_POSTGRES=1 make test-sql` verde.
- [x] **T05** — Cookbook en/pt, CHANGELOG 0.147.0, ROADMAP. *Aceite:* `make test` verde.

## Registro da implementação

- O `sqltest` achou, na primeira execução, a falta da linha `notify-store` do lado da receita
  notify (store antes, notify depois): corrigido com `notifyStoreLink` nas duas receitas.
- A prova de que o app usou o banco não é o teste passar (passaria na memória): o `sqltest` abre
  o banco de cada teste e conta as linhas de `billing_*` e `notify_*`.
- O cluster PostgreSQL de teste escuta só em 127.0.0.1, sem socket Unix (o caminho temporário do
  macOS passa de 103 bytes), e para no fim do teste.
- `CtxPackCost`: billing 338 → 356, notify 197 → 232 (arquivos novos no pack).
- Evidências: `make test`, `UITEST_REQUIRED=1 make test-ui` e `SQLTEST_POSTGRES=1 make test-sql`
  verdes (SQLite nas duas ordens e PostgreSQL 14).
