package recipes

// notifyRecipe is telling people things without annoying them: one call that
// picks the channel the person chose, holds what arrives during their quiet
// hours, groups what they asked to receive once a day, stops a channel that
// is sending too much, and keeps every notification in an outbox an
// administrator can read and resend from.
//
// It composes what exists (spec 162): mail for the e-mail channel and the
// digest, webhook.Hooks for the webhook channel, the channel-whatsapp recipe
// for WhatsApp, trilha.Settings for the limits, trilha.Limiter for the rate,
// task for the digest button. The two channels that depend on other recipes
// are tied by Insert.If, in whichever order the recipes arrive.
func notifyRecipe() Recipe {
	return Recipe{
		Name:        "notify",
		CtxPackCost: 197,
		Summary: map[string]string{
			"en": "notifications by e-mail, webhook or WhatsApp: per-person preferences, quiet hours, daily digest, per-channel limit, and the outbox with resend",
			"pt": "notificações por e-mail, webhook ou WhatsApp: preferências por pessoa, horário silencioso, digesto diário, limite por canal, e a fila com reenvio",
		},
		Doc:   "/cookbook/notifications",
		Needs: []Need{{Recipe: "login", File: "internal/sessao/sessao.go"}},
		Files: []File{
			{Rel: "internal/notificar/notificar.go", Go: true, Body: notifyEngine},
			{Rel: "internal/notificar/canais.go", Go: true, Body: notifyChannels},
			{Rel: "internal/notificar/notificar_test.go", Go: true, Body: notifyEngineTest},
			{Rel: "{{.At}}notificacoes/middleware.go", Go: true, Body: notifyMiddleware},
			{Rel: "{{.At}}notificacoes/page.go", Go: true, Body: notifyPrefsPage},
			{Rel: "{{.At}}notificacoes/fila/middleware.go", Go: true, Body: notifyOutboxMiddleware},
			{Rel: "{{.At}}notificacoes/fila/page.go", Go: true, Body: notifyOutboxPage},
			{Rel: "notificacoes_test.go", Go: true, Body: notifyTest},
		},
		Setup: []Insert{
			notifyWebhookLink("internal/avisos/avisos.go"),
			notifyWhatsAppLink("internal/whatsapp/whatsapp.go"),
			{
				Marker: "// trilha:add notify",
				Line:   "\tif err := notificar.Setup(a); err != nil {\n\t\treturn err\n\t}\n",
			},
		},
		Imports: []string{"{{.Module}}/internal/notificar"},
		Next: map[string]string{
			"en": "Notify from where the thing happens: " +
				"`trilha.Use[*notificar.Notificador](c).Notificar(c, notificar.Pessoa{Sujeito: u.Subject, Email: u.Email}, title, body)`. " +
				"Each person picks a channel at {{.URL}}notificacoes; the outbox and the limits are at {{.URL}}notificacoes/fila. " +
				"E-mail is wired; `trilha add webhooks` and `trilha add channel-whatsapp` wire the other two channels, in any order.",
			"pt": "Notifique de onde a coisa acontece: " +
				"`trilha.Use[*notificar.Notificador](c).Notificar(c, notificar.Pessoa{Sujeito: u.Subject, Email: u.Email}, titulo, corpo)`. " +
				"Cada pessoa escolhe o canal em {{.URL}}notificacoes; a fila e os limites ficam em {{.URL}}notificacoes/fila. " +
				"O e-mail vem ligado; `trilha add webhooks` e `trilha add channel-whatsapp` ligam os outros dois canais, em qualquer ordem.",
		},
	}
}

// notifyWebhookLink is the line that wires the webhook channel: the event goes
// into the closed list of internal/avisos before avisos.Setup reads it, and
// the channel starts emitting it. The notify and webhooks recipes both carry
// it, each conditioned on the other's file, and it lands above both Setup
// calls whichever recipe arrives second — new lines go at the top of Setup.
func notifyWebhookLink(ifFile string) Insert {
	return Insert{
		Marker:  "// trilha:link notify-webhooks",
		Line:    "\tavisos.Eventos = append(avisos.Eventos, notificar.Evento)\n\tnotificar.Canais[notificar.CanalWebhook] = notificar.PorWebhook\n",
		If:      ifFile,
		Imports: []string{"{{.Module}}/internal/avisos", "{{.Module}}/internal/notificar"},
	}
}

// notifyWhatsAppLink is the line that wires the WhatsApp channel through the
// client of the channel-whatsapp recipe. It lives in setup.go and not in
// internal/notificar because it names a package that exists only when that
// recipe is there.
func notifyWhatsAppLink(ifFile string) Insert {
	return Insert{
		Marker: "// trilha:link notify-whatsapp",
		Line: "\tnotificar.Canais[notificar.CanalWhatsApp] = notificar.PorTexto(func(c *trilha.Ctx, para, texto string) error {\n" +
			"\t\tcli, err := whatsapp.NewClient(c)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n" +
			"\t\t_, err = cli.SendText(c.Context(), para, texto)\n\t\treturn err\n\t})\n",
		If:      ifFile,
		Imports: []string{"{{.Module}}/internal/notificar", "{{.Module}}/internal/whatsapp"},
	}
}

