package trilha

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// TestingT is the part of *testing.T these helpers use. It exists so the
// runtime never imports testing: importing trilha in production must not drag
// the test flags into the binary.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// TestOption changes one request. The options are shared by every helper.
type TestOption func(*testOptions)

type testOptions struct {
	app     *App
	header  http.Header
	cookies map[string]string
	signed  map[string]string
	body    func(t TestingT) (contentType string, body []byte)
	csrf    bool
}

func newTestOptions(opts []TestOption) *testOptions {
	o := &testOptions{
		header:  http.Header{},
		cookies: map[string]string{},
		signed:  map[string]string{},
		csrf:    true,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithApp runs the route on this app instead of the throwaway one, so the
// route sees the real Config (secret, Public, CSRFForAPI). Only TestRoute and
// TestPage read it, and they register the route on the app, so give a fresh
// app: registering the same pattern twice panics in net/http.
func WithApp(a *App) TestOption { return func(o *testOptions) { o.app = a } }

// WithHeader adds a request header.
func WithHeader(name, value string) TestOption {
	return func(o *testOptions) { o.header.Add(name, value) }
}

// WithCookie sends a cookie, replacing the one the client holds.
func WithCookie(name, value string) TestOption {
	return func(o *testOptions) { o.cookies[name] = value }
}

// WithSigned sends a cookie signed like Ctx.SetSigned would, valid for an
// hour. It is how a test opens a session route without replaying the login.
func WithSigned(name, value string) TestOption {
	return func(o *testOptions) { o.signed[name] = value }
}

// WithForm sends the values as an HTML form.
func WithForm(form url.Values) TestOption {
	return func(o *testOptions) {
		o.body = func(TestingT) (string, []byte) {
			return "application/x-www-form-urlencoded", []byte(form.Encode())
		}
	}
}

// WithJSON sends v as a JSON body.
func WithJSON(v any) TestOption {
	return func(o *testOptions) {
		o.body = func(t TestingT) (string, []byte) {
			t.Helper()
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("trilha: WithJSON: %v", err)
			}
			return "application/json", b
		}
	}
}

// WithBody sends the body as it is.
func WithBody(contentType, body string) TestOption {
	return func(o *testOptions) {
		o.body = func(TestingT) (string, []byte) { return contentType, []byte(body) }
	}
}

// WithoutCSRF drops the CSRF cookie and header, so the request exercises the
// rejection the browser would get.
func WithoutCSRF() TestOption { return func(o *testOptions) { o.csrf = false } }

// TestResponse is what the app answered. It embeds the recorder, so Code,
// Body and Header are the usual ones; the Want methods stop the test with the
// body in the message.
type TestResponse struct {
	*httptest.ResponseRecorder
	// Request is what was sent, already carrying cookies and CSRF.
	Request *http.Request
	// Node is the node the page returned, before the layouts. Only TestPage
	// fills it in.
	Node h.Node

	t TestingT
}

// WantStatus fails unless the status matches.
func (r *TestResponse) WantStatus(code int) *TestResponse {
	r.t.Helper()
	if r.Code != code {
		r.t.Fatalf("status = %d, want %d\n%s", r.Code, code, r.snippet())
	}
	return r
}

// WantContains fails unless every string is in the body.
func (r *TestResponse) WantContains(subs ...string) *TestResponse {
	r.t.Helper()
	body := r.Body.String()
	for _, s := range subs {
		if !strings.Contains(body, s) {
			r.t.Fatalf("body does not contain %q\n%s", s, r.snippet())
		}
	}
	return r
}

// WantHeader fails unless the response header matches.
func (r *TestResponse) WantHeader(name, want string) *TestResponse {
	r.t.Helper()
	if got := r.Header().Get(name); got != want {
		r.t.Fatalf("header %s = %q, want %q", name, got, want)
	}
	return r
}

// JSON decodes the body into v.
func (r *TestResponse) JSON(v any) *TestResponse {
	r.t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), v); err != nil {
		r.t.Fatalf("invalid JSON: %v\n%s", err, r.snippet())
	}
	return r
}

