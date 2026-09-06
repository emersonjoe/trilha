package trilha

import (
	"net/http"
	"strings"
	"time"
)

// ETag declares the version of what this route is about to answer and reports
// whether the browser already has it:
//
//	func Page(c *trilha.Ctx) (h.Node, error) {
//		p, ok := posts.Get(c.Param("slug"))
//		if !ok {
//			return nil, trilha.ErrNotFound
//		}
//		if c.ETag(p.Rev) {
//			return nil, nil // 304: nothing else to answer
//		}
//		return pagina(p), nil
//	}
//
// The tag is whatever identifies the version of the data — a revision, an
// updated_at, a hash of the row. It is not the hash of the response: the CSP
// nonce changes on every request, so a body hash would never match twice.
//
// Quotes are added when missing, and a tag already written as "abc" or W/"abc"
// is sent as it is. Only GET and HEAD answer 304; on any other method the
// header is written and the return is false.
func (c *Ctx) ETag(tag string) bool {
	if tag == "" {
		return false
	}
	tag = quoteETag(tag)
	c.w.Header().Set("ETag", tag)
	if !c.revalidating() {
		return false
	}
	inm := c.r.Header.Get("If-None-Match")
	if inm == "" || !etagMatches(inm, tag) {
		return false
	}
	c.notModified()
	return true
}

// LastModified declares when what this route answers last changed, and reports
// whether the browser already has that version. Same contract as ETag, with
// two differences: the comparison happens at the second, which is all an HTTP
// date carries, and a request that brought If-None-Match is left to it — a
// strong validator is not overruled by a weak one (RFC 9110 §13.1.3).
func (c *Ctx) LastModified(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	t = t.UTC().Truncate(time.Second)
	c.w.Header().Set("Last-Modified", t.Format(http.TimeFormat))
	if !c.revalidating() || c.r.Header.Get("If-None-Match") != "" {
		return false
	}
	since, err := http.ParseTime(c.r.Header.Get("If-Modified-Since"))
	if err != nil {
		return false // no date, or one nobody can read: not a question
	}
	if t.After(since) {
		return false
	}
	c.notModified()
	return true
}

// CacheControl sets the response policy. A page carrying data that belongs to
// one person needs private in it: without that, a shared cache is allowed to
// hand one visitor's page to the next one.
func (c *Ctx) CacheControl(v string) { c.w.Header().Set("Cache-Control", v) }

// revalidating reports a method that can be answered with 304. A POST that
// carries If-None-Match is asking about writing, which is another story.
func (c *Ctx) revalidating() bool {
	return c.r.Method == http.MethodGet || c.r.Method == http.MethodHead
}

// notModified writes the empty answer. The validator headers are already set,
// and Vary goes with them: a fragment and the page it belongs to are two
// entries, and a cache that forgets that serves one in place of the other.
func (c *Ctx) notModified() {
	hd := c.w.Header()
	hd.Set("Vary", fragmentHeader)
	hd.Del("Content-Type")
	hd.Del("Content-Length")
	c.w.WriteHeader(http.StatusNotModified)
}

// quoteETag puts the tag in the shape the header wants, leaving alone one that
// already arrived that way.
func quoteETag(tag string) string {
	if strings.HasPrefix(tag, `W/"`) || (strings.HasPrefix(tag, `"`) && strings.HasSuffix(tag, `"`) && len(tag) > 1) {
		return tag
	}
	return `"` + strings.Trim(tag, `"`) + `"`
}

// etagMatches compares the way RFC 9110 §8.8.3.2 asks for If-None-Match: a
// list, "*" for anything, and weakly — W/"x" and "x" are the same version of
// the same page.
func etagMatches(header, tag string) bool {
	want := strings.TrimPrefix(tag, "W/")
	for _, part := range strings.Split(header, ",") {
		p := strings.TrimSpace(part)
		if p == "*" || strings.TrimPrefix(p, "W/") == want {
			return true
		}
	}
	return false
}
