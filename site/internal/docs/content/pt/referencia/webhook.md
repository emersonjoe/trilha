---
title: webhook
description: Hooks, Options, Emit, Subscribe, Verify, Store e o painel — a API do pacote webhook, com os padrões e o que cada campo muda.
---

`import "github.com/emersonjoe/trilha/webhook"` — avisar outra aplicação de que algo aconteceu,
sem fazer o visitante esperar o servidor de outra pessoa, sem deixar o receptor incapaz de
distinguir a sua chamada da de um estranho, e sem perder o evento porque o parceiro reiniciou às
três da manhã.

## O remetente

```go
func New(o Options) *Hooks                        // não inicia nada
func (h *Hooks) Setup(a *trilha.App) error        // workers + relógio + Shutdown no app
func (h *Hooks) Shutdown(ctx context.Context) error
func (h *Hooks) Events() []string
```

O `Setup` liga os workers e o relógio. É o relógio que transforma "tenta de novo em doze horas"
numa linha com hora marcada, em vez de uma goroutine dormindo que um deploy esqueceria. O
`Shutdown` espera as tentativas em curso; o que está pendente continua pendente, porque é uma
linha e não uma goroutine.

| `Options` | Padrão | O que faz |
|---|---|---|
| `Store Store` | `Memory()` | onde assinaturas e entregas moram |
| `Events []string` | — | a lista fechada do que esta aplicação emite |
| `Env trilha.Env` | `Prod` | se `http://` vale; passe `a.Env()` |
| `Workers int` | 2 | quantas entregas saem ao mesmo tempo |
| `Timeout time.Duration` | 10 s | uma tentativa |
| `Backoff []time.Duration` | 1 m, 5 m, 30 m, 2 h, 12 h | as esperas; o tamanho é quantas tentativas existem |
| `Tick time.Duration` | 15 s | de quanto em quanto o relógio procura o que venceu |
| `HTTP *http.Client` | sem redirecionamento | um 302 é um destino que ninguém conferiu |
| `AllowPrivateURL bool` | false | desliga a checagem de endereço — veja abaixo |

O valor zero de `Env` é `Prod`, que é o lado seguro para um campo que alguém esquece.

## Emitindo

```go
func (h *Hooks) Emit(c *trilha.Ctx, event string, payload any) error
```

Grava uma entrega por assinatura que ouve o evento, e volta. **Não espera a rede**, que é a razão
inteira de existir. O erro dele é sobre esta aplicação — evento desconhecido, store que não
gravou — e nunca sobre o parceiro.

A lista de `Events` é fechada: um erro de digitação num `Emit` é erro onde está escrito, e não um
evento que ninguém assina — que, de fora, é indistinguível de um parceiro que não está ouvindo.
Contexto `nil` é um job de fundo; de um manipulador, passe `c` e o tenant e a linha de auditoria
vêm junto.

O `trilha openapi` lê essas chamadas: um `Emit` com o nome do evento escrito nele vira uma entrada
na seção `webhooks` do documento, com o schema do payload como corpo e os quatro cabeçalhos da
entrega descritos. Um struct que já é componente de uma rota entra ali por `$ref`, não copiado —
então quem integra do outro lado lê um documento em vez do seu código. Um `Emit` cujo nome vem de
uma variável não é documentado: um documento não pode dizer um nome que só existe na hora de
rodar.

## O que vai na rede

```
X-Webhook-Id: dlv_…              estável entre as tentativas de uma entrega
X-Webhook-Event: documento.processado
X-Webhook-Timestamp: 1757343845
X-Webhook-Signature: sha256=…    HMAC-SHA256 de "timestamp.corpo"
```

```go
func Sign(secret, timestamp string, body []byte) string
```

O horário está **dentro** da string assinada. Assinar só o corpo faria toda entrega de um evento
ser idêntica byte a byte para sempre, então uma requisição capturada poderia ser repetida um ano
depois e a assinatura ainda conferiria.

2xx dentro do `Timeout` é `Delivered`. Qualquer outra coisa espera e tenta de novo; depois do
último backoff vira `Failed`, guardando o último status e o primeiro quilobyte da resposta — que
é o que transforma "falhou" em "422, campo destinatario obrigatório".

## Administrando

```go
func (h *Hooks) Subscribe(c *trilha.Ctx, s Subscription) (trilha.Secret, error)
func (h *Hooks) Revoke(c *trilha.Ctx, id string) error
func (h *Hooks) Retry(c *trilha.Ctx, deliveryID string) (string, error)
func (h *Hooks) Ping(c *trilha.Ctx, subscriptionID string) (string, error)
func (h *Hooks) Handle(c *trilha.Ctx) error       // um POST, despachado pelo "action"
func TakeSecret(c *trilha.Ctx) string             // uma vez, logo depois do Subscribe
```

