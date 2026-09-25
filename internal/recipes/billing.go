package recipes

// billingRecipe is charging for something without coupling the application to
// whoever charges the card: plans, subscriptions with a closed state machine,
// invoices, the signed events the provider sends, the reminders that go out
// when a charge fails, and the screens for the people who look after money.
//
// The provider is the part that changes between applications and this recipe
// does not pick one. It charges, and it tells the application what happened
// at /webhooks/billing, signed over timestamp.body. What that means —
// past_due after a failed charge, canceled after the third — is the part only
// the application knows, and it is the part written here.
//
// It composes what exists (spec 162): the session and roles of login, the
// sealed secret of connections, webhook.Verify, task, mail, auth.Policy,
// Ctx.CSV and Ctx.Audit. Nothing new enters the framework.
func billingRecipe() Recipe {
	return Recipe{
		Name:        "billing",
		CtxPackCost: 356,
		Summary: map[string]string{
			"en": "charging without a coupled provider: plans, subscriptions trial → active → past_due → canceled, signed idempotent webhook, dunning by e-mail, CSV",
			"pt": "cobrança sem provedor acoplado: planos, assinaturas trial → active → past_due → canceled, webhook assinado e idempotente, dunning por e-mail, CSV",
		},
		Doc: "/cookbook/billing",
		// The secret the provider signs with lives in a sealed connection,
		// and the screens decide by role: both have to exist first.
		Needs: []Need{
			{Recipe: "login", File: "internal/sessao/sessao.go"},
			{Recipe: "connections", File: "internal/conexoes/conexoes.go"},
		},
		Files: []File{
			{Rel: "internal/cobranca/cobranca.go", Go: true, Body: billingDomain},
			{Rel: "internal/cobranca/webhook.go", Go: true, Body: billingWebhook},
			{Rel: "internal/cobranca/dunning.go", Go: true, Body: billingDunning},
			{Rel: "internal/cobranca/sql.go", Go: true, Body: billingSQL},
			{Rel: "internal/cobranca/cobranca_test.go", Go: true, Body: billingDomainTest},
			{Rel: "internal/cobranca/cobrancatest/cobrancatest.go", Go: true, Body: billingContract},
			{Rel: "migrations/0100_billing.sql", Body: billingMigration},
			// The address is fixed on purpose: it is typed into the provider's
			// dashboard, so it does not move with the screens.
			{Rel: "app/webhooks/billing/route.go", Go: true, Body: billingRoute},
			{Rel: "app/webhooks/billing/kind.go", Go: true, Body: billingKind},
			{Rel: "{{.At}}billing/middleware.go", Go: true, Body: billingMiddleware},
			{Rel: "{{.At}}billing/page.go", Go: true, Body: billingPage},
			{Rel: "{{.At}}billing/planos/middleware.go", Go: true, Body: billingAdminMiddleware("planos")},
			{Rel: "{{.At}}billing/planos/page.go", Go: true, Body: billingPlansPage},
			{Rel: "{{.At}}billing/faturas/page.go", Go: true, Body: billingInvoicesPage},
			{Rel: "{{.At}}billing/faturas/csv/middleware.go", Go: true, Body: billingAdminMiddleware("csv")},
			{Rel: "{{.At}}billing/faturas/csv/route.go", Go: true, Body: billingCSV},
			{Rel: "billing_test.go", Go: true, Body: billingTest},
		},
		Setup: []Insert{billingStoreLink("internal/store/store.go"), {
			Marker: "// trilha:add billing",
			Line:   "\tif err := cobranca.Setup(a); err != nil {\n\t\treturn err\n\t}\n",
		}},
		Imports: []string{"{{.Module}}/internal/cobranca"},
		Next: map[string]string{
			"en": "Open {{.URL}}conexoes and create a connection of kind API named `billing-webhook` whose " +
				"secret is the one your provider signs with. Point the provider at " +
				"https://<your host>/webhooks/billing — it signs timestamp.body in X-Webhook-Timestamp and " +
				"X-Webhook-Signature; a provider with another scheme is the parse in internal/cobranca/webhook.go, " +
				"and nothing else. Give somebody the role billing:admin or billing:reader and open {{.URL}}billing. " +
				"The rows are in memory until `trilha add store` is in the project, before or after: then they go to " +
				"the tables of migrations/0100_billing.sql, with no screen changing.",
			"pt": "Abra {{.URL}}conexoes e crie uma conexão do tipo API chamada `billing-webhook` cujo segredo " +
				"é o que o seu provedor usa para assinar. Aponte o provedor para " +
				"https://<seu host>/webhooks/billing — ele assina timestamp.body em X-Webhook-Timestamp e " +
				"X-Webhook-Signature; um provedor com outro esquema é o parse em internal/cobranca/webhook.go, e " +
				"mais nada. Dê a alguém o papel billing:admin ou billing:reader e abra {{.URL}}billing. " +
				"As linhas ficam em memória até o `trilha add store` estar no projeto, antes ou depois: aí vão para " +
				"as tabelas de migrations/0100_billing.sql, sem nenhuma tela mudar.",
		},
	}
}

// billingStoreLink is the line that moves the billing rows to the database
// when the store recipe is there: Banco answers the pool and the dialect's
// placeholder, and cobranca.Setup uses NovoSQL. Billing and store both carry
// it, each conditioned on the other's file, so the tie happens whichever
// arrives second. Banco is a function because store.Setup, which opens the
// pool, may run after cobranca.Setup.
func billingStoreLink(ifFile string) Insert {
	return Insert{
		Marker:  "// trilha:link billing-store",
		Line:    "\tcobranca.Banco = func() (*sql.DB, func(int) string) { return store.DB, store.D.Arg }\n",
		If:      ifFile,
		Imports: []string{"database/sql", "{{.Module}}/internal/cobranca", "{{.Module}}/internal/store"},
	}
}

