package recipes

// publicLookupRecipe is the third public pattern, after the share link and the
// audience: the person types a code.
//
// A protocol number, the code printed on a receipt — plus a second factor they
// already have — and they see what happened to their case. No account, no
// session, nobody to call. The hand-written version of this screen is the one
// that answers "no such protocol" for a code that does not exist and "wrong
// code" for a factor that does not match, which is a free oracle: whoever is
// walking the number space now knows which numbers are real.
//
// What this writes says no the same way every time, refuses a code that
// cannot exist before touching the database, and limits the attempts by
// address and by code, because the enumeration everybody worries about is
// cheap exactly when nothing counts it.
func publicLookupRecipe() Recipe {
	return Recipe{
		Name: "public-lookup",
		Summary: map[string]string{
			"en": "a code plus a second factor: the public timeline of one case, without an account",
			"pt": "um código mais um segundo fator: a linha do tempo pública de um caso, sem conta",
		},
		Doc: "/cookbook/public-link",
		Files: []File{
			{Rel: "internal/consulta/consulta.go", Go: true, Body: lookupStore},
			{Rel: "internal/consulta/consulta_test.go", Go: true, Body: lookupStoreTest},
			// A folder of its own, and not under {{.At}}: whoever types a
			// protocol number is precisely whoever has no account, so this
			// screen must not land inside a tree somebody put behind a login.
			{Rel: "app/consulta/page.go", Go: true, Body: lookupPage},
			{Rel: "app/consulta/kind.go", Go: true, Body: lookupKind},
			{Rel: "consulta_test.go", Go: true, Body: lookupTest},
		},
		Next: map[string]string{
			"en": "Point `consulta.Buscar` at your own table — the example one is in memory — and mint " +
				"codes with `trilha.CheckDigit`: `codigo := base + trilha.CheckDigit(base)`. Mark as " +
				"`Visivel` only the events you would read out over the phone, because that is what this " +
				"screen is. Leave /consulta out of any login: an `Options.Audience` of its own is how an " +
				"app with two publics keeps this folder out of the private one.",
			"pt": "Aponte o `consulta.Buscar` para a sua tabela — a de exemplo é em memória — e emita os " +
				"códigos com o `trilha.CheckDigit`: `codigo := base + trilha.CheckDigit(base)`. Marque como " +
				"`Visivel` só os eventos que você leria no telefone, porque é isso que esta tela é. Deixe " +
				"/consulta fora de qualquer login: um `Options.Audience` próprio é como um app com dois " +
				"públicos mantém esta pasta fora do público privado.",
		},
	}
}

