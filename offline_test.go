package trilha

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// renderNode is the HTML of a node, which is how a test reads what a component
// wrote.
func renderNode(t *testing.T, n h.Node) string {
	t.Helper()
	out, err := h.Render(n)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// offlineApp is one POST route that answers what Idempotent said, so a test
// asserts on the decision and not on a store.
func offlineApp(t *testing.T, s AuditSink, ttl time.Duration, store IdempotencyStore) *App {
	t.Helper()
	a := New(Config{
		Env: Prod, Audit: s, Idempotency: store,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	a.Register(Route{Pattern: "/coleta", Kind: KindAPI, Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error {
			replay, err := Idempotent(c, ttl)
			if err != nil {
				return err
			}
			if replay {
				return c.Text(http.StatusOK, "replay")
			}
			return c.Text(http.StatusOK, "first")
		},
	}})
	return a
}

func post(t *testing.T, a *App, form url.Values, header string) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/coleta", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if header != "" {
		req.Header.Set("Idempotency-Key", header)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// #268 — the outbox resends what it could not send, and the second arrival of
// the same submission has to be recognised as the same one. The first call is
// the work; every one after it is the same answer.
func TestIdempotentRecognisesTheResend(t *testing.T) {
	a := offlineApp(t, &sink{}, time.Hour, nil)
	form := url.Values{OfflineKeyField: {"a1b2c3"}, "nota": {"ok"}}
	if got := post(t, a, form, ""); got != "first" {
		t.Fatalf("first submission = %q", got)
	}
	if got := post(t, a, form, ""); got != "replay" {
		t.Fatalf("resend = %q", got)
	}
}

// The key travels in the header when the outbox sends it with fetch and in the
// hidden field when the browser posts the form itself; both are the same
// submission.
func TestIdempotentReadsHeaderAndField(t *testing.T) {
	a := offlineApp(t, &sink{}, time.Hour, nil)
	if got := post(t, a, url.Values{"nota": {"ok"}}, "k-header"); got != "first" {
		t.Fatalf("header first = %q", got)
	}
	if got := post(t, a, url.Values{OfflineKeyField: {"k-header"}}, ""); got != "replay" {
		t.Fatalf("the same key in the field is the same submission: %q", got)
	}
	// The header wins when both are there: it is what the outbox sets.
	if got := post(t, a, url.Values{OfflineKeyField: {"k-header"}}, "k-other"); got != "first" {
		t.Fatalf("header = %q", got)
	}
}

// A form that never asked for this is not made to fail by being sent twice.
func TestIdempotentWithoutAKeyIsNotAReplay(t *testing.T) {
	a := offlineApp(t, &sink{}, time.Hour, nil)
	if got := post(t, a, url.Values{"nota": {"ok"}}, ""); got != "first" {
		t.Fatalf("= %q", got)
	}
	if got := post(t, a, url.Values{"nota": {"ok"}}, ""); got != "first" {
		t.Fatalf("no key, no memory: %q", got)
	}
}

// The window closes: a key older than the ttl is a new submission, because the
// outbox that still holds it has long given up.
func TestIdempotentForgetsAfterTheTTL(t *testing.T) {
	a := offlineApp(t, &sink{}, time.Millisecond, nil)
	form := url.Values{OfflineKeyField: {"expira"}}
	if got := post(t, a, form, ""); got != "first" {
		t.Fatalf("= %q", got)
	}
	time.Sleep(3 * time.Millisecond)
	if got := post(t, a, form, ""); got != "first" {
		t.Fatalf("after the ttl it is a new submission: %q", got)
	}
}

// The trail says the submission arrived twice, with the key masked and the
// client's stamp beside it: that is what somebody reading it later needs to
// explain which write won.
func TestReplayIsAudited(t *testing.T) {
	s := &sink{}
	a := offlineApp(t, s, time.Hour, nil)
	form := url.Values{
		OfflineKeyField:      {"0123456789abcdef"},
		OfflineQueuedAtField: {"2026-09-18T10:00:00Z"},
	}
	post(t, a, form, "")
	post(t, a, form, "")
	if len(s.recs) != 1 {
		t.Fatalf("wrote %d records, want only the replay", len(s.recs))
	}
	r := s.recs[0]
	if r.Action != "offline.replay" {
		t.Fatalf("action = %q", r.Action)
	}
	if strings.Contains(r.Target, "456789ab") || r.Target == "" {
		t.Fatalf("the key goes into the trail masked: %q", r.Target)
	}
	if r.Fields["queued_at"] != "2026-09-18T10:00:00Z" {
		t.Fatalf("queued_at = %v", r.Fields["queued_at"])
	}
}

// The client's stamp is read as RFC 3339 and nothing else; garbage answers
// false instead of a zero time a handler would compare against.
func TestQueuedAtParsesTheClientStamp(t *testing.T) {
	var got time.Time
	var ok bool
	a := New(Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	a.Register(Route{Pattern: "/q", Kind: KindAPI, Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error {
			got, ok = QueuedAt(c)
			return c.Text(http.StatusOK, "ok")
		},
	}})
	send := func(v string) {
		req := httptest.NewRequest("POST", "/q", strings.NewReader(url.Values{OfflineQueuedAtField: {v}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		a.Handler().ServeHTTP(httptest.NewRecorder(), req)
	}
	send("2026-09-18T10:00:00Z")
	if !ok || !got.Equal(time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("QueuedAt = %v, %v", got, ok)
	}
	send("ontem")
	if ok {
		t.Fatal("a stamp that does not parse is no stamp")
	}
	send("")
	if ok {
		t.Fatal("an empty field is no stamp")
	}
}

// The form carries the key from the server and the empty slot the script
// fills, and the attribute that tells the script this form may be queued.
func TestOfflineFormCarriesTheKeyAndTheSlot(t *testing.T) {
	var html string
	a := New(Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	a.Register(Route{Pattern: "/f", Kind: KindAPI, Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error {
			html = renderNode(t, OfflineForm(c))
			return c.Text(http.StatusOK, "ok")
		},
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/f", nil))
	for _, want := range []string{
		`data-trilha-offline=""`,
		`name="` + OfflineKeyField + `"`,
		`name="` + OfflineQueuedAtField + `" value=""`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("OfflineForm is missing %q:\n%s", want, html)
		}
	}
	if renderNode(t, OfflineForm(nil)) == html {
		t.Fatal("every form gets its own key")
	}
}

func TestNewIdempotencyKeyIsRandomHex(t *testing.T) {
	k := NewIdempotencyKey()
	if len(k) != 32 {
		t.Fatalf("key = %q", k)
	}
	if k == NewIdempotencyKey() {
		t.Fatal("two keys came out the same")
	}
	for _, r := range k {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("key is not hex: %q", k)
		}
	}
}

// The app's own store is used when Config names one: an app on two replicas
// has to be able to answer for both.
func TestConfigIdempotencyIsUsed(t *testing.T) {
	store := &countingStore{}
	a := offlineApp(t, &sink{}, time.Hour, store)
	post(t, a, url.Values{OfflineKeyField: {"x"}}, "")
	if store.calls != 1 {
		t.Fatalf("the configured store was called %d times", store.calls)
	}
}

type countingStore struct{ calls int }

func (s *countingStore) Seen(context.Context, string, time.Duration) (bool, error) {
	s.calls++
	return false, nil
}

// The in-process store does not grow without a bound: an outbox that replays
// for a week must not be a leak.
func TestMemoryIdempotencyIsBounded(t *testing.T) {
	s := &memIdempotency{}
	for i := 0; i < maxIdempotencyKeys+100; i++ {
		if _, err := s.Seen(context.Background(), NewIdempotencyKey(), time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.m) > maxIdempotencyKeys {
		t.Fatalf("the store holds %d keys", len(s.m))
	}
}