O `Subscribe` devolve o segredo **uma vez**. O que fica guardado é um `trilha.Secret` — cifrado
na coluna, mascarado no log — e uma aplicação que conseguisse mostrar de novo seria uma que o
mantém legível, o que faz o segredo valer exatamente o que vale o backup do banco. Ele atravessa
o redirecionamento num cookie assinado próprio; o `TakeSecret` lê e apaga.

O `Revoke` não apaga: as entregas já feitas apontam para aquela assinatura, e uma tela que não
consegue dizer de qual endereço era a falha não serve para nada depois. O `Subscriptions` traz as
revogadas por isso; quem para a entrega é o `Subscription.Wants`.

O `Retry` devolve um id **novo** apontando para o antigo, e manda os mesmos bytes — então um
parceiro que confere o `X-Webhook-Id` vê outra entrega do mesmo evento, e não um corpo que mudou
sob o mesmo nome. A tentativa que falhou continua lá.

O `Ping` passa por tudo que uma entrega de verdade passa, porque um teste que toma atalho passa
para uma configuração que não vai funcionar.

## Recebendo

```go
func Verify(r *http.Request, secret string) ([]byte, error)
var Tolerance = 5 * time.Minute
var ErrSignature = errors.New("webhook: signature does not check out")
```

Lê o corpo, confere a assinatura e a idade, e devolve os bytes — essa assinatura existe para quem
chama nunca ficar com um reader já consumido. A idade é conferida nos dois sentidos: um horário
do futuro é um relógio errado ou uma assinatura que alguém está preparando. A comparação é em
tempo constante, e o `ErrSignature` é um erro só para todas as formas de falhar, porque dizer
qual parte falhou diz a um atacante o quanto ele chegou perto.

**Responda 2xx antes de fazer o trabalho.** Um receptor que processa primeiro é um receptor que o
remetente considera fora do ar e reenvia. O `X-Webhook-Id` é estável entre as tentativas de uma
entrega: guarde-o e recuse repetido, porque "pelo menos uma vez" é o que o remetente promete e
"exatamente uma vez" é o que o receptor decide.

## A checagem de endereço

`https` sempre; `http` só em `Dev`. Todo endereço para o qual o nome resolve tem de ser público —
*todo* endereço, porque um nome que responde um IP público e um loopback passaria — e é conferido
no cadastro **e outra vez na entrega**, porque um nome que resolvia para o parceiro naquele
momento e resolve para `169.254.169.254` agora é o ataque, não o acidente.

Em `Dev`, loopback e rede privada passam: um receptor em `localhost:4000` é como qualquer pessoa
experimenta isto na primeira vez, e uma checagem que recusa isso é uma que alguém desliga
inteira. Link-local (`169.254.0.0/16`, onde moram os metadados da nuvem), não especificado,
multicast e reservado são recusados em todo ambiente.

O `AllowPrivateURL` desliga a checagem. Está escrito por extenso porque ligá-lo é desligar a
defesa contra um webhook apontado para o próprio serviço de metadados; ele existe para os testes
deste pacote e para uma aplicação cujos parceiros estão mesmo na mesma rede privada.

## Store

```go
type Store interface {
	SaveSubscription(ctx context.Context, s Subscription) error
	Subscription(ctx context.Context, id string) (Subscription, error)
	Subscriptions(ctx context.Context, tenant string) ([]Subscription, error)
	SaveDelivery(ctx context.Context, d Delivery) error
	Delivery(ctx context.Context, id string) (Delivery, error)
	Deliveries(ctx context.Context, p ListParams) ([]Delivery, error)
	Due(ctx context.Context, at time.Time, limit int) ([]string, error)
}

func Memory() Store
```

O `Due` é o que o relógio chama, e o que merece um índice atrás. Não há implementação SQL aqui —
a mesma escolha de todo store deste framework — e a [receita](/pt/receitas/webhooks) traz o
arquivo inteiro.

## A tela

```go
func ui.WebhooksPanel(c *trilha.Ctx, subs []ui.WebhookRow, deliveries []ui.DeliveryRow, o ui.WebhooksOpts) h.Node
```

O `WebhooksOpts` recebe `Action` (para onde os formulários postam; vazio desenha um painel só de
leitura), `CSRF`, `Events` e `Secret`. Ele desenha linhas simples, então a aplicação mapeia o que
o módulo guarda para o que a tela mostra — e o `ui.WebhookRow` não tem campo para o segredo, que
é o motivo de ele não conseguir vazar numa listagem.

## O que não está aqui

Assinatura assimétrica e mais de um segredo ativo por assinatura — rotação é real e é a próxima
coisa; hoje o caminho é um segundo `Subscribe`. Entrega entre processos: aqui é um remetente só,
e duas réplicas mandam tudo duas vezes.