const notifyEngine = `// Package notificar is how this application tells people things.
//
// One call — Notificar — and the rest is the person's choice and the
// application's limits: the channel they picked, the hours they do not want
// to hear from anybody, the digest they asked for instead of one message per
// event, and a ceiling per channel so a loop somewhere does not become a
// thousand messages. Everything that was decided lands in an outbox an
// administrator can read and resend from.
//
// The rows live in memory, like every recipe's first version: the outbox and
// the preferences are two tables behind the same methods the day they need to
// survive a restart.
package notificar

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/mail"
	"github.com/emersonjoe/trilha/task"
)

// The channels. E-mail is always wired; the other two are wired by setup.go
// when the webhooks and channel-whatsapp recipes are there.
const (
	CanalEmail    = "mail"
	CanalWebhook  = "webhook"
	CanalWhatsApp = "whatsapp"
)

// CodigoLimite is the stable code of a notification the channel's limit
// held back. It has a page at /docs/errors/E_NOTIFY_RATE.
const CodigoLimite = "E_NOTIFY_RATE"

// ErrLimite is Notificar answering that the channel reached its limit for
// this person in this hour. The notification is in the outbox as limited.
var ErrLimite = errors.New("notificar: " + CodigoLimite + ": the channel reached its limit for this recipient")

// ErrNaoExiste is an id the outbox does not have.
var ErrNaoExiste = errors.New("notificar: no such notification")

// ErrSemTelefone is a text channel for somebody who gave no phone number.
var ErrSemTelefone = errors.New("notificar: the person has no phone number for a text channel")

// Estado is what happened to a notification.
type Estado string

// The states of the outbox.
const (
	Enviada   Estado = "enviada"  // it left
	Retida    Estado = "retida"   // it arrived during the person's quiet hours
	NoDigesto Estado = "digesto"  // it waits for the daily digest
	Limitada  Estado = "limitada" // the channel's limit held it back
	Falhou    Estado = "falhou"   // the channel answered an error
)

// Pessoa is who a notification is for: the session's subject is the key of
// the preferences, and the e-mail is where the e-mail channel and the digest
// go.
type Pessoa struct {
	Sujeito string
	Email   string
}

// Notificacao is one row of the outbox.
type Notificacao struct {
	ID       string
	Para     Pessoa
	Telefone string
	Canal    string
	Titulo   string
	Corpo    string
	Estado   Estado
	// Erro is why it did not leave: the code of the limit, or the channel's
	// answer. Never the body — the outbox is read by administrators, and the
	// log by whoever reads logs.
	Erro       string
	Tentativas int
	Criada     time.Time
	Enviada    time.Time
}

// Preferencias is what a person chose. The tags are the form's rules: the
// same struct is the screen, the validation and the row.
type Preferencias struct {
	Canal       string ` + "`" + `form:"canal" validate:"required,oneof=mail webhook whatsapp" label:"{{.T.notify_channel}}"` + "`" + `
	Digesto     bool   ` + "`" + `form:"digesto" label:"{{.T.notify_digest}}"` + "`" + `
	SilencioDe  int    ` + "`" + `form:"silencio_de" validate:"min=0,max=23" label:"{{.T.notify_quiet_from}}"` + "`" + `
	SilencioAte int    ` + "`" + `form:"silencio_ate" validate:"min=0,max=23" label:"{{.T.notify_quiet_to}}"` + "`" + `
	Telefone    string ` + "`" + `form:"telefone" validate:"max=20" label:"{{.T.notify_phone}}"` + "`" + `
}

// Padrao is what somebody who never opened the screen gets: e-mail, one
// message per notification, no quiet hours.
var Padrao = Preferencias{Canal: CanalEmail}

// Silencio says whether hour falls in the quiet hours. From 22 to 7 wraps
// midnight; from and to equal is no quiet hours at all.
func (p Preferencias) Silencio(hora int) bool {
	switch {
	case p.SilencioDe == p.SilencioAte:
		return false
	case p.SilencioDe < p.SilencioAte:
		return hora >= p.SilencioDe && hora < p.SilencioAte
	default:
		return hora >= p.SilencioDe || hora < p.SilencioAte
	}
}

// Ajustes is what an administrator changes without a deploy.
type Ajustes struct {
	LimitePorHora int ` + "`" + `json:"limite_por_hora" form:"limite_por_hora" validate:"min=1,max=1000" label:"{{.T.notify_limit}}"` + "`" + `
	HoraDigesto   int ` + "`" + `json:"hora_digesto" form:"hora_digesto" validate:"min=0,max=23" label:"{{.T.notify_digest_hour}}"` + "`" + `
}

// Config is the settings section of this module: how many messages one
// channel may send one person per hour, and the hour the digest goes out.
var Config = trilha.NewSettings("notificar", Ajustes{LimitePorHora: 10, HoraDigesto: 8})

// Canal sends one notification. It receives the request, because the
// webhook and WhatsApp channels need it to reach what the application
// provided.
type Canal func(c *trilha.Ctx, n Notificacao) error

// Canais is every wired channel. setup.go adds to it; the preferences screen
// only offers what is here.
var Canais = map[string]Canal{CanalEmail: PorEmail}

// Disponiveis lists the wired channels, in a stable order.
func Disponiveis() []string {
	var out []string
	for _, c := range []string{CanalEmail, CanalWebhook, CanalWhatsApp} {
		if _, ok := Canais[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

// Mailer sends the e-mail channel and the digest. A package variable on
// purpose: a test puts a mail.Outbox in its place.
var Mailer = mail.New(mail.FromEnv())

// Marca is the name in the messages.
const Marca = "{{.T.mail_brand}}"

// TarefaDigesto is the task the outbox screen starts to send today's digest.
const TarefaDigesto = "notificar.digesto"

// Notificador is the module: preferences, the outbox, the limits.
type Notificador struct {
	mu      sync.Mutex
	seq     int
	prefs   map[string]Preferencias
	fila    []*Notificacao
	ultimo  string // the day of the last digest, YYYY-MM-DD
	limite  int
	limites *trilha.Limiter
	tarefas *task.Tasks
	// Agora is the clock. A test sets it to put the quiet hours where it
	// wants them.
	Agora func() time.Time
}

// Novo is an empty notifier.
func Novo() *Notificador {
	return &Notificador{prefs: map[string]Preferencias{}, Agora: time.Now}
}

// Setup binds the settings, builds the notifier, hands it to the application,
// starts the digest engine and the clock that sends the digest once a day.
func Setup(a *trilha.App) error {
	if err := Config.Bind(a, nil); err != nil {
		return err
	}
	n := Novo()
	n.tarefas = task.New(task.Options{Logger: a.Logger()})
	n.tarefas.Handle(TarefaDigesto, func(ctx context.Context, p *task.Progress) error {
		_, err := n.Digesto(ctx)
		return err
	})
	trilha.Provide(a, n)
	pare := make(chan struct{})
	go n.relogio(pare)
	a.OnShutdown(func(*trilha.App) error { close(pare); return nil })
	return n.tarefas.Setup(a)
}

// relogio looks at the hour once an hour and sends the digest at the hour of
// the settings. Digesto itself refuses a second run on the same day, so a
// restart at the digest hour does not send it twice.
func (n *Notificador) relogio(pare <-chan struct{}) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-pare:
			return
		case <-t.C:
			if n.Agora().Hour() == Config.Get().HoraDigesto {
				n.Digesto(context.Background())
			}
		}
	}
}

// Preferencias is what the person chose, or Padrao.
func (n *Notificador) Preferencias(sujeito string) Preferencias {
	n.mu.Lock()
	defer n.mu.Unlock()
	if p, ok := n.prefs[sujeito]; ok {
		return p
	}
	return Padrao
}

// SalvarPreferencias writes what the person chose.
func (n *Notificador) SalvarPreferencias(sujeito string, p Preferencias) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.prefs[sujeito] = p
}

// Notificar is the one call. It decides by the person's preferences and the
// channel's limit, and always leaves the notification in the outbox —
// ErrLimite when the limit held it back, the channel's error when it failed.
func (n *Notificador) Notificar(c *trilha.Ctx, para Pessoa, titulo, corpo string) (Notificacao, error) {
	p := n.Preferencias(para.Sujeito)
	canal := p.Canal
	if _, ok := Canais[canal]; !ok {
		// A preference for a channel that was unwired since falls back to
		// e-mail rather than to silence.
		canal = CanalEmail
	}
	n.mu.Lock()
	n.seq++
	no := &Notificacao{ID: "n-" + strconv.Itoa(n.seq), Para: para, Telefone: p.Telefone, Canal: canal,
		Titulo: titulo, Corpo: corpo, Criada: n.Agora()}
	n.fila = append(n.fila, no)
	n.mu.Unlock()

	switch {
	case p.Digesto:
		n.marcar(no, NoDigesto, "")
		return *no, nil
	case p.Silencio(n.Agora().Hour()):
		n.marcar(no, Retida, "")
		return *no, nil
	}
	return n.entregar(c, no)
}

// Reenviar sends again what the limit held back, what failed or what the
// quiet hours retained — the button of the outbox. The quiet hours still
// hold: resending at 3 a.m. is still 3 a.m. for the person.
func (n *Notificador) Reenviar(c *trilha.Ctx, id string) (Notificacao, error) {
	n.mu.Lock()
	var no *Notificacao
	for _, x := range n.fila {
		if x.ID == id {
			no = x
		}
	}
	n.mu.Unlock()
	if no == nil {
		return Notificacao{}, ErrNaoExiste
	}
	if n.copia(no).Estado == Enviada {
		return n.copia(no), nil
	}
	if n.Preferencias(no.Para.Sujeito).Silencio(n.Agora().Hour()) {
		n.marcar(no, Retida, "")
		return n.copia(no), nil
	}
	return n.entregar(c, no)
}

// entregar is the limit and the channel.
func (n *Notificador) entregar(c *trilha.Ctx, no *Notificacao) (Notificacao, error) {
	if !n.permite(no.Canal + "\x00" + no.Para.Sujeito) {
		n.marcar(no, Limitada, CodigoLimite)
		return n.copia(no), ErrLimite
	}
	n.mu.Lock()
	no.Tentativas++
	copia := *no
	n.mu.Unlock()
	if err := Canais[copia.Canal](c, copia); err != nil {
		n.marcar(no, Falhou, err.Error())
		return n.copia(no), err
	}
	n.marcar(no, Enviada, "")
	return n.copia(no), nil
}

// permite is the per-channel, per-person limit. The limiter is rebuilt when
// an administrator changes the number, so the new limit counts from then.
func (n *Notificador) permite(chave string) bool {
	limite := Config.Get().LimitePorHora
	n.mu.Lock()
	if n.limites == nil || n.limite != limite {
		n.limite = limite
		n.limites = trilha.NewLimiter(trilha.RateLimit{RPS: float64(limite) / 3600, Burst: limite})
	}
	l := n.limites
	n.mu.Unlock()
	ok, _ := l.Allow(chave)
	return ok
}

func (n *Notificador) marcar(no *Notificacao, e Estado, erro string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	no.Estado, no.Erro = e, erro
	if e == Enviada {
		no.Enviada = n.Agora()
	}
}

func (n *Notificador) copia(no *Notificacao) Notificacao {
	n.mu.Lock()
	defer n.mu.Unlock()
	return *no
}

// Fila is the outbox, the newest first; para filters by subject when not
// empty.
func (n *Notificador) Fila(para string) []Notificacao {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]Notificacao, 0, len(n.fila))
	for i := len(n.fila) - 1; i >= 0; i-- {
		if para == "" || n.fila[i].Para.Sujeito == para {
			out = append(out, *n.fila[i])
		}
	}
	return out
}

// Digesto sends, once a day, one e-mail per person with what waited for it:
// what they asked to receive as a digest, and what their quiet hours held.
// Somebody who is inside their quiet hours right now is left for the next
// digest — the digest is a message too. It answers how many e-mails left.
func (n *Notificador) Digesto(ctx context.Context) (int, error) {
	agora := n.Agora()
	dia := agora.Format("2006-01-02")
	n.mu.Lock()
	if n.ultimo == dia {
		n.mu.Unlock()
		return 0, nil
	}
	n.ultimo = dia
	porPessoa := map[string][]*Notificacao{}
	var ordem []string
	for _, no := range n.fila {
		if no.Estado != NoDigesto && no.Estado != Retida {
			continue
		}
		if _, ok := porPessoa[no.Para.Sujeito]; !ok {
			ordem = append(ordem, no.Para.Sujeito)
		}
		porPessoa[no.Para.Sujeito] = append(porPessoa[no.Para.Sujeito], no)
	}
	n.mu.Unlock()
	sort.Strings(ordem)

	enviados := 0
	var erros []string
	for _, sujeito := range ordem {
		if n.Preferencias(sujeito).Silencio(agora.Hour()) {
			continue
		}
		lista := porPessoa[sujeito]
		itens := make([]h.Node, 0, len(lista))
		for _, no := range lista {
			itens = append(itens, h.Li(h.Strong(h.Text(no.Titulo)), h.Text(" — "+no.Corpo)))
		}
		err := Mailer.Send(ctx, mail.Message{
			To:      []string{lista[0].Para.Email},
			Subject: "{{.T.notify_digest_subject}} — " + Marca,
			Body:    mail.Layout(Marca, h.P(h.Text("{{.T.notify_digest_intro}}")), h.Ul(itens...)),
		})
		for _, no := range lista {
			if err != nil {
				n.marcar(no, Falhou, err.Error())
			} else {
				n.marcar(no, Enviada, "")
			}
		}
		if err != nil {
			erros = append(erros, err.Error())
			continue
		}
		enviados++
	}
	if len(erros) > 0 {
		return enviados, errors.New("notificar: digest: " + strings.Join(erros, "; "))
	}
	return enviados, nil
}

// DispararDigesto is the outbox's button: the digest as a task, with the day
// as the key, so two clicks are one digest.
func (n *Notificador) DispararDigesto(c *trilha.Ctx) error {
	_, err := n.tarefas.Run(c, TarefaDigesto, n.Agora().Format("2006-01-02"))
	return err
}
`

