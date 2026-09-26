package approval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
)

// A decision link is approval without an account: "confirm your
// subscription", "approve the payment", "accept the invitation", the owner of
// a site with no login who approves a trial from an e-mail. Whoever holds the
// link decides, once.
//
// The application writes the page, where every other page lives:
//
//	// app/aprovar/{token}/page.go
//	func Page(c *trilha.Ctx) (h.Node, error) {
//		r, err := Fila.ByLink(c, c.Param("token"))
//		if err != nil {
//			return nil, err // 404
//		}
//		return formulario(r), nil // shows; the buttons POST with the CSRF field
//	}
//
//	func POST(c *trilha.Ctx) error {
//		return Fila.DecideByLink(c, c.Param("token"), c.Form("decisao"), c.Form("motivo"))
//	}
//
// GET only shows. The decision is a POST, which a page route checks for CSRF:
// the link preview of a mail client fetches the address, and fetching must
// not approve anything.

// ErrLinkInvalid is every way a link fails — unknown, used, expired, or for a
// request that is no longer pending — as the same 404. Saying which one tells
// whoever is guessing how close they got.
var ErrLinkInvalid = trilha.Errorf(http.StatusNotFound, "this link is not valid")

// LinkOptions is who gets a link, and for how long.
type LinkOptions struct {
	// To is who the link is sent to — an address. The decision is recorded as
	// By "link:"+To, so the trail says who decided without inventing a
	// session.
	To string
	// TTL is how long the link works (default 7 days). An expired link is
	// forgotten by the deadline clock.
	TTL time.Duration
}

// Link is one link as the store keeps it: the hash of the token, never the
// token. A copy of the table is not a set of working links.
type Link struct {
	Hash    string
	ID      string
	To      string
	Expires time.Time
}

// LinkStore is where the links live. Memory implements it; a Store that
// implements it too keeps the links next to the requests, and one that does
// not gets them in memory.
type LinkStore interface {
	SaveLink(ctx context.Context, l Link) error
	// GetLink answers ErrLinkInvalid for a hash it does not know.
	GetLink(ctx context.Context, hash string) (Link, error)
	// TakeLink is GetLink and delete in one step, so two clicks on the same
	// link are one decision.
	TakeLink(ctx context.Context, hash string) (Link, error)
	// PurgeLinks forgets the links expired at now, and answers how many.
	PurgeLinks(ctx context.Context, now time.Time) (int, error)
}

// Link issues a decision link for a pending request and answers its token:
// 32 random bytes, URL-safe, for the application to put in the address it
// mails. Only the SHA-256 of it is stored.
func (a *Approvals) Link(ctx context.Context, id string, o LinkOptions) (string, error) {
	if o.To == "" {
		return "", errors.New("approval: a link needs LinkOptions.To — who it is sent to")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rec, err := a.store.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if rec.State != Pending {
		return "", ErrNotYours
	}
	if o.TTL <= 0 {
		o.TTL = 7 * 24 * time.Hour
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	if err := a.links.SaveLink(ctx, Link{Hash: hashToken(token), ID: id, To: o.To, Expires: a.now().Add(o.TTL)}); err != nil {
		return "", err
	}
	return token, nil
}

// ByLink answers the request a link is for, to show it. It decides nothing,
// and it marks the response private: the token is in the address, so the page
// is not indexed, not cached, and does not leak the address as a Referer to
// whatever it links to.
func (a *Approvals) ByLink(c *trilha.Ctx, token string) (Record, error) {
	private(c)
	l, err := a.links.GetLink(ctxOf(c), hashToken(token))
	if err != nil {
		return Record{}, ErrLinkInvalid
	}
	return a.live(c, l)
}

// DecideByLink decides the request with the link, once: the link is used up
// even when the request needs more votes. The decision is recorded as By
// "link:<recipient>".
func (a *Approvals) DecideByLink(c *trilha.Ctx, token, state, reason string) error {
	private(c)
	switch state {
	case Approved, Rejected:
	default:
		return trilha.Errorf(http.StatusBadRequest, "%q is not a decision", state)
	}
	ctx := ctxOf(c)
	l, err := a.links.TakeLink(ctx, hashToken(token))
	if err != nil {
		return ErrLinkInvalid
	}
	rec, err := a.live(c, l)
	if err != nil {
		return err
	}
	if err := a.decide(c, rec, "link:"+l.To, state, reason); err != nil {
		if errors.Is(err, ErrAlreadyVoted) {
			return ErrLinkInvalid
		}
		// The store failed: the link goes back, so the person can try again.
		if serr := a.links.SaveLink(ctx, l); serr != nil {
			a.log.Error("approval: a link was lost after a failed decision", "id", l.ID, "err", serr)
		}
		return err
	}
	return nil
}

// live is the request of a link that still works.
func (a *Approvals) live(c *trilha.Ctx, l Link) (Record, error) {
	if !a.now().Before(l.Expires) {
		return Record{}, ErrLinkInvalid
	}
	rec, err := a.store.Get(ctxOf(c), l.ID)
	if err != nil || rec.State != Pending {
		return Record{}, ErrLinkInvalid
	}
	return rec, nil
}

// private is the headers of a page whose address is a credential.
func private(c *trilha.Ctx) {
	if c == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex, nofollow")
}

// hashToken is what the store keys by. A lookup by hash is not a comparison
// of the secret: the timing of a map says nothing about a token nobody holds.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
