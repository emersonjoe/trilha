# Tarefas — spec 146

Teste antes do código, em ordem de execução. Uma rodada de `make test` por bloco, não por arquivo.

## Bloco 1 — os testes que falham

- [x] **T001** `internal/client`: `TestPrefixoDaTagSoCaiEmFronteiraDePalavra` ganha os dois casos
      medidos na #205 — `obter_resumo_dos_idiomas` na tag `idiomas` e `responder_card` na tag
      `cards` — e o caso do sufixo simples (`listar_regras` na tag `regras`) passa a esperar o
      nome inteiro.
- [x] **T002** `internal/client`: teste do relatório — o corte do prefixo (`config_reset` na tag
      `config`) vira linha das `Notes`.
- [x] **T003** `internal/client`: teste de `WithResponse` contra `httptest` pelo cliente golden —
      `Set-Cookie` colhido por um coletor que vem do `context`, corpo da operação decodificado do
      mesmo jeito, e a resposta de erro também passando pelo gancho.
- [x] **T004** `internal/client`: teste de `required` + anulável — `["string","null"]`,
      `["integer","null"]`, `nullable: true`, fatia anulável e o `anyOf` com `required`.
- [x] **T005** `testdata/openapi/acervo.json`: dois campos `required` anuláveis em `Document`.
- [x] **T006** `go test ./internal/client` vermelho nos cinco pontos, por prova e não por
      compilação.

## Bloco 2 — implementação

- [x] **T007** `gen.go`: `methodName` como método do `builder`, só `trimWordPrefix(name, gName)`,
      linha do relatório quando corta.
- [x] **T008** `types.go`: `nullableType`, `pointerForNull`, ponteiro e `validate` sem `required`
      em `object`; o mesmo no parâmetro de query em `gen.go`.
- [x] **T009** `emit.go`: campo `response` no `Client`, `WithResponse`, gancho em `do` depois do
      `c.http.Do`.
- [x] **T010** `make golden` e revisão do diff de `internal/client/api/client.go`.
- [x] **T011** `examples/cookbook` e `examples/local-login`: clientes regerados pela CLI e
      chamadas com os nomes novos.
- [x] **T012** `make test` verde.

## Bloco 3 — documentação e fechamento

- [x] **T013** `site/.../en/reference/cli.md` e `site/.../pt/referencia/cli.md`: § `trilha client`
      com o corte do nome, `WithResponse` e o campo anulável.
- [x] **T014** Receita da API que já existe nas duas locales: nomes de método atualizados e o
      trecho do login por cookie.
- [x] **T015** `CHANGELOG.md` (0.125.0, `Added`/`Fixed`/`Changed`), `const version` em
      `cmd/trilha/main.go`, linha e itens do `ROADMAP.md`.
- [x] **T016** `make test` verde, `verifica-trilha.sh --sem-testes` e revisão do diff inteiro.