const notifyChannels = `package notificar

import (
	"context"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/mail"
	"github.com/emersonjoe/trilha/webhook"
)

// Evento is what the webhook channel emits. setup.go adds it to the closed
// list of internal/avisos when the webhooks recipe is there.
const Evento = "notificacao.enviada"

// PorEmail is the e-mail channel.
func PorEmail(c *trilha.Ctx, n Notificacao) error {
	ctx := context.Background()
	if c != nil {
		ctx = c.Context()
	}
	return Mailer.Send(ctx, mail.Message{
		To:      []string{n.Para.Email},
		Subject: n.Titulo + " — " + Marca,
		Body:    mail.Layout(Marca, h.P(h.Text(n.Corpo))),
	})
}

// carga is what the webhook channel sends: the notification, without the
// e-mail address — whoever integrates gets the subject, which is this
// application's id for the person, and not a way to write to them.
type carga struct {
	ID     string ` + "`" + `json:"id"` + "`" + `
	Para   string ` + "`" + `json:"subject"` + "`" + `
	Titulo string ` + "`" + `json:"title"` + "`" + `
	Corpo  string ` + "`" + `json:"body"` + "`" + `
}

// PorWebhook is the webhook channel: the deliverer the application provided
// signs it, retries it and refuses private addresses.
func PorWebhook(c *trilha.Ctx, n Notificacao) error {
	return trilha.Use[*webhook.Hooks](c).Emit(c, Evento, carga{ID: n.ID, Para: n.Para.Sujeito, Titulo: n.Titulo, Corpo: n.Corpo})
}

// EnviaTexto sends text to a phone number. It is what a text channel needs
// from its provider — the WhatsApp client answers it in setup.go.
type EnviaTexto func(c *trilha.Ctx, para, texto string) error

// PorTexto turns a text sender into a channel: the phone comes from the
// person's preferences, and a person without one is an error and not a
// message sent to nobody.
func PorTexto(envia EnviaTexto) Canal {
	return func(c *trilha.Ctx, n Notificacao) error {
		if n.Telefone == "" {
			return ErrSemTelefone
		}
		return envia(c, n.Telefone, n.Titulo+"\n\n"+n.Corpo)
	}
}
`