const lookupStore = `// Package consulta is the public side of a case: what somebody sees when they
// type a protocol number and the code that came with it.
//
// Two rules live here and nowhere else. The first is that a record answers for
// itself whether each of its events may be read by whoever holds the code —
// Evento.Visivel — because the timeline an operator sees and the timeline the
// citizen sees are not the same timeline, and a filter written in the page is
// a filter somebody forgets on the second page. The second is that a code and
// a factor that do not match answer exactly like a code that does not exist:
// one error, one message, one duration.
package consulta

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
)

// Emitir mints a code: the check digits travel with the number, printed on the
// receipt beside it. Two of them catch every single-character typo and every
// swap of two neighbours, and only one code in ninety-seven is worth a query
// at all — which is what makes enumeration expensive before anything counts it.
func Emitir(base string) string { return base + trilha.CheckDigit(base) }

// ErrNaoEncontrado is the only failure this package has. There is no
// "wrong factor" next to it on purpose: two errors become two messages, two
// messages become a way to tell a real code from an invented one, and that is
// the whole game somebody enumerating protocol numbers is playing.
var ErrNaoEncontrado = errors.New("consulta: nada encontrado")

// Registro is one case, as the public sees it.
type Registro struct {
	// Codigo is the protocol number, already with its check digits.
	Codigo string
	// Titulo is what the case is called on the screen.
	Titulo string
	// Eventos is the whole history, visible and not. Publicos is what the
	// screen renders.
	Eventos []Evento
}

// Evento is one thing that happened to the case.
type Evento struct {
	// Em is when it happened.
	Em time.Time
	// Titulo is the step: "Received", "Under review", "Answered".
	Titulo string
	// Texto is the detail, when there is one to give.
	Texto string
	// Visivel says the server marked this event as public. The default is
	// false, which is the safe default: an event nobody thought about does
	// not leak because somebody forgot a flag.
	Visivel bool
}

// Publicos is the events of r that may be shown, oldest first. It is the only
// thing the screen iterates: anything else would be the internal note
// somebody wrote for a colleague.
func Publicos(r Registro) []Evento {
	out := make([]Evento, 0, len(r.Eventos))
	for _, e := range r.Eventos {
		if e.Visivel {
			out = append(out, e)
		}
	}
	return out
}

// Buscar resolves a code plus its second factor into a record. Replace it with
// your own lookup — a query on your table — keeping the contract:
//
//   - a code nobody issued and a factor that does not match give the same
//     ErrNaoEncontrado, and take the same time to give it;
//   - the factor is compared in constant time, because a comparison that
//     stops at the first wrong byte tells whoever is trying how much of it
//     was right.
//
// The signature takes a context because a real one talks to a database and
// the request may go away while it does.
var Buscar = func(ctx context.Context, codigo, fator string) (Registro, error) {
	return buscarExemplo(ctx, codigo, fator)
}

// caso is the example table: a record plus the factor its owner was given.
// The factor is stored here for the example; in a real application it is a
// column, and a good one is something the person already has and a stranger
// does not — the last digits of the phone the case was opened with.
type caso struct {
	reg   Registro
	fator string
}

// exemplo is one case so the screen has something to show on the first run.
// The code carries its check digits: trilha.CheckDigit("20260001") is "04".
var exemplo = map[string]caso{
	"2026000104": {
		fator: "4321",
		reg: Registro{
			Codigo: "2026000104",
			Titulo: "Pedido 2026-0001",
			Eventos: []Evento{
				{Em: time.Now().Add(-72 * time.Hour), Titulo: "Recebido", Texto: "O pedido entrou na fila.", Visivel: true},
				{Em: time.Now().Add(-40 * time.Hour), Titulo: "Em análise", Texto: "Um analista está com ele.", Visivel: true},
				{Em: time.Now().Add(-2 * time.Hour), Titulo: "Nota interna", Texto: "Conferir com o jurídico.", Visivel: false},
			},
		},
	},
}

// buscarExemplo is the in-memory lookup. It is worth reading once even though
// it gets replaced: the shape of the answer is the part that matters.
func buscarExemplo(_ context.Context, codigo, fator string) (Registro, error) {
	c, ok := exemplo[Normalizar(codigo)]
	if !ok {
		// A code nobody issued still pays for a comparison. Without this the
		// answer for an unknown code comes back measurably sooner than the
		// answer for a wrong factor, and the difference is the oracle this
		// whole screen exists to not be.
		c = caso{fator: "0000000000000000"}
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(fator)), []byte(c.fator)) != 1 || !ok {
		return Registro{}, ErrNaoEncontrado
	}
	return c.reg, nil
}

// Normalizar is the code as it is compared and keyed: upper case, without the
// spaces and hyphens it was printed with. Whoever copies "2026-0001-04" off a
// letter and whoever types "2026000104" are asking about the same case, and
// the rate limit has to agree with them or it counts one person twice.
func Normalizar(codigo string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(codigo)) {
		if r == ' ' || r == '-' || r == '.' || r == '/' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Mascarar is the code as the audit trail keeps it: the last three characters
// and nothing else. Enough to match a complaint to a line in the log, not
// enough for the log itself to become the list of valid protocol numbers.
func Mascarar(codigo string) string {
	n := Normalizar(codigo)
	if len(n) <= 3 {
		return strings.Repeat("•", len(n))
	}
	return strings.Repeat("•", len(n)-3) + n[len(n)-3:]
}
`

