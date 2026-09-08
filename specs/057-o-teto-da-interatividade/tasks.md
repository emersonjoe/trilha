# Tarefas 057

Teste antes do código, em ordem de execução. Uma rodada de `make test` por bloco.

## Bloco 1 — enquanto o servidor responde (#81)

- [ ] T001 Teste em `ui/ui_test.go`: `Indicator`, `PendingAfter` e `NoTransition` rendem os
      atributos esperados e compõem como qualquer `h.Node`.
- [ ] T002 `ui/ui.go`: os três símbolos, com doc comment.
- [ ] T003 `ui/assets/ui.js`: limiar, marcação em alvo/gatilho/indicador, guarda de voo por
      alvo, eventos `trilha:pending`/`trilha:settled`, transição na substituição.
- [ ] T004 `ui/assets/ui.nav.js`: o mesmo, na navegação.
- [ ] T005 `ui/assets/ui.css`: indicador escondido, gatilho em espera, transição.

## Bloco 2 — a ilha que chega por troca (#82)

- [ ] T006 Teste que falha: um fragmento com ilha, numa página sem ilha, não monta.
- [ ] T007 `ui/assets/ui.js`: montagem idempotente de ilha dentro do `hydrate`.
- [ ] T008 `island.go`: comentário dizendo quem mais monta e por quê.

## Bloco 3 — o exemplo (#83)

- [ ] T009 Rota em `examples/blog` com a ilha de arrastar e soltar, módulo ES em `public/`,
      ordem gravada por `POST` comum.
- [ ] T010 Teste de integração em `examples/blog/blog_test.go`.

## Bloco 4 — documentação (#83, #84)

- [ ] T011 `site/internal/docs/docs.go`: os dois slugs por locale, na mesma posição.
- [ ] T012 As quatro páginas (`en` e `pt`), e a de interatividade apontando para elas.
- [ ] T013 Referência do `ui` nas duas línguas com os três símbolos novos.

## Bloco 5 — fechar

- [ ] T014 `make api` (os três símbolos em `api/current.txt`).
- [ ] T015 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, itens 45–48 do `ROADMAP.md`.
- [ ] T016 `make test` verde e `scripts/release.sh 0.40.0 --issues "81 82 83 84"`.
