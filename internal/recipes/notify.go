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
		CtxPackCost: 232,
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
			{Rel: "internal/notificar/contrato_test.go", Go: true, Body: notifyContractTest},
			{Rel: "internal/notificar/notificartest/notificartest.go", Go: true, Body: notifyContract},
			{Rel: "internal/notificar/sql.go", Go: true, Body: notifySQL},
			{Rel: "migrations/0110_notify.sql", Body: notifyMigration},
			{Rel: "{{.At}}notificacoes/middleware.go", Go: true, Body: notifyMiddleware},
			{Rel: "{{.At}}notificacoes/page.go", Go: true, Body: notifyPrefsPage},
			{Rel: "{{.At}}notificacoes/fila/middleware.go", Go: true, Body: notifyOutboxMiddleware},
			{Rel: "{{.At}}notificacoes/fila/page.go", Go: true, Body: notifyOutboxPage},
			{Rel: "notificacoes_test.go", Go: true, Body: notifyTest},
		},
		Setup: []Insert{
			notifyWebhookLink("internal/avisos/avisos.go"),
			notifyWhatsAppLink("internal/whatsapp/whatsapp.go"),
			notifyStoreLink("internal/store/store.go"),
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
// The rows live behind the Store interface: in memory until the project has a
// database, and in the tables of migrations/0110_notify.sql once
// ` + "`trilha add store`" + ` is there (sql.go) — without a screen changing.
package notificar

import (
	"context"
	"database/sql"
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

// Store is where the rows live: the preferences and the outbox, the tables
// of migrations/0110_notify.sql. Novo keeps them in memory; with
// ` + "`trilha add store`" + ` in the project, setup.go points Banco at the
// database and Setup uses NovoSQL (sql.go) — the same methods, so no screen
// changes.
type Store interface {
	// Preferencias is what the person chose, and whether they chose at all.
	Preferencias(ctx context.Context, sujeito string) (Preferencias, bool, error)
	SalvarPreferencias(ctx context.Context, sujeito string, p Preferencias) error
	// Guardar writes a new row of the outbox and gives it its id.
	Guardar(ctx context.Context, n Notificacao) (Notificacao, error)
	// Atualizar writes what happened to a row: state, error, attempts, when
	// it left.
	Atualizar(ctx context.Context, n Notificacao) error
	// Uma is one row, or ErrNaoExiste.
	Uma(ctx context.Context, id string) (Notificacao, error)
	// Fila is the outbox, the newest first; para filters by subject.
	Fila(ctx context.Context, para string) ([]Notificacao, error)
	// Esperando is what waits for the digest — asked for it, or held by the
	// quiet hours — the oldest first.
	Esperando(ctx context.Context) ([]Notificacao, error)
	// Dia records the day of a digest and says whether it was the first
	// time: the digest goes once a day, across restarts and replicas.
	Dia(ctx context.Context, dia string) (bool, error)
}

// Memoria is the outbox and the preferences in memory.
type Memoria struct {
	mu    sync.Mutex
	seq   int
	prefs map[string]Preferencias
	fila  []Notificacao
	dias  map[string]bool
}

// NovaMemoria is an empty store in memory.
func NovaMemoria() *Memoria {
	return &Memoria{prefs: map[string]Preferencias{}, dias: map[string]bool{}}
}

func (m *Memoria) Preferencias(_ context.Context, sujeito string) (Preferencias, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.prefs[sujeito]
	return p, ok, nil
}

func (m *Memoria) SalvarPreferencias(_ context.Context, sujeito string, p Preferencias) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prefs[sujeito] = p
	return nil
}

func (m *Memoria) Guardar(_ context.Context, n Notificacao) (Notificacao, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	n.ID = "n-" + strconv.Itoa(m.seq)
	m.fila = append(m.fila, n)
	return n, nil
}

func (m *Memoria) Atualizar(_ context.Context, n Notificacao) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.fila {
		if m.fila[i].ID == n.ID {
			m.fila[i] = n
			return nil
		}
	}
	return ErrNaoExiste
}

func (m *Memoria) Uma(_ context.Context, id string) (Notificacao, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.fila {
		if n.ID == id {
			return n, nil
		}
	}
	return Notificacao{}, ErrNaoExiste
}