const billingDomain = `// Package cobranca is what this application charges for: plans,
// subscriptions, invoices, and the events the payment provider sends about
// them.
//
// No provider is coupled here. The provider charges the card and says what
// happened, signed, at /webhooks/billing; this package decides what it means.
// Changing provider is rewriting the parse in webhook.go and nothing else.
//
// The rows live behind the Store interface: in memory until the project has
// a database, and in the tables of migrations/0100_billing.sql once
// ` + "`trilha add store`" + ` is there (sql.go) — without a screen changing.
package cobranca

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/task"
)

// The two roles of this module. They are roles and not an if in a handler:
// Politica below turns them into levels, and each folder's middleware.go asks
// the policy, so a screen written tomorrow is guarded the same way.
const (
	// PapelAdmin edits plans and exports invoices.
	PapelAdmin = "billing:admin"
	// PapelLeitor reads the screens and changes nothing.
	PapelLeitor = "billing:reader"
)

// Politica is who may do what with money. "admin" is here because the person
// who administers the application should not need a second role to look at
// what it charges.
var Politica = auth.Policy{
	Modules: []string{"billing"},
	Levels:  auth.Levels{"ler", "administrar"},
	Roles: map[string]auth.Grants{
		PapelLeitor: {"billing": "ler"},
		PapelAdmin:  {"billing": "administrar"},
		"admin":     {"billing": "administrar"},
	},
}

// Estado is where a subscription is in its life.
type Estado string

// The four states. The strings are the provider-neutral names the tables and
// the CSV use.
const (
	Trial     Estado = "trial"
	Ativa     Estado = "active"
	Atrasada  Estado = "past_due"
	Cancelada Estado = "canceled"
)

// Estados lists them in the order the screen offers them.
func Estados() []Estado { return []Estado{Trial, Ativa, Atrasada, Cancelada} }

// Transicoes is the whole machine. Nothing changes a subscription's state
// without passing through Mover, and Mover only allows what is written here —
// a canceled subscription does not come back to life because an old event
// arrived late.
var Transicoes = map[Estado][]Estado{
	Trial:     {Ativa, Atrasada, Cancelada},
	Ativa:     {Atrasada, Cancelada},
	Atrasada:  {Ativa, Cancelada},
	Cancelada: nil,
}

// ErrTransicao is a change the machine does not allow.
var ErrTransicao = errors.New("cobranca: that transition is not allowed")

// ErrNaoExiste is an id the store does not have.
var ErrNaoExiste = errors.New("cobranca: no such row")

// ErrPlanoEmUso is deleting a plan somebody is still subscribed to.
var ErrPlanoEmUso = errors.New("cobranca: a subscription still uses this plan")

// Mover says whether a subscription may go from de to para.
func Mover(de, para Estado) error {
	for _, p := range Transicoes[de] {
		if p == para {
			return nil
		}
	}
	return fmt.Errorf("%w: %s → %s", ErrTransicao, de, para)
}

// MaxTentativas is how many failed charges in a row a subscription survives.
// Each one sends a reminder; the last one cancels.
const MaxTentativas = 3

// Plano is what somebody subscribes to. The tags are the form's rules and
// the screen's labels, in one place.
type Plano struct {
	ID        string
	Nome      string ` + "`" + `form:"nome" validate:"required,max=80" label:"{{.T.billing_plan_name}}"` + "`" + `
	Centavos  int64  ` + "`" + `form:"centavos" validate:"min=0" label:"{{.T.billing_plan_cents}}"` + "`" + `
	Moeda     string ` + "`" + `form:"moeda" validate:"required,len=3" label:"{{.T.billing_plan_currency}}"` + "`" + `
	Intervalo string ` + "`" + `form:"intervalo" validate:"required,oneof=month year" label:"{{.T.billing_plan_interval}}"` + "`" + `
}

// Assinatura is one customer on one plan.
type Assinatura struct {
	ID     string
	Email  string
	Plano  string
	Estado Estado
	// Tentativas is how many charges failed in a row. A paid invoice puts it
	// back to zero.
	Tentativas int
	Criada     time.Time
	Atualizada time.Time
}

// Fatura is one charge.
type Fatura struct {
	ID         string
	Assinatura string
	Centavos   int64
	Moeda      string
	Paga       bool
	Emitida    time.Time
}

// LinhaCSV is what the export writes: the columns an accountant reads, and
// not the struct the code happens to have.
type LinhaCSV struct {
	Fatura     string    ` + "`" + `csv:"{{.T.billing_invoice}}"` + "`" + `
	Assinatura string    ` + "`" + `csv:"{{.T.billing_subscription}}"` + "`" + `
	Valor      float64   ` + "`" + `csv:"{{.T.billing_amount}}"` + "`" + `
	Moeda      string    ` + "`" + `csv:"{{.T.billing_plan_currency}}"` + "`" + `
	Paga       bool      ` + "`" + `csv:"{{.T.billing_paid}}"` + "`" + `
	Emitida    time.Time ` + "`" + `csv:"{{.T.billing_issued}}"` + "`" + `
}

// Store is the four tables of migrations/0100_billing.sql. NovoStore keeps
// them in memory; with ` + "`trilha add store`" + ` in the project, setup.go points
// Banco at the database and Setup uses NovoSQL (sql.go) — the same methods,
// so no screen changes. Every method takes the request's context and answers
// an error: a database can fail where a map cannot.
type Store interface {
	// Planos is every plan, by name.
	Planos(ctx context.Context) ([]Plano, error)
	// SalvarPlano writes a plan, giving it an id when it has none.
	SalvarPlano(ctx context.Context, p Plano) (Plano, error)
	// ApagarPlano removes a plan nobody is subscribed to (ErrPlanoEmUso).
	ApagarPlano(ctx context.Context, id string) error
	// Assinaturas lists the subscriptions in one state, or all of them when
	// estado is empty, the newest first.
	Assinaturas(ctx context.Context, estado Estado) ([]Assinatura, error)
	// Assinatura finds one, or ErrNaoExiste.
	Assinatura(ctx context.Context, id string) (Assinatura, error)
	// Assinar creates a subscription in trial — what the provider's
	// subscription.created does, and where a test starts from.
	Assinar(ctx context.Context, id, email, plano string, agora time.Time) (Assinatura, error)
	// Faturas is every invoice, the newest first.
	Faturas(ctx context.Context) ([]Fatura, error)
	// Visto records an event id and says whether it had been seen already.
	// The provider promises at least once; this is where that becomes
	// exactly once.
	Visto(ctx context.Context, id string, agora time.Time) (bool, error)
	// Esquecer undoes Visto, for an event the store failed to apply: the
	// provider's retry has to be processed, not answered "duplicate".
	Esquecer(ctx context.Context, id string) error
	// Pagou records a paid invoice: active, and the failures forgotten.
	Pagou(ctx context.Context, assinatura, fatura string, centavos int64, moeda string, agora time.Time) (Mudanca, error)
	// Falhou records a failed charge: past due, one more failure counted,
	// and the last one allowed cancels.
	Falhou(ctx context.Context, assinatura, fatura string, centavos int64, moeda string, agora time.Time) (Mudanca, error)
	// Cancelar ends a subscription.
	Cancelar(ctx context.Context, assinatura string, agora time.Time) (Mudanca, error)
}

// Mudanca is what one event did to a subscription, for the audit trail.
type Mudanca struct {
	Assinatura Assinatura
	De         Estado
}

// The three events, as changes to a subscription. Both stores apply them
// with aplicar, so the rule lives in one place.
func pagou(a *Assinatura) Estado { a.Tentativas = 0; return Ativa }

func falhou(a *Assinatura) Estado {
	a.Tentativas++
	if a.Tentativas >= MaxTentativas {
		return Cancelada
	}
	return Atrasada
}

func cancelar(*Assinatura) Estado { return Cancelada }

// aplicar is the one place a subscription changes state. The whole change is
// checked before any of it is written: a refused transition leaves the row,
// the counter and the invoice as they were.
func aplicar(a Assinatura, para func(*Assinatura) Estado, agora time.Time) (Assinatura, error) {
	novo := a
	destino := para(&novo)
	if destino != a.Estado {
		if err := Mover(a.Estado, destino); err != nil {
			return a, err
		}
	}
	novo.Estado, novo.Atualizada = destino, agora
	return novo, nil
}

// Memoria is the four tables in memory: the store of a project without a
// database, and of every test that does not need one.
type Memoria struct {
	mu          sync.Mutex
	seq         int
	planos      map[string]Plano
	assinaturas map[string]Assinatura
	faturas     map[string]Fatura
	eventos     map[string]time.Time
}

// NovoStore is an empty store in memory.
func NovoStore() *Memoria {
	return &Memoria{planos: map[string]Plano{}, assinaturas: map[string]Assinatura{},
		faturas: map[string]Fatura{}, eventos: map[string]time.Time{}}
}

func (s *Memoria) novoID(prefixo string) string {
	s.seq++
	return prefixo + strconv.Itoa(s.seq)
}

func (s *Memoria) Planos(context.Context) ([]Plano, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Plano, 0, len(s.planos))
	for _, p := range s.planos {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out, nil
}

func (s *Memoria) SalvarPlano(_ context.Context, p Plano) (Plano, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = s.novoID("plano-")
	}
	p.Moeda = strings.ToUpper(p.Moeda)
	s.planos[p.ID] = p
	return p, nil
}

func (s *Memoria) ApagarPlano(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.planos[id]; !ok {
		return ErrNaoExiste
	}
	for _, a := range s.assinaturas {
		if a.Plano == id && a.Estado != Cancelada {
			return ErrPlanoEmUso
		}
	}
	delete(s.planos, id)
	return nil
}

func (s *Memoria) Assinaturas(_ context.Context, estado Estado) ([]Assinatura, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Assinatura, 0, len(s.assinaturas))
	for _, a := range s.assinaturas {
		if estado == "" || a.Estado == estado {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Criada.Equal(out[j].Criada) {
			return out[i].Criada.After(out[j].Criada)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Memoria) Assinatura(_ context.Context, id string) (Assinatura, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assinaturas[id]
	if !ok {
		return Assinatura{}, ErrNaoExiste
	}
	return a, nil
}

func (s *Memoria) Assinar(_ context.Context, id, email, plano string, agora time.Time) (Assinatura, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = s.novoID("assinatura-")
	}
	a := Assinatura{ID: id, Email: email, Plano: plano, Estado: Trial, Criada: agora, Atualizada: agora}
	s.assinaturas[id] = a
	return a, nil
}

func (s *Memoria) Faturas(context.Context) ([]Fatura, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Fatura, 0, len(s.faturas))
	for _, f := range s.faturas {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Emitida.Equal(out[j].Emitida) {
			return out[i].Emitida.After(out[j].Emitida)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Memoria) Visto(_ context.Context, id string, agora time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.eventos[id]; ok {
		return true, nil
	}
	s.eventos[id] = agora
	return false, nil
}

func (s *Memoria) Esquecer(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.eventos, id)
	return nil
}

func (s *Memoria) Pagou(_ context.Context, assinatura, fatura string, centavos int64, moeda string, agora time.Time) (Mudanca, error) {
	return s.mudar(assinatura, fatura, centavos, moeda, true, agora, pagou)
}

func (s *Memoria) Falhou(_ context.Context, assinatura, fatura string, centavos int64, moeda string, agora time.Time) (Mudanca, error) {
	return s.mudar(assinatura, fatura, centavos, moeda, false, agora, falhou)
}

func (s *Memoria) Cancelar(_ context.Context, assinatura string, agora time.Time) (Mudanca, error) {
	return s.mudar(assinatura, "", 0, "", false, agora, cancelar)
}

func (s *Memoria) mudar(id, fatura string, centavos int64, moeda string, paga bool, agora time.Time, para func(*Assinatura) Estado) (Mudanca, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assinaturas[id]
	if !ok {
		return Mudanca{}, ErrNaoExiste
	}
	novo, err := aplicar(a, para, agora)
	if err != nil {
		return Mudanca{Assinatura: a, De: a.Estado}, err
	}
	s.assinaturas[id] = novo
	if fatura != "" {
		f, ok := s.faturas[fatura]
		if !ok {
			f = Fatura{ID: fatura, Assinatura: id, Centavos: centavos, Moeda: strings.ToUpper(moeda), Emitida: agora}
		}
		f.Paga = f.Paga || paga
		s.faturas[fatura] = f
	}
	return Mudanca{Assinatura: novo, De: a.Estado}, nil
}

// Billing is what the application provides: the rows and the engine that
// sends the reminders. The engine is this module's own and not provided —
// the tasks recipe provides one too, and one application with two values of
// the same type would lose one of them.
type Billing struct {
	Store   Store
	tarefas *task.Tasks
}

// Banco is the database, when the project has one: setup.go sets it when the
// store recipe is there (trilha:link billing-store), and Setup then keeps the
// rows in migrations/0100_billing.sql instead of in memory. It is a function
// because the pool opens in store.Setup, which may run after this one; the
// store asks for it on the first query. arg is the dialect's placeholder.
var Banco func() (db *sql.DB, arg func(n int) string)

// Setup builds the module, hands it to the application and starts the
// reminder engine, whose Shutdown is hung on the app.
func Setup(a *trilha.App) error {
	var s Store = NovoStore()
	if Banco != nil {
		s = NovoSQL(Banco)
	}
	b := &Billing{Store: s, tarefas: task.New(task.Options{Logger: a.Logger()})}
	b.tarefas.Handle(TarefaLembrete, b.lembrete)
	trilha.Provide(a, b)
	return b.tarefas.Setup(a)
}

// Pode is what a page asks before it draws a button. Hiding is cosmetic:
// the rule that holds is the middleware of the folder.
func Pode(u *auth.User, nivel string) bool { return Politica.Can(u, "billing", nivel) }
`

