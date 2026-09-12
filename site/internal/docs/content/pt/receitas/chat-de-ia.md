---
title: Chat de IA
description: Um chat com modelo dentro do seu app, do zero à tela — onde fica a chave, o que a página conta ao modelo, quem guarda o histórico, o que acontece quando o provedor cai e como o teste roda sem chave.
---

O [Aprender](/pt/aprender/ia-e-agentes) ensina os pedaços: uma chamada, uma ferramenta, um
agente, o `ai.Serve` e o `ui.Chat`. Aqui está tudo montado, na ordem em que um aplicativo
precisa, com as decisões que ninguém escreve: onde fica a chave, o que a página conta ao
modelo, quem guarda o histórico, o que a pessoa lê quando o provedor está num dia ruim, e
como o teste roda sem chave e sem rede.

A tela onde isso termina é [a demo](/pt/demos/ai-chat): um chat de atendimento ao lado do
pedido de que ele fala.

## 1. O cliente

A chave vem do ambiente, como a URL do banco. Nunca está no código, nunca num commit e nunca
num log — o `ai.Client` a mantém fora dos três.

```go
// AIChatClient is the provider, built once at startup. The key never appears
// in the code and never in a log — it comes from the environment, which is the
// same place the database URL comes from.
var AIChatClient = newAIChatClient()

// newAIChatClient picks the provider from what the environment has. Anthropic
// answers the same chat-completions shape at its OpenAI-compatible base URL,
// so one Client covers both and the app has no provider branch past this
// function. With neither key the client still builds, and only a request
// fails: a missing key must not stop the app from starting.
func newAIChatClient() *ai.Client {
	c := ai.NewFromEnv() // OPENAI_API_KEY, OPENAI_BASE_URL, TRILHA_AI_MODEL
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" && os.Getenv("OPENAI_API_KEY") == "" {
		c.BaseURL, c.APIKey = "https://api.anthropic.com/v1", key
	}
	c.HTTPClient = &http.Client{Timeout: AIChatTimeout}
	return c
}
```

O tempo limite é decisão, não padrão:

```go
// AIChatTimeout is how long one answer may take. The default client waits two
// minutes, which is longer than anybody sits in front of a support chat: a
// provider having a bad day should give up before the visitor does.
const AIChatTimeout = 60 * time.Second
```

:::note
O `ai.Client` fala o formato *chat completions*. OpenAI, Anthropic (na URL compatível com a
OpenAI), Groq, OpenRouter, Ollama e vLLM respondem esse formato, então trocar de provedor é
uma URL e uma chave, não uma reescrita.
:::

