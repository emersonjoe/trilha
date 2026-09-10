package recipes

// webhooksRecipe is what an application tells the outside: the closed list of
// events, the deliverer, and the screen whoever integrates with you uses.
//
// The webhook module has existed since 0.65.0 and examples/blog uses it. This
// is the case the issue describes exactly: somebody who needs it today only
// finds it if they already know it is there — and, knowing, still copies six
// files out of an example changing the module path by hand.
func webhooksRecipe() Recipe {
	return Recipe{
		Name: "webhooks",
		Summary: map[string]string{
			"en": "what this app tells the outside: signed delivery with retry, and the screen for it",
			"pt": "o que este app avisa para fora: entrega assinada com retry, e a tela dela",
		},
		Doc: "/reference/webhook",
		Files: []File{
			{Rel: "internal/avisos/avisos.go", Go: true, Body: hooksDecl},
			{Rel: "internal/avisos/avisos_test.go", Go: true, Body: hooksDeclTest},
			{Rel: "{{.At}}webhooks/page.go", Go: true, Body: hooksPage},
			{Rel: "webhooks_test.go", Go: true, Body: hooksTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add webhooks",
			Line:   "\tif err := avisos.Setup(a); err != nil {\n\t\treturn err\n\t}\n",
		}},
		Imports: []string{"{{.Module}}/internal/avisos"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}webhooks, and guard that folder: whoever reaches it " +
				"sees where this application sends data and can point it somewhere else. Then emit where " +
				"the thing actually happens — " +
				"`trilha.Use[*webhook.Hooks](c).Emit(c, \"pedido.pago\", pedido)` — and add the event to the " +
				"list in internal/avisos: it is closed on purpose, so a typo fails where it is written.",
			"pt": "Rode `trilha dev` e abra {{.URL}}webhooks, e guarde essa pasta: quem chega nela vê para " +
				"onde este app manda dados e pode apontá-lo para outro lugar. Depois emita onde a coisa " +
				"acontece de verdade — " +
				"`trilha.Use[*webhook.Hooks](c).Emit(c, \"pedido.pago\", pedido)` — e acrescente o evento à " +
				"lista em internal/avisos: ela é fechada de propósito, para um erro de digitação falhar onde " +
				"está escrito.",
		},
	}
}

const hooksDecl = `// Package avisos is what this application tells the outside.
//
// The list of events is closed on purpose: a typo in an Emit is an error where
// it is written, rather than an event nobody subscribed to — which, seen from
// outside, is indistinguishable from a partner who is not listening.
package avisos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/webhook"
)

// Eventos is everything this application announces. Add yours here; the screen
// offers this list, and Emit refuses anything outside it.
var Eventos = []string{"exemplo.aconteceu"}

// Setup builds the deliverer, hands it to the application and starts it.
//
// Env comes from the app, and it decides one thing: http:// only exists in
// dev. An address on a private network is refused either way — a webhook is a
// request your server makes to an address somebody else typed, and that is the
// shape of half the SSRF there is.
func Setup(a *trilha.App) error {
	hooks := webhook.New(webhook.Options{
		Events: Eventos,
		Env:    a.Env(),
		Logger: a.Logger(),
	})
	trilha.Provide(a, hooks)
	// Setup starts the workers and the clock, and hangs Shutdown on the app:
	// a deploy in the middle of a delivery waits instead of cutting it.
	return hooks.Setup(a)
}
`

const hooksDeclTest = `package avisos

import "testing"

// A lista é fechada, e é isso que faz um erro de digitação falhar onde ele
// está escrito. Um teste que só olha o tamanho da lista não vale nada; este
// olha que os nomes têm a forma que o resto do app espera.
func TestEventosTemNome(t *testing.T) {
	if len(Eventos) == 0 {
		t.Fatal("um app sem eventos não precisa desta receita")
	}
	visto := map[string]bool{}
	for _, e := range Eventos {
		if e == "" {
			t.Fatal("evento sem nome")
		}
		if visto[e] {
			t.Fatalf("evento repetido: %s", e)
		}
		visto[e] = true
	}
}
`

