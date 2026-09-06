# Spec 050 — A auditoria só acusa métrica quando existe endereço de métrica

- **Pedido**: do usuário, nesta sessão — `trilha audit --no-vuln` dentro de `examples/blog`
  acusa o item crítico "metrics exposed without a token or a trusted network" num app que
  não publica endereço de métrica nenhum.
- **Branch**: `050-metricas-na-auditoria`
- **Versão**: correção; entra na próxima release.

## Por quê

A auditoria decide que o endereço de métricas está no ar procurando a substring `Metrics:`
no fonte do projeto (`cmd/trilha/audit.go`, `runAudit`). Quem abre o endereço é
`Config.Observability.Metrics` — e só ele: `applyObservability` liga o `a.instrument` e o
`serveObservability` a partir desse campo. Mas `Metrics:` também é o nome de uma **opção do
cache**, que só diz em qual registro o cache publica os contadores e não expõe endereço
algum. O app de referência usa exatamente essa opção
(`examples/blog/app/setup.go:71`, `cache.New(cache.Options{…, Metrics: a.Metrics()})`), e
por isso a auditoria do próprio exemplo termina em um item crítico falso — e o `trilha
check`, que roda a auditoria, falha junto.

Um crítico falso no app de referência é pior que um item faltando: quem segue o exemplo
aprende que a auditoria mente, e passa a ignorar o item que um dia vai ser verdadeiro.

## O que muda

Só o heurístico de `runAudit`. O item de métricas passa a ligar quando, e apenas quando:

- `TRILHA_METRICS` está no ambiente (é o que `trilha.go:144` lê para o campo), ou
- o fonte configura o campo do `Observability`: a atribuição `Observability.Metrics` ou o
  literal `Observability{… Metrics: …}` (inclusive com o tipo elidido,
  `Observability: {…}`).

Qualquer outro `Metrics:` — a opção do cache, um campo de struct do app com o mesmo nome —
deixa de ligar o item, que fica em `metrics off`.

```go
// liga o item: existe endereço no ar
cfg.Observability.Metrics = "/_trilha/metrics"
cfg.Observability = trilha.Observability{Metrics: "/_trilha/metrics"}

// não liga: o cache só publica contadores no registro
store.Cache = cache.New(cache.Options{Name: "posts", Metrics: a.Metrics()})
```

## Fora de escopo

- **O heurístico do `Trusted:`** (mesma linha do `metricsOn`) tem a mesma forma e o mesmo
  risco, mas hoje ele só é lido quando o item de métricas está ligado, e nenhum falso
  positivo apareceu — mexer nele agora seria mudar o que não quebrou.
- **Ler o fonte com o `go/parser`** em vez de substring: a auditoria roda em projeto que
  pode nem compilar, e o resto dos itens é de substring; trocar a base de todos é outra
  spec.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `strings`, nada novo |
| VI — teste primeiro | `TestAuditoriaDeMetricas` falha com o heurístico antigo |
| VII — segurança por padrão | o item continua crítico quando o endereço está mesmo no ar, com ou sem token |

## Tarefas

- [x] T001 Teste que falha: fonte com a opção do cache e mais nada não liga o item; a
      atribuição e o literal do `Observability` ligam; `TRILHA_METRICS` liga.
- [x] T002 `metricsConfigured` no `audit.go` e uso no `runAudit`.
- [x] T003 `trilha audit --no-vuln` verde em `examples/blog` (logo, `trilha check` também).
- [x] T004 `CHANGELOG.md` em `Unreleased`; `make test` verde.

## Aceitação

- **SC-001** Com `TRILHA_SECRET` de 32 bytes, `trilha audit --no-vuln` dentro de
  `examples/blog` sai com zero itens críticos e mostra `metrics off`.
- **SC-002** Um projeto cujo único `Metrics:` é a opção do cache não liga o item.
- **SC-003** Um projeto que atribui `Observability.Metrics` sem token e sem rede confiável
  continua com o item crítico.