const notifyEngineTest = `package notificar

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/mail"
)

// canalDeTeste troca o e-mail por uma lista, e devolve a lista.
func canalDeTeste(t *testing.T) *[]Notificacao {
	t.Helper()
	var enviadas []Notificacao
	antes := Canais[CanalEmail]
	Canais[CanalEmail] = func(_ *trilha.Ctx, n Notificacao) error {
		enviadas = append(enviadas, n)
		return nil
	}
	t.Cleanup(func() { Canais[CanalEmail] = antes })
	return &enviadas
}

// caixaDeTeste troca o Mailer por uma mail.Outbox.
func caixaDeTeste(t *testing.T) *mail.Outbox {
	t.Helper()
	caixa := &mail.Outbox{}
	antes := Mailer
	Mailer = mail.New(mail.Options{From: "avisos@example.com", Transport: caixa})
	t.Cleanup(func() { Mailer = antes })
	return caixa
}

func as(h int) func() time.Time {
	return func() time.Time { return time.Date(2026, 9, 24, h, 30, 0, 0, time.UTC) }
}

var ana = Pessoa{Sujeito: "u-ana", Email: "ana@example.com"}

// Das 22 às 7 nada sai: a notificação fica retida, e sai no digesto quando
// a janela acaba — o digesto também é uma mensagem.
func TestNotifyQuietHours(t *testing.T) {
	enviadas := canalDeTeste(t)
	caixa := caixaDeTeste(t)
	n := Novo()
	n.SalvarPreferencias(ana.Sujeito, Preferencias{Canal: CanalEmail, SilencioDe: 22, SilencioAte: 7})

	for _, c := range []struct {
		hora     int
		silencio bool
	}{ {21, false}, {22, true}, {23, true}, {0, true}, {6, true}, {7, false}, {12, false} } {
		if got := n.Preferencias(ana.Sujeito).Silencio(c.hora); got != c.silencio {
			t.Errorf("%dh: silêncio = %v", c.hora, got)
		}
	}
	if (Preferencias{SilencioDe: 5, SilencioAte: 5}).Silencio(5) {
		t.Error("de e até iguais é sem silêncio")
	}

	n.Agora = as(23)
	no, err := n.Notificar(nil, ana, "Pedido pago", "O pedido 42 foi pago.")
	if err != nil || no.Estado != Retida || len(*enviadas) != 0 {
		t.Fatalf("às 23h: %+v %v, enviadas %d", no, err, len(*enviadas))
	}
	// O reenvio manual às 23h continua sendo 23h para a pessoa.
	if no, _ = n.Reenviar(nil, no.ID); no.Estado != Retida {
		t.Fatalf("o reenvio furou o silêncio: %+v", no)
	}
	// O digesto das 23h30 também não sai.
	if k, err := n.Digesto(context.Background()); err != nil || k != 0 || len(caixa.Messages()) != 0 {
		t.Fatalf("digesto no silêncio: %d %v", k, err)
	}
	// No dia seguinte, às 8h, sai.
	n.Agora = func() time.Time { return time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC) }
	if k, err := n.Digesto(context.Background()); err != nil || k != 1 {
		t.Fatalf("digesto das 8h: %d %v", k, err)
	}
	if !strings.Contains(caixa.Last().HTML, "Pedido pago") {
		t.Fatalf("o digesto não levou a retida:\n%s", caixa.Last().HTML)
	}
	// E ao meio-dia a notificação sai na hora.
	n.Agora = as(12)
	if no, err = n.Notificar(nil, ana, "Outro", "corpo"); err != nil || no.Estado != Enviada || len(*enviadas) != 1 {
		t.Fatalf("ao meio-dia: %+v %v", no, err)
	}
}

// Quem pediu digesto recebe um e-mail por dia com tudo, e o segundo digesto
// do mesmo dia não manda nada.
func TestNotifyDigest(t *testing.T) {
	enviadas := canalDeTeste(t)
	caixa := caixaDeTeste(t)
	n := Novo()
	n.Agora = as(9)
	n.SalvarPreferencias(ana.Sujeito, Preferencias{Canal: CanalEmail, Digesto: true})
	bia := Pessoa{Sujeito: "u-bia", Email: "bia@example.com"}
	n.SalvarPreferencias(bia.Sujeito, Preferencias{Canal: CanalEmail, Digesto: true})

	for _, titulo := range []string{"Um", "Dois", "Três"} {
		if no, err := n.Notificar(nil, ana, titulo, "corpo de "+titulo); err != nil || no.Estado != NoDigesto {
			t.Fatalf("%s: %+v %v", titulo, no, err)
		}
	}
	n.Notificar(nil, bia, "Da Bia", "corpo")
	if len(*enviadas) != 0 {
		t.Fatalf("saiu antes do digesto: %d", len(*enviadas))
	}
	k, err := n.Digesto(context.Background())
	if err != nil || k != 2 {
		t.Fatalf("digesto: %d %v", k, err)
	}
	var daAna mail.Sent
	for _, m := range caixa.Messages() {
		if m.To[0] == ana.Email {
			daAna = m
		}
	}
	for _, titulo := range []string{"Um", "Dois", "Três"} {
		if !strings.Contains(daAna.HTML, titulo) {
			t.Errorf("o digesto da Ana não tem %q", titulo)
		}
	}
	if strings.Contains(daAna.HTML, "Da Bia") {
		t.Fatal("o digesto da Ana levou a notificação da Bia")
	}
	if k, _ := n.Digesto(context.Background()); k != 0 || len(caixa.Messages()) != 2 {
		t.Fatalf("o segundo digesto do dia mandou %d", k)
	}
	for _, no := range n.Fila(ana.Sujeito) {
		if no.Estado != Enviada {
			t.Fatalf("depois do digesto: %+v", no)
		}
	}
}

// O limite é por canal e por pessoa: a terceira da Ana na mesma hora fica
// limitada com o código, e a da Bia sai.
func TestNotifyRateLimit(t *testing.T) {
	enviadas := canalDeTeste(t)
	antes := Config.Get()
	if err := Config.Set(Ajustes{LimitePorHora: 2, HoraDigesto: 8}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Config.Set(antes) })
	n := Novo()
	n.Agora = as(12)

	for i := 0; i < 2; i++ {
		if _, err := n.Notificar(nil, ana, "ok", "corpo"); err != nil {
			t.Fatal(err)
		}
	}
	no, err := n.Notificar(nil, ana, "demais", "corpo")
	if !errors.Is(err, ErrLimite) || no.Estado != Limitada || no.Erro != CodigoLimite {
		t.Fatalf("a terceira: %+v %v", no, err)
	}
	if !strings.Contains(err.Error(), "E_NOTIFY_RATE") {
		t.Fatalf("o erro não carrega o código: %v", err)
	}
	if _, err := n.Notificar(nil, Pessoa{Sujeito: "u-bia", Email: "bia@example.com"}, "ok", "corpo"); err != nil {
		t.Fatalf("o limite da Ana segurou a Bia: %v", err)
	}
	if len(*enviadas) != 3 {
		t.Fatalf("enviadas = %d", len(*enviadas))
	}
	// Subir o limite vale dali em diante, e o reenvio manda a que ficou.
	if err := Config.Set(Ajustes{LimitePorHora: 10, HoraDigesto: 8}); err != nil {
		t.Fatal(err)
	}
	if no, err = n.Reenviar(nil, no.ID); err != nil || no.Estado != Enviada || no.Tentativas != 1 {
		t.Fatalf("reenvio: %+v %v", no, err)
	}
}

// Um canal de texto sem telefone é um erro, e não uma mensagem para ninguém.
func TestPorTextoPedeTelefone(t *testing.T) {
	var para, texto string
	canal := PorTexto(func(_ *trilha.Ctx, p, tx string) error { para, texto = p, tx; return nil })
	if err := canal(nil, Notificacao{Titulo: "t", Corpo: "c"}); !errors.Is(err, ErrSemTelefone) {
		t.Fatalf("sem telefone: %v", err)
	}
	if err := canal(nil, Notificacao{Telefone: "5511999999999", Titulo: "Título", Corpo: "Corpo"}); err != nil {
		t.Fatal(err)
	}
	if para != "5511999999999" || texto != "Título\n\nCorpo" {
		t.Fatalf("para %q texto %q", para, texto)
	}
}
`

