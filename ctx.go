package trilha

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/h"
)

type routeKind int

const (
	kindPage routeKind = iota
	kindAPI
)

// Ctx wraps one request/response pair. It is created per request and is not
// safe for use from other goroutines after the handler returns.
type Ctx struct {
	w            *responseWriter
	r            *http.Request
	app          *App
	route        *Route
	kind         routeKind
	values       map[string]any
	title        string
	status       int
	requestID    string
	formErr      error
	parsed       bool
	nonce        string
	secEmitted   bool
	traceID      string
	traceParsed  bool
	logger       *slog.Logger
	fragment     string
	fragParsed   bool
	islandLoader bool
}

func newCtx(a *App, w *responseWriter, r *http.Request, kind routeKind) *Ctx {
	id := r.Header.Get("X-Request-ID")
	if id == "" {
		var b [8]byte
		_, _ = rand.Read(b[:])
		id = hex.EncodeToString(b[:])
	}
	return &Ctx{w: w, r: r, app: a, kind: kind, values: map[string]any{}, requestID: id}
}

// Request returns the underlying *http.Request.
func (c *Ctx) Request() *http.Request { return c.r }

// Writer returns the underlying http.ResponseWriter.
func (c *Ctx) Writer() http.ResponseWriter { return c.w }

// Context returns the request context.
func (c *Ctx) Context() context.Context { return c.r.Context() }

// SetContext replaces the request context, so a middleware can pass values
// to code that only receives *http.Request (templates, stdlib helpers).
func (c *Ctx) SetContext(ctx context.Context) { c.r = c.r.WithContext(ctx) }

// SetRequest replaces the request (rewritten URL, wrapped body...). Values
// already read from the old request (form, request id) are kept.
func (c *Ctx) SetRequest(r *http.Request) { c.r = r }

// App returns the application.
func (c *Ctx) App() *App { return c.app }

// Env returns the runtime environment.
func (c *Ctx) Env() Env { return c.app.cfg.Env }

// Fragment returns the part of the page this request asked for, or "" on a
// normal navigation. It is the whole protocol: the same route serves the page
// and the piece, and decides what to return.
//
//	func Page(c *trilha.Ctx) (h.Node, error) {
//		lista := listaDe(c)
//		if c.Fragment() == "lista" {
//			return lista, nil // sem layouts
//		}
//		return h.Div(busca(), lista), nil
//	}
//
// A fragment response carries no layout, no document envelope and no dev
// script; every HTML response gets Vary: Trilha-Fragment so a cache never
// serves one in place of the other.
func (c *Ctx) Fragment() string {
	if !c.fragParsed {
		c.fragParsed = true
		// Canonical spelling: Header.Get would allocate otherwise.
		c.fragment = c.r.Header.Get(fragmentHeader)
		if len(c.fragment) > 128 {
			c.fragment = "" // um alvo desse tamanho é abuso, não um id
		}
	}
	return c.fragment
}

// RequestID returns the X-Request-ID header or a generated id.
func (c *Ctx) RequestID() string { return c.requestID }

// Param returns a path parameter ({slug} or {path...}).
func (c *Ctx) Param(name string) string { return c.r.PathValue(name) }

// Query returns the first value of a query-string parameter.
func (c *Ctx) Query(name string) string { return c.r.URL.Query().Get(name) }

// parseForm parses the form once, honouring the body size limit.
func (c *Ctx) parseForm() error {
	if c.parsed {
		return c.formErr
	}
	c.parsed = true
	ct := c.r.Header.Get("Content-Type")
	var err error
	if strings.HasPrefix(ct, "multipart/form-data") {
		err = c.r.ParseMultipartForm(c.app.cfg.MaxBodyBytes)
	} else {
		err = c.r.ParseForm()
	}
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			c.formErr = &HTTPError{Code: http.StatusRequestEntityTooLarge, Message: "body too large"}
		} else {
			c.formErr = &HTTPError{Code: http.StatusBadRequest, Message: "invalid form"}
		}
	}
	return c.formErr
}

// Form returns a form field (POST body or query string). Returns "" if the
// form cannot be parsed; use FormErr to distinguish.
func (c *Ctx) Form(name string) string {
	_ = c.parseForm()
	return c.r.FormValue(name)
}

// FormErr returns the error from parsing the form, if any (413 or 400).
func (c *Ctx) FormErr() error { return c.parseForm() }

// BindJSON decodes the request body into v. Returns an HTTPError 400 on
// malformed JSON and 413 when the body exceeds the limit.
func (c *Ctx) BindJSON(v any) error {
	dec := json.NewDecoder(c.r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return &HTTPError{Code: http.StatusRequestEntityTooLarge, Message: "body too large"}
		}
		return &HTTPError{Code: http.StatusBadRequest, Message: "invalid JSON: " + err.Error()}
	}
	return nil
}

// Header sets a response header.
func (c *Ctx) Header(k, v string) { c.w.Header().Set(k, v) }

// Status sets the status code used by the next page render.
func (c *Ctx) Status(code int) { c.status = code }

// JSON writes a JSON response.
func (c *Ctx) JSON(code int, v any) error {
	c.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.w.WriteHeader(code)
	enc := json.NewEncoder(c.w)
	return enc.Encode(v)
}

// Text writes a plain-text response.
func (c *Ctx) Text(code int, s string) error {
	c.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.w.WriteHeader(code)
	_, err := c.w.Write([]byte(s))
	return err
}

// HTML renders a node as the whole response, without layouts.
func (c *Ctx) HTML(code int, n h.Node) error {
	return c.app.writeHTML(c, code, n)
}

// Redirect returns a redirect error (303). Use as `return c.Redirect("/x")`.
func (c *Ctx) Redirect(url string) error { return Redirect(url) }

// Cookie returns a request cookie.
func (c *Ctx) Cookie(name string) (*http.Cookie, error) { return c.r.Cookie(name) }

// SetCookie adds a Set-Cookie header.
func (c *Ctx) SetCookie(ck *http.Cookie) { http.SetCookie(c.w, ck) }

// Set stores a per-request value (typically from middleware).
func (c *Ctx) Set(key string, v any) { c.values[key] = v }

// Get reads a per-request value; nil when absent.
func (c *Ctx) Get(key string) any { return c.values[key] }

// Title returns the page title set by the page (for layouts).
func (c *Ctx) Title() string { return c.title }

// SetTitle sets the page title; layouts read it with Title.
func (c *Ctx) SetTitle(t string) { c.title = t }

// NoWriteDeadline disables the server write timeout for this response; call
// it before streaming (SSE, long downloads).
func (c *Ctx) NoWriteDeadline() error {
	return http.NewResponseController(c.w).SetWriteDeadline(time.Time{})
}

// Written reports whether the response has already started.
func (c *Ctx) Written() bool { return c.w.wrote }

// responseWriter tracks status and bytes for logging and error handling.
type responseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
	bytes  int
}

func (w *responseWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		if !w.wrote {
			w.WriteHeader(http.StatusOK)
		}
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the original writer.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