// Cookie returns the cookie the response set, or nil.
func (r *TestResponse) Cookie(name string) *http.Cookie {
	for _, ck := range r.Result().Cookies() {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

// snippet is the body cut to what fits in a failure message.
func (r *TestResponse) snippet() string {
	const max = 2000
	body := r.Body.String()
	if len(body) > max {
		body = body[:max] + "\n… (truncated)"
	}
	return body
}

// TestClient sends requests through the app keeping the cookies, so a login
// and the page after it are two lines.
type TestClient struct {
	t       TestingT
	app     *App
	handler http.Handler
	jar     map[string]string
}

// NewTestClient builds a client over the app's handler.
func NewTestClient(t TestingT, a *App) *TestClient {
	t.Helper()
	return &TestClient{t: t, app: a, handler: a.Handler(), jar: map[string]string{}}
}

// Request sends one request and stores the cookies the app set.
func (c *TestClient) Request(method, target string, opts ...TestOption) *TestResponse {
	c.t.Helper()
	o := newTestOptions(opts)

	var contentType string
	var body []byte
	if o.body != nil {
		contentType, body = o.body(c.t)
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for name, values := range o.header {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}

	// One value per name, so nothing is sent twice: the jar first, then what
	// this request says.
	cookies := map[string]string{}
	for name, v := range c.jar {
		cookies[name] = v
	}
	for name, v := range o.cookies {
		cookies[name] = v
	}
	for name, v := range o.signed {
		cookies[name] = c.sign(name, v)
	}
	if o.csrf && bodyMethods[method] {
		// Double submit is only the cookie against the header, so the client
		// mints the token: it is the same proof the browser gives.
		tok, ok := cookies[CSRFCookie]
		if !ok {
			tok = newCSRFToken()
			c.jar[CSRFCookie] = tok
			cookies[CSRFCookie] = tok
		}
		req.Header.Set(CSRFHeader, tok)
	} else if !o.csrf {
		delete(cookies, CSRFCookie)
	}
	for _, name := range sortedKeys(cookies) {
		req.AddCookie(&http.Cookie{Name: name, Value: cookies[name]})
	}

	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 {
			delete(c.jar, ck.Name)
			continue
		}
		c.jar[ck.Name] = ck.Value
	}
	return &TestResponse{ResponseRecorder: rec, Request: req, t: c.t}
}

// Get sends a GET.
func (c *TestClient) Get(target string, opts ...TestOption) *TestResponse {
	c.t.Helper()
	return c.Request(http.MethodGet, target, opts...)
}

// PostForm sends a form, CSRF included.
func (c *TestClient) PostForm(target string, form url.Values, opts ...TestOption) *TestResponse {
	c.t.Helper()
	return c.Request(http.MethodPost, target, append([]TestOption{WithForm(form)}, opts...)...)
}

// PostJSON sends a JSON body.
func (c *TestClient) PostJSON(target string, v any, opts ...TestOption) *TestResponse {
	c.t.Helper()
	return c.Request(http.MethodPost, target, append([]TestOption{WithJSON(v)}, opts...)...)
}

func (c *TestClient) sign(name, value string) string {
	c.t.Helper()
	tok, err := c.app.signer.Sign(value, time.Now().Add(time.Hour))
	if err != nil {
		c.t.Fatalf("trilha: WithSigned(%q): %v", name, err)
	}
	return tok
}

// TestRequest sends one request through the whole app: middlewares, CSRF,
// error pages, everything ListenAndServe would run.
func TestRequest(t TestingT, a *App, method, target string, opts ...TestOption) *TestResponse {
	t.Helper()
	return NewTestClient(t, a).Request(method, target, opts...)
}

// TestRoute sends one request to a single route with its middlewares, without
// generating the app. Pass WithApp to give it a Config of its own.
func TestRoute(t TestingT, r Route, method, target string, opts ...TestOption) *TestResponse {
	t.Helper()
	o := newTestOptions(opts)
	a := o.app
	if a == nil {
		a = New(Config{Env: Dev, Logger: silentLogger()})
	}
	a.Register(r)
	return NewTestClient(t, a).Request(method, target, opts...)
}

// TestPage renders one page with its layouts and fills TestResponse.Node with
// what the page returned, so a test can look at the node instead of the HTML.
func TestPage(t TestingT, r Route, target string, opts ...TestOption) *TestResponse {
	t.Helper()
	var node h.Node
	page := r.Page
	if page == nil {
		t.Fatalf("trilha: TestPage on a route without a page: use TestRoute")
		return nil
	}
	r.Page = func(c *Ctx) (h.Node, error) {
		n, err := page(c)
		node = n
		return n, err
	}
	res := TestRoute(t, r, http.MethodGet, target, opts...)
	res.Node = node
	return res
}

func newCSRFToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// silentLogger keeps the throwaway app from writing the access log of a test.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