const notifyMiddleware = `package notificacoes

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// exige asks for a session: preferences are personal.
var exige = sessao.Flow.Require()

// Middleware guards this folder and the outbox below it, which asks for more.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const notifyPrefsPage = `// Package notificacoes is where a person says how they want to be told
// things: the channel, the digest, the quiet hours.
package notificacoes

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/notificar"
	"{{.Module}}/internal/sessao"
)

// Page renders GET {{.URL}}notificacoes.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.notify_title}}")
	u := sessao.Atual(c)
	return tela(c, trilha.Use[*notificar.Notificador](c).Preferencias(u.Subject), nil), nil
}

// POST saves the preferences. The tags validate the shape; the channel has
// to be one this application wired, which only the running application knows.
func POST(c *trilha.Ctx) error {
	u := sessao.Atual(c)
	var p notificar.Preferencias
	err := c.Bind(&p)
	var fe trilha.FieldErrors
	if err != nil && !errors.As(err, &fe) {
		return err
	}
	if !slices.Contains(notificar.Disponiveis(), p.Canal) {
		if fe == nil {
			fe = trilha.FieldErrors{}
		}
		fe.Add("canal", "{{.T.notify_unwired}}")
	}
	if len(fe) > 0 {
		return c.Render(http.StatusUnprocessableEntity, tela(c, p, fe))
	}
	trilha.Use[*notificar.Notificador](c).SalvarPreferencias(u.Subject, p)
	c.Flash(ui.FlashSuccess, "{{.T.notify_saved}}")
	return c.Redirect("{{.URL}}notificacoes")
}