const lookupStoreTest = `package consulta

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// Só sai o que o servidor marcou como público. O evento interno é o caso que
// importa: ele está no registro, e não pode estar na tela.
func TestPublicosFiltraOInterno(t *testing.T) {
	r := Registro{Eventos: []Evento{
		{Titulo: "a", Visivel: true},
		{Titulo: "b"},
		{Titulo: "c", Visivel: true},
	}}
	got := Publicos(r)
	if len(got) != 2 || got[0].Titulo != "a" || got[1].Titulo != "c" {
		t.Fatalf("Publicos devolveu %+v", got)
	}
}

// Código errado e fator errado dão o mesmo erro. É a regra inteira da receita
// em duas linhas.
func TestBuscarNaoDizQualMetadeEstavaCerta(t *testing.T) {
	ctx := context.Background()
	if _, err := Buscar(ctx, "2026000104", "4321"); err != nil {
		t.Fatalf("o caso de exemplo não abre: %v", err)
	}
	semCodigo := erroDe(t, ctx, "9999999905", "4321")
	semFator := erroDe(t, ctx, "2026000104", "0000")
	if !errors.Is(semCodigo, ErrNaoEncontrado) || !errors.Is(semFator, ErrNaoEncontrado) {
		t.Fatalf("erros diferentes: %v e %v", semCodigo, semFator)
	}
	if semCodigo.Error() != semFator.Error() {
		t.Fatalf("mensagens diferentes: %q e %q", semCodigo, semFator)
	}
}

// O código emitido passa na conferência que a tela faz na entrada. É o par
// inteiro: quem emite e quem confere usam a mesma conta.
func TestEmitirSaiValido(t *testing.T) {
	for _, base := range []string{"20260001", "1", "AB99", "770000123456"} {
		codigo := Emitir(base)
		if !trilha.HasCheckDigit(codigo) {
			t.Fatalf("Emitir(%q) = %q, que não passa no HasCheckDigit", base, codigo)
		}
	}
	if Emitir("20260001") != "2026000104" {
		t.Fatalf("Emitir mudou de resposta: %q", Emitir("20260001"))
	}
}

// O código é o mesmo com ou sem os separadores com que foi impresso.
func TestNormalizarEMascarar(t *testing.T) {
	if got := Normalizar(" 2026-0001.04 "); got != "2026000104" {
		t.Fatalf("Normalizar devolveu %q", got)
	}
	if _, err := Buscar(context.Background(), "2026-0001-04", "4321"); err != nil {
		t.Fatalf("o código impresso com hífen não abre: %v", err)
	}
	if got := Mascarar("2026-0001-04"); got != "•••••••104" {
		t.Fatalf("Mascarar devolveu %q", got)
	}
	if got := Mascarar("12"); got != "••" {
		t.Fatalf("Mascarar de um código curto devolveu %q", got)
	}
}

// O exemplo tem um evento de cada lado, senão a tela nasce vazia e ninguém vê
// o filtro funcionando.
func TestExemploTemVisivelEInvisivel(t *testing.T) {
	r, err := Buscar(context.Background(), "2026000104", "4321")
	if err != nil {
		t.Fatal(err)
	}
	if len(Publicos(r)) == 0 || len(Publicos(r)) == len(r.Eventos) {
		t.Fatalf("o exemplo não mostra o filtro: %d de %d", len(Publicos(r)), len(r.Eventos))
	}
	for _, e := range r.Eventos {
		if e.Em.After(time.Now()) {
			t.Fatalf("evento no futuro: %v", e.Em)
		}
	}
}

func erroDe(t *testing.T, ctx context.Context, codigo, fator string) error {
	t.Helper()
	_, err := Buscar(ctx, codigo, fator)
	if err == nil {
		t.Fatalf("%s/%s abriu", codigo, fator)
	}
	return err
}
`

