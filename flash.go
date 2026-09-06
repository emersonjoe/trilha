package trilha

import (
	"encoding/base64"
	"encoding/json"
	"time"
	"unicode/utf8"
)

// flashCookie carries the messages from the request that wrote them to the
// one that shows them — the redirect in between is the whole point.
const flashCookie = "trilha_flash"

const (
	flashTTL = 5 * time.Minute
	// A cookie the browser refuses loses every message, so the list is
	// bounded on both axes and the oldest message is the one dropped.
	maxFlashes    = 5
	maxFlashRunes = 200
)

// Flash is a message for the page the browser is about to load: the line that
// says what the POST did, read once and then gone.
type Flash struct {
	Kind string `json:"k"` // "", "success" or "error" (see ui.FlashSuccess)
	Text string `json:"t"` // plain text; whoever renders it escapes it
}

// Flash queues a message for the next request, in a signed cookie of its own.
// It is the answer to "the POST worked, and the redirect ate the news":
//
//	if err := posts.Delete(c.Context(), slug); err != nil {
//		return err
//	}
//	c.Flash(ui.FlashSuccess, "Post deleted")
//	return c.Redirect("/blog")
//
// The page that follows shows them with ui.Flashes(c). There is no error to
// handle: without TRILHA_SECRET the message is not written and the app says so
// once in the log, the same as SetSigned. On a fragment response there is no
// redirect to survive, so the messages travel in a header and ui.js shows them.
func (c *Ctx) Flash(kind, text string) {
	if text == "" {
		return
	}
	if utf8.RuneCountInString(text) > maxFlashRunes {
		text = string([]rune(text)[:maxFlashRunes])
	}
	c.flashOut = append(c.flashOut, Flash{Kind: kind, Text: text})
	if n := len(c.flashOut); n > maxFlashes {
		c.flashOut = c.flashOut[n-maxFlashes:]
	}
}

// Flashes returns the messages left by the previous request, plus any this one
// wrote and has not sent yet — a handler that flashes and renders instead of
// redirecting shows the message on the spot. Reading them takes them: the
// cookie is cleared, and calling it again gives the same list, because a layout
// does not know who read first.
func (c *Ctx) Flashes() []Flash {
	if !c.flashRead {
		c.flashRead = true
		if v, ok := c.Signed(flashCookie); ok {
			c.flashIn = decodeFlashes(v)
			c.ClearCookie(flashCookie)
		} else if _, err := c.r.Cookie(flashCookie); err == nil {
			c.ClearCookie(flashCookie) // expired or tampered: drop it, quietly
		}
	}
	if len(c.flashOut) > 0 {
		c.flashIn = append(c.flashIn, c.flashOut...)
		c.flashOut = nil // shown here; nothing left to carry over
	}
	return c.flashIn
}

// writeFlashes runs once, just before the header goes out: what nobody
// rendered in this response is what the next request has to receive.
func (c *Ctx) writeFlashes() {
	if len(c.flashOut) == 0 {
		return
	}
	v := encodeFlashes(c.flashOut)
	c.flashOut = nil
	// A fragment answer has no redirect for a cookie to survive — unless it
	// is telling the browser to navigate after all, and then the page that
	// arrives is the one that has to show the message.
	if c.Fragment() != "" && c.w.Header().Get(locationHeader) == "" {
		c.w.Header().Set(flashHeader, v)
		return
	}
	_ = c.SetSigned(flashCookie, v, flashTTL)
}

func encodeFlashes(f []Flash) string {
	b, err := json.Marshal(f)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeFlashes(v string) []Flash {
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil
	}
	var f []Flash
	if err := json.Unmarshal(b, &f); err != nil {
		return nil
	}
	if len(f) > maxFlashes {
		f = f[len(f)-maxFlashes:]
	}
	return f
}