func tela(c *trilha.Ctx, p notificar.Preferencias, errs trilha.FieldErrors) h.Node {
	var canais []ui.Option
	for _, canal := range notificar.Disponiveis() {
		canais = append(canais, ui.Option{Value: canal, Label: RotuloCanal(canal)})
	}
	var digesto h.Node = h.Group()
	if p.Digesto {
		digesto = h.Checked()
	}
	recentes := trilha.Use[*notificar.Notificador](c).Fila(sessao.Atual(c).Subject)
	var lista h.Node = ui.Muted(h.Text("{{.T.notify_empty}}"))
	if len(recentes) > 0 {
		linhas := make([]h.Node, 0, len(recentes))
		for _, n := range recentes {
			linhas = append(linhas, h.Tr(h.Td(h.Text(n.Titulo)), h.Td(h.Text(RotuloCanal(n.Canal))),
				h.Td(ui.Badge(ui.Outline(), h.Text(RotuloEstado(n.Estado)))), h.Td(ui.Date(c, n.Criada))))
		}
		lista = ui.Table(h.Thead(h.Tr(h.Th(h.Text("{{.T.notify_subject}}")), h.Th(h.Text("{{.T.notify_channel}}")),
			h.Th(h.Text("{{.T.notify_state}}")), h.Th())), h.Tbody(linhas...))
	}
	return ui.Stack(
		ui.PageHeader("{{.T.notify_title}}"),
		ui.Muted(h.Text("{{.T.notify_desc}}")),
		h.Form(h.Method("post"), h.Action("{{.URL}}notificacoes"), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			ui.Field("canal", "{{.T.notify_channel}}",
				ui.Select(h.ID("canal"), h.Name("canal"), ui.SelectOptions(canais, p.Canal), ui.InvalidIf(errs, "canal")),
				ui.Errors(errs, "canal")),
			ui.Field("digesto", "{{.T.notify_digest}}",
				ui.Checkbox(h.ID("digesto"), h.Name("digesto"), h.Value("true"), digesto)),
			ui.Field("silencio_de", "{{.T.notify_quiet_from}}",
				ui.Input(h.ID("silencio_de"), h.Name("silencio_de"), h.Type("number"), h.Attr("min", "0"), h.Attr("max", "23"),
					h.Value(strconv.Itoa(p.SilencioDe)), ui.InvalidIf(errs, "silencio_de")), ui.Errors(errs, "silencio_de")),
			ui.Field("silencio_ate", "{{.T.notify_quiet_to}}",
				ui.Input(h.ID("silencio_ate"), h.Name("silencio_ate"), h.Type("number"), h.Attr("min", "0"), h.Attr("max", "23"),
					h.Value(strconv.Itoa(p.SilencioAte)), ui.InvalidIf(errs, "silencio_ate")), ui.Errors(errs, "silencio_ate")),
			ui.Field("telefone", "{{.T.notify_phone}}",
				ui.Input(h.ID("telefone"), h.Name("telefone"), h.Type("tel"), h.Value(p.Telefone),
					ui.InvalidIf(errs, "telefone")), ui.Errors(errs, "telefone")),
			h.Div(ui.Submit(h.Text("{{.T.notify_save}}"))),
		),
		ui.H2(h.Text("{{.T.notify_recent}}")),
		lista,
	)
}