O que um administrador muda sem deploy não é a chave — é o modelo, o quanto ele inventa e
quem o assistente é. Isso é uma [seção de configuração](/pt/referencia/app#settings): uma
struct, as tags dela e uma tela que o framework monta a partir disso.

```go
// AIChatConfig is the part of the assistant an administrator changes without a
// deploy: which model answers, how much it invents, and who it is told to be.
// The tags do three jobs at once — json stores it, form names the input, and
// validate is the rule, the same one on the screen and on anything else that
// saves it.
type AIChatConfig struct {
	Model        string  `json:"model"        form:"model"        validate:"max=60"            label:"Model"        help:"the name the provider knows; empty keeps the one the environment chose"`
	Temperature  float64 `json:"temperature"  form:"temperature"  validate:"min=0,max=2"        label:"Temperature"  help:"0 answers the same way every time; 2 invents"`
	Instructions string  `json:"instructions" form:"instructions" validate:"required,max=2000" label:"Instructions" help:"who the assistant is, and what it must not answer"`
}

// AIChatSettings is the section, with the defaults that answer before anybody
// has saved anything. It is a package var because configuration is not built
// per request — it is read on every one.
var AIChatSettings = trilha.NewSettings("ai-chat", AIChatConfig{
	Temperature: 0.2,
	Instructions: "You are the support assistant of an online store. Answer about the " +
		"order the visitor has open, in the language the question was asked in. " +
		"When the answer is not in what you were told, say so instead of guessing.",
})
```

O agente é montado por requisição, sobre uma cópia da configuração, para que a mudança na
tela de administração valha a partir da próxima mensagem:

```go
// AIChatAgent is the agent of one request. It is built per request and from a
// copy of the settings, so changing the model on the administration screen
// takes effect on the next message — without a restart, and without a request
// already running seeing the agent change underneath it.
func AIChatAgent() *ai.Agent {
	cfg := AIChatSettings.Get()
	return &ai.Agent{
		Name:         "support",
		Instructions: cfg.Instructions,
		Model:        cfg.Model,
		Temperature:  &cfg.Temperature,
	}
}
```

:::note
Guardar a chave do provedor na própria seção é uma opção de verdade — o `trilha.Secret` a
mantém cifrada em repouso e nunca a devolve para a tela. Vale quando alguém além do deploy
precisa trocá-la.
:::

## 2. A rota

A rota é o `ai.Serve` e, em volta dele, só o que o app sabe: quem está perguntando, com que
frequência pode perguntar, e o que significa a página em que a pessoa está.

```go
// AIChatPOST is app/api/chat/route.go, the whole route. ai.Serve reads the
// message, runs the agent and answers: a stream of events for ui.chat.js, the
// finished answer for a plain form submit. What the app adds around it is what
// only the app knows — who is asking, how often they may ask, and what the
// page they are on means.
func AIChatPOST(c *trilha.Ctx) error {
	who := AIChatWho(c)
	if ok, after := AIChatLimit.Allow(who); !ok {
		c.Header("Retry-After", strconv.Itoa(after))
		return trilha.Errorf(http.StatusTooManyRequests, "too many questions in a row; try again in %ds", after)
	}
	// The question is audited, the answer is not: the trail says who asked
	// what and when, and the conversation itself stays where it belongs.
	c.Audit("ai.chat.asked", who)
	return ai.ServeOpts{
		HTML:    ui.ChatHTML,
		Context: AIChatContext,
		Page:    AIChatAnswerPage,
	}.Serve(c, AIChatClient, AIChatAgent())
}
```

O `ai.Serve` lê a mensagem, roda o agente e responde de dois jeitos: um fluxo de eventos
nomeados quando o cliente pediu `text/event-stream`, e a resposta inteira de uma vez quando
não pediu. A segunda é a que chega com o JavaScript desligado — mesma rota, mesmo agente,
mesma linha de auditoria.

```go
// AIChatAnswerPage answers the request that did not ask for a stream — the
// plain form submit, which is what arrives when JavaScript is not there. It
// runs after the whole answer is ready, so the turn goes into the log and the
// visitor is sent back to the page that has it. A redirect and not a render:
// reloading must not ask the model a second time.
func AIChatAnswerPage(c *trilha.Ctx, res *ai.Result) error {
	id := c.Form("ctx.order_id")
	AIChatLog.Append(AIChatWho(c), c.Form("message"), res.Output)
	if id == "" {
		return c.Redirect("/")
	}
	return c.Redirect("/orders/" + url.PathEscape(id))
}
```

Um redirecionamento e não uma renderização, pelo motivo de sempre: recarregar a página não
pode perguntar de novo ao modelo.

## 3. A tela

O `ui.Chat` é a conversa, o campo e o botão; o `ui.ChatScript` é o cliente que faz a resposta
chegar palavra por palavra. Nenhum dos dois é obrigatório para a página funcionar.

```go
// AIChatScreen is the page with the chat in it: the conversation, the field
// and the button, plus the script that makes the answer arrive word by word.
// Without the script the same form posts to the same route and the page comes
// back with the answer in it — nothing on the screen depends on JavaScript.
func AIChatScreen(c *trilha.Ctx, o AIChatOrder) h.Node {
	return h.Div(h.Class("ui-stack"),
		ui.Chat(c, ui.ChatOpts{
			Action:   "/api/chat",
			History:  AIChatLog.Read(AIChatWho(c)),
			Greeting: "Ask anything about this order — where it is, when it arrives, what it cost.",
			// What the page knows travels with every message, as JSON with
			// the script and as hidden fields without it.
			Context: map[string]string{"order_id": o.ID},
		}),
		ui.ChatScript(c),
	)
}
```

O `Context` é a parte que faz um assistente ser útil em vez de impressionante: o que a página
sabe e o modelo não. Cada item vira um campo escondido `ctx.<chave>`, enviado junto da
mensagem pelo script e pelo formulário puro, e lido do outro lado pelo
`ai.ServeOpts.Context`:

```go
// AIChatContext turns the ctx.* fields the page sent into the sentence the
// model reads before the conversation. It is what the page knows and the model
// does not.
//
// The fields come from the request, so they are what the visitor sent and not
// what the server knows: the lookup takes the owner as well as the id, which
// makes it the permission check. An id that is not theirs finds nothing, and
// the model is told nothing.
//
// The messages are user turns and not system ones. The history travels through
// the browser, so a run drops every system message it finds in it — that is
// what stops a page from being talked into new instructions, and it applies
// here too. What the app really wants to fix goes in Agent.Instructions, which
// no request can touch.
func AIChatContext(c *trilha.Ctx, fields map[string]string) []ai.Message {
	id := fields["order_id"]
	if id == "" {
		return nil
	}
	o, ok := AIChatFindOrder(AIChatWho(c), id)
	if !ok {
		return []ai.Message{ai.User("I have no order open. Answer only general questions about the store.")}
	}
	return []ai.Message{ai.User(fmt.Sprintf(
		"I am looking at order %s, placed on %s by %s. Status: %s. Total: %s. Answer about this order and no other.",
		o.ID, o.Placed.Format("2006-01-02"), o.Customer, o.Status, o.Total))}
}
```

O registro que ele lê é do app, e é a única coisa que o modelo fica sabendo:

```go
// AIChatFindOrder is the app's query, with the owner in the WHERE clause. It
// is a var because this file has no database; in an application it is the
// same function the order page calls. The default finds nothing, which is the
// safe way to notice it was never wired.
var AIChatFindOrder = func(who, id string) (AIChatOrder, bool) { return AIChatOrder{}, false }
```

:::note
Os campos `ctx.*` vêm da requisição: são o que o visitante mandou, não o que o servidor sabe.
A consulta recebe o dono junto do id, e é isso que a transforma na conferência de permissão
em vez de uma busca. Trate um id daí exatamente como um id na URL.
:::

Texto de modelo é Markdown, e o `ui.ChatHTML` é o adaptador que o renderiza do mesmo jeito
que o histórico é renderizado: um `<script>` na resposta é texto na página, não uma tag no
documento.

## 4. O histórico

O histórico é do app. O framework não guarda sessão de conversa, e isso é de propósito: uma
conversa é um registro, e onde ficam os registros não é decisão de framework.

Com o script, a conversa viaja junto de cada mensagem e o `ai.ServeOpts.MaxHistory` limita
quanto dela volta para o modelo — quem manda é o navegador, então o teto fica no servidor.
Isso basta para um chat que vive enquanto a página está aberta.

Quando a conversa precisa sobreviver à página, quem guarda é o app. Este store é um mapa
atrás de um mutex, o tamanho certo para um processo e o errado para dois:

```go
// AIChatMaxTurns is how many messages one conversation keeps. The ceiling is
// the bill: every turn is sent again with the next question, so a conversation
// that never forgets is a conversation that costs more every time.
const AIChatMaxTurns = 20

// AIChatStore is the conversation, which belongs to the app — the framework
// keeps no chat session. This one is a map behind a mutex, which is the right
// size for one process and the wrong size for two: in an application it is a
// table keyed by (owner, order) with the same two methods, and the recipe for
// that table is the database one.
type AIChatStore struct {
	mu sync.Mutex
	by map[string][]ui.ChatMessage
}

// NewAIChatStore builds an empty store.
func NewAIChatStore() *AIChatStore { return &AIChatStore{by: map[string][]ui.ChatMessage{}} }

// AIChatLog is the store the routes above use.
var AIChatLog = NewAIChatStore()
```

```go
// Append writes the turn that just happened and drops the oldest ones past
// AIChatMaxTurns.
func (s *AIChatStore) Append(who, question, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log := append(s.by[who],
		ui.ChatMessage{Role: "user", Text: question},
		ui.ChatMessage{Role: "assistant", Text: answer})
	if len(log) > AIChatMaxTurns {
		log = log[len(log)-AIChatMaxTurns:]
	}
	s.by[who] = log
}
```

O teto é a conta, não arrumação: cada turno é enviado de novo junto da pergunta seguinte.

E a rota que lê e escreve nele é o `ai.Serve` mais as duas linhas para as quais o `ai.Serve`
não tem lugar — o passado vem do store em vez de vir do navegador, e o turno que acabou de
acontecer volta para lá:

```go
// AIChatKeepPOST is the route to use when the transcript has to survive the
// page — an audit trail, a conversation two people continue, a chat that
// reloads where it stopped. It is ai.Serve with the two lines ai.Serve leaves
// no room for: the past comes from the app's store instead of from the
// browser, and the turn that just happened goes back into it.
//
// Everything else is the same contract: the same event names, the same answer
// without JavaScript, the same ui.Chat on the other side. A chat that only
// lives while the page is open does not need any of this — AIChatPOST is
// shorter and reads the history the browser already keeps.
func AIChatKeepPOST(c *trilha.Ctx) error {
	who := AIChatWho(c)
	message, err := aiChatMessage(c)
	if err != nil {
		return err
	}
	past := AIChatPast(AIChatLog.Read(who))
	agent := AIChatAgent()
	if !strings.Contains(c.Request().Header.Get("Accept"), "text/event-stream") {
		res, err := ai.Run(c.Context(), AIChatClient, agent, message, past...)
		if err != nil {
			return err
		}
		AIChatLog.Append(who, message, res.Output)
		return AIChatAnswerPage(c, res)
	}
	s := c.Stream()
	res, err := ai.RunStream(c.Context(), AIChatClient, agent, message, func(ev ai.Event) {
		if ev.Type == "text" {
			_ = s.Send("text", ev.Text)
		}
	}, past...)
	if err != nil {
		// The provider's own words never reach the screen: they are for the
		// log, where they say which provider failed and how.
		c.Log().Warn("ai chat", "err", err)
		return s.JSON("error", map[string]string{"message": AIChatFailure(err)})
	}
	AIChatLog.Append(who, message, res.Output)
	return s.JSON("done", map[string]any{"output": res.Output, "html": ui.ChatHTML(res.Output)})
}
```

```go
// AIChatPast is the stored conversation in the shape the model reads.
func AIChatPast(msgs []ui.ChatMessage) []ai.Message {
	out := make([]ai.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "assistant" {
			out = append(out, ai.Assistant(m.Text))
			continue
		}
		out = append(out, ai.User(m.Text))
	}
	return out
}
```

:::note
Num aplicativo o store é uma tabela: `(dono, pedido_id, papel, texto, em)` com os mesmos dois
métodos, escrita com a [receita de banco de dados](/pt/receitas/banco-de-dados). Em memória
funciona com um processo e quebra com dois — a segunda instância responde com uma conversa
que a primeira nunca viu.
:::

## 5. Limites e falhas

Um chat é a única tela em que uma pessoa sozinha consegue gastar dinheiro de verdade em
sequência. O teto mora ao lado da rota que gasta:

```go
// AIChatLimit is one bucket per visitor. The limit is not about abuse only: a
// chat is the one screen in an application where a single person can spend
// real money in a loop, and the ceiling belongs next to the route that spends
// it.
var AIChatLimit = trilha.NewLimiter(trilha.RateLimit{RPS: 0.2, Burst: 5})
```

Passado o limite, a rota responde `429` com `Retry-After`. O `ui.chat.js` devolve a pergunta
para o campo e mostra a situação como nota, então ninguém perde mais que tempo.

Quando quem diz não é o provedor, o que a pessoa lê é a frase do app, nas palavras do app, e
ela diz o que fazer em seguida — nunca a frase do provedor:

```go
// AIChatFailure is what the visitor reads when the provider says no. The
// sentence is the app's, in the app's language, and it says what to do next;
// the status code and the provider's message go to the log, where they are
// useful and where they leak nothing.
func AIChatFailure(err error) string {
	var perr *ai.Error
	switch {
	case errors.As(err, &perr) && perr.Status == http.StatusTooManyRequests:
		return "The assistant is answering a lot of people right now. Try again in a few seconds."
	case errors.Is(err, context.DeadlineExceeded):
		return "The answer took too long. Ask again, or ask something shorter."
	case errors.Is(err, ai.ErrMaxTurns):
		return "I could not finish this one. Try asking it in smaller parts."
	}
	return "The assistant is unavailable right now. Everything else on this page still works."
}
```

A mensagem do provedor vai para o log, onde é útil e onde não vaza nada. E a pergunta é
auditada, que é a linha de que um aplicativo precisa muito antes de precisar de métricas:

```go
	c.Audit("ai.chat.asked", who)
```

## 6. Testar sem chave

O teste troca uma coisa só: o `http.Client` que o framework ia chamar de qualquer jeito. Sem
chave, sem conta, sem rede e sem instabilidade — a rota, a tela e a função de contexto rodam
exatamente como em produção.

```go
// AIChatFake is the provider in a test: it answers from a script and keeps
// what it was asked. No key, no bill and no network — the route, the screen
// and the context function run exactly as they do in production, because the
// only thing replaced is the http.Client the framework was always going to
// call.
type AIChatFake struct {
	// Answer is what the model says, every time.
	Answer string

	mu   sync.Mutex
	sent []ai.Message
}

// Client is the ai.Client to put where the real one goes.
func (f *AIChatFake) Client() *ai.Client {
	return &ai.Client{
		BaseURL:    "https://example.invalid/v1",
		APIKey:     "test-key",
		Model:      "test-model",
		HTTPClient: &http.Client{Transport: f},
	}
}
```

Aí o teste confere o que importa: que o contexto da página chegou ao modelo, que a resposta
voltou renderizada e que a tela continua funcionando com o JavaScript desligado.

```go
// The chat recipe is tested the way the page says it is: a scripted provider,
// no key in the environment and nothing dialled. What runs is the real route,
// the real component and the real context function.
func newAIChatTest(t *testing.T, answer string) (*trilha.TestClient, *AIChatFake) {
	t.Helper()
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")

	fake := &AIChatFake{Answer: answer}
	swap(t, &AIChatClient, fake.Client())
	swap(t, &AIChatFindOrder, func(who, id string) (AIChatOrder, bool) {
		return aiChatTestOrder, id == aiChatTestOrder.ID // another visitor's id finds nothing
	})
	swap(t, &AIChatLog, NewAIChatStore())

	a := trilha.New(trilha.ConfigFromEnv())
	a.Register(trilha.Route{Pattern: "/api/chat", Methods: map[string]trilha.HandlerFunc{"POST": AIChatPOST}})
	a.Register(trilha.Route{Pattern: "/orders/{id}", Page: func(c *trilha.Ctx) (h.Node, error) {
		return AIChatScreen(c, aiChatTestOrder), nil
	}})
	return trilha.NewTestClient(t, a), fake
}
```

```go
func TestAIChatAnswersWithoutAKeyOrNetwork(t *testing.T) {
	c, fake := newAIChatTest(t, "Order **1043** shipped on Tuesday.")

	// The screen: the form, what the page knows, and the script.
	c.Get("/orders/1043").WantStatus(200).WantContains(
		`data-trilha-chat="/api/chat"`,
		`name="ctx.order_id" value="1043"`,
		`src="/ui.chat.js`,
	)

	// The submit that arrives when JavaScript is not there: the same route
	// answers it whole and sends the visitor back to the page.
	c.PostForm("/api/chat", url.Values{"message": {"where is my order?"}, "ctx.order_id": {"1043"}}).
		WantStatus(303).WantHeader("Location", "/orders/1043")

	// The page's context reached the model, and the instructions came first.
	sent := fake.Sent()
	if len(sent) < 3 || sent[0].Role != "system" {
		t.Fatalf("the agent's instructions must open the conversation: %#v", sent)
	}
	if !strings.Contains(sent[1].Content, "order 1043") || !strings.Contains(sent[1].Content, "shipped") {
		t.Errorf("the open order must reach the model: %#v", sent[1])
	}
	if last := sent[len(sent)-1]; last.Role != "user" || last.Content != "where is my order?" {
		t.Errorf("the question must be the last turn: %#v", last)
	}

	// The turn is in the log, and the page renders the answer as Markdown.
	if got := AIChatLog.Read("192.0.2.1"); len(got) != 2 || got[0].Text != "where is my order?" {
		t.Fatalf("conversation not stored: %#v", got)
	}
	c.Get("/orders/1043").WantContains("where is my order?", "<strong>1043</strong>")
}
```

:::note
O `make test` roda este arquivo a cada push. Receita cujo teste precisa de chave é receita que
deixa de ser rodada — e, logo depois, deixa de ser verdade.
:::

## 7. A demo

[O chat acima, rodando](/pt/demos/ai-chat). O componente, o cliente de streaming e o contexto
que a página envia são os reais; só o modelo é roteirizado, no navegador, porque este site é
estático e não tem servidor para responder.

## 8. O mesmo chat no canto

O `ui.Assistant` é este chat dentro de um diálogo com um botão de abrir, para o app que o quer
em todas as páginas em vez de em uma:

```go
// AIChatCorner is the same chat in the corner of every page. ui.Assistant is
// ui.Chat inside a dialog with a launcher, and the launcher is a link before
// it is a button: with JavaScript off it goes to Page, the same conversation
// rendered as a page of its own. The route on the other side does not change.
func AIChatCorner(c *trilha.Ctx, o AIChatOrder) h.Node {
	return ui.Assistant(c, ui.AssistantOpts{
		ID:     "order-assistant",
		Action: "/api/chat",
		Page:   "/orders/" + url.PathEscape(o.ID) + "#chat",
		Label:  "Ask about this order",
		Title:  "Order assistant",
		Chat:   ui.ChatOpts{Context: map[string]string{"order_id": o.ID}},
	})
}
```

A rota do outro lado não muda. O botão é um link antes de ser um botão: com o JavaScript
desligado ele vai para `Page`, a mesma conversa renderizada como página — que é
[a demo do assistente](/pt/demos/assistant).

## Para onde ir depois

- [IA e agentes](/pt/aprender/ia-e-agentes) — ferramentas, agentes, streaming, MCP.
- [Referência do `ai`](/pt/referencia/ai) — cada símbolo, inclusive o contrato dos eventos.
- [`ui.Chat`](/pt/referencia/ui#chat) — cada campo do `ChatOpts`.
- [`examples/assistente`](https://github.com/emersonjoe/trilha/tree/main/examples/assistente) —
  os mesmos pedaços num aplicativo completo, com ferramentas e tela de administração.
