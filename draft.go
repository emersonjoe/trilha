package trilha

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// A form in three screens has one hard question, and it is not the HTML: where
// does step one live while somebody is on step two? What gets written instead
// is a page full of <input type=hidden> (which the first upload breaks), or a
// half-filled row in the database (which the reports then have to learn to
// ignore), or one enormous screen with everything on it.

// ErrNoDraft is what Load answers when there is nothing to continue: never
// saved, already finished, expired, or from another browser. It is not a
// failure — it is the answer that sends somebody back to step one.
var ErrNoDraft = errors.New("trilha: no draft")

// DraftStore keeps a draft that does not fit in a cookie. Nil — the default —
// means the cookie is all there is, and a draft above the limit is an error
// that names this field.
//
// The key is opaque and generated per save; an app implementing this over
// Redis or a table stores bytes and honours the deadline, and nothing else.
type DraftStore interface {
	Save(key string, data []byte, ttl time.Duration) error
	Load(key string) ([]byte, bool)
	Delete(key string) error
}

// maxDraftCookie is how much JSON still fits a cookie. The browser limit is
// about 4 KB for the whole cookie, and what goes in it is base64 (a third
// bigger) plus the expiry and the signature — so the room for the draft
// itself is around 2.8 KB, and 2 KB is the number that leaves it comfortable
// with a name and a path in front. The issue proposed 3 KB; 3 KB of JSON
// signs into a cookie the browser drops without a word.
const maxDraftCookie = 2 << 10

// Draft is a form in progress, kept by the framework between one step and the
// next.
type Draft struct {
	c    *Ctx
	name string
}

// Draft names a multi-step form. The name is the form, not the person: two
// people filling the same wizard have their own drafts, because the draft
// travels in their own cookie.
//
//	var in Importacao
//	if err := c.Draft("import").Load(&in); err != nil {
//		return c.Redirect("/import/1") // nothing to continue
//	}
//
//	c.Draft("import").Save(in, 30*time.Minute) // end of a step
//	c.Draft("import").Clear()                  // it became a record
//
// A draft is signed, so it cannot be edited by hand — and it is **not
// secret**: what goes in a cookie travels to the browser and can be read
// there. Keep a price, a discount or somebody else's name out of it, or give
// Config.Drafts a store and let the cookie carry only the key.
//
//	see: Ctx.Bind, ui.Steps
func (c *Ctx) Draft(name string) *Draft { return &Draft{c: c, name: name} }

// Save writes the draft and starts its clock. The value is anything
// encoding/json can write — the same struct Bind fills, usually, so a step
// validates on the way in and the draft holds what passed.
//
// Under 2 KB of JSON it goes in a signed cookie. Above that it needs
// Config.Drafts; without one, the error says so rather than quietly dropping a
// cookie the browser would refuse.
func (d *Draft) Save(v any, ttl time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("trilha: draft %s: %w", d.name, err)
	}
	if len(raw) <= maxDraftCookie {
		if err := d.c.SetSigned(d.cookie(), string(raw), ttl); err != nil {
			if errors.Is(err, ErrNoSecret) {
				// Without a secret there is nowhere to put the draft, and the
				// step would fail with a 500 that says nothing. Saying which
				// draft and which variable turns it into a five-second fix.
				return fmt.Errorf("trilha: draft %s needs TRILHA_SECRET to be signed: %w", d.name, err)
			}
			return err
		}
		return nil
	}
	store := d.c.app.cfg.Drafts
	if store == nil {
		return fmt.Errorf("trilha: draft %s is %d bytes and a cookie holds %d; give Config.Drafts a store",
			d.name, len(raw), maxDraftCookie)
	}
	key, err := draftKey()
	if err != nil {
		return err
	}
	if err := store.Save(key, raw, ttl); err != nil {
		return fmt.Errorf("trilha: draft %s: %w", d.name, err)
	}
	// A deliberate choice, recorded once: whoever reads the boot log can see
	// that this form outgrew the cookie, which is also the first thing to
	// look at when a step starts losing state in production.
	d.c.app.infoOnce("draft:"+d.name, "trilha: draft kept in Config.Drafts, too large for a cookie",
		"draft", d.name, "bytes", len(raw))
	return d.c.SetSigned(d.cookie(), draftRef+key, ttl)
}

// Load fills v with what the last step saved, or answers ErrNoDraft. A draft
// somebody tampered with is ErrNoDraft too: the signature failed, and there is
// nothing to continue.
func (d *Draft) Load(v any) error {
	raw, ok := d.c.Signed(d.cookie())
	if !ok {
		return ErrNoDraft
	}
	if key, isRef := strings.CutPrefix(raw, draftRef); isRef {
		store := d.c.app.cfg.Drafts
		if store == nil {
			return ErrNoDraft
		}
		b, ok := store.Load(key)
		if !ok {
			return ErrNoDraft
		}
		raw = string(b)
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		// A draft written by an older version of the struct is not a 500: the
		// field was renamed between deploys and this person has a cookie from
		// before. Back to step one is the only honest answer.
		return ErrNoDraft
	}
	return nil
}

// Clear ends the draft: the step that turned it into a record calls this, and
// the next visit to step two starts over instead of resuming something that
// already happened.
func (d *Draft) Clear() {
	if raw, ok := d.c.Signed(d.cookie()); ok {
		if key, isRef := strings.CutPrefix(raw, draftRef); isRef {
			if store := d.c.app.cfg.Drafts; store != nil {
				_ = store.Delete(key)
			}
		}
	}
	d.c.ClearCookie(d.cookie())
}

// draftRef marks a cookie that holds a key instead of the draft. A colon
// cannot appear in the base64 of a key, so the two can never be confused.
const draftRef = "ref:"

func (d *Draft) cookie() string { return "_draft_" + safeCookieName(d.name) }

// safeCookieName keeps a name from becoming a second cookie attribute. A draft
// name is written by the developer, not by a request, so this is a guard
// against a typo rather than against an attacker.
func safeCookieName(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		}
		return '_'
	}, s)
}

func draftKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
