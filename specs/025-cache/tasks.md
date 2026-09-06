# Tasks: Cache com TTL, tags e invalidação

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — o cache (FR-001…FR-005)

- [x] T001 Teste que falha em `cache/cache_test.go`: TTL vencido é ausência; `Invalidate` por
      tag derruba só o que tem a tag; regravar troca as tags; teto despeja o mais velho;
      `Clear` zera inclusive o mapa de tags.
- [x] T002 `cache/cache.go`: `Options`, `New`, `Key`, LRU com `container/list`, tags e o
      `drop(key)` único por onde toda remoção passa.

## Bloco 2 — camada tipada (FR-006…FR-008)

- [x] T003 Teste que falha em `cache/typed_test.go`: `Get[T]` com tipo errado; `Do` busca uma
      vez e reusa; erro não é guardado; 50 goroutines na chave fria dão uma chamada só
      (`-race`); `Do` aninhado não trava; `Once` uma vez por requisição e nada entre duas.
- [x] T004 `cache/typed.go`: `Get[T]`, `Do[T]` com voo único, `Once[T]` sobre o `Ctx.Set/Get`.

## Bloco 3 — métricas (FR-009)

- [x] T005 Teste que falha: com `Options.Metrics`, o texto do `/metrics` traz as quatro séries
      com o rótulo `cache`.
- [x] T006 Registro das quatro séries em `New`; HELP das cinco métricas do framework em inglês.

## Bloco 4 — exemplo (SC-004, SC-005)

- [x] T007 Teste que falha em `examples/blog/blog_test.go`: publicar muda a lista na resposta
      seguinte, e `/metrics` mostra acerto de cache depois de duas visitas.
- [x] T008 `examples/blog`: cache no `Setup`, lista de posts por `cache.Do`, `Invalidate` no
      `Create`/`Delete`, `Once` no layout.

## Bloco 5 — documentação e fechamento

- [ ] T009 `learn/data` + `pt/aprender/dados` (capítulo novo): onde o dado é buscado, quanto
      vale, o que o derruba, e a diferença entre `Do` e `Once`.
- [ ] T010 `reference/cache` + `pt/referencia/cache`, e a linha do pacote no índice das duas
      locales.
- [ ] T011 `CHANGELOG.md` (0.16.0), `version`, ROADMAP (Fase 2, item 6).
- [ ] T012 `make test` verde e `make release VERSION=0.16.0 ISSUES="25"`.
