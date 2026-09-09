package trilha

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Three flows in every internal application happen with no login: somebody
// outside fills in a form, somebody checks a document by a code, somebody
// answers a request from an e-mail. What the beginner writes is a random
// string in a table, in the clear, with no deadline — and a URL token that is
// short enough to guess is guessed.

// ErrNoLink is what Claim answers for a token that is not valid, not this
// link's, expired, or already used up. It is one error for all of them on
// purpose: telling a stranger which of the four happened is telling them how
// close they are.
var ErrNoLink = errors.New("trilha: invalid or expired link")

// LinkOpts is what a link carries and how long it lives.
type LinkOpts struct {
	// Data travels inside the token. It is signed, so nobody can change it —
	// and it is **not secret**: anybody holding the link can read it. Put an
	// id in it, not a name, a price or a reason.
	Data map[string]string
	// TTL is how long the link works. Zero is an hour, because a public link
	// with no deadline is a password that never expires — and a negative one
	// is an error rather than a default: a caller computing "until the end of
	// the day" after midnight would otherwise get a valid link out of a bug.
	TTL time.Duration
	// Uses limits how many times it can be claimed. Zero is unlimited until it
	// expires, and that case needs no storage at all: checking is arithmetic
	// over a signature.
	Uses int
	// Path is the public route the link points at; the token is the last
	// segment. Empty uses the current path.
	Path string
}

// Link is what Claim answers: the data the link was made with, and the means
// to spend one use of it.
type Link struct {
	Name      string
	ID        string
	Data      map[string]string
	ExpiresAt time.Time
	Uses      int

	c *Ctx
}

// LinkStore counts the uses of a link that has a limit. Two methods, and the
// hard one is Consume: it has to be atomic, because two people opening the
// same one-use link at the same instant is exactly the case this exists for.
// A SQL implementation is an UPDATE with a WHERE on the count; Redis is an
// INCR compared to the limit.
//
// Nil keeps the count in memory, which is honest about one replica and said
// once in the log.
type LinkStore interface {
	// Used is how many times the link has been consumed. Reading is not
	// spending: opening the page has to be able to ask whether there is a use
	// left without taking it, or a reload would burn the link somebody is
	// still filling in.
	Used(id string) (int, error)
	// Consume records one use if there is room.
	Consume(id string, max int) (ok bool, err error)
	Forget(id string) error
}

// Link builds a signed URL that works without a session.
//
//	url, err := c.Link("form", trilha.LinkOpts{
//		Data: map[string]string{"task": taskID},
//		TTL:  7 * 24 * time.Hour,
//		Uses: 1,
//		Path: "/form",
//	})
//
// name is the purpose. A token made for one purpose does not open another —
// a link to a form is not a link to a document, even signed by the same
// application.
//
// With Uses zero the whole link is the token: no row, no lookup, and
// verifying is a signature check. With a limit, the count lives in
// Config.Links.
//
//	see: Ctx.Claim, Signer
func (c *Ctx) Link(name string, o LinkOpts) (string, error) {
	switch {
	case o.TTL < 0:
		return "", fmt.Errorf("trilha: link %s has a negative TTL (%s); a deadline in the past is a bug in whoever computed it, not a link", name, o.TTL)
	case o.TTL == 0:
		o.TTL = time.Hour
	}
	id, err := linkID()
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(linkBody{Name: name, ID: id, Data: o.Data, Uses: o.Uses})
	if err != nil {
		return "", err
	}
	if len(body) > maxLinkBody {
		return "", fmt.Errorf("trilha: link %s carries %d bytes of data and the limit is %d; put an id in it and keep the rest in your own table",
			name, len(body), maxLinkBody)
	}
	signed, err := c.app.signer.Sign(string(body), time.Now().Add(o.TTL))
	if err != nil {
		return "", err
	}
	path := o.Path
	if path == "" {
		path = c.r.URL.Path
	}
	return strings.TrimSuffix(path, "/") + "/" + pathSafe(signed), nil
}

