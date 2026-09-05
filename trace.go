package trilha

import "log/slog"

// traceparent is the W3C Trace Context header: version-traceid-spanid-flags.
const traceparentHeader = "Traceparent"

// TraceID returns the trace identifier the caller propagated in the
// traceparent header, or "" when it is absent or malformed. Trilha only
// carries the identifier into the logs; it does not sample or export spans.
func (c *Ctx) TraceID() string {
	if !c.traceParsed {
		c.traceParsed = true
		c.traceID = parseTraceparent(c.r.Header.Get(traceparentHeader))
	}
	return c.traceID
}

// Log returns a logger already carrying request_id and, when present,
// trace_id, so every line of one request can be found together
// (NIST SP 800-53 AU-3).
func (c *Ctx) Log() *slog.Logger {
	if c.logger != nil {
		return c.logger
	}
	c.logger = c.app.log.With("request_id", c.requestID)
	if tid := c.TraceID(); tid != "" {
		c.logger = c.logger.With("trace_id", tid)
	}
	return c.logger
}

// parseTraceparent validates the header and returns the trace id. Anything
// unexpected is dropped: a malformed value must never be propagated as if it
// were a real trace, and it must never reach a log as attacker-chosen text.
func parseTraceparent(v string) string {
	// 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
	if len(v) != 55 || v[2] != '-' || v[35] != '-' || v[52] != '-' {
		return ""
	}
	if !hexOnly(v[0:2]) || !hexOnly(v[36:52]) || !hexOnly(v[53:55]) {
		return ""
	}
	id := v[3:35]
	if !hexOnly(id) {
		return ""
	}
	for i := 0; i < len(id); i++ {
		if id[i] != '0' {
			return id
		}
	}
	return "" // an all-zero trace id is invalid
}

func hexOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}