const billingSQL = `package cobranca

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// SQL is the four tables of migrations/0100_billing.sql. It speaks only
// database/sql: the driver and the dialect are the store recipe's, handed in
// by Banco, so this package never imports them.
//
// Every query is written here with placeholders; nothing that came from a
// request is ever part of the SQL text.
type SQL struct {
	banco func() (*sql.DB, func(int) string)
}

// NovoSQL is the store over the database Banco answers.
func NovoSQL(banco func() (*sql.DB, func(int) string)) *SQL { return &SQL{banco: banco} }

// errConcorrente is a row that changed between the read and the write — the
// same subscription touched by two replicas at once. mudar reads it again.
var errConcorrente = errors.New("cobranca: the subscription changed underneath")

func idNovo(prefixo string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return prefixo + hex.EncodeToString(b[:])
}

// linha is what Scan fills: a row of the database, whatever the driver.
type linha interface{ Scan(dest ...any) error }

func lerPlano(r linha) (Plano, error) {
	var p Plano
	err := r.Scan(&p.ID, &p.Nome, &p.Centavos, &p.Moeda, &p.Intervalo)
	return p, err
}

func lerAssinatura(r linha) (Assinatura, error) {
	var a Assinatura
	var estado string
	err := r.Scan(&a.ID, &a.Email, &a.Plano, &estado, &a.Tentativas, &a.Criada, &a.Atualizada)
	a.Estado = Estado(estado)
	return a, err
}

func lerFatura(r linha) (Fatura, error) {
	var f Fatura
	err := r.Scan(&f.ID, &f.Assinatura, &f.Centavos, &f.Moeda, &f.Paga, &f.Emitida)
	return f, err
}

const (
	colPlano      = "id, name, cents, currency, period"
	colAssinatura = "id, email, plan_id, state, attempts, created_at, updated_at"
	colFatura     = "id, subscription_id, cents, currency, paid, issued_at"
)

func (s *SQL) Planos(ctx context.Context) ([]Plano, error) {
	db, _ := s.banco()
	rows, err := db.QueryContext(ctx, "SELECT "+colPlano+" FROM billing_plans ORDER BY name, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plano
	for rows.Next() {
		p, err := lerPlano(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *SQL) SalvarPlano(ctx context.Context, p Plano) (Plano, error) {
	db, arg := s.banco()
	if p.ID == "" {
		p.ID = idNovo("plano-")
	}
	p.Moeda = strings.ToUpper(p.Moeda)
	_, err := db.ExecContext(ctx, "INSERT INTO billing_plans ("+colPlano+") VALUES ("+
		arg(1)+", "+arg(2)+", "+arg(3)+", "+arg(4)+", "+arg(5)+") ON CONFLICT (id) DO UPDATE SET "+
		"name = excluded.name, cents = excluded.cents, currency = excluded.currency, period = excluded.period",
		p.ID, p.Nome, p.Centavos, p.Moeda, p.Intervalo)
	return p, err
}

func (s *SQL) ApagarPlano(ctx context.Context, id string) error {
	db, arg := s.banco()
	return emTx(ctx, db, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM billing_plans WHERE id = "+arg(1), id).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return ErrNaoExiste
		}
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM billing_subscriptions WHERE plan_id = "+arg(1)+
			" AND state <> "+arg(2), id, string(Cancelada)).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrPlanoEmUso
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM billing_plans WHERE id = "+arg(1), id)
		return err
	})
}

func (s *SQL) Assinaturas(ctx context.Context, estado Estado) ([]Assinatura, error) {
	db, arg := s.banco()
	q, args := "SELECT "+colAssinatura+" FROM billing_subscriptions", []any{}
	if estado != "" {
		q, args = q+" WHERE state = "+arg(1), append(args, string(estado))
	}
	rows, err := db.QueryContext(ctx, q+" ORDER BY created_at DESC, id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Assinatura
	for rows.Next() {
		a, err := lerAssinatura(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *SQL) Assinatura(ctx context.Context, id string) (Assinatura, error) {
	db, arg := s.banco()
	return umaAssinatura(ctx, db, arg, id)
}

// consulta is what a pool and a transaction have in common.
type consulta interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func umaAssinatura(ctx context.Context, q consulta, arg func(int) string, id string) (Assinatura, error) {
	a, err := lerAssinatura(q.QueryRowContext(ctx, "SELECT "+colAssinatura+" FROM billing_subscriptions WHERE id = "+arg(1), id))
	if errors.Is(err, sql.ErrNoRows) {
		return Assinatura{}, ErrNaoExiste
	}
	return a, err
}

func (s *SQL) Assinar(ctx context.Context, id, email, plano string, agora time.Time) (Assinatura, error) {
	db, arg := s.banco()
	if id == "" {
		id = idNovo("assinatura-")
	}
	a := Assinatura{ID: id, Email: email, Plano: plano, Estado: Trial, Criada: agora, Atualizada: agora}
	_, err := db.ExecContext(ctx, "INSERT INTO billing_subscriptions ("+colAssinatura+") VALUES ("+
		arg(1)+", "+arg(2)+", "+arg(3)+", "+arg(4)+", 0, "+arg(5)+", "+arg(6)+") ON CONFLICT (id) DO UPDATE SET "+
		"email = excluded.email, plan_id = excluded.plan_id, state = excluded.state, attempts = 0, "+
		"created_at = excluded.created_at, updated_at = excluded.updated_at",
		a.ID, a.Email, a.Plano, string(a.Estado), a.Criada, a.Atualizada)
	return a, err
}

func (s *SQL) Faturas(ctx context.Context) ([]Fatura, error) {
	db, _ := s.banco()
	rows, err := db.QueryContext(ctx, "SELECT "+colFatura+" FROM billing_invoices ORDER BY issued_at DESC, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Fatura
	for rows.Next() {
		f, err := lerFatura(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Visto leans on the primary key: the insert of an id already there changes
// nothing, and that is the answer — no read-then-write for two replicas to
// race through.
func (s *SQL) Visto(ctx context.Context, id string, agora time.Time) (bool, error) {
	db, arg := s.banco()
	res, err := db.ExecContext(ctx, "INSERT INTO billing_events (id, received_at) VALUES ("+arg(1)+", "+arg(2)+
		") ON CONFLICT (id) DO NOTHING", id, agora)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 0, err
}

func (s *SQL) Esquecer(ctx context.Context, id string) error {
	db, arg := s.banco()
	_, err := db.ExecContext(ctx, "DELETE FROM billing_events WHERE id = "+arg(1), id)
	return err
}

func (s *SQL) Pagou(ctx context.Context, assinatura, fatura string, centavos int64, moeda string, agora time.Time) (Mudanca, error) {
	return s.mudar(ctx, assinatura, fatura, centavos, moeda, true, agora, pagou)
}

func (s *SQL) Falhou(ctx context.Context, assinatura, fatura string, centavos int64, moeda string, agora time.Time) (Mudanca, error) {
	return s.mudar(ctx, assinatura, fatura, centavos, moeda, false, agora, falhou)
}

func (s *SQL) Cancelar(ctx context.Context, assinatura string, agora time.Time) (Mudanca, error) {
	return s.mudar(ctx, assinatura, "", 0, "", false, agora, cancelar)
}

// mudar reads the subscription, applies the change with the same rule as
// memory, and writes it only if the row is still what was read — an
// optimistic check that works the same on both databases. A row that moved
// is read again, a few times, before giving up.
func (s *SQL) mudar(ctx context.Context, id, fatura string, centavos int64, moeda string, paga bool, agora time.Time, para func(*Assinatura) Estado) (Mudanca, error) {
	db, arg := s.banco()
	for tentativa := 0; ; tentativa++ {
		var m Mudanca
		err := emTx(ctx, db, func(tx *sql.Tx) error {
			a, err := umaAssinatura(ctx, tx, arg, id)
			if err != nil {
				return err
			}
			m = Mudanca{Assinatura: a, De: a.Estado}
			novo, err := aplicar(a, para, agora)
			if err != nil {
				return err
			}
			res, err := tx.ExecContext(ctx, "UPDATE billing_subscriptions SET state = "+arg(1)+", attempts = "+arg(2)+
				", updated_at = "+arg(3)+" WHERE id = "+arg(4)+" AND state = "+arg(5)+" AND attempts = "+arg(6),
				string(novo.Estado), novo.Tentativas, novo.Atualizada, id, string(a.Estado), a.Tentativas)
			if err != nil {
				return err
			}
			if n, err := res.RowsAffected(); err != nil || n != 1 {
				return errConcorrente
			}
			if fatura != "" {
				if _, err := tx.ExecContext(ctx, "INSERT INTO billing_invoices ("+colFatura+") VALUES ("+
					arg(1)+", "+arg(2)+", "+arg(3)+", "+arg(4)+", "+arg(5)+", "+arg(6)+") ON CONFLICT (id) DO UPDATE SET "+
					"paid = billing_invoices.paid OR excluded.paid",
					fatura, id, centavos, strings.ToUpper(moeda), paga, agora); err != nil {
					return err
				}
			}
			m.Assinatura = novo
			return nil
		})
		if errors.Is(err, errConcorrente) && tentativa < 3 {
			continue
		}
		return m, err
	}
}

// emTx runs fn in a transaction. The rollback is deferred without a
// condition: rolling back a committed transaction does nothing.
func emTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // see above
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
`

