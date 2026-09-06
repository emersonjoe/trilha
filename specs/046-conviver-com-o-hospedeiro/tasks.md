# Tarefas — Spec 046

## Fase 1 — cabeçalhos e nonce (#52)

- [x] T101 `security_test.go`: resposta com `Delegated` não traz nenhum dos sete cabeçalhos e
      preserva o que o hospedeiro escreveu antes; `Security.Nonce` mandando no `c.Nonce()` e no
      `NonceAttr`, inclusive o vazio que não vira atributo.
- [x] T102 `security.go`: campos `Delegated` e `Nonce`, saída antecipada em `applySecurity`,
      `Ctx.Nonce` consultando a função, `NonceAttr` com nó vazio.
- [x] T103 Aviso no log ao subir com `Delegated` (uma linha, nível info): cabeçalho ausente é
      decisão, não acidente.
- [x] T104 `make test`.

## Fase 2 — nomes do CSRF (#54)

- [x] T201 `csrf_test.go`: com `cfg.CSRF` renomeado, o input sai com o nome novo, o cookie sai
      com o nome novo, o cabeçalho novo passa e o antigo não.
- [x] T202 `trilha.go`/`csrf.go`: `type CSRF`, `Config.CSRF`, padrões em `applyConfig`, leitura
      em `CSRFToken`, `CSRFInput`, `checkCSRF`.
- [x] T203 `cors.go` e `testing.go`: cabeçalho permitido e cliente de teste lendo os nomes do
      app, não as constantes.
- [x] T204 `make test`.

## Fase 3 — `Provide` e `Use` (#55)

- [x] T301 `values_test.go`: `Provide`/`Use` de ponteiro, de valor e de interface; pânico
      nomeando o tipo quando falta; dois apps no mesmo processo sem interferência.
- [x] T302 `trilha.go`: `Provide[T]`, `Use[T]`, chave por tipo, documentação dizendo que o
      lugar é o `Setup`.
- [x] T303 `examples/blog/internal/posts`: `type Store` com as funções viradas métodos, sem
      `var` de pacote.
- [x] T304 `examples/blog`: `Provide` no `Setup`, `Use` nas páginas e no middleware,
      `trilha_gen.go` regerado, `blog_test.go` subindo dois apps.
- [x] T305 `make test`.

## Fase 4 — documentação e release

- [x] T401 `make api` (superfície pública) e revisão do diff.
- [x] T402 `reference/security` + pt: `Delegated`, `Nonce`, e a tabela do que sai em cada caso.
- [x] T403 `reference/csrf` (ou onde o CSRF mora) + pt: os três nomes e quando trocá-los.
- [x] T404 `reference/app` + pt e `learn/*`: `Provide`/`Use` como a receita de dependências,
      com o aviso sobre estado de pacote.
- [x] T405 `cookbook/migration` + pt: a seção do app embutido ganha os três — cabeçalhos do
      hospedeiro, nomes do CSRF e dependências.
- [x] T406 CHANGELOG (0.35.0), ROADMAP, `version` em `cmd/trilha/main.go`.
- [x] T407 `make test`, rebase em `origin/main` e `make release VERSION=0.35.0 ISSUES="52 54 55"`.
