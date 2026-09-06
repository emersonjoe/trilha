# Spec 033 — `-race` e fuzzing no CI

- **Issue**: [#33](https://github.com/emersonjoe/trilha/issues/33) (ROADMAP, Fase 3, item 15)
- **Branch**: `033-race-e-fuzzing`
- **Versão**: 0.24.0

## Por quê

O CI roda `go test ./...`. Para uma biblioteca comum isso bastaria; para um framework web não
basta, porque as duas classes de falha mais caras dele não aparecem numa suíte determinística:

- **Corrida de dados.** Um `App` atende milhares de requisições ao mesmo tempo, e o runtime
  guarda estado entre elas — cache de hash dos estáticos, contadores de métrica, janelas do
  rate limit, `Values()` do `Setup`. Uma escrita sem proteção não quebra o teste: ela corrompe
  um contador em produção, ou pior, devolve o dado de uma sessão para outra pessoa. `go test`
  sem `-race` nunca vê isso, e a suíte de hoje quase não roda nada em paralelo, então mesmo o
  `-race` veria pouco.
- **Entrada de terceiros.** Cinco pontos do runtime leem bytes que um atacante escolhe: o alvo
  da requisição (casamento de rota e estáticos), o corpo do `Bind`, o cookie assinado, o
  cabeçalho `traceparent` e — do outro lado — o texto que o `h` escreve dentro do HTML. Cada um
  tem hoje testes de exemplo escritos por quem já sabe qual é a resposta certa. Fuzzing procura
  o caso que ninguém pensou em escrever, que é justamente o que o atacante procura.

As duas redes são baratas: vêm na ferramenta padrão, não acrescentam dependência e cabem em
menos de dois minutos de CI.

## O que muda

Nada na API pública. O que muda é o que o repositório roda.

### `-race` com concorrência de verdade

Um teste novo põe várias goroutines contra o mesmo `*App`, passando pelos pontos com estado —
estático versionado, métrica, rate limit, cookie assinado, `Values()` — e o CI roda a suíte
inteira sob `-race` num job próprio:

```bash
make race        # go test -race ./...
```

### Alvos de fuzzing

Seis alvos, com corpus semeado no próprio código (`f.Add`), rodando como teste normal na suíte
e como fuzzing sob demanda:

| Alvo | Onde | O que o alvo prova |
|---|---|---|
| `FuzzRouteMatch` | raiz | qualquer alvo de requisição é respondido sem pânico, e nenhum caminho serve arquivo fora de `Public` |
| `FuzzBindForm` | raiz | corpo de formulário arbitrário: ou erro, ou struct que respeita as regras de `validate` |
| `FuzzBindJSON` | raiz | idem para JSON, incluindo corpo truncado e tipo trocado |
| `FuzzSignedVerify` | raiz | um token só passa no `Verify` se for exatamente o que o `Sign` produziria |
| `FuzzParseTraceparent` | raiz | a saída é `""` ou 32 hex minúsculos vindos da entrada — nunca texto escolhido pelo atacante |
| `FuzzRenderEscapes` | `h/` | o texto e o atributo renderizados desescapam de volta no valor original e não trazem `<` nem `"` soltos |

```bash
make fuzz             # 20s por alvo, o que o CI roda
make fuzz-long        # 5min por alvo, para uma investigação
FUZZTIME=2m make fuzz # qualquer duração
```

`scripts/fuzz.sh` percorre os alvos porque `go test -fuzz` só aceita um por vez.

### Falha vira regressão

Quando o fuzzing acha um caso, o Go grava o arquivo em `testdata/fuzz/<Alvo>/`. Esse arquivo é
commitado: a partir daí o caso roda no `go test ./...` de todo mundo, para sempre, sem depender
de o fuzzing achá-lo de novo.

## Fora de escopo

- **OSS-Fuzz ou fuzzing contínuo em serviço.** Faz sentido perto da 1.0, com API estável; hoje
  o alvo mudaria a cada spec.
- **`-race` na matriz inteira.** Um job com a versão estável do Go basta: corrida não é
  específica de versão, e dobrar a matriz dobra o tempo por nada.
- **Fuzzing do `internal/scan` e do gerador.** Eles leem código do próprio projeto, não entrada
  de terceiros; o risco é outro (issue própria, se aparecer).
- **Alvo para `tmpl`.** O `html/template` da biblioteca padrão já é fuzzado pelo Go.
- **Detector de deadlock ou teste de carga.** Outra ferramenta, outro objetivo.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `testing.F` e `-race` vêm no Go; nenhuma dependência nova, `TestNoExternalDeps` intocado |
| V — produção em um binário | fuzzing e `-race` são ferramentas de teste; nada entra no binário que o usuário publica |
| VI — teste primeiro | a spec é toda teste: os alvos entram antes de qualquer correção que eles motivem |
| VII — segurança por padrão | os cinco pontos de entrada de terceiros passam a ter prova por busca, não só por exemplo |

## Aceitação

- **SC-001** `go test -race ./...` verde, com um teste que de fato exercita o `App` de várias
  goroutines (remover o `sync` de um ponto com estado faz o teste acusar corrida).
- **SC-002** Os seis alvos existem, rodam no `go test ./...` com o corpus semeado e passam.
- **SC-003** `make fuzz` roda os seis por 20s cada e sai zero; `FUZZTIME` muda a duração.
- **SC-004** O CI tem job de `-race` e job de fuzzing curto, e o tempo total do workflow
  continua abaixo de cinco minutos.
- **SC-005** `FuzzSignedVerify` recusa qualquer token que não venha do `Sign`, e
  `FuzzParseTraceparent` nunca devolve algo que não seja substring hex da entrada.