const billingWebhook = `package cobranca

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/webhook"

	"{{.Module}}/internal/conexoes"
)

// ConexaoWebhook is the sealed connection that carries the secret the provider
// signs with. It is a connection and not an environment variable: the secret
// is sealed at rest, masked everywhere else, and rotated on a screen.
const ConexaoWebhook = "billing-webhook"

// CodigoNaoAssinado is the stable code of every refusal of this endpoint. It
// has a page at /docs/errors/E_BILLING_WEBHOOK_UNSIGNED.
const CodigoNaoAssinado = "E_BILLING_WEBHOOK_UNSIGNED"

// Evento is what the provider sends. The names are this application's; a
// provider with other names is a different struct here, and nothing else.
type Evento struct {
	ID         string ` + "`" + `json:"id"` + "`" + `
	Tipo       string ` + "`" + `json:"type"` + "`" + `
	Assinatura string ` + "`" + `json:"subscription"` + "`" + `
	Email      string ` + "`" + `json:"customer_email"` + "`" + `
	Plano      string ` + "`" + `json:"plan"` + "`" + `
	Fatura     string ` + "`" + `json:"invoice"` + "`" + `
	Centavos   int64  ` + "`" + `json:"amount"` + "`" + `
	Moeda      string ` + "`" + `json:"currency"` + "`" + `
}

// The events this module understands. Anything else is recorded as seen and
// answered 200: the provider should not retry what the application chose to
// ignore.
const (
	EventoAssinou  = "subscription.created"
	EventoPagou    = "invoice.paid"
	EventoFalhou   = "invoice.payment_failed"
	EventoCancelou = "subscription.canceled"
)

// Receber is the whole endpoint: signature over timestamp.body, the window,
// the event seen once, the change, the trail, the answer.
//
// webhook.Verify checks the signature in constant time and refuses a
// timestamp more than webhook.Tolerance (five minutes) away from now, in
// either direction — a captured request stops working before anybody could
// replay it. Every way of failing is the same 401 with the same code: saying
// which part failed tells a stranger how close they got.
func (b *Billing) Receber(c *trilha.Ctx) error {
	segredo, err := Segredo(c)
	if err != nil {
		c.Log().Warn("cobranca: webhook refused", "code", CodigoNaoAssinado, "why", "no connection")
		return naoAssinado()
	}
	corpo, err := webhook.Verify(c.Request(), segredo)
	if err != nil {
		c.Log().Warn("cobranca: webhook refused", "code", CodigoNaoAssinado)
		return naoAssinado()
	}
	var ev Evento
	if err := json.Unmarshal(corpo, &ev); err != nil || ev.ID == "" {
		return trilha.Errorf(http.StatusBadRequest, "invalid event")
	}
	// Signed and not seen before, and only then: an unsigned request must not
	// be able to burn an id a real event will carry later.
	agora := time.Now().UTC()
	visto, err := b.Store.Visto(c.Context(), ev.ID, agora)
	if err != nil {
		return err // 500: the provider tries again
	}
	if visto {
		return c.JSON(http.StatusOK, map[string]string{"status": "duplicate"})
	}
	if err := b.aplicar(c, ev, agora); err != nil {
		var falha falhaDoStore
		if errors.As(err, &falha) {
			// The store failed and wrote nothing: the id is forgotten and the
			// answer is 500, so the provider's retry is processed.
			if ferr := b.Store.Esquecer(c.Context(), ev.ID); ferr != nil {
				c.Log().Error("cobranca: event id kept after a failure", "event", ev.ID, "err", ferr)
			}
			return falha.err
		}
		// The event was understood and refused by the machine — an old event
		// arriving after a cancel, say. It is logged with the reason, and the
		// answer is still 200: retrying it would not make it valid.
		c.Log().Warn("cobranca: event not applied", "event", ev.ID, "type", ev.Tipo, "err", err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// falhaDoStore is an error of the store itself — the database, not the
// machine — for which nothing was written.
type falhaDoStore struct{ err error }

func (f falhaDoStore) Error() string { return f.err.Error() }

func doStore(err error) error {
	if err == nil || errors.Is(err, ErrTransicao) || errors.Is(err, ErrNaoExiste) {
		return err
	}
	return falhaDoStore{err}
}

// aplicar turns an event into a change. Every change that touches money is
// written to the audit trail, with the event id as the target and the
// transition in the fields.
func (b *Billing) aplicar(c *trilha.Ctx, ev Evento, agora time.Time) error {
	var (
		m   Mudanca
		err error
	)
	switch ev.Tipo {
	case EventoAssinou:
		a, err := b.Store.Assinar(c.Context(), ev.Assinatura, ev.Email, ev.Plano, agora)
		if err != nil {
			return doStore(err)
		}
		c.Audit("billing.assinatura_criada", a.ID, trilha.Fields{"evento": ev.ID, "plano": ev.Plano})
		return nil
	case EventoPagou:
		m, err = b.Store.Pagou(c.Context(), ev.Assinatura, ev.Fatura, ev.Centavos, ev.Moeda, agora)
	case EventoFalhou:
		m, err = b.Store.Falhou(c.Context(), ev.Assinatura, ev.Fatura, ev.Centavos, ev.Moeda, agora)
	case EventoCancelou:
		m, err = b.Store.Cancelar(c.Context(), ev.Assinatura, agora)
	default:
		return nil
	}
	if err != nil {
		return doStore(err)
	}
	c.Audit("billing."+strings.ReplaceAll(ev.Tipo, ".", "_"), m.Assinatura.ID, trilha.Fields{
		"evento": ev.ID, "fatura": ev.Fatura, "de": string(m.De), "para": string(m.Assinatura.Estado),
		"centavos": ev.Centavos, "tentativa": m.Assinatura.Tentativas,
	})
	if ev.Tipo == EventoFalhou {
		// The reminder goes out as a task: the provider is waiting for this
		// answer, and an e-mail server is not a reason to make it wait.
		return b.Lembrar(c, m.Assinatura)
	}
	return nil
}

// Segredo is the secret, out of the sealed connection. This is the one place
// that reveals it: an HMAC of the body is the one thing Connections cannot
// do on the application's behalf.
func Segredo(c *trilha.Ctx) (string, error) {
	lista, err := conexoes.Conexoes.List(c)
	if err != nil {
		return "", err
	}
	for _, conn := range lista {
		if strings.EqualFold(conn.Name, ConexaoWebhook) && !conn.Secret.Empty() {
			return conn.Secret.Reveal(), nil
		}
	}
	return "", errors.New("cobranca: there is no connection named " + ConexaoWebhook)
}

func naoAssinado() error {
	return &trilha.Problem{Status: http.StatusUnauthorized, Title: "invalid signature",
		Extra: map[string]any{"code": CodigoNaoAssinado}}
}
`

const billingDunning = `package cobranca

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/mail"
	"github.com/emersonjoe/trilha/task"
)

// TarefaLembrete is the task that tells a customer a charge failed.
const TarefaLembrete = "cobranca.lembrete"

// Mailer is what sends the reminders. A package variable on purpose: a test
// puts a mail.Outbox in its place and asserts on what left.
var Mailer = mail.New(mail.FromEnv())

// Marca is the name in the messages.
const Marca = "{{.T.mail_brand}}"

// Lembrar queues the reminder for the failure the subscription just counted.
// The key is the subscription and the attempt, so the same failure delivered
// twice is one reminder and three failures are three.
func (b *Billing) Lembrar(c *trilha.Ctx, a Assinatura) error {
	_, err := b.tarefas.Run(c, TarefaLembrete, a.ID+":"+strconv.Itoa(a.Tentativas))
	return err
}

// lembrete is the task: one e-mail per failed charge, and the last one says
// the subscription is over. It reads the subscription when it runs and not
// when it was queued, so the message never contradicts the screen.
func (b *Billing) lembrete(ctx context.Context, p *task.Progress) error {
	id, n, ok := strings.Cut(p.Key, ":")
	if !ok {
		return fmt.Errorf("cobranca: reminder key %q is not subscription:attempt", p.Key)
	}
	tentativa, err := strconv.Atoi(n)
	if err != nil {
		return fmt.Errorf("cobranca: reminder key %q: %w", p.Key, err)
	}
	a, err := b.Store.Assinatura(ctx, id)
	if err != nil {
		return err
	}
	assunto := fmt.Sprintf("{{.T.billing_mail_failed_subject}} (%d/%d)", tentativa, MaxTentativas)
	texto := "{{.T.billing_mail_failed_body}}"
	if tentativa >= MaxTentativas {
		assunto, texto = "{{.T.billing_mail_canceled_subject}}", "{{.T.billing_mail_canceled_body}}"
	}
	p.Step("mail", 1, 1)
	return Mailer.Send(ctx, mail.Message{
		To:      []string{a.Email},
		Subject: assunto + " — " + Marca,
		Body:    mail.Layout(Marca, h.P(h.Text(texto))),
	})
}
`

const billingDomainTest = `package cobranca_test

import (
	"errors"
	"testing"

	"{{.Module}}/internal/cobranca"
	"{{.Module}}/internal/cobranca/cobrancatest"
)

// A máquina é fechada: o que a tabela permite passa, o resto é recusado — e
// uma assinatura cancelada não volta porque um evento velho chegou atrasado.
// Depois, o contrato do store inteiro sobre a memória.
func TestBillingStates(t *testing.T) {
	casos := []struct {
		de, para cobranca.Estado
		pode     bool
	}{
		{cobranca.Trial, cobranca.Ativa, true}, {cobranca.Trial, cobranca.Atrasada, true},
		{cobranca.Trial, cobranca.Cancelada, true}, {cobranca.Ativa, cobranca.Atrasada, true},
		{cobranca.Ativa, cobranca.Cancelada, true}, {cobranca.Ativa, cobranca.Trial, false},
		{cobranca.Atrasada, cobranca.Ativa, true}, {cobranca.Atrasada, cobranca.Cancelada, true},
		{cobranca.Atrasada, cobranca.Trial, false}, {cobranca.Cancelada, cobranca.Ativa, false},
		{cobranca.Cancelada, cobranca.Atrasada, false}, {cobranca.Cancelada, cobranca.Trial, false},
	}
	for _, c := range casos {
		err := cobranca.Mover(c.de, c.para)
		if (err == nil) != c.pode {
			t.Errorf("%s → %s: err = %v, pode = %v", c.de, c.para, err, c.pode)
		}
		if err != nil && !errors.Is(err, cobranca.ErrTransicao) {
			t.Errorf("%s → %s: o erro não é ErrTransicao: %v", c.de, c.para, err)
		}
	}
	if len(cobranca.Transicoes) != len(cobranca.Estados()) {
		t.Fatalf("a tabela tem %d estados e a lista %d", len(cobranca.Transicoes), len(cobranca.Estados()))
	}
	cobrancatest.Contrato(t, cobranca.NovoStore())
}
`

