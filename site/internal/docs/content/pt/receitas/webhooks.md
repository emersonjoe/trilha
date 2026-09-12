---
title: Avisar outra aplicação
description: trilha/webhook — entrega assinada, retentativa com espera crescente, registro de cada tentativa, e a checagem de endereço que impede o seu servidor de ler as próprias credenciais da nuvem.
---

Avisar outra aplicação de que algo aconteceu é o que todo app interno acaba precisando, e o que
se escreve para isso é um `http.Post` dentro do manipulador. Isso tem quatro problemas, e nenhum
deles aparece no dia em que o código é escrito.

A requisição do visitante espera o servidor de outra pessoa — outra empresa, outra rede, às vezes
outra década. Não há assinatura, então o receptor não tem como distinguir a sua chamada da de
quem descobriu a URL. Não há retentativa: o parceiro reiniciou às três da manhã e aquele evento
simplesmente não existiu. E não há registro, então quando alguém pergunta "vocês mandaram?" a
resposta é procurar no log.

Tem um quinto, mais feio. A URL é do parceiro, mas quem digita trabalha para você. Um webhook
apontado para `http://169.254.169.254/` é o seu servidor buscando as credenciais da nuvem daquela
máquina e entregando para quem cadastrou o endereço.

## Ou: `trilha add webhooks`

```bash
trilha add webhooks --dry-run
```

```text
  + internal/avisos/avisos.go
  + internal/avisos/avisos_test.go
  + app/webhooks/page.go
  + webhooks_test.go
  ~ app/setup.go (uma linha acrescentada)

--dry-run: nada foi escrito
```

Isso é a lista fechada de eventos, o entregador ligado ao `Env` do app, e a tela
`ui.WebhooksPanel` — entrega assinada, retentativa e o registro de cada tentativa, rodando
antes de você escrever uma linha. O que ela não escreve é o middleware que guarda a tela,
porque não tem como saber como o seu projeto autentica; a própria última linha do comando
diz isso. O que esta página soma é tudo abaixo do piso da receita: o que realmente vai no
fio e por que o timestamp fica dentro da assinatura, a checagem de endereço e por que ela
roda duas vezes, e o esquema em SQL para quando o relógio da retentativa precisa sobreviver
a um restart.

## Emitindo

```go
	hooks := webhook.New(webhook.Options{
		Events: []string{"documento.processado", "documento.falhou"},
		Env:    a.Env(),
	})
	trilha.Provide(a, hooks)
	if err := hooks.Setup(a); err != nil {
		return err
	}
```

A lista de `Events` é fechada, e é esse o ponto: um erro de digitação num `Emit` vira erro onde
está escrito, em vez de um evento que ninguém assinou — que, visto de fora, é idêntico a um
parceiro que não está ouvindo.

O `Setup` liga os workers e o relógio. O relógio é o que torna o backoff real: uma retentativa em
doze horas é uma linha com hora marcada, e não uma goroutine dormindo que um deploy esqueceria.

Depois, uma linha onde a coisa acontece:

```go
func (m *motor) avisa(evento string, doc documentos.Documento) {
	if m.hooks == nil {
		return
	}
	if err := m.hooks.Emit(nil, evento, map[string]any{
		"id": doc.ID, "nome": doc.Nome, "tipo": doc.Tipo, "bytes": doc.Bytes,
	}); err != nil {
		slog.Default().Error("webhook", "evento", evento, "documento", doc.ID, "err", err)
	}
}
```

O `Emit` grava uma entrega por assinatura que ouve, e volta. Não espera a rede, que é a razão
inteira de ele existir. O erro dele é sobre *esta* aplicação — evento desconhecido, store que não
gravou — e nunca sobre o parceiro: se o parceiro respondeu é um registro de entrega, e um
manipulador não tem por que esperar para descobrir.

O contexto `nil` é um job de fundo, onde nunca houve requisição. De um manipulador, passe `c` e o
tenant e a linha de auditoria vêm junto.

## O que vai na rede

