# Spec 058 — O fuzz que não reprova por acaso

- **Issue**: #86 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `058-ilhas-segunda-geracao` (o nome ficou de um escopo que não foi adiante)
- **Versão**: 0.40.1

## Por quê

O trabalho `fuzz` é obrigatório no PR e reprova de vez em quando sem ter achado nada: o prazo
do `-fuzztime` estoura dentro de uma iteração e o `go test` reporta isso como falha do alvo.
Aconteceu duas vezes até agora, em alvos diferentes — o que é justamente o que diz que o
problema não é de nenhum deles.

Um trabalho que reprova por acaso ensina a reexecutar sem ler, e o dia em que ele reprovar de
verdade vai parecer mais um. O `fuzz` é a única barreira que este repositório tem contra
entrada malformada; ela não pode ser a que ninguém lê.

## O que muda

`scripts/fuzz.sh` passa a distinguir as duas reprovações, que **são** distinguíveis:

| Sinal | Achado de verdade | Prazo estourado |
|---|---|---|
| Arquivo novo em `testdata/fuzz/<Alvo>/` | sim | não |
| `Failing input written to` na saída | sim | não |
| `context deadline exceeded` na saída | não | sim |

Com os três sinais apontando para o prazo, o alvo roda **uma segunda vez**. Se reprovar de
novo pelo mesmo motivo, o script reprova e diz que duas vezes seguidas não é acaso — um
travamento de verdade repete, e continua sendo pego.

Não há mudança no framework: nada que quem usa o Trilha veja.

## Fora de escopo

- **Aumentar o `FUZZTIME` no CI** — reduz a frequência sem mudar a natureza do problema, e
  cobra minutos de todo PR pelo acaso de alguns.
- **Ignorar a reprovação sem repetir** — seria mascarar. Um alvo que entra em laço infinito
  também estoura o prazo, e a segunda passada é o que separa um caso do outro.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | É `bash` e `go test`; nada entra no módulo. |
| VI — teste primeiro | O próprio script é a verificação: rodá-lo com os seis alvos é a prova, e foi assim que apareceu um bug meu — com `pipefail`, o `find` num diretório que não existe derrubava o script antes do primeiro alvo. |

## Tarefas

- [x] T001 `corpus_de`, `roda` e `so_prazo` em `scripts/fuzz.sh`, com a segunda passada.
- [x] T002 Rodar os seis alvos de ponta a ponta e conferir saída 0.
- [x] T003 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`.
- [ ] T004 `make test` verde e `scripts/release.sh 0.40.1 --issues "86"`.

## Aceitação

- **SC-001** `./scripts/fuzz.sh 5s` roda os seis alvos e sai 0.
- **SC-002** Reprovação com entrada escrita continua reprovando na primeira passada.
- **SC-003** O CI do PR desta spec fecha verde.
