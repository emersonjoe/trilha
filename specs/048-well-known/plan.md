# Plano — Spec 048

Entrada: [spec.md](./spec.md). Mudança pequena em tamanho, mas toca três pacotes
(`internal/scan`, `internal/openapi`, `internal/dev`) e cria convenção em `app/` — por isso
a forma completa.

## Os fatos que decidem o desenho

1. **A regra do ponto está em três varreduras.** Scanner (`scan.go:307`), índice de tipos do
   `openapi` (`schema.go:87`) e watcher do `dev` (`watch.go:45`). Três cópias divergem; a
   exceção vira uma constante exportada, `scan.WellKnown`, e os três a leem. `openapi` e
   `dev` já importam `internal/scan`, então não há ciclo nem dependência nova.
2. **O pacote com ponto no caminho compila.** Verificado com um módulo de teste: `go build`,
   `go vet` e `go run` limpos importando `example.com/m/.well-known/x` pelo caminho
   explícito. O que a ferramenta Go faz é **não casar** esse caminho no padrão `./...`;
   como `trilha_gen.go` importa explicitamente, o pacote entra no binário. É limitação
   conhecida e documentada, não obstáculo.
3. **O alias já resolve o nome.** `s.alias` troca todo caractere fora de `[A-Za-z0-9_]` por
   `_`: `app/.well-known/security.txt` → `app__well_known_security_txt`. Nada a fazer.
4. **`parseSegment` já aceita literal com ponto.** O `default` devolve `segment{literal: dir}`
   sem checar identificador — é o que faz `app.css` funcionar desde a #14. `.well-known`
   entra pelo mesmo caminho assim que parar de ser pulado.
5. **O aviso é uma varredura rasa e rara.** Só acontece em diretório que já ia ser pulado, só
   procura `page.go` e `route.go`, e não desce em `.git/` nem `node_modules/`. Custo zero no
   caminho feliz (projeto sem pasta oculta dentro de `app/`).
6. **Ponto grita, sublinhado cala.** O `_x/` é a forma documentada de tirar uma pasta do
   roteamento sem apagá-la; se ele também virasse erro, a mensagem não teria conserto para
   oferecer. O erro do ponto aponta para o sublinhado.

## Fases

1. **Scanner**: `WellKnown`, exceção no pulo, `E_HIDDEN_ROUTE` com `fixes`, fixtures.
2. **As outras duas varreduras**: `openapi` e `dev` com a mesma exceção, cada uma com teste.
3. **Uso real**: rota `/.well-known/security.txt` no `examples/blog` (+ `trilha_gen.go`
   regerado) e rota tipada no fixture do `openapi`, que prova o índice de tipos.
4. **Fechamento**: e2e, documentação nas duas locales, CHANGELOG, ROADMAP, versão.

## Riscos

- **Churn de golden.** Mexer no fixture `testdata/apps/openapi` regrava `openapi`, `ctx.json`
  e `ctx.md`. É `make golden` e uma leitura do diff — aceito, porque é a prova de que a rota
  atravessa a cadeia inteira.
- **Falso positivo do aviso.** Um `page.go` de fixture dentro de `app/.fixtures/` viraria
  erro. Mitigado por só valer para o ponto e por a mensagem dizer como parar (renomear com
  `_`).