```
POST https://parceiro/hook
Content-Type: application/json
X-Webhook-Id: dlv_…
X-Webhook-Event: documento.processado
X-Webhook-Timestamp: 1757343845
X-Webhook-Signature: sha256=…
```

A assinatura é HMAC-SHA256 de `timestamp.corpo`. O horário está **dentro** da string assinada, e
não apenas ao lado: assinar só o corpo faria toda entrega do mesmo evento ser idêntica byte a
byte para sempre, então quem capturasse uma poderia repeti-la um ano depois e ela ainda
conferiria.

2xx em dez segundos é entregue. Qualquer outra coisa espera e tenta de novo — 1 min, 5, 30, 2 h,
12 h — e desiste na sexta, guardando o último status e o primeiro quilobyte do corpo. **É por
esse corpo que ele é guardado**: "422 campo destinatario obrigatório" resolve em um minuto o que
"falhou" não resolve em uma tarde.

## Recebendo

O outro lado, em Go:

```go
func Receber(c *trilha.Ctx) error {
	corpo, err := webhook.Verify(c.Request(), hookSecret())
	if err != nil {
		// Uma resposta só para todas as formas de falhar: dizer qual delas é
		// dizer a um estranho o quanto ele chegou perto.
		return trilha.Errorf(http.StatusUnauthorized, "assinatura inválida")
	}
	var ev struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	}
	if err := json.Unmarshal(corpo, &ev); err != nil {
		return trilha.Errorf(http.StatusBadRequest, "corpo inválido")
	}
	// O id da entrega é estável entre as tentativas da mesma entrega: guardá-lo
	// e recusar repetido é o que transforma o "pelo menos uma vez" que o
	// remetente promete no "exatamente uma vez" que você decide.
	if visto(c.Request().Header.Get(webhook.HeaderID)) {
		return c.Text(http.StatusOK, "ok")
	}
	enfileira(ev.ID)
	return c.Text(http.StatusOK, "ok")
}
```

Três linhas e uma ordem, e a ordem é a parte que se erra. **Responda 2xx antes de fazer o
trabalho.** Um receptor que processa e só depois responde é um receptor que o remetente considera
fora do ar — então ele reenvia, e o trabalho aconteceu duas vezes.

O `Verify` lê o corpo e devolve os bytes, que é por que a assinatura dele é essa: deixar quem
chama com um reader já consumido é o engano que ele existe para tornar impossível. Ele confere a
idade nos dois sentidos — um horário do futuro é um relógio errado ou uma assinatura que alguém
está preparando para usar depois — e compara em tempo constante.

## A checagem de endereço

É a parte que ninguém escreve sozinho, e o motivo é que o caso perigoso não parece perigoso: a
URL é de um parceiro, mas o formulário é preenchido por alguém de dentro.

- `https` sempre; `http` só quando o `Env` é `Dev`.
- Todo endereço para o qual o nome resolve tem de ser público. **Todo**, porque um nome que
  responde com um IP público e um loopback passaria.
- Conferido no cadastro **e outra vez na hora de entregar**, porque um nome que resolvia para o
  parceiro no cadastro e resolve para `169.254.169.254` agora é o ataque; uma checagem só seria o
  acidente.
- Em dev, loopback e rede privada valem — um receptor em `localhost:4000` é como qualquer pessoa
  experimenta isto na primeira vez, e uma checagem que recusa isso é uma que alguém desliga
  inteira. Link-local é recusado em todo ambiente, porque não é o ambiente de desenvolvimento de
  ninguém e uma configuração de dev que sobe para produção é exatamente como esse caso morde.

O cliente também não segue redirecionamento: um 302 do host do parceiro para outro lugar é um
destino que ninguém conferiu.

## A tela

O `ui.WebhooksPanel` é cadastrar, listar, reenviar e testar. Ele desenha dados simples, então a
aplicação mapeia o que o módulo guarda para o que a tela mostra — e é nesse mapeamento que "o que
a tela mostra" é decidido:

