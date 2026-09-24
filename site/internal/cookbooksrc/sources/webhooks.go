package cookbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/webhook"
)

// Receber é o outro lado: o app em Go que consome os webhooks de outra
// aplicação. São três linhas e uma ordem, e a ordem é a parte que se erra.
//
// Responda 2xx antes de fazer o trabalho. Um receptor que processa e só depois
// responde é um receptor que o remetente considera fora do ar — e então
// reenvia, e então o trabalho aconteceu duas vezes.
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

func hookSecret() string   { return "whsec_do_ambiente" }
func visto(id string) bool { return false }
func enfileira(id string)  {}

// WebhookSQL guarda assinaturas e entregas numa tabela, para o histórico
// sobreviver a um reinício — e para a espera de doze horas ser uma linha com
// hora marcada em vez de uma goroutine dormindo, que um deploy esqueceria.
//
// Está aqui e não no módulo pelo mesmo motivo de todo store deste framework:
// um SQL que viesse pronto teria de escolher dialeto de placeholder e ser dono
// de um DDL. Isto é Postgres; SQLite e MySQL são o mesmo arquivo com ? no
// lugar de $1.
type WebhookSQL struct{ DB *sql.DB }

var _ webhook.Store = WebhookSQL{}

// WebhookSchema são as duas tabelas. O índice parcial de deliveries é o que o
// relógio consulta a cada tique: sem ele, uma varredura completa a cada quinze
// segundos numa tabela que só cresce.
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

// SaveSubscription grava a assinatura. O segredo vai como trilha.Secret, que
// implementa driver.Valuer: o que chega na coluna é o selo cifrado, e não
// aparece no log de queries lentas nem no dump.
func (s WebhookSQL) SaveSubscription(ctx context.Context, w webhook.Subscription) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO trilha_webhooks (id, tenant, url, events, secret, label, created, revoked)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET url = EXCLUDED.url, events = EXCLUDED.events,
			label = EXCLUDED.label, revoked = EXCLUDED.revoked`,
		w.ID, w.Tenant, w.URL, listaJSON(w.Events), w.Secret, w.Label, w.Created, w.Revoked)
	return err
}

func (s WebhookSQL) Subscription(ctx context.Context, id string) (webhook.Subscription, error) {
	row := s.DB.QueryRowContext(ctx, hookCols+` WHERE id = $1`, id)
	w, err := scanHook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return webhook.Subscription{}, webhook.ErrNotFound
	}
	return w, err
}

// Subscriptions traz as revogadas também: a tela precisa delas, porque as
// entregas já feitas apontam para ali. Quem para a entrega é o Wants.
func (s WebhookSQL) Subscriptions(ctx context.Context, tenant string) ([]webhook.Subscription, error) {
	rows, err := s.DB.QueryContext(ctx, hookCols+`
		WHERE ($1 = '' OR tenant = $1) ORDER BY created DESC`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []webhook.Subscription
	for rows.Next() {
		w, err := scanHook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s WebhookSQL) SaveDelivery(ctx context.Context, d webhook.Delivery) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO trilha_webhook_deliveries
			(id, webhook_id, tenant, event, payload, state, attempt, next_try,
			 status, err, response, retry_of, created, ended)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET state = EXCLUDED.state, attempt = EXCLUDED.attempt,
			next_try = EXCLUDED.next_try, status = EXCLUDED.status, err = EXCLUDED.err,
			response = EXCLUDED.response, ended = EXCLUDED.ended`,
		d.ID, d.SubscriptionID, d.Tenant, d.Event, d.Payload, string(d.State), d.Attempt,
		nullTime(d.NextTry), d.Status, d.Err, d.Response, d.RetryOf, d.Created, nullTime(d.Ended))
	return err
}

func (s WebhookSQL) Delivery(ctx context.Context, id string) (webhook.Delivery, error) {
	row := s.DB.QueryRowContext(ctx, deliveryCols+` WHERE id = $1`, id)
	d, err := scanDelivery(row)
	if errors.Is(err, sql.ErrNoRows) {
		return webhook.Delivery{}, webhook.ErrNotFound
	}
	return d, err
}

func (s WebhookSQL) Deliveries(ctx context.Context, p webhook.ListParams) ([]webhook.Delivery, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, deliveryCols+`
		WHERE ($1 = '' OR tenant = $1) AND ($2 = '' OR webhook_id = $2)
		  AND ($3 = '' OR event = $3) AND ($4 = '' OR state = $4)
		ORDER BY id DESC LIMIT $5 OFFSET $6`,
		p.Tenant, p.SubscriptionID, p.Event, string(p.State), limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []webhook.Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Due é a consulta do relógio, e a única que roda o tempo todo. O índice
// parcial acima existe para esta linha.
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

const hookCols = `SELECT id, tenant, url, events, secret, label, created, revoked FROM trilha_webhooks`

const deliveryCols = `SELECT id, webhook_id, tenant, event, payload, state, attempt,
	next_try, status, err, response, retry_of, created, ended FROM trilha_webhook_deliveries`

func scanHook(r scanner) (webhook.Subscription, error) {
	var w webhook.Subscription
	var eventos string
	err := r.Scan(&w.ID, &w.Tenant, &w.URL, &eventos, &w.Secret, &w.Label, &w.Created, &w.Revoked)
	w.Events = listaDe(eventos)
	return w, err
}

func scanDelivery(r scanner) (webhook.Delivery, error) {
	var d webhook.Delivery
	var estado string
	var proxima, fim sql.NullTime
	err := r.Scan(&d.ID, &d.SubscriptionID, &d.Tenant, &d.Event, &d.Payload, &estado,
		&d.Attempt, &proxima, &d.Status, &d.Err, &d.Response, &d.RetryOf, &d.Created, &fim)
	d.State = webhook.State(estado)
	d.NextTry, d.Ended = proxima.Time, fim.Time
	return d, err
}

// listaJSON e listaDe guardam a lista de eventos numa coluna de texto. Uma
// tabela de ligação seria mais correta e não paga: são três eventos, lidos
// sempre inteiros e nunca consultados por um deles.
func listaJSON(v []string) string {
	if len(v) == 0 {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func listaDe(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	json.Unmarshal([]byte(s), &out)
	return out
}
