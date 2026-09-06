# Plano: `-race` e fuzzing no CI

## Fatos que decidem o desenho

1. **`go test -race ./...` já passa hoje.** Verificado antes de escrever este plano: a suíte
   atual não acusa nenhuma corrida. Isso não é bom sinal, é um sinal fraco — a suíte quase não
   roda nada concorrente, então o detector não tem o que ver. Ligar o `-race` sem escrever um
   teste concorrente compra a etiqueta sem comprar a garantia. Por isso a fase 1 é o teste, não
   a flag.
2. **O estado do runtime está em poucos lugares.** Cache de hash do `Asset`, contadores do
   `/metrics`, janelas do rate limit, `Values()` e o `Signer`. O teste concorrente precisa
   passar por todos eles no mesmo `*App`; um `t.Parallel` espalhado por vários testes não serve,
   porque cada um monta o seu app e o estado não é compartilhado.
3. **`go test -fuzz` aceita um alvo por vez, num pacote por vez.** Não existe `-fuzz ./...`.
   Daí um script pequeno (`scripts/fuzz.sh`) com a lista de pares pacote/alvo, usado pelo
   `make fuzz` e pelo CI com durações diferentes. A lista fica no script, num lugar só.
4. **Sem fuzzing, o alvo ainda roda.** `go test` executa cada `FuzzX` contra o corpus semeado
   como se fosse um teste comum. Então o corpus semeado por `f.Add` é o que protege quem só
   roda `make test`, e é onde entra cada caso que o fuzzing achar.
5. **Invariante é melhor que oráculo.** Um alvo que compara a saída com uma segunda
   implementação testa as duas. Os seis alvos afirmam propriedades verificáveis sozinhas:
   round-trip (`h`: desescapar volta ao original), autoridade (`Verify` só aceita o que `Sign`
   produz), forma (`traceparent`: hex minúsculo vindo da entrada), contrato (`Bind` sem erro ⇒
   `validate` respeitado) e ausência de pânico (rota, estáticos).
6. **O alvo de rota precisa de um app de verdade.** Rota estática, `{id}`, `{path...}`, grupo,
   API e `Public` com um arquivo — e, fora da raiz servida, um arquivo isca. Se o corpo da
   resposta contiver a isca, o fuzzing achou travessia de caminho. É a asserção que justifica o
   alvo.
7. **O CI não pode dobrar de tamanho.** `-race` num job só, com o Go estável, sem matriz;
   fuzzing curto (20s por alvo ≈ 2 min com o build) num job paralelo aos outros. Os dois jobs
   correm ao mesmo tempo que `test` e `vuln`, então o tempo de parede do workflow mal muda.
8. **Corrida ou pânico encontrado é escopo desta spec.** O que o fuzzing achar de bug real
   entra aqui com o caso commitado; o que for mudança de comportamento discutível vira issue e
   fica registrado na spec.

## Fases

1. **Concorrência.** `race_test.go` na raiz: N goroutines contra um `*App` com estático
   versionado, métrica, rate limit e cookie assinado. Rodar com `-race`.
2. **Alvos.** `fuzz_test.go` na raiz (cinco alvos) e `h/fuzz_test.go` (um), cada um com seu
   corpus semeado. Rodar `make test` e depois cada alvo com fuzzing curto de verdade, para ver
   se algum acusa algo antes de o CI acusar.
3. **Ferramenta.** `scripts/fuzz.sh`, alvos `race`, `fuzz` e `fuzz-long` no `Makefile`, jobs
   `race` e `fuzz` no `ci.yml`.
4. **Documentação.** Seção no `learn/testing` e no `pt/aprender/testes` (como fuzzar um handler
   do próprio app com `TestRequest`), `CONTRIBUTING.md` e `docs/pt-BR/CONTRIBUTING.md` com os
   comandos novos e a regra do corpus commitado.
5. **Fechamento.** CHANGELOG, `version`, ROADMAP, release.

## Riscos

- **O fuzzing acha um bug de verdade na fase 2.** É o objetivo, mas atrasa a release.
  Mitigação: caso pequeno, corrige aqui com o corpus commitado; caso grande, a spec entrega os
  alvos com o caso marcado `t.Skip` e uma issue nova aponta para o arquivo do corpus — a rede
  entra mesmo assim.
- **Alvo lento derruba o orçamento do CI.** Mitigação: 20s por alvo, medido; se o job passar de
  três minutos, cai para 10s no CI e o longo fica sob demanda.
- **`-race` intermitente por causa de tempo (rate limit, TTL de cookie).** Mitigação: o teste
  concorrente não afirma número de requisições recusadas, só que ninguém entra em pânico e que
  os invariantes de cada resposta valem.