```go
func linhas(subs []webhook.Subscription) []ui.WebhookRow {
	out := make([]ui.WebhookRow, 0, len(subs))
	for _, s := range subs {
		out = append(out, ui.WebhookRow{ID: s.ID, Label: s.Label, URL: s.URL,
			Events: s.Events, Created: s.Created, Revoked: s.Revoked})
	}
	return out
}
```

O segredo não está nessa linha e não pode estar. Ele aparece uma vez, quando a assinatura é
criada, e atravessa o redirecionamento num cookie próprio — o `webhook.TakeSecret(c)` lê e apaga.
O que fica guardado é um `trilha.Secret`: cifrado na coluna, mascarado no log.

```go
func POST(c *trilha.Ctx) error {
	return trilha.Use[*webhook.Hooks](c).Handle(c)
}
```

Um POST para cadastrar, revogar, reenviar e testar, despachado por um campo `action` escondido,
porque são uma tela só — e um formulário que posta para si mesmo volta para si mesmo quando algo
está errado.

Revogar não é apagar. As entregas já feitas apontam para aquela assinatura, e uma tela que não
consegue dizer de qual endereço era a falha não serve para ninguém depois.

## Guardando numa tabela

O `Memory()` é o padrão. Uma tabela compra o histórico e, mais importante, faz o calendário das
retentativas sobreviver a um reinício:

```go
const WebhookSchema = `
CREATE TABLE IF NOT EXISTS trilha_webhooks (
	id       TEXT PRIMARY KEY,
	tenant   TEXT NOT NULL DEFAULT '',
	url      TEXT NOT NULL,
	events   TEXT NOT NULL DEFAULT '',
	secret   BYTEA NOT NULL,
	label    TEXT NOT NULL DEFAULT '',
	created  TIMESTAMPTZ NOT NULL,
	revoked  BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE TABLE IF NOT EXISTS trilha_webhook_deliveries (
	id          TEXT PRIMARY KEY,
	webhook_id  TEXT NOT NULL REFERENCES trilha_webhooks(id),
	tenant      TEXT NOT NULL DEFAULT '',
	event       TEXT NOT NULL,
	payload     BYTEA NOT NULL,
	state       TEXT NOT NULL,
	attempt     INTEGER NOT NULL DEFAULT 0,
	next_try    TIMESTAMPTZ,
	status      INTEGER NOT NULL DEFAULT 0,
	err         TEXT NOT NULL DEFAULT '',
	response    TEXT NOT NULL DEFAULT '',
	retry_of    TEXT NOT NULL DEFAULT '',
	created     TIMESTAMPTZ NOT NULL,
	ended       TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS trilha_webhook_due ON trilha_webhook_deliveries (next_try) WHERE state = 'pending';
CREATE INDEX IF NOT EXISTS trilha_webhook_recent ON trilha_webhook_deliveries (id DESC);
`
```

```go
func (s WebhookSQL) Due(ctx context.Context, at time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id FROM trilha_webhook_deliveries
		WHERE state = 'pending' AND next_try <= $1
		ORDER BY next_try LIMIT $2`, at, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
```

O `Due` é a única consulta que roda o tempo todo; o índice parcial existe para essa linha. O
arquivo inteiro está em `examples/cookbook/webhooks.go`, com sabor Postgres.

:::warning
Com duas réplicas, dois relógios acham a mesma entrega vencida e o parceiro recebe duas vezes.
Acrescente `FOR UPDATE SKIP LOCKED` ao `Due`, dentro de uma transação que também tire a linha de
`pending`, ou rode o remetente numa instância só. O módulo é honesto quanto a isso: é um
processo, e o [módulo de tarefas](/pt/receitas/tarefas) diz a mesma coisa pelo mesmo motivo.
:::

## O que está testado

A assinatura é conferida rodando o `Verify` de verdade contra o que o remetente de verdade
produziu — e não recomputando o HMAC no teste, o que provaria só que uma fórmula bate com ela
mesma.

O backoff é testado contra um relógio que o teste controla, para provar uma espera de doze horas
sem levar doze horas. A checagem de endereço é testada contra um resolvedor que o teste controla,
inclusive no caso que uma checagem descuidada deixaria passar: um nome que responde com um
endereço bom e um ruim.