const lookupPage = `// Package consulta is the screen somebody opens with a piece of paper in
// their hand: a protocol number, the code that came with it, and the history
// of their own case.
//
// It is public, and public here means public: no session, no account, nothing
// to remember. That is also why it is a folder of its own rather than a screen
// under the rest of the app — a middleware at the root of app/ would ask
// whoever types a protocol number for the login they precisely do not have.
// An application with two publics gives its login flow an Options.Audience of
// its own, and this folder stays outside it: the audience separates the people
// who sign in from the people who only ever type a number.
//
// The answer to a code that does not exist and to a factor that does not match
// is one answer, in one message. Everything else on this page — the check
// digits, the two buckets, the constant-time comparison inside consulta.Buscar
// — exists to keep it that way while somebody tries ten thousand numbers.
package consulta

import (
	"net/http"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/consulta"
)

// porIP and porCodigo are the two budgets, and they are two because the
// attacks are two. One address walking the number space is stopped by the
// first; one code being hammered from a botnet — a real protocol number
// somebody is trying factors against — is stopped only by the second.
//
// Five attempts, then one more every twenty seconds. A person who mistyped
// their own code twice never meets this; a script meets it on the sixth try.
var (
	porIP     = trilha.NewLimiter(trilha.RateLimit{RPS: 0.05, Burst: 5})
	porCodigo = trilha.NewLimiter(trilha.RateLimit{RPS: 0.05, Burst: 5})
)

// Page renders the form.
func Page(c *trilha.Ctx) (h.Node, error) {
	return formulario(c, ""), nil
}

// POST answers the form, and answers it the same way for every kind of no.
func POST(c *trilha.Ctx) error {
	codigo, fator := c.Form("codigo"), c.Form("fator")

	// The address first, before anything looks at what was typed: it is the
	// budget that bounds the whole page, including the attempts that never
	// get as far as being a code.
	if ok, espera := porIP.Allow(c.ClientIP()); !ok {
		c.Header("Retry-After", strconv.Itoa(espera))
		return trilha.ErrRateLimited
	}

	// A code that fails its own check digits cannot have been issued, so it
	// is refused here — with the same words as a code that simply is not
	// there, because "malformed" and "not found" are two answers, and two
	// answers are a way of sorting invented numbers from real ones.
	if !trilha.HasCheckDigit(codigo) {
		return naoEncontrado(c, codigo)
	}

	// And now the code's own budget. It comes after the check digits so that
	// garbage never keys a bucket, and before the lookup so that a real code
	// under attack stops costing a query.
	if ok, espera := porCodigo.Allow(consulta.Normalizar(codigo)); !ok {
		c.Header("Retry-After", strconv.Itoa(espera))
		return trilha.ErrRateLimited
	}

	reg, err := consulta.Buscar(c.Request().Context(), codigo, fator)
	if err != nil {
		return naoEncontrado(c, codigo)
	}
	auditar(c, codigo, "ok")
	return c.Render(http.StatusOK, linhaDoTempo(c, reg))
}

// naoEncontrado is the one no. A wrong factor on a real code and a code that
// was never issued come through here, and what leaves is the same bytes and
// the same status.
func naoEncontrado(c *trilha.Ctx, codigo string) error {
	auditar(c, codigo, "nao_encontrado")
	return c.Render(http.StatusUnprocessableEntity, formulario(c, "{{.T.lookup_none}}"))
}

// auditar records the attempt. The code is masked to its last three
// characters: a trail that keeps whole protocol numbers is a list of valid
// protocol numbers, which is the thing this screen refuses to hand out.
func auditar(c *trilha.Ctx, codigo, resultado string) {
	c.Audit("consulta.publica", consulta.Mascarar(codigo), trilha.Fields{
		"ip":        c.ClientIP(),
		"agent":     c.Request().UserAgent(),
		"resultado": resultado,
	})
}

// formulario is the form, and the failed form: the same node, so there is one
// screen to keep accessible instead of two.
//
// Nothing of the request comes back in it — not even the code that was typed.
// Echoing it would be kinder to somebody who mistyped a digit, and it would
// also make the refusal of a code that exists a different page from the
// refusal of one that does not, which is the one difference this screen is
// built to not have.
func formulario(c *trilha.Ctx, erro string) h.Node {
	c.SetTitle("{{.T.lookup_title}}")
	return ui.Stack(
		ui.PageHeader("{{.T.lookup_title}}"),
		ui.Muted(h.Text("{{.T.lookup_desc}}")),
		ui.Card(ui.CardContent(
			h.Form(h.Method("post"), h.Action("/consulta"), h.Class("ui-stack"), trilha.CSRFInput(c),
				h.If(erro != "", ui.Alert(erro, ui.Destructive(), ui.Icon("triangle-alert"))),
				// autocomplete off on both: neither of these is a field a
				// browser should be remembering on a shared phone, and the
				// inputmode is what puts the numeric keypad under a thumb.
				ui.Field("codigo", "{{.T.lookup_code}}",
					ui.Input(h.ID("codigo"), h.Name("codigo"),
						h.Type("text"), h.Inputmode("numeric"), h.Autocomplete("off"),
						h.Aria("describedby", "codigo-help"), h.Maxlength("40"), h.Required()),
					ui.Help("{{.T.lookup_code_help}}")),
				ui.Field("fator", "{{.T.lookup_factor}}",
					ui.Input(h.ID("fator"), h.Name("fator"),
						h.Type("text"), h.Inputmode("numeric"), h.Autocomplete("off"),
						h.Aria("describedby", "fator-help"), h.Maxlength("40"), h.Required()),
					ui.Help("{{.T.lookup_factor_help}}")),
				h.Div(ui.Submit(h.Text("{{.T.lookup_submit}}"))),
			),
		)),
	)
}

// linhaDoTempo is the case: the steps as an ordered list, and under it what
// each one was and when. Only consulta.Publicos is iterated — the internal
// note is not on this page, and not in its HTML either.
func linhaDoTempo(c *trilha.Ctx, reg consulta.Registro) h.Node {
	eventos := consulta.Publicos(reg)
	passos := make([]ui.Step, 0, len(eventos))
	detalhes := make([]h.Node, 0, len(eventos)+1)
	detalhes = append(detalhes, h.Class("ui-stack"))
	for _, e := range eventos {
		passos = append(passos, ui.Step{Label: e.Titulo})
		detalhes = append(detalhes, h.Li(
			h.Strong(h.Text(e.Titulo)), h.Text(" "),
			// The relative date is the one a person reads: "2 days ago"
			// answers "is anything happening", and the exact moment stays in
			// the title for whoever needs it.
			ui.Date(c, e.Em, ui.Relative()),
			h.If(e.Texto != "", h.P(h.Text(e.Texto))),
		))
	}
	c.SetTitle(reg.Titulo)
	var vazio h.Node = h.Fragment()
	if len(eventos) == 0 {
		vazio = ui.Muted(h.Text("{{.T.lookup_empty}}"))
	}
	return ui.Stack(
		ui.PageHeader(reg.Titulo),
		ui.Muted(h.Textf("{{.T.lookup_protocol}}", reg.Codigo)),
		ui.Steps(passos, len(passos)),
		vazio,
		h.Ol(detalhes...),
		h.P(ui.ButtonLink("/consulta", h.Text("{{.T.lookup_again}}"))),
	)
}
`