// RotuloCanal is what a channel is called on the screen.
func RotuloCanal(canal string) string {
	switch canal {
	case notificar.CanalEmail:
		return "{{.T.notify_channel_mail}}"
	case notificar.CanalWebhook:
		return "{{.T.notify_channel_webhook}}"
	case notificar.CanalWhatsApp:
		return "{{.T.notify_channel_whatsapp}}"
	}
	return canal
}

// RotuloEstado is what a state is called on the screen.
func RotuloEstado(e notificar.Estado) string {
	switch e {
	case notificar.Enviada:
		return "{{.T.notify_state_sent}}"
	case notificar.Retida:
		return "{{.T.notify_state_held}}"
	case notificar.NoDigesto:
		return "{{.T.notify_state_digest}}"
	case notificar.Limitada:
		return "{{.T.notify_state_limited}}"
	case notificar.Falhou:
		return "{{.T.notify_state_failed}}"
	}
	return string(e)
}
`

const notifyOutboxMiddleware = `package fila

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// exige is the outbox's rule: it shows what the application told everybody,
// and its buttons send messages — administrators only.
var exige = sessao.Flow.RequireRole("admin")

// Middleware guards this folder on top of the session asked above it.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const notifyOutboxPage = `// Package fila is the outbox: everything the application decided to tell
// somebody, what happened to it, the resend button, today's digest, and the
// limits.
package fila

import (
	"errors"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/notificar"
)

// Page renders GET {{.URL}}notificacoes/fila.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.notify_outbox}}")
	fila := trilha.Use[*notificar.Notificador](c).Fila("")
	var lista h.Node = ui.Muted(h.Text("{{.T.notify_outbox_empty}}"))
	if len(fila) > 0 {
		linhas := make([]h.Node, 0, len(fila))
		for _, n := range fila {
			var acao h.Node = h.Group()
			if n.Estado != notificar.Enviada {
				acao = h.Form(h.Method("post"), h.Action("{{.URL}}notificacoes/fila"),
					trilha.CSRFInput(c),
					h.Input(h.Type("hidden"), h.Name("acao"), h.Value("reenviar")),
					h.Input(h.Type("hidden"), h.Name("id"), h.Value(n.ID)),
					ui.Submit(ui.Outline(), h.Text("{{.T.notify_resend}}")))
			}
			linhas = append(linhas, h.Tr(
				h.Td(h.Text(n.Para.Email)), h.Td(h.Text(n.Titulo)), h.Td(h.Text(n.Canal)),
				h.Td(ui.Badge(ui.Outline(), h.Text(string(n.Estado)))),
				h.Td(h.Code(h.Text(n.Erro))), h.Td(ui.Date(c, n.Criada)), h.Td(acao),
			))
		}
		lista = ui.Table(h.Thead(h.Tr(h.Th(h.Text("{{.T.notify_to}}")), h.Th(h.Text("{{.T.notify_subject}}")),
			h.Th(h.Text("{{.T.notify_channel}}")), h.Th(h.Text("{{.T.notify_state}}")), h.Th(h.Text("{{.T.notify_error}}")),
			h.Th(), h.Th())), h.Tbody(linhas...))
	}
	return ui.Stack(
		ui.PageHeader("{{.T.notify_outbox}}",
			h.Form(h.Method("post"), h.Action("{{.URL}}notificacoes/fila"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("acao"), h.Value("digesto")),
				ui.Submit(ui.Outline(), h.Text("{{.T.notify_digest_now}}")))),
		ui.Muted(h.Text("{{.T.notify_outbox_desc}}")),
		lista,
		ui.H2(h.Text("{{.T.notify_settings}}")),
		ui.SettingsForm(c, notificar.Config, nil),
	), nil
}

