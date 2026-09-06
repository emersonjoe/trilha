# Tarefas — Spec 046

## Fase 1 — cabeçalhos e nonce (#52)

- [ ] T101 `security_test.go`: resposta com `Delegated` não traz nenhum dos sete cabeçalhos e
      preserva o que o hospedeiro escreveu antes; `Security.Nonce` mandando no `c.Nonce()` e no
      `NonceAttr`, inclusive o vazio que não vira atributo.
- [ ] T102 `security.go`: campos `Delegated` e `Nonce`, saída antecipada em `applySecurity`,
      `Ctx.Nonce` consultando a função, `NonceAttr` com nó vazio.
- [ ] T103 Aviso no log ao subir com `Delegated` (uma linha, nível info): cabeçalho ausente é
      decisão, não acidente.
- [ ] T104 `make test`.

## Fase 2 — nomes do CSRF (#54)

- [ ] T201 `csrf_test.go`: com `cfg.CSRF` renomeado, o input sai com o nome novo, o cookie sai
      com o nome novo, o cabeçalho novo passa e o antigo não.
- [ ] T202 `trilha.go`/`csrf.go`: `type CSRF`, `Config.CSRF`, padrões em `applyConfig`, leitura
      em `CSRFToken`, `CSRFInput`, `checkCSRF`.
- [ ] T203 `cors.go` e `testing.go`: cabeçalho permitido e cliente de teste lendo os nomes do
      app, não as constantes.
- [ ] T204 `make test`.

## Fase 3 — `Provide` e `Use` (#55)

- [ ] T301 `values_test.go`: `Provide`/`Use` de ponteiro, de valor e de interface; pânico
      nomeando o tipo quando falta; dois apps no mesmo processo sem interferência.
- [ ] T302 `trilha.go`: `Provide[T]`, `Use[T]`, chave por tipo, documentação dizendo que o
      lugar é o `Setup`.
- [ ] T303 `examples/blog/internal/posts`: `type Store` com as funções viradas métodos, sem
      `var` de pacote.
- [ ] T304 `examples/blog`: `Provide` no `Setup`, `Use` nas páginas e no middleware,
      `trilha_gen.go` regerado, `blog_test.go` subindo dois apps.
- [ ] T305 `make test`.

## Fase 4 — documentação e release

- [ ] T401 `make api` (superfície pública) e revisão do diff.
- [ ] T402 `reference/security` + pt: `Delegated`, `Nonce`, e a tabela do que sai em cada caso.
- [ ] T403 `reference/csrf` (ou onde o CSRF mora) + pt: os três nomes e quando trocá-los.
- [ ] T404 `reference/app` + pt e `learn/*`: `Provide`/`Use` como a receita de dependências,
      com o aviso sobre estado de pacote.
- [ ] T405 `cookbook/migration` + pt: a seção do app embutido ganha os três — cabeçalhos do
      hospedeiro, nomes do CSRF e dependências.
- [ ] T406 CHANGELOG (0.35.0), ROADMAP, `version` em `cmd/trilha/main.go`.
- [ ] T407 `make test`, rebase em `origin/main` e `make release VERSION=0.35.0 ISSUES="52 54 55"`.