// billingContract is what every Store promises, in a package of its own so
// the testing package never reaches the binary. The package's test runs it on
// memory and the project's billing_test.go on whatever the app wired — the
// database, once the store recipe is there — so the two cannot drift apart.
const billingContract = `// Package cobrancatest is the contract of cobranca.Store, for a test to run
// against any store: the one in memory and the one in the database answer
// the same, or one of them is wrong.
package cobrancatest

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"{{.Module}}/internal/cobranca"
)

// Contrato runs the whole store: the cycle of a subscription, the invoices,
// the event seen once, the plan in use. Every id carries a prefix of this
// run, so a database with other rows neither disturbs it nor is disturbed.
func Contrato(t *testing.T, s cobranca.Store) {
	t.Helper()
	ctx := context.Background()
	px := strconv.FormatInt(time.Now().UnixNano(), 36) + "-"
	nossas := func(fs []cobranca.Fatura) []cobranca.Fatura {
		var out []cobranca.Fatura
		for _, f := range fs {
			if strings.HasPrefix(f.ID, px) {
				out = append(out, f)
			}
		}
		return out
	}
	agora := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	id := px + "a-1"

	// O ciclo: falha, paga, falha até cancelar.
	if _, err := s.Assinar(ctx, id, "cliente@example.com", px+"plano-1", agora); err != nil {
		t.Fatal(err)
	}
	m, err := s.Falhou(ctx, id, px+"f-1", 4900, "brl", agora)
	if err != nil || m.De != cobranca.Trial || m.Assinatura.Estado != cobranca.Atrasada || m.Assinatura.Tentativas != 1 {
		t.Fatalf("primeira falha: %+v %v", m, err)
	}
	if m, err = s.Pagou(ctx, id, px+"f-1", 4900, "brl", agora); err != nil || m.Assinatura.Estado != cobranca.Ativa || m.Assinatura.Tentativas != 0 {
		t.Fatalf("pagou: %+v %v", m, err)
	}
	for i := 1; i <= cobranca.MaxTentativas; i++ {
		if m, err = s.Falhou(ctx, id, px+"f-2", 4900, "brl", agora); err != nil {
			t.Fatal(err)
		}
	}
	if m.Assinatura.Estado != cobranca.Cancelada {
		t.Fatalf("depois de %d falhas: %+v", cobranca.MaxTentativas, m)
	}
	// Recusada, a mudança não deixa rastro: nem o contador nem a fatura.
	if _, err := s.Pagou(ctx, id, px+"f-3", 4900, "brl", agora); !errors.Is(err, cobranca.ErrTransicao) {
		t.Fatalf("uma cancelada voltou: %v", err)
	}
	a, err := s.Assinatura(ctx, id)
	if err != nil || a.Estado != cobranca.Cancelada || a.Tentativas != cobranca.MaxTentativas || !a.Criada.Equal(agora) {
		t.Fatalf("lida de volta: %+v %v", a, err)
	}
	if _, err := s.Assinatura(ctx, px+"nenhuma"); !errors.Is(err, cobranca.ErrNaoExiste) {
		t.Fatalf("uma que não existe: %v", err)
	}
	if _, err := s.Cancelar(ctx, px+"nenhuma", agora); !errors.Is(err, cobranca.ErrNaoExiste) {
		t.Fatalf("mudar uma que não existe: %v", err)
	}
	todas, err := s.Faturas(ctx)
	f := nossas(todas)
	if err != nil || len(f) != 2 || f[0].Moeda != "BRL" {
		t.Fatalf("faturas = %+v %v", f, err)
	}
	for _, x := range f {
		if x.ID == px+"f-1" && !x.Paga {
			t.Fatal("a fatura paga depois de falhar não ficou paga")
		}
	}

	// A mais nova primeiro, e o filtro por estado.
	if _, err := s.Assinar(ctx, px+"a-2", "outra@example.com", px+"plano-1", agora.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var ordem []string
	lista, err := s.Assinaturas(ctx, "")
	for _, x := range lista {
		if strings.HasPrefix(x.ID, px) {
			ordem = append(ordem, x.ID)
		}
	}
	if err != nil || strings.Join(ordem, ",") != px+"a-2,"+id {
		t.Fatalf("ordem = %v %v", ordem, err)
	}
	canceladas, err := s.Assinaturas(ctx, cobranca.Cancelada)
	for _, x := range canceladas {
		if x.Estado != cobranca.Cancelada || x.ID == px+"a-2" {
			t.Fatalf("o filtro deixou passar %+v (%v)", x, err)
		}
	}

	// O mesmo evento é visto uma vez; esquecido, é novo outra vez.
	ev := px + "ev-1"
	for i, quero := range []bool{false, true} {
		if visto, err := s.Visto(ctx, ev, agora); err != nil || visto != quero {
			t.Fatalf("visto #%d = %v %v", i+1, visto, err)
		}
	}
	if err := s.Esquecer(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if visto, err := s.Visto(ctx, ev, agora); err != nil || visto {
		t.Fatalf("esquecido e ainda visto: %v %v", visto, err)
	}

	// Um plano com assinatura viva não se apaga; salvar de novo atualiza.
	p, err := s.SalvarPlano(ctx, cobranca.Plano{Nome: px + "Pro", Centavos: 4900, Moeda: "brl", Intervalo: "month"})
	if err != nil || p.Moeda != "BRL" || p.ID == "" {
		t.Fatalf("plano = %+v %v", p, err)
	}
	p.Centavos = 5900
	if _, err := s.SalvarPlano(ctx, p); err != nil {
		t.Fatal(err)
	}
	planos, err := s.Planos(ctx)
	achados := 0
	for _, x := range planos {
		if x.ID == p.ID {
			achados++
			if x.Centavos != 5900 {
				t.Fatalf("o plano não foi atualizado: %+v", x)
			}
		}
	}
	if err != nil || achados != 1 {
		t.Fatalf("o plano aparece %d vezes (%v)", achados, err)
	}
	if _, err := s.Assinar(ctx, px+"a-3", "c@example.com", p.ID, agora); err != nil {
		t.Fatal(err)
	}
	if err := s.ApagarPlano(ctx, p.ID); !errors.Is(err, cobranca.ErrPlanoEmUso) {
		t.Fatalf("apagou um plano em uso: %v", err)
	}
	if _, err := s.Cancelar(ctx, px+"a-3", agora); err != nil {
		t.Fatal(err)
	}
	if err := s.ApagarPlano(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ApagarPlano(ctx, p.ID); !errors.Is(err, cobranca.ErrNaoExiste) {
		t.Fatalf("apagou duas vezes: %v", err)
	}
}
`

const billingMigration = `-- As tabelas da cobrança, na convenção do ` + "`" + `trilha add store` + "`" + `: aplicadas uma vez, em
-- ordem de nome, e conferidas depois. Sem banco, o internal/cobranca guarda o
-- mesmo formato em memória; com o store, sql.go lê e grava aqui.
--
-- Dinheiro é inteiro em centavos, com a moeda ao lado: ponto flutuante não
-- soma dinheiro.

CREATE TABLE IF NOT EXISTS billing_plans (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	cents      BIGINT NOT NULL CHECK (cents >= 0),
	currency   CHAR(3) NOT NULL,
	period     TEXT NOT NULL CHECK (period IN ('month', 'year'))
);

-- plan_id é o id do plano no provedor, sem chave estrangeira: o evento
-- subscription.created chega com o plano de lá, que pode não estar (ainda) em
-- billing_plans, e um plano apagado não pode levar consigo o histórico.
CREATE TABLE IF NOT EXISTS billing_subscriptions (
	id          TEXT PRIMARY KEY,
	email       TEXT NOT NULL,
	plan_id     TEXT NOT NULL,
	state       TEXT NOT NULL CHECK (state IN ('trial', 'active', 'past_due', 'canceled')),
	attempts    INTEGER NOT NULL DEFAULT 0,
	created_at  TIMESTAMP NOT NULL,
	updated_at  TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS billing_subscriptions_plan ON billing_subscriptions (plan_id);

CREATE TABLE IF NOT EXISTS billing_invoices (
	id               TEXT PRIMARY KEY,
	subscription_id  TEXT NOT NULL REFERENCES billing_subscriptions (id),
	cents            BIGINT NOT NULL,
	currency         CHAR(3) NOT NULL,
	paid             BOOLEAN NOT NULL DEFAULT FALSE,
	issued_at        TIMESTAMP NOT NULL
);

-- Um evento do provedor é visto uma vez: a chave primária é o id que ele
-- manda, e é ela que transforma "pelo menos uma vez" em "exatamente uma".
CREATE TABLE IF NOT EXISTS billing_events (
	id           TEXT PRIMARY KEY,
	received_at  TIMESTAMP NOT NULL
);
`

const billingRoute = `// Package billing is the address the payment provider calls.
//
// It is an API — kind.go says so — which is what keeps CSRF out of the way:
// the client here is the provider, not a form of this site, and the HMAC over
// timestamp.body stands in for the token. The address is fixed because it is
// typed into the provider's dashboard.
package billing

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/cobranca"
)

// POST receives one event. Everything — signature, window, seen once, the
// change and the trail — is cobranca.Receber, so this file stays the address
// and nothing else.
func POST(c *trilha.Ctx) error {
	return trilha.Use[*cobranca.Billing](c).Receber(c)
}
`

