package trilha

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Connections is the list of external things this application talks to: a
// third-party API, an MCP server, a provider — each with a URL, a way to
// authenticate, a secret that is sealed at rest and masked everywhere else, and
// a button that says whether it works.
//
// It is one screen written three times in the application that was measured
// (integrations, MCP servers, the LLM provider), and the three were the same
// screen: name, base URL, kind of auth, a secret that "leaves blank to keep",
// test, last result. Here it is one type, and the screen is ui.ConnectionsPanel.
//
// What it deliberately does not do is hand the secret to the application:
// Client returns an *http.Client that already carries it, and Test runs the
// kind's own check. The secret is read by the transport and by nothing else.
type Connections struct {
	store   ConnectionStore
	kinds   []ConnectionKind
	timeout time.Duration
}

// Connection is one external service.
type Connection struct {
	ID     string
	Tenant string
	// Kind is the key of a declared ConnectionKind.
	Kind string
	Name string
	// URL is the base address. It is validated by ValidateExternalURL: in
	// production it cannot point at this machine or the private network.
	URL string
	// Auth is one of the kind's ways to authenticate: none, bearer, header
	// or basic.
	Auth string
	// Username is the user of a basic auth.
	Username string
	// Header is the name of the header a "header" auth sends the secret in.
	Header string
	// Secret is the token, the header value or the password. It is masked in
	// %v, JSON and the log, and sealed when a database stores it.
	Secret Secret
	// Headers are fixed headers sent on every request. Not a place for a
	// secret: they are shown on the screen.
	Headers map[string]string
	// LastTest is the result of the last Test, or nil when it never ran.
	LastTest  *ConnectionTest
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ConnectionTest is what one Test answered.
type ConnectionTest struct {
	At      time.Time
	OK      bool
	Message string
}

// ConnectionKind is one sort of external service and how to test it.
type ConnectionKind struct {
	Key   string
	Label string
	// Auth lists the ways this kind may authenticate; the default is none.
	Auth []string
	// Test checks the connection. TestHTTP is the one for an API; a kind that
	// speaks another protocol writes its own, with conn.Client for the
	// authenticated requests. Nil means the kind cannot be tested.
	Test func(ctx context.Context, conn Connection) error
}

// ConnectionsOpts configures the list.
type ConnectionsOpts struct {
	// Store is where the connections live; ConnectionMemory when nil.
	Store ConnectionStore
	Kinds []ConnectionKind
	// Timeout is the deadline of a Test and of the clients Client returns.
	// 30 seconds when zero; there is no way to get a client without one.
	Timeout time.Duration
}

// ConnectionStore keeps the connections. Memory is the default; a table
// behind database/sql is forty lines in the application, with the Secret
// column sealed by the type itself (it is a driver.Valuer and an sql.Scanner).
type ConnectionStore interface {
	List(ctx context.Context, tenant string) ([]Connection, error)
	// Get answers ErrNotFound when there is no such id in that tenant.
	Get(ctx context.Context, tenant, id string) (Connection, error)
	// Save inserts or replaces the whole record.
	Save(ctx context.Context, conn Connection) error
	Delete(ctx context.Context, tenant, id string) error
}

// ConnectionAuths are the ways a connection authenticates, in the order a
// select shows them.
var ConnectionAuths = Enum{
	{Value: "none", Label: "None"},
	{Value: "bearer", Label: "Bearer token"},
	{Value: "header", Label: "Header"},
	{Value: "basic", Label: "Basic (user and password)"},
}

// NewConnections builds the list.
func NewConnections(o ConnectionsOpts) *Connections {
	if o.Store == nil {
		o.Store = ConnectionMemory()
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	kinds := make([]ConnectionKind, len(o.Kinds))
	for i, k := range o.Kinds {
		if len(k.Auth) == 0 {
			k.Auth = []string{"none"}
		}
		kinds[i] = k
	}
	return &Connections{store: o.Store, kinds: kinds, timeout: o.Timeout}
}

// Kinds is what was declared, in order.
func (x *Connections) Kinds() []ConnectionKind { return x.kinds }

// Kind is one declared kind, or false.
func (x *Connections) Kind(key string) (ConnectionKind, bool) {
	for _, k := range x.kinds {
		if k.Key == key {
			return k, true
		}
	}
	return ConnectionKind{}, false
}

// List is every connection of the current tenant, by kind and then by name.
func (x *Connections) List(c *Ctx) ([]Connection, error) {
	list, err := x.store.List(ctxOrBackground(c), tenantOf(c))
	if err != nil {
		return nil, err
	}
	order := map[string]int{}
	for i, k := range x.kinds {
		order[k.Key] = i
	}
	sort.SliceStable(list, func(i, j int) bool {
		if order[list[i].Kind] != order[list[j].Kind] {
			return order[list[i].Kind] < order[list[j].Kind]
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
	return list, nil
}

// Get is one connection of the current tenant.
func (x *Connections) Get(c *Ctx, id string) (Connection, error) {
	return x.store.Get(ctxOrBackground(c), tenantOf(c), id)
}

// Save validates and stores a connection, new (empty ID) or existing.
//
// What fails validation comes back as FieldErrors, keyed by the field, which
// is what a form renders next to the input. An update whose Secret is empty
// keeps the one stored — that is what "leave blank to keep" means, and the
// screen never has the old value to send back.
func (x *Connections) Save(c *Ctx, conn Connection) (Connection, error) {
	ctx := ctxOrBackground(c)
	conn.Tenant = tenantOf(c)
	conn.Name = strings.TrimSpace(conn.Name)
	conn.URL = strings.TrimSpace(conn.URL)
	conn.Header = strings.TrimSpace(conn.Header)
	conn.Username = strings.TrimSpace(conn.Username)

	errs := FieldErrors{}
	kind, ok := x.Kind(conn.Kind)
	if !ok {
		errs.Add("kind", "unknown kind")
	}
	if conn.Name == "" {
		errs.Add("name", "required")
	}
	env := envOf(c)
	if err := ValidateExternalURL(conn.URL, env); err != nil {
		errs.Add("url", err.Error())
	} else if env != Prod && privateURL(conn.URL) && c != nil {
		c.Log().Warn("connections: private URL accepted outside production", "name", conn.Name, "url", conn.URL)
	}
	if conn.Auth == "" {
		conn.Auth = "none"
	}
	if ok && !contains(kind.Auth, conn.Auth) {
		errs.Add("auth", "not one of "+strings.Join(kind.Auth, ", "))
	}
	switch conn.Auth {
	case "header":
		if conn.Header == "" {
			errs.Add("header", "required")
		}
	case "basic":
		if conn.Username == "" {
			errs.Add("username", "required")
		}
	case "none":
		conn.Secret = ""
	}
	for k := range conn.Headers {
		if strings.EqualFold(k, "Authorization") {
			errs.Add("headers", "Authorization is the auth, not a fixed header")
		}
	}

	now := time.Now().UTC()
	if conn.ID == "" {
		if errs.Any() {
			return conn, errs
		}
		if conn.Auth != "none" && conn.Secret.Empty() {
			errs.Add("secret", "required")
			return conn, errs
		}
		id, err := linkID()
		if err != nil {
			return conn, err
		}
		conn.ID, conn.CreatedAt, conn.LastTest = id, now, nil
	} else {
		prev, err := x.store.Get(ctx, conn.Tenant, conn.ID)
		if err != nil {
			return conn, err
		}
		if conn.Secret.Empty() {
			conn.Secret = prev.Secret
		}
		if conn.Auth != "none" && conn.Secret.Empty() {
			errs.Add("secret", "required")
		}
		if errs.Any() {
			return conn, errs
		}
		conn.CreatedAt = prev.CreatedAt
		if prev.URL != conn.URL || prev.Auth != conn.Auth || prev.Secret != conn.Secret {
			// The old result was about another address or another
			// credential; a green badge that no longer applies is worse
			// than none.
			conn.LastTest = nil
		} else {
			conn.LastTest = prev.LastTest
		}
	}
	conn.UpdatedAt = now
	if err := x.store.Save(ctx, conn); err != nil {
		return conn, err
	}
	audit(c, "connection.save", conn.ID, Fields{"kind": conn.Kind, "name": conn.Name, "url": conn.URL, "auth": conn.Auth})
	return conn, nil
}

// Delete removes one connection of the current tenant.
func (x *Connections) Delete(c *Ctx, id string) error {
	ctx := ctxOrBackground(c)
	conn, err := x.store.Get(ctx, tenantOf(c), id)
	if err != nil {
		return err
	}
	if err := x.store.Delete(ctx, conn.Tenant, id); err != nil {
		return err
	}
	audit(c, "connection.delete", id, Fields{"kind": conn.Kind, "name": conn.Name})
	return nil
}

// Test runs the kind's check within the deadline, stores the result on the
// connection and audits it. The result is the answer even when the check
// failed; the error is for a connection that does not exist or a kind that
// has no test.
func (x *Connections) Test(c *Ctx, id string) (ConnectionTest, error) {
	ctx := ctxOrBackground(c)
	conn, err := x.store.Get(ctx, tenantOf(c), id)
	if err != nil {
		return ConnectionTest{}, err
	}
	kind, ok := x.Kind(conn.Kind)
	if !ok || kind.Test == nil {
		return ConnectionTest{}, fmt.Errorf("trilha: connections: kind %q has no test", conn.Kind)
	}
	tctx, cancel := context.WithTimeout(ctx, x.timeout)
	defer cancel()
	res := ConnectionTest{At: time.Now().UTC(), OK: true}
	if err := kind.Test(tctx, conn); err != nil {
		res.OK, res.Message = false, err.Error()
	}
	conn.LastTest = &res
	conn.UpdatedAt = res.At
	if err := x.store.Save(ctx, conn); err != nil {
		return res, err
	}
	audit(c, "connection.test", id, Fields{"kind": conn.Kind, "name": conn.Name, "ok": res.OK, "message": res.Message})
	return res, nil
}

// Client is an HTTP client for one connection: the auth and the fixed headers
// on every request, the deadline set, and a transport that only speaks to the
// connection's host — a redirect elsewhere does not carry the secret along.
func (x *Connections) Client(c *Ctx, id string) (*http.Client, error) {
	conn, err := x.store.Get(ctxOrBackground(c), tenantOf(c), id)
	if err != nil {
		return nil, err
	}
	return conn.Client(x.timeout), nil
}

// Client is the authenticated client of this connection, for whoever writes
// a ConnectionKind.Test. A zero timeout is 30 seconds; there is no client
// without one.
func (conn Connection) Client(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	host := ""
	if u, err := url.Parse(conn.URL); err == nil {
		host = u.Host
	}
	return &http.Client{Timeout: timeout, Transport: &connTransport{conn: conn, host: host, base: http.DefaultTransport}}
}

// connTransport is where the secret is read, and the only place.
type connTransport struct {
	conn Connection
	host string
	base http.RoundTripper
}

func (t *connTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != t.host {
		return nil, fmt.Errorf("trilha: connection %q only speaks to host %s, not %s", t.conn.Name, t.host, req.URL.Host)
	}
	req = req.Clone(req.Context())
	for k, v := range t.conn.Headers {
		req.Header.Set(k, v)
	}
	switch t.conn.Auth {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+t.conn.Secret.Reveal())
	case "header":
		req.Header.Set(t.conn.Header, t.conn.Secret.Reveal())
	case "basic":
		req.SetBasicAuth(t.conn.Username, t.conn.Secret.Reveal())
	}
	return t.base.RoundTrip(req)
}

// TestHTTP is the test of an API: one request to path under the base URL,
// and any answer below 400 is a pass. What the server said otherwise is the
// message, status line included, which is what somebody fixing the
// credential needs to read.
func TestHTTP(method, path string) func(ctx context.Context, conn Connection) error {
	return func(ctx context.Context, conn Connection) error {
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(conn.URL, "/")+path, nil)
		if err != nil {
			return err
		}
		resp, err := conn.Client(0).Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		return nil
	}
}

// ValidateExternalURL says whether raw is the address of a third party:
// http or https, with a host and without credentials. In production it also
// refuses this machine and the private network — loopback, RFC 1918,
// link-local (the cloud metadata address lives there), and names such as
// localhost or *.internal — because a URL somebody types into a screen is the
// classic way to make a server call itself. Outside production the private
// network is where the real service runs, and it is allowed.
//
// The name is not resolved: a public name that answers a private address is
// a DNS question, not a URL one.
func ValidateExternalURL(raw string, env Env) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("must be a valid http(s) URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("must start with http:// or https://")
	}
	if u.Hostname() == "" {
		return errors.New("needs a host")
	}
	if u.User != nil {
		return errors.New("credentials do not go in the URL; use the auth fields")
	}
	if env == Prod && privateURL(raw) {
		return errors.New("private or local address is not allowed in production")
	}
	return nil
}

// privateURL reports whether the host of raw is this machine or the private
// network, by address or by name.
func privateURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".lan", ".home.arpa"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func tenantOf(c *Ctx) string {
	if c == nil {
		return ""
	}
	return c.Actor().Tenant
}

// envOf is the environment a validation runs in; without a request it is
// production, the strict one.
func envOf(c *Ctx) Env {
	if c == nil {
		return Prod
	}
	return c.Env()
}

// ConnectionMemory keeps the connections in this process.
func ConnectionMemory() ConnectionStore { return &connMemory{rows: map[string]Connection{}} }

type connMemory struct {
	mu   sync.RWMutex
	rows map[string]Connection // tenant + "\x00" + id
}

func connKey(tenant, id string) string { return tenant + "\x00" + id }

func (m *connMemory) List(_ context.Context, tenant string) ([]Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Connection
	for _, r := range m.rows {
		if r.Tenant == tenant {
			out = append(out, copyConn(r))
		}
	}
	return out, nil
}

func (m *connMemory) Get(_ context.Context, tenant, id string) (Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rows[connKey(tenant, id)]
	if !ok {
		return Connection{}, ErrNotFound
	}
	return copyConn(r), nil
}

func (m *connMemory) Save(_ context.Context, conn Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[connKey(conn.Tenant, conn.ID)] = copyConn(conn)
	return nil
}

func (m *connMemory) Delete(_ context.Context, tenant, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, connKey(tenant, id))
	return nil
}

// copyConn is a deep enough copy: the map and the pointer are the two things a
// caller could change behind the store's back.
func copyConn(c Connection) Connection {
	if c.Headers != nil {
		h := make(map[string]string, len(c.Headers))
		for k, v := range c.Headers {
			h[k] = v
		}
		c.Headers = h
	}
	if c.LastTest != nil {
		t := *c.LastTest
		c.LastTest = &t
	}
	return c
}
