# Spec 094 — o teste do webhook adiantava o relógio cedo demais

- **Issue**: sem issue — a CI da 0.73.0 falhou e a falha é do teste, não da entrega.
- **Branch**: `094-webhook-teste-relogio`
- **Versão**: 0.74.1

## Por quê

A CI da 0.73.0 falhou em `TestErroDoParceiroEsperaETentaDeNovo`, só sob `-race`, com
`esperei demais por: a entrega`. Não é o motor: é o teste.

Ele esperava a segunda tentativa **chegar ao parceiro** e, no instante seguinte, adiantava o
relógio seis minutos. Entre a requisição chegar do outro lado e o motor gravar o resultado dela
existe uma janela; quando o relógio anda dentro dessa janela, a próxima hora é marcada **a partir
do futuro** — `NextTry = agora_adiantado + 5min` — e a terceira tentativa nunca vence dentro dos
dez segundos que o `espera` dá.

Um teste que passa ou falha conforme quem ganha essa corrida não mede o backoff; mede o
escalonador. E uma CI que falha assim ensina a ignorar CI vermelha, que é o mais caro dos
prejuízos.

## O que muda

Nada no `webhook`. Dois testes passam a esperar a tentativa **ser gravada** — que é o estado que o
relógio deles depende — em vez de só ter chegado:

```go
espera(t, "a segunda tentativa ser gravada", func() bool { return entrega(t, h, d.ID).Attempt == 2 })
r.anda(6 * time.Minute)
```

`TestDepoisDeTodasAsTentativasDesiste` tinha a mesma corrida no laço do backoff, ainda sem ter
falhado, e foi corrigido junto: uma corrida conhecida que se deixa em pé volta como falha
intermitente num dia pior.

## Fora de escopo

- **Um relógio que espera sozinho** (um `anda` que só volta quando o motor terminou o tique).
  Seria a forma certa se houvesse um terceiro teste com o mesmo problema; com dois, o custo é a
  API de teste ficar maior que o que ela testa.

## Aceitação

- **SC-001** Os dois testes esperam por estado gravado antes de mexer no relógio.
- **SC-002** `go test ./webhook -count=10` verde, e a CI com `-race` verde.