const billingKind = `package billing

import "github.com/emersonjoe/trilha"

// Kind says this is an API: the client is the provider, which cannot carry a
// token of this site, and errors go out as JSON because nobody here reads a
// page.
var Kind = trilha.KindAPI
`

const billingMiddleware = `package billing

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/cobranca"
	"{{.Module}}/internal/sessao"
)

// exige is the rule: reading money needs the level "ler" of the billing
// module — billing:reader, billing:admin or admin. Everybody else is refused,
// including somebody signed in with another role.
var exige = sessao.Flow.RequirePolicy(cobranca.Politica, "billing", "ler")

// Middleware guards this folder and everything below it.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

// billingAdminMiddleware is the stricter door of a subfolder: plans are
// edited and invoices exported only by whoever administers billing.
func billingAdminMiddleware(pkg string) string {
	return `package ` + pkg + `

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/cobranca"
	"{{.Module}}/internal/sessao"
)

// exige is the stricter rule of this folder: "administrar" of the billing
// module, which a billing:reader does not have.
var exige = sessao.Flow.RequirePolicy(cobranca.Politica, "billing", "administrar")

// Middleware guards this folder on top of the one above it.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`
}

const billingPage = `// Package billing is the screen of the subscriptions: who is on which plan,
// in which state, and how many charges failed in a row.
package billing

import (
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/cobranca"
)

// consulta is what this screen reads from the address.
type consulta struct {
	trilha.ListParams
	Estado string ` + "`" + `form:"estado"` + "`" + `
}

var colunas = ui.Columns[cobranca.Assinatura]{
	{Key: "email", Label: "{{.T.billing_customer}}", Cell: func(a cobranca.Assinatura) h.Node { return h.Text(a.Email) }},
	{Key: "plano", Label: "{{.T.billing_plan}}", Cell: func(a cobranca.Assinatura) h.Node { return h.Text(a.Plano) }},
	{Key: "estado", Label: "{{.T.billing_state}}", Cell: func(a cobranca.Assinatura) h.Node {
		return ui.Badge(ui.Outline(), h.Text(Rotulo(a.Estado)))
	}},
	{Key: "tentativas", Label: "{{.T.billing_attempts}}", Num: true, Cell: func(a cobranca.Assinatura) h.Node {
		return h.Text(strconv.Itoa(a.Tentativas))
	}},
}

// Page answers GET {{.URL}}billing: the whole screen, or only the table when
// the kit asks for the fragment.
func Page(c *trilha.Ctx) (h.Node, error) {
	var q consulta
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	estado := valido(q.Estado)
	todas, err := trilha.Use[*cobranca.Billing](c).Store.Assinaturas(c.Context(), estado)
	if err != nil {
		return nil, err
	}
	if q.Q != "" {
		filtradas := todas[:0:0]
		for _, a := range todas {
			if strings.Contains(strings.ToLower(a.Email), strings.ToLower(q.Q)) {
				filtradas = append(filtradas, a)
			}
		}
		todas = filtradas
	}
	total := len(todas)
	fim := q.Offset() + q.Limit()
	if fim > total {
		fim = total
	}
	pagina := []cobranca.Assinatura{}
	if q.Offset() < total {
		pagina = todas[q.Offset():fim]
	}
	tabela := ui.DataTable(c, colunas, pagina, ui.ListState{
		Params: q.ListParams, Total: total, ID: "assinaturas",
		Search: "{{.T.billing_search}}", Filters: filtro(string(estado)),
		Caption: "{{.T.billing_title}}", Cards: true,
		Empty: ui.Muted(h.Text("{{.T.billing_empty}}")),
	})
	if c.Fragment() == "assinaturas" {
		return tabela, nil
	}
	c.SetTitle("{{.T.billing_title}}")
	return ui.Stack(
		ui.PageHeader("{{.T.billing_title}}",
			ui.ButtonLink("{{.URL}}billing/planos", ui.Outline(), h.Text("{{.T.billing_plans}}")),
			ui.ButtonLink("{{.URL}}billing/faturas", ui.Outline(), h.Text("{{.T.billing_invoices}}"))),
		ui.Muted(h.Text("{{.T.billing_desc}}")),
		tabela,
	), nil
}

// Rotulo is what a state is called on the screen.
func Rotulo(e cobranca.Estado) string {
	switch e {
	case cobranca.Trial:
		return "{{.T.billing_state_trial}}"
	case cobranca.Ativa:
		return "{{.T.billing_state_active}}"
	case cobranca.Atrasada:
		return "{{.T.billing_state_past_due}}"
	case cobranca.Cancelada:
		return "{{.T.billing_state_canceled}}"
	}
	return string(e)
}

// valido lets through only a state the machine has: what comes from the
// address reaches the store only when it is one of the values offered.
func valido(s string) cobranca.Estado {
	for _, e := range cobranca.Estados() {
		if string(e) == s {
			return e
		}
	}
	return ""
}

func filtro(estado string) h.Node {
	opcoes := []ui.Option{ {Value: "", Label: "{{.T.billing_all}}"} }
	for _, e := range cobranca.Estados() {
		opcoes = append(opcoes, ui.Option{Value: string(e), Label: Rotulo(e)})
	}
	return ui.Select(h.Name("estado"), h.Aria("label", "{{.T.billing_state}}"), ui.SelectOptions(opcoes, estado))
}
`

const billingPlansPage = `// Package planos is where the plans are created and removed. Only whoever
// administers billing gets here: middleware.go asks for it.
package planos

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/cobranca"
)

// Page renders GET {{.URL}}billing/planos.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.billing_plans}}")
	return tela(c, cobranca.Plano{Moeda: "BRL", Intervalo: "month"}, nil)
}

// POST creates a plan or deletes one. Both are written to the audit trail:
// a price is money, and money that changed without a name next to it is the
// question an audit exists to answer.
func POST(c *trilha.Ctx) error {
	store := trilha.Use[*cobranca.Billing](c).Store
	if c.Form("acao") == "apagar" {
		id := c.Form("id")
		switch err := store.ApagarPlano(c.Context(), id); {
		case errors.Is(err, cobranca.ErrPlanoEmUso):
			return trilha.Errorf(http.StatusConflict, "%s", "{{.T.billing_plan_in_use}}")
		case errors.Is(err, cobranca.ErrNaoExiste):
			return trilha.ErrNotFound
		case err != nil:
			return err
		}
		c.Audit("billing.plano_apagado", id)
		return c.Redirect("{{.URL}}billing/planos")
	}
	var p cobranca.Plano
	if err := c.Bind(&p); err != nil {
		var fe trilha.FieldErrors
		if errors.As(err, &fe) {
			pagina, err := tela(c, p, fe)
			if err != nil {
				return err
			}
			return c.Render(http.StatusUnprocessableEntity, pagina)
		}
		return err
	}
	p, err := store.SalvarPlano(c.Context(), p)
	if err != nil {
		return err
	}
	c.Audit("billing.plano_salvo", p.ID, trilha.Fields{"nome": p.Nome, "centavos": p.Centavos, "moeda": p.Moeda})
	return c.Redirect("{{.URL}}billing/planos")
}

func tela(c *trilha.Ctx, p cobranca.Plano, errs trilha.FieldErrors) (h.Node, error) {
	planos, err := trilha.Use[*cobranca.Billing](c).Store.Planos(c.Context())
	if err != nil {
		return nil, err
	}
	var lista h.Node = ui.Empty(ui.EmptyOpts{Icon: "plus", Title: "{{.T.billing_plans_empty}}"})
	if len(planos) > 0 {
		linhas := make([]h.Node, 0, len(planos))
		for _, pl := range planos {
			linhas = append(linhas, h.Tr(
				h.Td(h.Text(pl.Nome)),
				h.Td(h.Text(pl.Moeda+" "), ui.Number(c, float64(pl.Centavos)/100, ui.Decimals(2))),
				h.Td(h.Text(pl.Intervalo)),
				h.Td(h.Form(h.Method("post"), h.Action("{{.URL}}billing/planos"),
					trilha.CSRFInput(c),
					h.Input(h.Type("hidden"), h.Name("acao"), h.Value("apagar")),
					h.Input(h.Type("hidden"), h.Name("id"), h.Value(pl.ID)),
					ui.Submit(ui.Outline(), h.Text("{{.T.billing_plan_delete}}")))),
			))
		}
		lista = ui.Table(
			h.Thead(h.Tr(h.Th(h.Text("{{.T.billing_plan_name}}")), h.Th(h.Text("{{.T.billing_amount}}")),
				h.Th(h.Text("{{.T.billing_plan_interval}}")), h.Th())),
			h.Tbody(linhas...),
		)
	}
	intervalos := []ui.Option{ {Value: "month", Label: "{{.T.billing_plan_month}}"}, {Value: "year", Label: "{{.T.billing_plan_year}}"} }
	return ui.Stack(
		ui.PageHeader("{{.T.billing_plans}}", ui.ButtonLink("{{.URL}}billing", ui.Outline(), h.Text("{{.T.billing_title}}"))),
		lista,
		h.Form(h.Method("post"), h.Action("{{.URL}}billing/planos"), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			ui.Field("nome", "{{.T.billing_plan_name}}",
				ui.Input(h.ID("nome"), h.Name("nome"), h.Value(p.Nome), h.Required(), ui.InvalidIf(errs, "nome")),
				ui.Errors(errs, "nome")),
			ui.Field("centavos", "{{.T.billing_plan_cents}}",
				ui.Input(h.ID("centavos"), h.Name("centavos"), h.Type("number"), h.Attr("min", "0"),
					h.Value(strconv.FormatInt(p.Centavos, 10)), ui.InvalidIf(errs, "centavos")), ui.Errors(errs, "centavos")),
			ui.Field("moeda", "{{.T.billing_plan_currency}}",
				ui.Input(h.ID("moeda"), h.Name("moeda"), h.Value(p.Moeda), h.Attr("maxlength", "3"),
					ui.InvalidIf(errs, "moeda")), ui.Errors(errs, "moeda")),
			ui.Field("intervalo", "{{.T.billing_plan_interval}}",
				ui.Select(h.ID("intervalo"), h.Name("intervalo"), ui.SelectOptions(intervalos, p.Intervalo),
					ui.InvalidIf(errs, "intervalo")),
				ui.Errors(errs, "intervalo")),
			h.Div(ui.Submit(h.Text("{{.T.billing_plan_save}}"))),
		),
	), nil
}
`

