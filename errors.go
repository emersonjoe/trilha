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
func Redirect(url string) error { return &RedirectError{URL: url, Code: http.StatusSeeOther} }

// RedirectCode returns a redirect error with a custom 3xx status.
func RedirectCode(url string, code int) error { return &RedirectError{URL: url, Code: code} }

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

// statusOf classifies an error into an HTTP status code.
func statusOf(err error) int {
	var he *HTTPError
	var fe FieldErrors
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.As(err, &fe):
		return http.StatusUnprocessableEntity
	case errors.As(err, &he):
		return he.Code
	default:
		return http.StatusInternalServerError
	}
}
