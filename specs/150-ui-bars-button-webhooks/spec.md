# Spec 150 — O rótulo do Bars, o type do Button e o religar do WebhooksPanel

- **Issues**: [#195](https://github.com/emersonjoe/trilha/issues/195),
  [#198](https://github.com/emersonjoe/trilha/issues/198) e
  [#202](https://github.com/emersonjoe/trilha/issues/202) — as issues são a fonte do escopo;
  aponte para elas, não as reescreva aqui.
- **Branch**: `feat/ui-bars-button-webhooks`
- **Versão**: 0.129.0

## Por quê

Três defeitos pequenos e independentes do pacote `ui/`, todos vindos de aplicações que já
portaram uma tela para o kit:

`ui.Bars` (#195) escreve o valor de cada barra dentro do `viewBox` a partir de
`barX+largura_da_barra+6`. A barra do maior valor sempre tem a largura máxima, então é sempre
ela que empurra o texto mais perto da borda — e o que passa do `viewBox` o navegador corta.
Qualquer série cujo `Datum.Text` da barra maior passe de uns cinco caracteres perde o rótulo
inteiro; não é caso de canto, é o caso comum de todo dashboard com valor em texto ("5 folhas ·
2 NC", "R$ 1.204,50").

`ui.Button` (#198) acrescenta `h.Type("button")` **depois** dos filhos que o chamador passou.
Um chamador que precisa de `type="submit"` (`ui.Button(h.Type("submit"), …)`, em vez de usar
`ui.Submit`) acaba com o atributo duas vezes — HTML inválido que hoje só funciona porque o
`type` do chamador aparece primeiro no HTML gerado e o navegador fica com a primeira ocorrência
de um atributo repetido. Nada no pacote garante essa ordem.

`ui.WebhooksPanel` (#202) desenha "Revoke" e "Ping" numa linha ativa e nada numa linha
`Revoked`. Uma aplicação cujo backend sabe reativar uma assinatura não tem como oferecer isso
pelo painel: sobra cadastrar de novo (o que troca o segredo, e obriga quem integra a
reconfigurar a própria ponta) ou reativar por uma via que o painel não mostra.

## O que muda

### `ui.Bars` — o `viewBox` cresce para caber o rótulo mais longo

```go
ui.Bars(data, attrs...) h.Node // assinatura igual
```

O `viewBox` passa a ser `max(320, barX+barMax+6+7×rúnes do maior Datum.text())` em vez de
sempre `320`. `7` é a mesma estimativa de largura de caractere (fonte de 11 px) que a issue
sugere — o framework não tem como medir glifo no servidor, só reservar largura suficiente para
não adivinhar curto. A barra em si (posição, `barMax`) não muda; só cresce o espaço reservado
depois dela. Uma série cujo texto mais longo já cabe nos 320 atuais desenha exatamente como
antes — os goldens de `ui/testdata/*.svg` com rótulos curtos não mudam.

### `ui.Button` — o `type` do chamador nunca duplica

```go
ui.Button(children ...h.Node) h.Node // assinatura igual
```

`Button` só acrescenta o `type="button"` padrão quando nenhum filho já define um `type`; um
`h.Type("submit")` do chamador substitui o padrão em vez de conviver com ele. `ui.Submit`, que
já escrevia o `type` certo desde sempre, não muda.

### `ui.WebhooksPanel` — a linha revogada ganha "Reactivate"

```go
type WebhooksOpts struct { /* campos iguais */ }
func WebhooksPanel(c *trilha.Ctx, subs []WebhookRow, deliveries []DeliveryRow, o WebhooksOpts) h.Node
```

Uma linha com `Revoked: true` passa a desenhar um botão "Reactivate"/"Reativar" que posta
`action=activate` e `id=<ID>` para `o.Action` — a mesma mecânica de formulário oculto que
"Revoke", "Retry" e "Ping" já usam. Sem `o.Action` (painel só de leitura) nenhum botão é
desenhado, como hoje. A linha ativa continua sem ganhar um botão a mais.

O módulo `webhook.Hooks.Handle` deste repositório não interpreta `action=activate` — ele não
ganha um método `Activate` nesta spec. O pedido da issue é o botão do painel: quem tem um
backend que sabe reativar (o caso relatado, com um `PATCH` próprio) já pode desenhá-lo; quem
usa `webhook.Hooks.Handle` sem mais nada tem o botão visível, mas o clique volta como "ação
desconhecida" até o módulo ganhar o método — trabalho maior, de outra spec.

## Fora de escopo

- Um método `Activate`/`Unrevoke` em `webhook.Hooks` e o caso em `Handle`: a issue #202 pede o
  botão do painel; a semântica de "revogar é reversível" no módulo é decisão de outra spec, e
  fica anotada como sugestão de issue no corpo da PR.
- `ui.Collapsible` com resumo customizável, mencionado como "um segundo, menor" na #198: não é
  o defeito que o título da issue nomeia, e vira sugestão de issue própria.
- Medir texto de verdade (canvas, fonte embutida): a estimativa de 7 px por caractere é a mesma
  aproximação que a própria issue #195 sugere; o framework continua sem depender de fonte.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Os três ajustes usam `math`/`strconv`/`strings`, já importados em `ui/`; nenhuma dependência nova |
| VI — teste primeiro | `ui/chart_test.go`, `ui/ui_test.go` e `ui/webhooks_test.go` ganham os testes que provam cada defeito antes da correção |
| VII — segurança por padrão | Nenhuma mudança em CSRF, sanitização ou política; o formulário novo de "Reactivate" carrega `o.CSRF` como os demais |

## Tarefas

- [x] T001 Teste que falha: `Bars` com o texto mais longo na barra de maior valor — o x do
      texto somado à largura estimada excede o `viewBox` atual
- [x] T002 Teste que falha: `Button(h.Type("submit"), …)` sai com dois atributos `type`
- [x] T003 Teste que falha: `WebhooksPanel` com uma linha `Revoked` não desenha `action=activate`
- [x] T004 Implementação dos três ajustes em `ui/chart.go`, `ui/ui.go`, `ui/webhooks.go`
- [x] T005 Demo do site (`site/internal/demos/kit.go`) ganha uma assinatura revogada, para o
      catálogo mostrar o botão novo nas duas línguas
- [x] T006 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T007 `make test` verde

## Aceitação

- `ui.Bars` com o `Datum.Text` da barra de maior valor tendo mais de 5 caracteres não corta o
  rótulo: `x` do `<text class="ui-chart-value">` mais a largura estimada do texto fica dentro
  do `viewBox`;
- uma série cujo rótulo mais longo já cabe em 320 unidades desenha o mesmo SVG de antes
  (goldens inalterados);
- `ui.Button(h.Type("submit"), h.Text("…"))` renderiza `type="submit"` uma única vez;
- `ui.WebhooksPanel` com uma `WebhookRow{Revoked: true}` e `o.Action` preenchido desenha um
  formulário com `value="activate"` apontando para o `ID` daquela linha; sem `o.Action` nenhum
  formulário é desenhado; a linha ativa não ganha o botão;
- `make test` verde.
