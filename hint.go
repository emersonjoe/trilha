package trilha

import "errors"

// Hint is an error that carries its own repair: a code, the sentence that says
// what to do, and where to read more.
//
// The framework already teaches at build time — the scanner's E_ codes come
// with a fix line — and in the audit. What was missing is the middle: the
// error that only happens once the app is running, where the answer today is a
// 500 and a stack trace of framework internals.
//
//	return NewHint(ErrRedirectAbsolute, err).
//		Fix("Redirect takes a path; to leave the site, RedirectExternal.").
//		Doc("/reference/errors")
//
// In Env: Dev the error page shows all three. In production it shows what it
// showed before, because the sentence is for whoever writes the code, and the
// person on the other side did not write it.
//
// A Hint wraps: errors.Is and errors.As reach straight through it, so code that
// already handled an error does not start handling a different one.
type Hint struct {
	// Code is the stable name of this failure — E_REDIRECT_ABSOLUTE — which is
	// what somebody pastes into a search box.
	Code string
	// Repair is the sentence that says what to do instead. One sentence: a
	// paragraph in an error message is a paragraph nobody finishes.
	Repair string
	// Docs is a path on the documentation site, for the whole story.
	Docs string

	err error
}

// Codes of the runtime failures that carry a repair. They are constants and
// not literals because a code that appears in two places has to be the same
// string in both — one of them being the documentation.
const (
	// ErrRedirectAbsolute is Redirect asked to leave the site. It is the shape
	// of an open redirect: the address almost always came from ?next=.
	ErrRedirectAbsolute = "E_REDIRECT_ABSOLUTE"
	// ErrSecretShort is a production app whose signing key is too short to be
	// worth signing with.
	ErrSecretShort = "E_SECRET_SHORT"
	// ErrFrozen is a write to a published version. The repair is one call —
	// open a draft — and the failure only reads as a bug without it.
	ErrFrozen = "E_VERSION_FROZEN"
)

// NewHint wraps err with a code. Fix and Doc add the rest.
func NewHint(code string, err error) *Hint { return &Hint{Code: code, err: err} }

// Fix sets the sentence that says what to do instead.
func (h *Hint) Fix(repair string) *Hint { h.Repair = repair; return h }

// Doc sets the documentation path.
func (h *Hint) Doc(path string) *Hint { h.Docs = path; return h }

// Error is the message with the code on it, so a log line carries the code
// even where nothing knows what a Hint is.
func (h *Hint) Error() string {
	if h == nil || h.err == nil {
		return "trilha: " + h.Code
	}
	return h.err.Error() + " (" + h.Code + ")"
}

// Unwrap is what makes errors.Is and errors.As see through the hint.
func (h *Hint) Unwrap() error { return h.err }

// HintOf finds the hint in an error chain, or nil. It is what the error page
// asks before deciding whether it has anything extra to say.
func HintOf(err error) *Hint {
	var h *Hint
	if errors.As(err, &h) {
		return h
	}
	return nil
}