const billingInvoicesPage = `// Package faturas is the list of invoices. Reading it is billing:reader;
// the export is billing:admin, and the button only appears for who has it.
package faturas

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/cobranca"
	"{{.Module}}/internal/sessao"
)

// Page renders GET {{.URL}}billing/faturas.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.billing_invoices}}")
	faturas, err := trilha.Use[*cobranca.Billing](c).Store.Faturas(c.Context())
	if err != nil {
		return nil, err
	}
	var acoes []h.Node
	// Hiding the button is cosmetic; the rule that holds is csv/middleware.go.
	if cobranca.Pode(sessao.Atual(c), "administrar") {
		acoes = append(acoes, ui.ButtonLink("{{.URL}}billing/faturas/csv", ui.Outline(), ui.Icon("download"), h.Text("{{.T.billing_csv}}")))
	}
	var lista h.Node = ui.Empty(ui.EmptyOpts{Icon: "info", Title: "{{.T.billing_invoices_empty}}"})
	if len(faturas) > 0 {
		linhas := make([]h.Node, 0, len(faturas))
		for _, f := range faturas {
			estado := "{{.T.billing_open}}"
			if f.Paga {
				estado = "{{.T.billing_paid}}"
			}
			linhas = append(linhas, h.Tr(
				h.Td(h.Text(f.ID)),
				h.Td(h.Text(f.Assinatura)),
				h.Td(h.Text(f.Moeda+" "), ui.Number(c, float64(f.Centavos)/100, ui.Decimals(2))),
				h.Td(ui.Badge(ui.Outline(), h.Text(estado))),
				h.Td(ui.Date(c, f.Emitida)),
			))
		}
		lista = ui.Table(
			h.Thead(h.Tr(h.Th(h.Text("{{.T.billing_invoice}}")), h.Th(h.Text("{{.T.billing_subscription}}")),
				h.Th(h.Text("{{.T.billing_amount}}")), h.Th(h.Text("{{.T.billing_state}}")),
				h.Th(h.Text("{{.T.billing_issued}}")))),
			h.Tbody(linhas...),
		)
	}
	return ui.Stack(
		ui.PageHeader("{{.T.billing_invoices}}", append(acoes, ui.ButtonLink("{{.URL}}billing", ui.Outline(), h.Text("{{.T.billing_title}}")))...),
		lista,
	), nil
}
`

const billingCSV = `// Package csv is the invoice export. Its middleware.go asks for
// billing:admin: an export is every invoice at once, and it leaves the
// application in somebody's downloads folder.
package csv

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/cobranca"
)

// GET streams the invoices as CSV, with the BOM and the locale's decimals, and
// writes the export to the audit trail.
func GET(c *trilha.Ctx) error {
	faturas, err := trilha.Use[*cobranca.Billing](c).Store.Faturas(c.Context())
	if err != nil {
		return err
	}
	linhas := make([]cobranca.LinhaCSV, 0, len(faturas))
	for _, f := range faturas {
		linhas = append(linhas, cobranca.LinhaCSV{Fatura: f.ID, Assinatura: f.Assinatura,
			Valor: float64(f.Centavos) / 100, Moeda: f.Moeda, Paga: f.Paga, Emitida: f.Emitida})
	}
	c.Audit("billing.csv_exportado", "faturas", trilha.Fields{"linhas": len(linhas)})
	return c.CSV("faturas.csv", linhas)
}
`