const lookupKind = `package consulta

import "github.com/emersonjoe/trilha"

// Kind says this branch answers pages: a 429 here is the application's error
// page and not problem+json, because whoever is on the other side typed a
// protocol number into a browser on a phone.
var Kind = trilha.KindPage
`

const lookupTest = `package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/consulta"
)

// clienteConsulta liga o app declarando como confiável só o endereço de onde o
// cliente de teste fala (192.0.2.1). É o que faz o X-Forwarded-For valer sem
// fazer valer demais: se a faixa confiável incluísse os IPs forjados, o
// ClientIP pularia todos eles e cairia no par, e os pedidos todos viriam do
// mesmo endereço — que é exatamente o erro que apaga o teste de enumeração.
func clienteConsulta(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("TRILHA_TRUSTED_PROXIES", "192.0.2.1/32")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return trilha.NewTestClient(t, newApp())
}

func consultar(t *testing.T, c *trilha.TestClient, ip, codigo, fator string) *trilha.TestResponse {
	t.Helper()
	return c.Request("POST", "/consulta",
		trilha.WithHeader("X-Forwarded-For", ip),
		trilha.WithForm(url.Values{"codigo": {codigo}, "fator": {fator}}))
}

// codigo devolve um protocolo válido a partir de um número, emitido pelo mesmo
// Emitir que a aplicação usa — é o par inteiro sendo exercitado, e não uma
// segunda conta escrita no teste.
func codigo(n int) string {
	return consulta.Emitir("77" + strconv.Itoa(100000+n))
}

// A tela abre sem sessão nenhuma — é o ponto dela — e o caso de exemplo
// aparece com a linha do tempo. O que o servidor não marcou como público não
// sai nem no HTML.
func TestConsultaPublicaAbreSemConta(t *testing.T) {
	c := clienteConsulta(t)
	c.Get("/consulta").WantStatus(http.StatusOK).WantContains("name=\"codigo\"", "name=\"fator\"", "autocomplete=\"off\"")

	r := consultar(t, c, "198.51.100.10", "2026000104", "4321").WantStatus(http.StatusOK)
	r.WantContains("Pedido 2026-0001", "Recebido", "Em análise", "<ol class=\"ui-steps\">")
	if b := r.Body.String(); strings.Contains(b, "Nota interna") || strings.Contains(b, "jurídico") {
		t.Fatal("o evento interno saiu na tela pública")
	}
}

// Código que não existe e fator errado num código que existe devolvem o mesmo
// corpo, byte a byte. É a garantia inteira da receita: de fora, as duas coisas
// são indistinguíveis.
func TestRespostaIdenticaParaInexistenteEFatorErrado(t *testing.T) {
	c := clienteConsulta(t)
	inexistente := consultar(t, c, "198.51.100.20", codigo(1), "4321").WantStatus(http.StatusUnprocessableEntity)
	errado := consultar(t, c, "198.51.100.21", "2026000104", "0000").WantStatus(http.StatusUnprocessableEntity)
	if semNonce(inexistente.Body.String()) != semNonce(errado.Body.String()) {
		t.Fatal("as duas negativas têm corpos diferentes: quem tenta números descobre quais existem")
	}
	// E um código que nem chega a ser um código responde igual aos dois.
	malformado := consultar(t, c, "198.51.100.22", "2026000199", "4321").WantStatus(http.StatusUnprocessableEntity)
	if semNonce(malformado.Body.String()) != semNonce(inexistente.Body.String()) {
		t.Fatal("um código com dígito verificador errado tem resposta própria")
	}
}

// semNonce tira os nonces da CSP, que mudam a cada resposta por definição e
// são a única diferença que duas negativas podem ter. O resto tem de bater
// byte a byte — inclusive o tamanho, senão o tempo de transmissão vira o
// oráculo que o texto igual não é.
func semNonce(s string) string {
	const marca = "nonce=\""
	var b strings.Builder
	for {
		i := strings.Index(s, marca)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		j := strings.Index(s[i+len(marca):], "\"")
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		// O que já foi olhado sai de s, senão o nonce vazio que acabou de
		// ficar para trás é encontrado outra vez e o laço não termina.
		b.WriteString(s[:i+len(marca)])
		s = s[i+len(marca)+j:]
	}
}

// Um código que não passa no dígito verificador não chega ao banco: o Buscar
// é contado, e a conta não sobe.
func TestDigitoVerificadorPoupaOBanco(t *testing.T) {
	original := consulta.Buscar
	t.Cleanup(func() { consulta.Buscar = original })
	chamadas := 0
	consulta.Buscar = func(ctx context.Context, cod, fator string) (consulta.Registro, error) {
		chamadas++
		return original(ctx, cod, fator)
	}

	c := clienteConsulta(t)
	consultar(t, c, "198.51.100.30", "2026000199", "4321").WantStatus(http.StatusUnprocessableEntity)
	consultar(t, c, "198.51.100.31", "abacaxi", "4321").WantStatus(http.StatusUnprocessableEntity)
	if chamadas != 0 {
		t.Fatalf("o Buscar foi chamado %d vezes por códigos que não podiam existir", chamadas)
	}
	consultar(t, c, "198.51.100.32", "2026000104", "4321").WantStatus(http.StatusOK)
	if chamadas != 1 {
		t.Fatalf("o código válido chamou o Buscar %d vezes", chamadas)
	}
}

// Enumeração por endereço: o mesmo IP tentando códigos diferentes bate no 429
// e o 429 diz quanto esperar.
func TestEnumeracaoPorIPPara(t *testing.T) {
	c := clienteConsulta(t)
	const ip = "198.51.100.40"
	bateu := false
	for i := 0; i < 12 && !bateu; i++ {
		r := consultar(t, c, ip, codigo(100+i), "4321")
		if r.Code == http.StatusTooManyRequests {
			bateu = true
			if r.Header().Get("Retry-After") == "" {
				t.Fatal("o 429 não diz quanto esperar")
			}
			continue
		}
		if r.Code != http.StatusUnprocessableEntity {
			t.Fatalf("tentativa %d respondeu %d", i, r.Code)
		}
	}
	if !bateu {
		t.Fatal("doze tentativas do mesmo endereço e nenhum 429")
	}
	// E o 429 não é um jeito novo de saber que o código existe.
	if consultar(t, c, ip, "2026000104", "4321").Code != http.StatusTooManyRequests {
		t.Fatal("o código de verdade passou pelo limite que barrou os outros")
	}
}

// Enumeração por código: o mesmo protocolo tentado de endereços diferentes —
// uma botnet procurando o fator — bate no 429 do código.
func TestEnumeracaoPorCodigoPara(t *testing.T) {
	c := clienteConsulta(t)
	alvo := codigo(200)
	bateu := false
	for i := 0; i < 12 && !bateu; i++ {
		r := consultar(t, c, "203.0.113."+strconv.Itoa(i+1), alvo, strconv.Itoa(1000+i))
		if r.Code == http.StatusTooManyRequests {
			bateu = true
			continue
		}
		if r.Code != http.StatusUnprocessableEntity {
			t.Fatalf("tentativa %d respondeu %d", i, r.Code)
		}
	}
	if !bateu {
		t.Fatal("doze endereços no mesmo código e nenhum 429")
	}
	// Um endereço novo num código novo continua passando: o balde é do código,
	// não do mundo.
	if consultar(t, c, "203.0.113.200", codigo(201), "4321").Code != http.StatusUnprocessableEntity {
		t.Fatal("o limite de um código fechou a tela inteira")
	}
}
`