// Claim reads the token of the current route, checks it, and answers the link.
//
//	func Page(c *trilha.Ctx) (h.Node, error) {
//		link, err := c.Claim("form")
//		if err != nil {
//			return nil, err // 404: invalid, expired or spent — never which
//		}
//		return form(link.Data["task"]), nil
//	}
//
// The token is the last path segment, which is where Link put it; a route with
// a {token} parameter is read by name instead.
//
// A wrong token counts against the address that sent it: guessing a token in a
// URL is brute force, and the budget for getting it wrong is small. Everything
// else — signature, purpose, deadline — is checked here, and all four failures
// answer the same thing.
func (c *Ctx) Claim(name string) (*Link, error) {
	raw := c.Param("token")
	if raw == "" {
		raw = lastSegment(c.r.URL.Path)
	}
	if raw == "" {
		return nil, c.linkDenied(name)
	}
	value, ok := c.app.signer.Verify(fromPathSafe(raw), time.Now())
	if !ok {
		return nil, c.linkDenied(name)
	}
	var b linkBody
	if err := json.Unmarshal([]byte(value), &b); err != nil || b.Name != name {
		return nil, c.linkDenied(name)
	}
	l := &Link{Name: b.Name, ID: b.ID, Data: b.Data, Uses: b.Uses, c: c}
	// A link with a limit is only worth answering if there is a use left, and
	// the check happens here so a GET that shows a spent form does not exist.
	if b.Uses > 0 {
		used, err := c.app.linkStore().Used(b.ID)
		if err != nil {
			return nil, err
		}
		if used >= b.Uses {
			return nil, c.linkDenied(name)
		}
	}
	c.SetActor(Actor{Subject: "link:" + b.ID, Via: "link"})
	return l, nil
}

// Consume spends one use of the link, and is what a POST calls after doing the
// thing the link was for. Nothing else spends a use: opening the page does
// not, or a reload would burn the link somebody was still filling in.
//
// A link with no limit consumes nothing and answers nil.
func (l *Link) Consume() error {
	if l == nil || l.Uses <= 0 {
		return nil
	}
	ok, err := l.c.app.linkStore().Consume(l.ID, l.Uses)
	if err != nil {
		return err
	}
	if !ok {
		return &HTTPError{Code: http.StatusGone, Message: "this link has already been used"}
	}
	l.c.Audit("link.gastou", l.ID, Fields{"link": l.Name})
	return nil
}

// linkDenied is the one answer for every way a link can fail, and it costs the
// address a point of the budget for getting it wrong.
func (c *Ctx) linkDenied(name string) error {
	if lim := c.app.linkGuard(); lim != nil {
		if ok, after := lim.Allow(c.ClientIP()); !ok {
			c.Header("Retry-After", fmt.Sprint(after))
			return &HTTPError{Code: http.StatusTooManyRequests, Message: "too many attempts"}
		}
	}
	c.Log().Warn("trilha: link refused", "link", name, "ip", c.ClientIP())
	return &HTTPError{Code: http.StatusNotFound, Message: "invalid or expired link"}
}

// linkBody is what travels inside the token.
type linkBody struct {
	Name string            `json:"n"`
	ID   string            `json:"i"`
	Data map[string]string `json:"d,omitempty"`
	Uses int               `json:"u,omitempty"`
}

const (
	// maxLinkBody keeps a URL a URL. A token is base64 of this plus a
	// signature, and a link nobody can paste into a message is a link that
	// does not work.
	maxLinkBody = 1 << 10
)

func linkID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// pathSafe turns the signer's token into one path segment. The three parts are
// base64url already; only the separator is not, and "~" is unreserved.
func pathSafe(token string) string   { return strings.ReplaceAll(token, "|", "~") }
func fromPathSafe(seg string) string { return strings.ReplaceAll(seg, "~", "|") }

func lastSegment(path string) string {
	path = strings.TrimSuffix(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// memLinks counts uses in the process.
type memLinks struct {
	mu sync.Mutex
	m  map[string]int
}

func (s *memLinks) Used(id string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id], nil
}

func (s *memLinks) Consume(id string, max int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]int{}
	}
	if s.m[id] >= max {
		return false, nil
	}
	s.m[id]++
	return true, nil
}

func (s *memLinks) Forget(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}