// billingTest is the recipe proving itself in the project that received it:
// the signed endpoint with its three refusals and its idempotency, the three
// reminders of the dunning cycle through a mail.Outbox, and the export that
// only billing:admin gets. The names are the plan's (spec 162), so the e2e
// test can require each of them.
const billingTest = `package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/mail"
	"github.com/emersonjoe/trilha/webhook"

	"{{.Module}}/internal/cobranca"
	"{{.Module}}/internal/cobranca/cobrancatest"
	"{{.Module}}/internal/conexoes"
	"{{.Module}}/internal/usuarios"
)

const segredoDoProvedor = "um-segredo-do-provedor-de-teste"

// bg is the context of what a test does to the store directly.
var bg = context.Background()

// trilhaDeTeste is the audit trail of one test, kept in memory.
type trilhaDeTeste struct {
	mu  sync.Mutex
	rec []trilha.AuditRecord
}

func (tr *trilhaDeTeste) Write(r trilha.AuditRecord) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.rec = append(tr.rec, r)
	return nil
}

func (tr *trilhaDeTeste) acoes() string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	var out []string
	for _, r := range tr.rec {
		out = append(out, r.Action)
	}
	return strings.Join(out, " ")
}

// appDeCobranca sobe o app com a conexão do provedor e uma trilha só deste
// teste.
func appDeCobranca(t *testing.T) (*trilha.App, *trilhaDeTeste) {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	tr := &trilhaDeTeste{}
	a.Config().Audit = tr
	// The list of connections is a package variable, so the one this test
	// creates is removed when it ends: the next test starts from its own.
	conn, err := conexoes.Conexoes.Save(nil, trilha.Connection{
		Kind: "api", Name: cobranca.ConexaoWebhook, URL: "https://provedor.example.com",
		Auth: "header", Header: webhook.HeaderSignature, Secret: trilha.Secret(segredoDoProvedor),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conexoes.Conexoes.Delete(nil, conn.ID) })
	return a, tr
}

// evento manda um evento assinado sobre timestamp.body, com a hora que o
// teste escolher.
func evento(c *trilha.TestClient, corpo string, quando time.Time, segredo string) *trilha.TestResponse {
	ts := strconv.FormatInt(quando.Unix(), 10)
	return c.Request(http.MethodPost, "/webhooks/billing",
		trilha.WithBody("application/json", corpo),
		trilha.WithHeader(webhook.HeaderTimestamp, ts),
		trilha.WithHeader(webhook.HeaderSignature, webhook.Sign(segredo, ts, []byte(corpo))))
}

func falhou(evID, fatura string) string {
	return ` + "`" + `{"id":"` + "`" + ` + evID + ` + "`" + `","type":"invoice.payment_failed","subscription":"a-1","invoice":"` + "`" + ` + fatura + ` + "`" + `","amount":4900,"currency":"brl"}` + "`" + `
}

// Assinado e dentro da janela, o evento muda a assinatura e deixa rastro na
// trilha — nenhuma escrita de dinheiro sem nome ao lado.
func TestBillingWebhookValid(t *testing.T) {
	a, tr := appDeCobranca(t)
	c := trilha.NewTestClient(t, a)
	b := trilha.Use[*cobranca.Billing](a)

	evento(c, ` + "`" + `{"id":"ev-1","type":"subscription.created","subscription":"a-1","customer_email":"cliente@example.com","plan":"pro"}` + "`" + `,
		time.Now(), segredoDoProvedor).WantStatus(http.StatusOK)
	evento(c, ` + "`" + `{"id":"ev-2","type":"invoice.paid","subscription":"a-1","invoice":"f-1","amount":4900,"currency":"brl"}` + "`" + `,
		time.Now(), segredoDoProvedor).WantStatus(http.StatusOK).WantContains("ok")

	as, err := b.Store.Assinatura(bg, "a-1")
	if err != nil || as.Estado != cobranca.Ativa {
		t.Fatalf("assinatura = %+v %v", as, err)
	}
	if f, _ := b.Store.Faturas(bg); len(f) != 1 || !f[0].Paga || f[0].Centavos != 4900 {
		t.Fatalf("faturas = %+v", f)
	}
	for _, quero := range []string{"billing.assinatura_criada", "billing.invoice_paid"} {
		if !strings.Contains(tr.acoes(), quero) {
			t.Fatalf("a trilha não registrou %s: %s", quero, tr.acoes())
		}
	}
}

// Sem assinatura, com a assinatura errada ou com o corpo alterado: o mesmo
// 401, o mesmo código, e nada muda.
func TestBillingWebhookUnsigned(t *testing.T) {
	a, tr := appDeCobranca(t)
	c := trilha.NewTestClient(t, a)
	b := trilha.Use[*cobranca.Billing](a)
	b.Store.Assinar(bg, "a-1", "cliente@example.com", "pro", time.Now())

	corpo := falhou("ev-1", "f-1")
	c.Request(http.MethodPost, "/webhooks/billing", trilha.WithBody("application/json", corpo)).
		WantStatus(http.StatusUnauthorized).WantContains(cobranca.CodigoNaoAssinado)
	evento(c, corpo, time.Now(), "outro-segredo").
		WantStatus(http.StatusUnauthorized).WantContains(cobranca.CodigoNaoAssinado)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	c.Request(http.MethodPost, "/webhooks/billing",
		trilha.WithBody("application/json", corpo+" "),
		trilha.WithHeader(webhook.HeaderTimestamp, ts),
		trilha.WithHeader(webhook.HeaderSignature, webhook.Sign(segredoDoProvedor, ts, []byte(corpo)))).
		WantStatus(http.StatusUnauthorized).WantContains(cobranca.CodigoNaoAssinado)

	if as, _ := b.Store.Assinatura(bg, "a-1"); as.Estado != cobranca.Trial || as.Tentativas != 0 {
		t.Fatalf("um evento não assinado mudou a assinatura: %+v", as)
	}
	if tr.acoes() != "" {
		t.Fatalf("a recusa deixou rastro de mudança: %s", tr.acoes())
	}
	// E a recusa não queima o id: o mesmo evento, assinado, ainda vale.
	evento(c, corpo, time.Now(), segredoDoProvedor).WantStatus(http.StatusOK)
	if as, _ := b.Store.Assinatura(bg, "a-1"); as.Estado != cobranca.Atrasada {
		t.Fatalf("o evento assinado depois da recusa não valeu: %+v", as)
	}
}

// Assinado mas velho: fora da janela de cinco minutos é recusado, e é isso
// que impede quem capturou uma requisição de repeti-la depois.
func TestBillingWebhookExpired(t *testing.T) {
	a, _ := appDeCobranca(t)
	c := trilha.NewTestClient(t, a)
	b := trilha.Use[*cobranca.Billing](a)
	b.Store.Assinar(bg, "a-1", "cliente@example.com", "pro", time.Now())

	evento(c, falhou("ev-1", "f-1"), time.Now().Add(-6*time.Minute), segredoDoProvedor).
		WantStatus(http.StatusUnauthorized).WantContains(cobranca.CodigoNaoAssinado)
	evento(c, falhou("ev-2", "f-1"), time.Now().Add(6*time.Minute), segredoDoProvedor).
		WantStatus(http.StatusUnauthorized)
	if as, _ := b.Store.Assinatura(bg, "a-1"); as.Estado != cobranca.Trial {
		t.Fatalf("um evento fora da janela mudou a assinatura: %+v", as)
	}
}

// O provedor reenvia; o app aplica uma vez.
func TestBillingIdempotentEvent(t *testing.T) {
	a, tr := appDeCobranca(t)
	c := trilha.NewTestClient(t, a)
	b := trilha.Use[*cobranca.Billing](a)
	b.Store.Assinar(bg, "a-1", "cliente@example.com", "pro", time.Now())
	cobranca.Mailer = mail.New(mail.Options{From: "cobranca@example.com", Transport: &mail.Outbox{}})

	evento(c, falhou("ev-1", "f-1"), time.Now(), segredoDoProvedor).WantStatus(http.StatusOK)
	evento(c, falhou("ev-1", "f-1"), time.Now(), segredoDoProvedor).WantStatus(http.StatusOK).WantContains("duplicate")
	if as, _ := b.Store.Assinatura(bg, "a-1"); as.Tentativas != 1 {
		t.Fatalf("o reenvio contou de novo: %+v", as)
	}
	if n := strings.Count(tr.acoes(), "billing.invoice_payment_failed"); n != 1 {
		t.Fatalf("a trilha registrou %d vezes: %s", n, tr.acoes())
	}
}

// Três falhas seguidas: três lembretes por e-mail, e a terceira cancela.
func TestBillingDunningCycle(t *testing.T) {
	a, tr := appDeCobranca(t)
	c := trilha.NewTestClient(t, a)
	b := trilha.Use[*cobranca.Billing](a)
	b.Store.Assinar(bg, "a-1", "cliente@example.com", "pro", time.Now())
	caixa := &mail.Outbox{}
	antes := cobranca.Mailer
	cobranca.Mailer = mail.New(mail.Options{From: "cobranca@example.com", Transport: caixa})
	t.Cleanup(func() { cobranca.Mailer = antes })

	for i := 1; i <= cobranca.MaxTentativas; i++ {
		evento(c, falhou("ev-"+strconv.Itoa(i), "f-1"), time.Now(), segredoDoProvedor).WantStatus(http.StatusOK)
	}
	// Os lembretes saem numa tarefa, fora da requisição: a caixa é consultada
	// até eles chegarem.
	limite := time.Now().Add(5 * time.Second)
	for len(caixa.Messages()) < cobranca.MaxTentativas && time.Now().Before(limite) {
		time.Sleep(10 * time.Millisecond)
	}
	msgs := caixa.Messages()
	if len(msgs) != cobranca.MaxTentativas {
		t.Fatalf("%d lembretes, esperava %d", len(msgs), cobranca.MaxTentativas)
	}
	assuntos := ""
	for _, m := range msgs {
		if len(m.To) != 1 || m.To[0] != "cliente@example.com" {
			t.Fatalf("lembrete para %v", m.To)
		}
		assuntos += m.Subject + "\n"
	}
	for _, quero := range []string{"(1/3)", "(2/3)"} {
		if !strings.Contains(assuntos, quero) {
			t.Fatalf("faltou o lembrete %s:\n%s", quero, assuntos)
		}
	}
	if as, _ := b.Store.Assinatura(bg, "a-1"); as.Estado != cobranca.Cancelada {
		t.Fatalf("depois de três falhas: %+v", as)
	}
	if n := strings.Count(tr.acoes(), "billing.invoice_payment_failed"); n != cobranca.MaxTentativas {
		t.Fatalf("a trilha registrou %d falhas: %s", n, tr.acoes())
	}
}

// O CSV é só de quem administra a cobrança: o leitor lê as telas e recebe 403
// na exportação, o anônimo vai para o login.
func TestBillingCSVAdminOnly(t *testing.T) {
	a, tr := appDeCobranca(t)
	b := trilha.Use[*cobranca.Billing](a)
	b.Store.Assinar(bg, "a-1", "cliente@example.com", "pro", time.Now())
	if _, err := b.Store.Pagou(bg, "a-1", "f-1", 4900, "brl", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := trilha.Use[*usuarios.Store](a).Add("u-leitor", "leitor@example.com", "Leitor",
		cobranca.PapelLeitor, "a-password-for-the-reader"); err != nil {
		t.Fatal(err)
	}

	anonimo := trilha.NewTestClient(t, a)
	anonimo.Get("{{.URL}}billing/faturas/csv").WantStatus(http.StatusUnauthorized)

	leitor := trilha.NewTestClient(t, a)
	leitor.PostForm("{{.URL}}entrar", url.Values{"email": {"leitor@example.com"},
		"password": {"a-password-for-the-reader"}}).WantStatus(http.StatusSeeOther)
	leitor.Get("{{.URL}}billing").WantStatus(http.StatusOK).WantContains("cliente@example.com")
	faturas := leitor.Get("{{.URL}}billing/faturas").WantStatus(http.StatusOK).Body.String()
	if strings.Contains(faturas, "billing/faturas/csv") {
		t.Fatal("o botão de exportar apareceu para o leitor")
	}
	leitor.Get("{{.URL}}billing/faturas/csv").WantStatus(http.StatusForbidden)
	leitor.Get("{{.URL}}billing/planos").WantStatus(http.StatusForbidden)

	admin := trilha.NewTestClient(t, a)
	admin.PostForm("{{.URL}}entrar", url.Values{"email": {"admin@example.com"},
		"password": {"a-password-nobody-guesses"}}).WantStatus(http.StatusSeeOther)
	admin.Get("{{.URL}}billing/faturas").WantStatus(http.StatusOK).WantContains("billing/faturas/csv")
	admin.Get("{{.URL}}billing/faturas/csv").WantStatus(http.StatusOK).
		WantHeader("Content-Type", "text/csv; charset=utf-8").WantContains("f-1")
	if !strings.Contains(tr.acoes(), "billing.csv_exportado") {
		t.Fatalf("a exportação não entrou na trilha: %s", tr.acoes())
	}

	// E o plano: validado pelas tags, gravado com nome na trilha. Na volta do
	// 422 o foco cai no primeiro campo errado — o que o leitor de tela anuncia.
	recusado := admin.PostForm("{{.URL}}billing/planos", url.Values{"nome": {""}, "centavos": {"-1"},
		"moeda": {"reais"}, "intervalo": {"week"}}).WantStatus(http.StatusUnprocessableEntity).Snapshot()
	if err := recusado.FocusedOnError("input[name=nome]"); err != nil {
		t.Error(err)
	}
	admin.PostForm("{{.URL}}billing/planos", url.Values{"nome": {"Pro"}, "centavos": {"4900"},
		"moeda": {"brl"}, "intervalo": {"month"}}).WantStatus(http.StatusSeeOther)
	admin.Get("{{.URL}}billing/planos").WantStatus(http.StatusOK).WantContains("Pro", "BRL")
	if !strings.Contains(tr.acoes(), "billing.plano_salvo") {
		t.Fatalf("o plano não entrou na trilha: %s", tr.acoes())
	}
}

// Cada tela da cobrança, lida como o navegador a recebe: todo formulário que
// escreve leva o token, todo script inline leva o nonce, os cookies são
// HttpOnly e o segredo do provedor não aparece em lugar nenhum.
func TestBillingScreensKeepProtections(t *testing.T) {
	a, _ := appDeCobranca(t)
	trilha.Use[*cobranca.Billing](a).Store.Assinar(bg, "a-1", "cliente@example.com", "pro", time.Now())
	admin := trilha.NewTestClient(t, a)
	admin.PostForm("{{.URL}}entrar", url.Values{"email": {"admin@example.com"},
		"password": {"a-password-nobody-guesses"}}).WantStatus(http.StatusSeeOther)
	for _, tela := range []string{"{{.URL}}billing", "{{.URL}}billing/planos", "{{.URL}}billing/faturas"} {
		snap := admin.Get(tela).WantStatus(http.StatusOK).Snapshot()
		for _, err := range []error{snap.HasCSRFToken(), snap.HasCSPNonce(), snap.HasSafeCookies(),
			snap.HasNoSecret(segredoDoProvedor)} {
			if err != nil {
				t.Errorf("%s: %v", tela, err)
			}
		}
	}
}

// O contrato do store sobre o que o app ligou: a memória num projeto sem
// banco, as tabelas de migrations/0100_billing.sql com o store — os mesmos
// passos de internal/cobranca/cobrancatest, contra o store de verdade.
func TestBillingStoreContract(t *testing.T) {
	a, _ := appDeCobranca(t)
	cobrancatest.Contrato(t, trilha.Use[*cobranca.Billing](a).Store)
}
`
