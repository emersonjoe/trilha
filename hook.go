package trilha

// RequestHook wraps one request. It is the seam an optional module hooks into
// — tracing, profiling, a per-request budget — without the core learning
// anything about it: the signature speaks only of *Ctx and of an int, so
// nothing here imports an exporter, an SDK or a vendor, and nothing here ever
// will. The module github.com/emersonjoe/trilha/otel is written against this
// and lives in its own Go module, so the framework keeps depending on the
// standard library alone.
//
// The hook runs once per request that reached a route, with the request id and
// the trace id already set and before the first middleware, and it must call
// next exactly once. next runs the whole rest of the request — middlewares,
// CSRF, the handler, the error page — and returns the status that was written,
// so a panic in the handler still comes back as 500 instead of unwinding
// through the hook.
//
//	cfg.OnRequest = func(c *trilha.Ctx, next func() int) {
//		ctx, span := tracer.Start(c.Context(), c.Pattern())
//		defer span.End()
//		c.SetContext(ctx) // travels to the handler, Upstream and ai
//		span.SetAttributes(attribute.Int("http.response.status_code", next()))
//	}
//
// Replacing the context with Ctx.SetContext before calling next is how a value
// reaches the handler: everything the framework sends out afterwards — an
// Upstream, the ai client, any http.NewRequestWithContext(c.Context(), ...) —
// carries it.
//
// The hook runs inside the request goroutine and must not panic: the recover
// that turns a handler panic into a 500 is inside next, not around it.
type RequestHook func(c *Ctx, next func() (status int))

// onRequest runs serve through Config.OnRequest when one is set. A hook that
// forgets to call next does not get to swallow the request: the answer is
// served anyway, because an unanswered request is a hung browser and a hook is
// an observer, not a router.
func (a *App) onRequest(c *Ctx, serve func() int) {
	hook := a.cfg.OnRequest
	if hook == nil {
		serve()
		return
	}
	served := false
	next := func() int {
		if served {
			// One call is the contract; a second one must not run the
			// handler twice, so it only reports what the first wrote.
			return c.w.status
		}
		served = true
		return serve()
	}
	hook(c, next)
	if !served {
		serve()
	}
}
