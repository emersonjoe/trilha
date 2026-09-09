package trilha

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrNotFound makes the framework respond with 404 using the app's not-found
// page (HTML routes) or a JSON error (API routes).
var ErrNotFound = errors.New("trilha: not found")

// RedirectError is returned by handlers to redirect the client.
type RedirectError struct {
	URL  string
	Code int
}

func (e *RedirectError) Error() string {
	return fmt.Sprintf("trilha: redirect %d to %s", e.Code, e.URL)
}

// Redirect returns a 303 See Other redirect error (POST → redirect → GET).
//
// It takes a path and refuses an address that leaves the site. That refusal is
// the point: the destination of a redirect almost always came from a form or a
// query string — `?next=` is how somebody gets back to the page that asked
// them to log in — and a redirect that follows it anywhere is an open redirect,
// which is a phishing link on your own domain.
//
// Sanitising instead of refusing is how an open redirect gets written with
// more steps. "//evil.com", "/\evil.com" and "https:/\evil.com" all exist
// because each of them walked past a check somebody thought was enough. What
// passes here is a path: it starts with "/" and does not start with "//" or
// "/\".
//
// To leave the site on purpose, say so:
//
//	return c.RedirectExternal("https://gov.example/pay")
func Redirect(url string) error { return RedirectCode(url, http.StatusSeeOther) }

// RedirectCode returns a redirect error with a custom 3xx status, and refuses
// the same addresses Redirect refuses.
func RedirectCode(url string, code int) error {
	if !localPath(url) {
		return NewHint(ErrRedirectAbsolute,
			fmt.Errorf("trilha: refusing to redirect to %q, which leaves this site", url)).
			Fix("Redirect takes a path, like \"/painel\". To leave the site on purpose, RedirectExternal.").
			Doc("/reference/errors")
	}
	return &RedirectError{URL: url, Code: code}
}

// RedirectExternal redirects to another site. It is a separate function so
// that leaving is written down: a reviewer reading RedirectExternal knows
// somebody meant it, and a reviewer reading Redirect knows nobody could have.
//
// It does not check the address, because there is nothing to check — it is a
// literal in your code, not a value from a request. Handing it something a
// visitor sent is the whole thing Redirect exists to prevent, done on purpose.
func RedirectExternal(url string) error {
	return &RedirectError{URL: url, Code: http.StatusSeeOther}
}

// localPath reports whether url is a path on this site.
//
// The rules are short because every extra rule is a place for a bypass to
// live. A path starts with a single "/" and its second character is not
// another slash or a backslash: "//evil.com" is a protocol-relative URL, and
// browsers read "/\evil.com" as one too.
func localPath(url string) bool {
	if url == "" || url[0] != '/' {
		return false
	}
	if len(url) > 1 && (url[1] == '/' || url[1] == '\\') {
		return false
	}
	return true
}

// HTTPError carries an HTTP status and a message safe to show to the client.
type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("trilha: %d %s", e.Code, http.StatusText(e.Code))
	}
	return fmt.Sprintf("trilha: %d %s", e.Code, e.Message)
}

// Errorf builds an HTTPError with a formatted client-visible message.
func Errorf(code int, format string, a ...any) error {
	return &HTTPError{Code: code, Message: fmt.Sprintf(format, a...)}
}

// StatusOf reports the HTTP status the framework will send for err. It is what
// app/error.go asks when the page differs by status — a 403 with the app's own
// wording, say — since the page receives the error, not the code.
func StatusOf(err error) int { return statusOf(err) }

// statusOf classifies an error into an HTTP status code.
func statusOf(err error) int {
	var he *HTTPError
	var fe FieldErrors
	var p *Problem
	switch {
	case err == nil:
		return http.StatusOK
	case errors.As(err, &p) && p.Status != 0:
		return p.Status
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.As(err, &fe):
		return http.StatusUnprocessableEntity
	case errors.As(err, &he):
		return he.Code
	case errors.As(err, new(*http.MaxBytesError)):
		// A handler reading the body itself and hitting the limit means the
		// request was too big, not that the app broke.
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}
