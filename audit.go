package trilha

import (
	"time"
)

// Fields is what an audit entry carries beyond the action and its target: the
// old value and the new one, the reason, the amount. Keep it small and keep a
// password out of it — this is written down and read back by people.
type Fields map[string]any

// Actor is who did it. Via says how they were recognised, which matters when
// the answer to "who deleted this" is a key and not a person.
type Actor struct {
	Subject string `json:"subject,omitempty"`
	Email   string `json:"email,omitempty"`
	Name    string `json:"name,omitempty"`
	// Via is "session", "api_key" or "system"; "anonymous" when nobody was
	// recognised, which is itself worth writing down.
	Via string `json:"via"`
	// Tenant is the organisation the action happened inside. In an app with
	// one column for it, this is the first filter of any investigation — and
	// the field that says whether "they saw the wrong rows" is about a query
	// or about somebody having changed organisation.
	Tenant string `json:"tenant,omitempty"`
}

// AuditRecord is one line of the trail: who, what, to what, from where.
type AuditRecord struct {
	At        time.Time `json:"at"`
	Action    string    `json:"action"`
	Target    string    `json:"target,omitempty"`
	Actor     Actor     `json:"actor"`
	IP        string    `json:"ip,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	// Route is the pattern and not the path: "/documents/{id}" aggregates,
	// while "/documents/42" is already in Target.
	Route  string `json:"route,omitempty"`
	Fields Fields `json:"fields,omitempty"`
}

// AuditSink is where the trail goes. One method, because the decision an
// application actually makes is "which table", not "which shape".
//
// An error coming back is logged and the request carries on. That is the
// deliberate part: an audit that takes the operation down with it is worse than
// an audit that fails loudly — the document was deleted either way, and
// refusing to answer would only lose the trail *and* confuse the person.
type AuditSink interface {
	Write(AuditRecord) error
}

// AuditFunc adapts a function to AuditSink.
type AuditFunc func(AuditRecord) error

// Write calls f.
func (f AuditFunc) Write(r AuditRecord) error { return f(r) }

// auditActor is how the framework finds who is acting. auth sets it; a job that
// runs without a request sets nothing and is recorded as "system".
type auditActorKey struct{}

// SetActor records who is acting, for the audit trail. The auth package calls
// it when it puts the session on the request, so an application that uses auth
// never has to.
//
// An application that authenticates its own way — an API key, a machine
// account — calls it once in its middleware, and every c.Audit below it is
// attributed without another line.
func (c *Ctx) SetActor(a Actor) {
	if a.Via == "" {
		a.Via = "session"
	}
	c.Set(auditActorKeyName, a)
}

const auditActorKeyName = "trilha.actor"

// Actor is who the request is attributed to, and "anonymous" when nobody was
// recognised.
func (c *Ctx) Actor() Actor {
	if v, ok := c.Get(auditActorKeyName).(Actor); ok {
		return v
	}
	return Actor{Via: "anonymous"}
}

// Audit writes one line of the trail: what happened, to what, and — filled in
// from the request — by whom, from where, on which route.
//
//	func DELETE(c *trilha.Ctx) error {
//		if err := docs.Delete(c, c.Param("id")); err != nil {
//			return err
//		}
//		c.Audit("document.deleted", c.Param("id"))
//		return c.Redirect("/documents")
//	}
//
//	c.Audit("permission.changed", role, trilha.Fields{"module": "docs", "from": "view", "to": "edit"})
//
// What makes this one line instead of six is that the actor, the address, the
// request id and the route are already known: writing them by hand is what
// every application does and what every application forgets in half its
// handlers.
//
// Without Config.Audit the record goes to the app's logger with kind=audit,
// which is enough to grep and enough to ship a first version with. An
// application that needs to show the trail on a screen gives Config.Audit a
// sink that writes to its own table.
func (c *Ctx) Audit(action, target string, fields ...Fields) {
	rec := AuditRecord{
		At:        time.Now().UTC(),
		Action:    action,
		Target:    target,
		Actor:     c.Actor(),
		IP:        c.ClientIP(),
		RequestID: c.RequestID(),
		Route:     c.Pattern(),
	}
	for _, f := range fields {
		if len(f) == 0 {
			continue
		}
		if rec.Fields == nil {
			rec.Fields = Fields{}
		}
		for k, v := range f {
			rec.Fields[k] = v
		}
	}
	c.app.writeAudit(c, rec)
}

// writeAudit sends the record to the sink, or to the log when there is none.
func (a *App) writeAudit(c *Ctx, rec AuditRecord) {
	if a.cfg.Audit == nil {
		c.Log().Info("audit", "kind", "audit",
			"action", rec.Action, "target", rec.Target,
			"actor", rec.Actor.Subject, "via", rec.Actor.Via,
			"ip", rec.IP, "route", rec.Route, "fields", rec.Fields)
		return
	}
	if err := a.cfg.Audit.Write(rec); err != nil {
		// Loud, and not fatal. The thing being audited already happened; taking
		// the response down now would lose the trail and confuse the person who
		// did it, which is two failures instead of one.
		c.Log().Error("audit: the trail was not written", "error", err,
			"action", rec.Action, "target", rec.Target, "actor", rec.Actor.Subject)
	}
}