func (m *Memoria) Fila(_ context.Context, para string) ([]Notificacao, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Notificacao, 0, len(m.fila))
	for i := len(m.fila) - 1; i >= 0; i-- {
		if para == "" || m.fila[i].Para.Sujeito == para {
			out = append(out, m.fila[i])
		}
	}
	return out, nil
}

func (m *Memoria) Esperando(context.Context) ([]Notificacao, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Notificacao
	for _, n := range m.fila {
		if n.Estado == NoDigesto || n.Estado == Retida {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *Memoria) Dia(_ context.Context, dia string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dias[dia] {
		return false, nil
	}
	m.dias[dia] = true
	return true, nil
}

// Notificador is the module: the rules — preferences, quiet hours, digest,
// limits — over a Store. The limit counts per process: it protects the
// channel from a loop, not the person from a fleet of replicas.
type Notificador struct {
	store   Store
	mu      sync.Mutex
	limite  int
	limites *trilha.Limiter
	tarefas *task.Tasks
	// Agora is the clock. A test sets it to put the quiet hours where it
	// wants them.
	Agora func() time.Time
}

// Novo is a notifier over an empty store in memory.
func Novo() *Notificador { return Sobre(NovaMemoria()) }

// Sobre is a notifier over the store given — the database, in Setup.
func Sobre(s Store) *Notificador { return &Notificador{store: s, Agora: time.Now} }

// Banco is the database, when the project has one: setup.go sets it when the
// store recipe is there (trilha:link notify-store), and Setup then keeps the
// rows in migrations/0110_notify.sql instead of in memory. A function because
// store.Setup, which opens the pool, may run after this one.
var Banco func() (db *sql.DB, arg func(n int) string)

// Setup binds the settings, builds the notifier, hands it to the application,
// starts the digest engine and the clock that sends the digest once a day.
func Setup(a *trilha.App) error {
	if err := Config.Bind(a, nil); err != nil {
		return err
	}
	n := Novo()
	if Banco != nil {
		n = Sobre(NovoSQL(Banco))
	}
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
func (n *Notificador) Preferencias(ctx context.Context, sujeito string) (Preferencias, error) {
	p, ok, err := n.store.Preferencias(ctx, sujeito)
	if err != nil || !ok {
		return Padrao, err
	}
	return p, nil
}

// SalvarPreferencias writes what the person chose.
func (n *Notificador) SalvarPreferencias(ctx context.Context, sujeito string, p Preferencias) error {
	return n.store.SalvarPreferencias(ctx, sujeito, p)
}

// Fila is the outbox, the newest first; para filters by subject when not
// empty.
func (n *Notificador) Fila(ctx context.Context, para string) ([]Notificacao, error) {
	return n.store.Fila(ctx, para)
}

func ctxDe(c *trilha.Ctx) context.Context {
	if c == nil {
		return context.Background()
	}
	return c.Context()
}

// Notificar is the one call. It decides by the person's preferences and the
// channel's limit, and always leaves the notification in the outbox —
// ErrLimite when the limit held it back, the channel's error when it failed.
func (n *Notificador) Notificar(c *trilha.Ctx, para Pessoa, titulo, corpo string) (Notificacao, error) {
	ctx := ctxDe(c)
	p, err := n.Preferencias(ctx, para.Sujeito)
	if err != nil {
		return Notificacao{}, err
	}
	canal := p.Canal
	if _, ok := Canais[canal]; !ok {
		// A preference for a channel that was unwired since falls back to
		// e-mail rather than to silence.
		canal = CanalEmail
	}
	no, err := n.store.Guardar(ctx, Notificacao{Para: para, Telefone: p.Telefone, Canal: canal,
		Titulo: titulo, Corpo: corpo, Criada: n.Agora()})
	if err != nil {
		return Notificacao{}, err
	}
	switch {
	case p.Digesto:
		return n.marcar(ctx, no, NoDigesto, "")
	case p.Silencio(n.Agora().Hour()):
		return n.marcar(ctx, no, Retida, "")
	}
	return n.entregar(c, no)
}

// Reenviar sends again what the limit held back, what failed or what the
// quiet hours retained — the button of the outbox. The quiet hours still
// hold: resending at 3 a.m. is still 3 a.m. for the person.
func (n *Notificador) Reenviar(c *trilha.Ctx, id string) (Notificacao, error) {
	ctx := ctxDe(c)
	no, err := n.store.Uma(ctx, id)
	if err != nil || no.Estado == Enviada {
		return no, err
	}
	p, err := n.Preferencias(ctx, no.Para.Sujeito)
	if err != nil {
		return no, err
	}
	if p.Silencio(n.Agora().Hour()) {
		return n.marcar(ctx, no, Retida, "")
	}
	return n.entregar(c, no)
}

// entregar is the limit and the channel.
func (n *Notificador) entregar(c *trilha.Ctx, no Notificacao) (Notificacao, error) {
	ctx := ctxDe(c)
	if !n.permite(no.Canal + "\x00" + no.Para.Sujeito) {
		no, err := n.marcar(ctx, no, Limitada, CodigoLimite)
		if err != nil {
			return no, err
		}
		return no, ErrLimite
	}
	no.Tentativas++
	if err := Canais[no.Canal](c, no); err != nil {
		no, merr := n.marcar(ctx, no, Falhou, err.Error())
		if merr != nil {
			return no, merr
		}
		return no, err
	}
	return n.marcar(ctx, no, Enviada, "")
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

// marcar writes what happened to one notification and answers it.
func (n *Notificador) marcar(ctx context.Context, no Notificacao, e Estado, erro string) (Notificacao, error) {
	no.Estado, no.Erro = e, erro
	if e == Enviada {
		no.Enviada = n.Agora()
	}
	return no, n.store.Atualizar(ctx, no)
}

// Digesto sends, once a day, one e-mail per person with what waited for it:
// what they asked to receive as a digest, and what their quiet hours held.
// Somebody who is inside their quiet hours right now is left for the next
// digest — the digest is a message too. It answers how many e-mails left.
func (n *Notificador) Digesto(ctx context.Context) (int, error) {
	agora := n.Agora()
	primeiro, err := n.store.Dia(ctx, agora.Format("2006-01-02"))
	if err != nil || !primeiro {
		return 0, err
	}
	esperando, err := n.store.Esperando(ctx)
	if err != nil {
		return 0, err
	}
	porPessoa := map[string][]Notificacao{}
	var ordem []string
	for _, no := range esperando {
		if _, ok := porPessoa[no.Para.Sujeito]; !ok {
			ordem = append(ordem, no.Para.Sujeito)
		}
		porPessoa[no.Para.Sujeito] = append(porPessoa[no.Para.Sujeito], no)
	}
	sort.Strings(ordem)

	enviados := 0
	var erros []string
	for _, sujeito := range ordem {
		p, err := n.Preferencias(ctx, sujeito)
		if err != nil {
			return enviados, err
		}
		if p.Silencio(agora.Hour()) {
			continue
		}
		lista := porPessoa[sujeito]
		itens := make([]h.Node, 0, len(lista))
		for _, no := range lista {
			itens = append(itens, h.Li(h.Strong(h.Text(no.Titulo)), h.Text(" — "+no.Corpo)))
		}
		err = Mailer.Send(ctx, mail.Message{
			To:      []string{lista[0].Para.Email},
			Subject: "{{.T.notify_digest_subject}} — " + Marca,
			Body:    mail.Layout(Marca, h.P(h.Text("{{.T.notify_digest_intro}}")), h.Ul(itens...)),
		})
		for _, no := range lista {
			var merr error
			if err != nil {
				_, merr = n.marcar(ctx, no, Falhou, err.Error())
			} else {
				_, merr = n.marcar(ctx, no, Enviada, "")
			}
			if merr != nil {
				return enviados, merr
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

// notifyStoreLink is the line that moves the outbox and the preferences to
// the database when the store recipe is there. Notify and store both carry
// it, each conditioned on the other's file.
func notifyStoreLink(ifFile string) Insert {
	return Insert{
		Marker:  "// trilha:link notify-store",
		Line:    "\tnotificar.Banco = func() (*sql.DB, func(int) string) { return store.DB, store.D.Arg }\n",
		If:      ifFile,
		Imports: []string{"database/sql", "{{.Module}}/internal/notificar", "{{.Module}}/internal/store"},
	}
}

const notifyMigration = `-- As tabelas do notificar, na convenção do ` + "`" + `trilha add store` + "`" + `: sem banco, o
-- internal/notificar guarda o mesmo formato em memória; com o store, sql.go lê
-- e grava aqui.

CREATE TABLE IF NOT EXISTS notify_preferences (
	subject     TEXT PRIMARY KEY,
	channel     TEXT NOT NULL,
	digest      BOOLEAN NOT NULL DEFAULT FALSE,
	quiet_from  INTEGER NOT NULL DEFAULT 0,
	quiet_to    INTEGER NOT NULL DEFAULT 0,
	phone       TEXT NOT NULL DEFAULT ''
);

-- O id começa pelo instante em hexadecimal de largura fixa: a ordem do id é a
-- ordem de chegada, em qualquer banco, e é por ela que a fila se lê.
CREATE TABLE IF NOT EXISTS notify_outbox (
	id          TEXT PRIMARY KEY,
	subject     TEXT NOT NULL,
	email       TEXT NOT NULL,
	phone       TEXT NOT NULL DEFAULT '',
	channel     TEXT NOT NULL,
	title       TEXT NOT NULL,
	body        TEXT NOT NULL,
	state       TEXT NOT NULL,
	error       TEXT NOT NULL DEFAULT '',
	attempts    INTEGER NOT NULL DEFAULT 0,
	created_at  TIMESTAMP NOT NULL,
	sent_at     TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS notify_outbox_subject ON notify_outbox (subject, id);
CREATE INDEX IF NOT EXISTS notify_outbox_state ON notify_outbox (state, id);

-- Um digesto por dia: a chave primária é o dia, e é ela que segura o segundo
-- digesto de um reinício ou de outra réplica.
CREATE TABLE IF NOT EXISTS notify_digests (
	day  TEXT PRIMARY KEY
);
`

const notifySQL = `package notificar

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// SQL is the preferences and the outbox in the tables of
// migrations/0110_notify.sql. It speaks only database/sql: the driver and the
// dialect are the store recipe's, handed in by Banco.
//
// Every query is written here with placeholders; nothing that came from a
// request is ever part of the SQL text.
type SQL struct {
	banco func() (*sql.DB, func(int) string)
}

// NovoSQL is the store over the database Banco answers.
func NovoSQL(banco func() (*sql.DB, func(int) string)) *SQL { return &SQL{banco: banco} }

// idNovo orders by arrival: the instant in fixed-width hex, then a few random
// bits so two replicas in the same nanosecond do not collide.
func idNovo() string {
	var b [2]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("n-%016x%04x", time.Now().UnixNano(), binary.BigEndian.Uint16(b[:]))
}

const colFila = "id, subject, email, phone, channel, title, body, state, error, attempts, created_at, sent_at"

type linha interface{ Scan(dest ...any) error }

func lerNotificacao(r linha) (Notificacao, error) {
	var n Notificacao
	var estado string
	var enviada sql.NullTime
	err := r.Scan(&n.ID, &n.Para.Sujeito, &n.Para.Email, &n.Telefone, &n.Canal, &n.Titulo, &n.Corpo,
		&estado, &n.Erro, &n.Tentativas, &n.Criada, &enviada)
	n.Estado = Estado(estado)
	if enviada.Valid {
		n.Enviada = enviada.Time
	}
	return n, err
}

func (s *SQL) Preferencias(ctx context.Context, sujeito string) (Preferencias, bool, error) {
	db, arg := s.banco()
	var p Preferencias
	err := db.QueryRowContext(ctx, "SELECT channel, digest, quiet_from, quiet_to, phone FROM notify_preferences WHERE subject = "+
		arg(1), sujeito).Scan(&p.Canal, &p.Digesto, &p.SilencioDe, &p.SilencioAte, &p.Telefone)
	if errors.Is(err, sql.ErrNoRows) {
		return Preferencias{}, false, nil
	}
	return p, err == nil, err
}

func (s *SQL) SalvarPreferencias(ctx context.Context, sujeito string, p Preferencias) error {
	db, arg := s.banco()
	_, err := db.ExecContext(ctx, "INSERT INTO notify_preferences (subject, channel, digest, quiet_from, quiet_to, phone) VALUES ("+
		arg(1)+", "+arg(2)+", "+arg(3)+", "+arg(4)+", "+arg(5)+", "+arg(6)+") ON CONFLICT (subject) DO UPDATE SET "+
		"channel = excluded.channel, digest = excluded.digest, quiet_from = excluded.quiet_from, "+
		"quiet_to = excluded.quiet_to, phone = excluded.phone",
		sujeito, p.Canal, p.Digesto, p.SilencioDe, p.SilencioAte, p.Telefone)
	return err
}

func (s *SQL) Guardar(ctx context.Context, n Notificacao) (Notificacao, error) {
	db, arg := s.banco()
	n.ID = idNovo()
	_, err := db.ExecContext(ctx, "INSERT INTO notify_outbox ("+colFila+") VALUES ("+
		arg(1)+", "+arg(2)+", "+arg(3)+", "+arg(4)+", "+arg(5)+", "+arg(6)+", "+arg(7)+", "+arg(8)+", "+arg(9)+", "+
		arg(10)+", "+arg(11)+", NULL)",
		n.ID, n.Para.Sujeito, n.Para.Email, n.Telefone, n.Canal, n.Titulo, n.Corpo, string(n.Estado), n.Erro,
		n.Tentativas, n.Criada)
	return n, err
}

func (s *SQL) Atualizar(ctx context.Context, n Notificacao) error {
	db, arg := s.banco()
	var enviada sql.NullTime
	if !n.Enviada.IsZero() {
		enviada = sql.NullTime{Time: n.Enviada, Valid: true}
	}
	res, err := db.ExecContext(ctx, "UPDATE notify_outbox SET state = "+arg(1)+", error = "+arg(2)+", attempts = "+
		arg(3)+", sent_at = "+arg(4)+" WHERE id = "+arg(5), string(n.Estado), n.Erro, n.Tentativas, enviada, n.ID)
	if err != nil {
		return err
	}
	if k, err := res.RowsAffected(); err == nil && k == 0 {
		return ErrNaoExiste
	}
	return nil
}

func (s *SQL) Uma(ctx context.Context, id string) (Notificacao, error) {
	db, arg := s.banco()
	n, err := lerNotificacao(db.QueryRowContext(ctx, "SELECT "+colFila+" FROM notify_outbox WHERE id = "+arg(1), id))
	if errors.Is(err, sql.ErrNoRows) {
		return Notificacao{}, ErrNaoExiste
	}
	return n, err
}

func (s *SQL) lista(ctx context.Context, q string, args ...any) ([]Notificacao, error) {
	db, _ := s.banco()
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notificacao
	for rows.Next() {
		n, err := lerNotificacao(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *SQL) Fila(ctx context.Context, para string) ([]Notificacao, error) {
	_, arg := s.banco()
	if para == "" {
		return s.lista(ctx, "SELECT "+colFila+" FROM notify_outbox ORDER BY id DESC")
	}
	return s.lista(ctx, "SELECT "+colFila+" FROM notify_outbox WHERE subject = "+arg(1)+" ORDER BY id DESC", para)
}

func (s *SQL) Esperando(ctx context.Context) ([]Notificacao, error) {
	_, arg := s.banco()
	return s.lista(ctx, "SELECT "+colFila+" FROM notify_outbox WHERE state IN ("+arg(1)+", "+arg(2)+") ORDER BY id",
		string(NoDigesto), string(Retida))
}

// Dia leans on the primary key: the second insert of the same day changes
// nothing, and that is the answer.
func (s *SQL) Dia(ctx context.Context, dia string) (bool, error) {
	db, arg := s.banco()
	res, err := db.ExecContext(ctx, "INSERT INTO notify_digests (day) VALUES ("+arg(1)+") ON CONFLICT (day) DO NOTHING", dia)
	if err != nil {
		return false, err
	}
	k, err := res.RowsAffected()
	return k == 1, err
}
`

// notifyContractTest runs the contract on memory, from outside the package —
// notificartest imports notificar, so the test that calls it cannot be
// inside.
const notifyContractTest = `package notificar_test

import (
	"testing"

	"{{.Module}}/internal/notificar"
	"{{.Module}}/internal/notificar/notificartest"
)

// O contrato do store sobre a memória: os mesmos passos que o
// notificacoes_test.go do projeto roda sobre o que o app ligou.
func TestNotifyStoreContract(t *testing.T) {
	notificartest.Contrato(t, notificar.Novo())
}
`

// notifyContract is what every Store of the notifier promises, run through
// the Notificador — the preferences, the outbox, the digest once a day — so
// the memory and the database cannot drift apart.
const notifyContract = `// Package notificartest is the contract of the notifier's store, for a test
// to run against any of them: the one in memory and the one in the database
// answer the same, or one of them is wrong.
package notificartest

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/mail"

	"{{.Module}}/internal/notificar"
)

// Contrato runs the whole store through n. The subjects carry a prefix of this
// run and the clock a day nobody else uses, so a database with other rows
// neither disturbs it nor is disturbed.
func Contrato(t *testing.T, n *notificar.Notificador) {
	t.Helper()
	ctx := context.Background()
	px := strconv.FormatInt(time.Now().UnixNano(), 36) + "-"
	dia := time.Date(2100+rand.Intn(800), time.Month(1+rand.Intn(12)), 1+rand.Intn(28), 0, 0, 0, 0, time.UTC)
	as := func(h int) func() time.Time { return func() time.Time { return dia.Add(time.Duration(h) * time.Hour) } }

	var enviadas []notificar.Notificacao
	antes, mailer := notificar.Canais[notificar.CanalEmail], notificar.Mailer
	notificar.Canais[notificar.CanalEmail] = func(_ *trilha.Ctx, no notificar.Notificacao) error {
		enviadas = append(enviadas, no)
		return nil
	}
	caixa := &mail.Outbox{}
	notificar.Mailer = mail.New(mail.Options{From: "avisos@example.com", Transport: caixa})
	t.Cleanup(func() { notificar.Canais[notificar.CanalEmail], notificar.Mailer = antes, mailer })

	ana := notificar.Pessoa{Sujeito: px + "ana", Email: "ana@example.com"}
	bia := notificar.Pessoa{Sujeito: px + "bia", Email: "bia@example.com"}

	// Quem nunca escolheu tem o padrão; o que se salva volta igual.
	if p, err := n.Preferencias(ctx, ana.Sujeito); err != nil || p != notificar.Padrao {
		t.Fatalf("padrão = %+v %v", p, err)
	}
	quer := notificar.Preferencias{Canal: notificar.CanalEmail, SilencioDe: 22, SilencioAte: 7, Telefone: "+5511999990000"}
	if err := n.SalvarPreferencias(ctx, ana.Sujeito, quer); err != nil {
		t.Fatal(err)
	}
	quer.SilencioAte = 6
	if err := n.SalvarPreferencias(ctx, ana.Sujeito, quer); err != nil {
		t.Fatal(err)
	}
	if p, err := n.Preferencias(ctx, ana.Sujeito); err != nil || p != quer {
		t.Fatalf("lidas de volta = %+v %v", p, err)
	}
	if err := n.SalvarPreferencias(ctx, bia.Sujeito, notificar.Preferencias{Canal: notificar.CanalEmail, Digesto: true}); err != nil {
		t.Fatal(err)
	}

	// Às 23h a da Ana fica retida; a da Bia vai para o digesto; ao meio-dia
	// a da Ana sai.
	n.Agora = as(23)
	retida, err := n.Notificar(nil, ana, "Retida", "corpo")
	if err != nil || retida.Estado != notificar.Retida || retida.ID == "" || retida.Telefone != quer.Telefone {
		t.Fatalf("retida = %+v %v", retida, err)
	}
	if no, err := n.Reenviar(nil, retida.ID); err != nil || no.Estado != notificar.Retida {
		t.Fatalf("o reenvio furou o silêncio: %+v %v", no, err)
	}
	if no, err := n.Notificar(nil, bia, "Da Bia", "corpo"); err != nil || no.Estado != notificar.NoDigesto {
		t.Fatalf("digesto = %+v %v", no, err)
	}
	n.Agora = as(12)
	saiu, err := n.Notificar(nil, ana, "Saiu", "corpo")
	if err != nil || saiu.Estado != notificar.Enviada || saiu.Tentativas != 1 || saiu.Enviada.IsZero() || len(enviadas) != 1 {
		t.Fatalf("ao meio-dia = %+v %v (%d enviadas)", saiu, err, len(enviadas))
	}
	if _, err := n.Reenviar(nil, px+"nenhuma"); !errors.Is(err, notificar.ErrNaoExiste) {
		t.Fatalf("reenviar uma que não existe: %v", err)
	}

	// A fila: a mais nova primeiro, e o filtro por pessoa.
	fila, err := n.Fila(ctx, ana.Sujeito)
	if err != nil || len(fila) != 2 || fila[0].ID != saiu.ID || fila[1].ID != retida.ID {
		t.Fatalf("fila da Ana = %+v %v", fila, err)
	}
	if fila[1].Para != ana || fila[1].Titulo != "Retida" || fila[1].Canal != notificar.CanalEmail {
		t.Fatalf("a linha não voltou inteira: %+v", fila[1])
	}

	// O digesto do dia seguinte, às 8h, leva a retida e a da Bia; o segundo
	// do mesmo dia não manda nada.
	n.Agora = func() time.Time { return dia.Add(32 * time.Hour) }
	if k, err := n.Digesto(ctx); err != nil || k != 2 || len(caixa.Messages()) != 2 {
		t.Fatalf("digesto = %d %v (%d e-mails)", k, err, len(caixa.Messages()))
	}
	if k, err := n.Digesto(ctx); err != nil || k != 0 || len(caixa.Messages()) != 2 {
		t.Fatalf("o segundo digesto do dia mandou %d (%v)", k, err)
	}
	for _, p := range []notificar.Pessoa{ana, bia} {
		fila, err := n.Fila(ctx, p.Sujeito)
		if err != nil {
			t.Fatal(err)
		}
		for _, no := range fila {
			if no.Estado != notificar.Enviada {
				t.Fatalf("depois do digesto: %+v", no)
			}
		}
	}
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

var bg = context.Background()

// Das 22 às 7 nada sai: a notificação fica retida, e sai no digesto quando
// a janela acaba — o digesto também é uma mensagem.
func TestNotifyQuietHours(t *testing.T) {
	enviadas := canalDeTeste(t)
	caixa := caixaDeTeste(t)
	n := Novo()
	n.SalvarPreferencias(bg, ana.Sujeito, Preferencias{Canal: CanalEmail, SilencioDe: 22, SilencioAte: 7})

	for _, c := range []struct {
		hora     int
		silencio bool
	}{ {21, false}, {22, true}, {23, true}, {0, true}, {6, true}, {7, false}, {12, false} } {
		if p, _ := n.Preferencias(bg, ana.Sujeito); p.Silencio(c.hora) != c.silencio {
			t.Errorf("%dh: silêncio deveria ser %v", c.hora, c.silencio)
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
	n.SalvarPreferencias(bg, ana.Sujeito, Preferencias{Canal: CanalEmail, Digesto: true})
	bia := Pessoa{Sujeito: "u-bia", Email: "bia@example.com"}
	n.SalvarPreferencias(bg, bia.Sujeito, Preferencias{Canal: CanalEmail, Digesto: true})

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
	fila, _ := n.Fila(bg, ana.Sujeito)
	for _, no := range fila {
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
	p, err := trilha.Use[*notificar.Notificador](c).Preferencias(c.Context(), u.Subject)
	if err != nil {
		return nil, err
	}
	return tela(c, p, nil)
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
		pagina, err := tela(c, p, fe)
		if err != nil {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, pagina)
	}
	if err := trilha.Use[*notificar.Notificador](c).SalvarPreferencias(c.Context(), u.Subject, p); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "{{.T.notify_saved}}")
	return c.Redirect("{{.URL}}notificacoes")
}

func tela(c *trilha.Ctx, p notificar.Preferencias, errs trilha.FieldErrors) (h.Node, error) {
	var canais []ui.Option
	for _, canal := range notificar.Disponiveis() {
		canais = append(canais, ui.Option{Value: canal, Label: RotuloCanal(canal)})
	}
	var digesto h.Node = h.Group()
	if p.Digesto {
		digesto = h.Checked()
	}
	recentes, err := trilha.Use[*notificar.Notificador](c).Fila(c.Context(), sessao.Atual(c).Subject)
	if err != nil {
		return nil, err
	}
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
	), nil
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
	fila, err := trilha.Use[*notificar.Notificador](c).Fila(c.Context(), "")
	if err != nil {
		return nil, err
	}
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
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/mail"

	"{{.Module}}/internal/notificar"
	"{{.Module}}/internal/notificar/notificartest"
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

	p, _ := trilha.Use[*notificar.Notificador](a).Preferencias(context.Background(), "u-1")
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
	fila, _ := trilha.Use[*notificar.Notificador](a).Fila(context.Background(), "")
	for _, no := range fila {
		if no.Estado != notificar.Enviada {
			t.Fatalf("depois do reenvio: %+v", no)
		}
	}
}

// O contrato do store sobre o que o app ligou: a memória num projeto sem
// banco, as tabelas de migrations/0110_notify.sql com o store.
func TestNotifyStoreContractOnTheApp(t *testing.T) {
	notificartest.Contrato(t, trilha.Use[*notificar.Notificador](appDeNotificacoes(t)))
}
`