// POST is resend, today's digest, or the settings. Each one is written to
// the audit trail — sending a message on somebody's behalf is a decision.
func POST(c *trilha.Ctx) error {
	n := trilha.Use[*notificar.Notificador](c)
	switch c.Form("acao") {
	case "reenviar":
		id := c.Form("id")
		no, err := n.Reenviar(c, id)
		switch {
		case errors.Is(err, notificar.ErrNaoExiste):
			return trilha.ErrNotFound
		case errors.Is(err, notificar.ErrLimite):
			c.Flash(ui.FlashError, "{{.T.notify_still_limited}}")
		case err != nil:
			c.Flash(ui.FlashError, err.Error())
		}
		c.Audit("notificacao.reenviada", id, trilha.Fields{"estado": string(no.Estado), "canal": no.Canal})
	case "digesto":
		if err := n.DispararDigesto(c); err != nil {
			return err
		}
		c.Audit("notificacao.digesto", "hoje")
	default:
		if err := notificar.Config.Update(c); err != nil {
			var fe trilha.FieldErrors
			if errors.As(err, &fe) {
				return trilha.Errorf(http.StatusUnprocessableEntity, "%s", "{{.T.notify_settings_invalid}}")
			}
			return err
		}
	}
	return c.Redirect("{{.URL}}notificacoes/fila")
}
`

// notifyTest is the recipe proving itself in the project: the preferences
// screen validates by the tags and by what is wired, and the outbox shows
// what the limit held and resends it — for administrators only.
const notifyTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/mail"

	"{{.Module}}/internal/notificar"
	"{{.Module}}/internal/usuarios"
)

func appDeNotificacoes(t *testing.T) *trilha.App {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return newApp()
}

func entrarComo(t *testing.T, a *trilha.App, email, senha string) *trilha.TestClient {
	t.Helper()
	c := trilha.NewTestClient(t, a)
	c.PostForm("{{.URL}}entrar", url.Values{"email": {email}, "password": {senha}}).WantStatus(http.StatusSeeOther)
	return c
}

// A tela de preferências valida pelas tags e pelo que está ligado: um canal
// que o app não ligou é recusado com a mensagem no campo.
func TestNotifyPreferences(t *testing.T) {
	a := appDeNotificacoes(t)
	trilha.NewTestClient(t, a).Get("{{.URL}}notificacoes").WantStatus(http.StatusUnauthorized)

	c := entrarComo(t, a, "admin@example.com", "a-password-nobody-guesses")
	c.Get("{{.URL}}notificacoes").WantStatus(http.StatusOK).WantContains(` + "`" + `name="silencio_de"` + "`" + `)

	// A tela lida como o navegador a recebe: token no formulário, nonce nos
	// scripts, cookies HttpOnly; e, na volta do 422, o foco no campo errado.
	snap := c.Get("{{.URL}}notificacoes").Snapshot()
	for _, err := range []error{snap.HasCSRFToken(), snap.HasCSPNonce(), snap.HasSafeCookies()} {
		if err != nil {
			t.Error(err)
		}
	}
	recusado := c.PostForm("{{.URL}}notificacoes", url.Values{"canal": {"mail"}, "silencio_de": {"25"}}).
		WantStatus(http.StatusUnprocessableEntity).Snapshot()
	if err := recusado.FocusedOnError("#silencio_de"); err != nil {
		t.Error(err)
	}
	if _, ligado := notificar.Canais[notificar.CanalWhatsApp]; !ligado {
		c.PostForm("{{.URL}}notificacoes", url.Values{"canal": {"whatsapp"}}).
			WantStatus(http.StatusUnprocessableEntity)
	}
	c.PostForm("{{.URL}}notificacoes", url.Values{"canal": {"mail"}, "digesto": {"true"},
		"silencio_de": {"22"}, "silencio_ate": {"7"}}).WantStatus(http.StatusSeeOther)

	p := trilha.Use[*notificar.Notificador](a).Preferencias("u-1")
	if p.Canal != "mail" || !p.Digesto || p.SilencioDe != 22 || p.SilencioAte != 7 {
		t.Fatalf("preferências = %+v", p)
	}
	c.Get("{{.URL}}notificacoes").WantStatus(http.StatusOK).WantContains(` + "`" + `value="22"` + "`" + `)
}

// A fila mostra o que o limite segurou, com o código, e o reenvio manda —
// só para quem administra.
func TestNotifyOutboxReplay(t *testing.T) {
	a := appDeNotificacoes(t)
	caixa := &mail.Outbox{}
	antes := notificar.Mailer
	notificar.Mailer = mail.New(mail.Options{From: "avisos@example.com", Transport: caixa})
	t.Cleanup(func() { notificar.Mailer = antes })
	ajustes := notificar.Config.Get()
	if err := notificar.Config.Set(notificar.Ajustes{LimitePorHora: 1, HoraDigesto: 8}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { notificar.Config.Set(ajustes) })

	// Uma notificação sai de dentro de uma requisição, que é onde o app a
	// dispara.
	var ids []string
	a.Register(trilha.Route{Pattern: "/_teste/notificar", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"POST": func(c *trilha.Ctx) error {
				no, _ := trilha.Use[*notificar.Notificador](c).Notificar(c,
					notificar.Pessoa{Sujeito: "u-cliente", Email: "cliente@example.com"}, "Pedido pago", "O pedido 42 foi pago.")
				ids = append(ids, no.ID)
				return c.Text(http.StatusOK, string(no.Estado))
			},
		}})
	c := trilha.NewTestClient(t, a)
	c.PostForm("/_teste/notificar", url.Values{}).WantStatus(http.StatusOK).WantContains("enviada")
	c.PostForm("/_teste/notificar", url.Values{}).WantStatus(http.StatusOK).WantContains("limitada")
	if len(caixa.Messages()) != 1 {
		t.Fatalf("saíram %d, o limite era 1", len(caixa.Messages()))
	}

	if err := trilha.Use[*usuarios.Store](a).Add("u-2", "pessoa@example.com", "Pessoa", "leitor",
		"a-password-for-somebody"); err != nil {
		t.Fatal(err)
	}
	entrarComo(t, a, "pessoa@example.com", "a-password-for-somebody").
		Get("{{.URL}}notificacoes/fila").WantStatus(http.StatusForbidden)

	admin := entrarComo(t, a, "admin@example.com", "a-password-nobody-guesses")
	admin.Get("{{.URL}}notificacoes/fila").WantStatus(http.StatusOK).WantContains("limitada", notificar.CodigoLimite)
	if err := notificar.Config.Set(notificar.Ajustes{LimitePorHora: 10, HoraDigesto: 8}); err != nil {
		t.Fatal(err)
	}
	admin.PostForm("{{.URL}}notificacoes/fila", url.Values{"acao": {"reenviar"}, "id": {ids[1]}}).
		WantStatus(http.StatusSeeOther)
	if len(caixa.Messages()) != 2 || !strings.Contains(caixa.Last().Subject, "Pedido pago") {
		t.Fatalf("o reenvio não saiu: %d mensagens", len(caixa.Messages()))
	}
	for _, no := range trilha.Use[*notificar.Notificador](a).Fila("") {
		if no.Estado != notificar.Enviada {
			t.Fatalf("depois do reenvio: %+v", no)
		}
	}
}
`