const hooksPage = `// Package webhooks is the screen an application gives whoever integrates with
// it: register an endpoint, see what was sent, send it again, test it.
//
// One GET and one POST. The POST is webhook.Handle in full, dispatched by a
// hidden action field, because register, revoke, retry and test are one screen
// — and a form that posts to itself comes back to itself when something is
// wrong.
//
// Guard this folder. Whoever reaches it sees every address this application
// sends data to, and can add one of their own.
package webhooks

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
	"github.com/emersonjoe/trilha/webhook"
)

// Page lists the endpoints and the deliveries at GET {{.URL}}webhooks.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.hooks_title}}")
	hooks := trilha.Use[*webhook.Hooks](c)

	subs, err := hooks.Subscriptions(c.Context(), c.Actor().Tenant)
	if err != nil {
		return nil, err
	}
	entregas, err := hooks.Deliveries(c.Context(), webhook.ListParams{
		Tenant: c.Actor().Tenant, Limit: 20})
	if err != nil {
		return nil, err
	}
	return h.Div(
		ui.PageHeader("{{.T.hooks_title}}"),
		ui.Muted(h.Text("{{.T.hooks_desc}}")),
		ui.WebhooksPanel(c, linhas(subs), entregasDe(entregas), ui.WebhooksOpts{
			Action: "{{.URL}}webhooks",
			CSRF:   trilha.CSRFInput(c),
			Events: hooks.Events(),
			// The secret of a subscription that was just created, once. Outside
			// that instant this is "" — it is stored as a hash, and there is no
			// screen that can show it a second time.
			Secret: webhook.TakeSecret(c),
		}),
	), nil
}

// POST is register, revoke, retry and test, all four.
func POST(c *trilha.Ctx) error {
	return trilha.Use[*webhook.Hooks](c).Handle(c)
}

// linhas maps what the module keeps to what the screen shows. It is five
// lines, and this is where "what the screen shows" is decided — the kit draws
// data, not another package's types.
//
// The secret does not cross: the row has no field for it, which is the reason
// the field does not exist.
func linhas(subs []webhook.Subscription) []ui.WebhookRow {
	out := make([]ui.WebhookRow, 0, len(subs))
	for _, s := range subs {
		out = append(out, ui.WebhookRow{ID: s.ID, Label: s.Label, URL: s.URL,
			Events: s.Events, Created: s.Created, Revoked: s.Revoked})
	}
	return out
}

func entregasDe(lista []webhook.Delivery) []ui.DeliveryRow {
	out := make([]ui.DeliveryRow, 0, len(lista))
	for _, d := range lista {
		out = append(out, ui.DeliveryRow{ID: d.ID, Event: d.Event, State: string(d.State),
			Attempt: d.Attempt, Status: d.Status, Err: d.Err, Response: d.Response,
			When: d.Created, Next: d.NextTry, RetryOf: d.RetryOf})
	}
	return out
}
`

// hooksTest is the screen answering in the project that received it. It does
// not register a subscription: an address that passes the check is an address
// this machine can resolve, and a test that needs the network is a test that
// fails on a train.
const hooksTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

func TestTelaDeWebhooks(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())

	// A tela responde vazia e oferece os eventos que este app declara: é a
	// lista fechada aparecendo onde alguém a escolhe.
	c.Get("{{.URL}}webhooks").WantStatus(http.StatusOK).WantContains("exemplo.aconteceu")

	// E um endereço de rede privada é recusado, sempre: um webhook é uma
	// requisição que o seu servidor faz para um endereço que outra pessoa
	// digitou. A recusa volta como a tela com o recado, e não como um erro —
	// quem errou o endereço está com o formulário aberto.
	c.PostForm("{{.URL}}webhooks", map[string][]string{
		"action": {"subscribe"},
		"label":  {"interno"},
		"url":    {"https://127.0.0.1:9/hook"},
		"events": {"exemplo.aconteceu"},
	}).WantStatus(http.StatusSeeOther)
	// A recusa aparece como recado, e a assinatura não existe: o rótulo dela
	// não está na tela. (O endereço está, dentro da mensagem de erro — é ele
	// que a pessoa precisa ler para consertar.)
	depois := c.Get("{{.URL}}webhooks").WantStatus(http.StatusOK).Body.String()
	if strings.Contains(depois, "interno") {
		t.Fatalf("a assinatura para a rede privada foi criada:\n%s", depois)
	}
}
`
