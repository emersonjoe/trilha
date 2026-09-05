package trilha

import "net/http"

// SecurityEvent describes a request the framework blocked or flagged.
type SecurityEvent struct {
	// Kind is one of csrf, auth, body, rate, panic.
	Kind      string
	Status    int
	Method    string
	Path      string
	IP        string
	RequestID string
}

// securityEvent logs the event and calls the hook, at most once per request.
func (a *App) securityEvent(c *Ctx, kind string, status int) {
	if c.secEmitted {
		return
	}
	c.secEmitted = true
	ev := SecurityEvent{Kind: kind, Status: status, Method: c.r.Method, Path: c.r.URL.Path, IP: c.ClientIP(), RequestID: c.requestID}
	a.log.Warn("security", "event", "security", "kind", ev.Kind, "status", ev.Status, "method", ev.Method, "path", ev.Path, "ip", ev.IP, "request_id", ev.RequestID)
	if a.cfg.OnSecurityEvent != nil {
		a.cfg.OnSecurityEvent(ev)
	}
}

// kindForStatus maps blocked statuses to event kinds.
func kindForStatus(code int) string {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "auth"
	case http.StatusRequestEntityTooLarge:
		return "body"
	case http.StatusTooManyRequests:
		return "rate"
	}
	return ""
}
