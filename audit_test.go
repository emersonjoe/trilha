package trilha

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type sink struct {
	mu   sync.Mutex
	recs []AuditRecord
	err  error
}

func (s *sink) Write(r AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, r)
	return s.err
}

func auditApp(t *testing.T, s AuditSink, logTo io.Writer, h HandlerFunc, mw ...MiddlewareFunc) *App {
	t.Helper()
	if logTo == nil {
		logTo = io.Discard
	}
	a := New(Config{Env: Prod, Audit: s, Logger: slog.New(slog.NewTextHandler(logTo, nil))})
	a.Register(Route{Pattern: "/docs/{id}", Kind: KindAPI,
		Methods: map[string]HandlerFunc{"DELETE": h}, Middlewares: mw})
	return a
}

// The line the application writes is one; everything else on the record is
// already known to the request, and that is the whole point — the actor, the
// address, the request id and the route are what every application forgets in
// half its handlers.
func TestAuditFillsInWhatTheRequestAlreadyKnows(t *testing.T) {
	s := &sink{}
	a := auditApp(t, s, nil, func(c *Ctx) error {
		c.Audit("documento.excluiu", c.Param("id"), Fields{"motivo": "duplicado"})
		return c.Text(200, "ok")
	}, func(c *Ctx, next Next) error {
		c.SetActor(Actor{Subject: "u_17", Email: "ana@org.br"})
		return next()
	})

	req := httptest.NewRequest("DELETE", "/docs/42", nil)
	req.Header.Set("X-Request-ID", "req-1")
	a.Handler().ServeHTTP(httptest.NewRecorder(), req)

	if len(s.recs) != 1 {
		t.Fatalf("gravou %d registros", len(s.recs))
	}
	r := s.recs[0]
	if r.Action != "documento.excluiu" || r.Target != "42" {
		t.Errorf("ação/alvo = %q/%q", r.Action, r.Target)
	}
	if r.Actor.Subject != "u_17" || r.Actor.Via != "session" {
		t.Errorf("ator = %+v", r.Actor)
	}
	if r.RequestID != "req-1" {
		t.Errorf("request id = %q", r.RequestID)
	}
	// The route is the pattern and not the path: "/docs/42" is already in
	// Target, and the pattern is what aggregates.
	if r.Route != "/docs/{id}" {
		t.Errorf("rota = %q, queria o gabarito", r.Route)
	}
	if r.Fields["motivo"] != "duplicado" {
		t.Errorf("campos = %v", r.Fields)
	}
	if r.At.IsZero() {
		t.Error("sem instante")
	}
}

// Nobody recognised is itself worth writing down: an audit trail that silently
// drops the anonymous action is an audit trail with a hole exactly where
// somebody would look.
func TestAuditRecordsTheAnonymousToo(t *testing.T) {
	s := &sink{}
	a := auditApp(t, s, nil, func(c *Ctx) error {
		c.Audit("documento.excluiu", "42")
		return c.Text(200, "ok")
	})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("DELETE", "/docs/42", nil))

	if len(s.recs) != 1 || s.recs[0].Actor.Via != "anonymous" {
		t.Fatalf("ator = %+v", s.recs)
	}
}

// The decision that matters most: a sink that fails must not take the response
// with it. The document was deleted either way — refusing to answer now would
// lose the trail *and* confuse the person who did it.
func TestAuditFailureIsLoudAndNotFatal(t *testing.T) {
	var logged strings.Builder
	s := &sink{err: errors.New("disco cheio")}
	a := auditApp(t, s, &logged, func(c *Ctx) error {
		c.Audit("documento.excluiu", "42")
		return c.Text(200, "excluído")
	})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("DELETE", "/docs/42", nil))

	if rec.Code != 200 || rec.Body.String() != "excluído" {
		t.Fatalf("a auditoria derrubou a operação: %d %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logged.String(), "disco cheio") {
		t.Fatalf("falhou em silêncio: %s", logged.String())
	}
}

// Without a sink the trail still goes somewhere greppable, which is what makes
// a first version shippable without a table.
func TestAuditWithoutASinkGoesToTheLog(t *testing.T) {
	var logged strings.Builder
	a := auditApp(t, nil, &logged, func(c *Ctx) error {
		c.Audit("documento.excluiu", "42")
		return c.Text(200, "ok")
	})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("DELETE", "/docs/42", nil))

	out := logged.String()
	if !strings.Contains(out, "kind=audit") || !strings.Contains(out, "documento.excluiu") {
		t.Fatalf("o registro não foi para o log: %s", out)
	}
}
