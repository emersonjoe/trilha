package trilha

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha/h"
)

// renderPage runs Page, wraps the result with the route's layouts (innermost
// first) and writes the document.
func (a *App) renderPage(c *Ctx, r *Route) error {
	node, err := r.Page(c)
	if err != nil {
		return err
	}
	if node == nil {
		// The page wrote the response itself (or nothing: wrap answers 204).
		return nil
	}
	for _, l := range r.Layouts {
		if node, err = l(c, node); err != nil {
			return err
		}
	}
	code := c.status
	if code == 0 {
		code = http.StatusOK
	}
	return a.writeHTML(c, code, node)
}

// writeHTML renders a node to a buffer (so a render error still yields a
// clean 500), prepends the doctype when missing and injects the dev script.
func (a *App) writeHTML(c *Ctx, code int, node h.Node) error {
	if c.w.wrote {
		// The handler already answered (http.NotFound, c.Text...): never
		// append a second document on top of it.
		return nil
	}
	var buf bytes.Buffer
	if node != nil {
		if err := node.Render(&buf); err != nil {
			return fmt.Errorf("render: %w", err)
		}
	}
	out := buf.Bytes()
	trimmed := bytes.TrimLeft(out, " \t\r\n")
	if !bytes.HasPrefix(bytes.ToLower(trimmed[:min(len(trimmed), 9)]), []byte("<!doctype")) {
		if !bytes.HasPrefix(bytes.ToLower(trimmed[:min(len(trimmed), 5)]), []byte("<html")) {
			out = append([]byte("<!doctype html><html><head><meta charset=\"utf-8\"></head><body>"), out...)
			out = append(out, []byte("</body></html>")...)
		} else {
			out = append([]byte("<!doctype html>"), out...)
		}
	}
	if a.cfg.Env == Dev && a.cfg.DevReload != Off {
		out = injectDevScript(out, c.Nonce())
	}
	c.w.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.w.WriteHeader(code)
	_, err := c.w.Write(out)
	return err
}

// wrapRoot applies the root layout, if any, to a framework-provided page.
func (a *App) wrapRoot(c *Ctx, node h.Node) (h.Node, error) {
	if a.rootLayout == nil {
		return node, nil
	}
	return a.rootLayout(c, node)
}

// handleError turns a handler error into a response.
func (a *App) handleError(c *Ctx, err error) {
	if err == nil {
		return
	}
	var re *RedirectError
	if errors.As(err, &re) {
		if !c.w.wrote {
			http.Redirect(c.w, c.r, re.URL, re.Code)
		}
		return
	}
	code := statusOf(err)
	if _, isPanic := err.(*panicError); isPanic {
		a.securityEvent(c, "panic", code)
	} else if k := kindForStatus(code); k != "" {
		a.securityEvent(c, k, code)
	}
	if c.w.wrote {
		// Response already started: nothing sensible to send.
		if code >= 500 {
			a.log.Error("handler error after write", "err", err, "path", c.r.URL.Path, "request_id", c.requestID)
		}
		return
	}
	if code >= 500 {
		a.log.Error("handler error", "err", err, "path", c.r.URL.Path, "request_id", c.requestID)
	}
	if c.kind == kindAPI {
		msg := http.StatusText(code)
		var he *HTTPError
		if errors.As(err, &he) && he.Message != "" && code < 500 {
			msg = he.Message
		}
		_ = c.JSON(code, map[string]any{"error": msg, "status": code})
		return
	}
	switch {
	case code == http.StatusNotFound:
		a.renderNotFound(c)
	case code >= 500:
		a.renderErrorPage(c, err)
	default:
		msg := http.StatusText(code)
		var he *HTTPError
		if errors.As(err, &he) && he.Message != "" {
			msg = he.Message
		}
		_ = a.writeHTML(c, code, simplePage(fmt.Sprintf("%d %s", code, http.StatusText(code)), msg, ""))
	}
}

func (a *App) renderNotFound(c *Ctx) {
	c.status = http.StatusNotFound
	if a.notFound != nil {
		node, err := a.notFound(c)
		if err == nil && node == nil {
			if c.w.wrote {
				return // the page answered by itself (any status/content type)
			}
			err = errors.New("not_found.go returned nil without writing a response")
		}
		if err == nil {
			node, err = a.wrapRoot(c, node)
		}
		if err == nil {
			if werr := a.writeHTML(c, http.StatusNotFound, node); werr == nil {
				return
			}
		}
		a.log.Error("not_found page failed", "err", err)
	}
	_ = a.writeHTML(c, http.StatusNotFound, simplePage("404 Not Found", "Página não encontrada.", c.r.URL.Path))
}

func (a *App) renderErrorPage(c *Ctx, cause error) {
	c.status = http.StatusInternalServerError
	if a.errorPage != nil {
		node, err := a.errorPage(c, cause)
		if err == nil && node == nil {
			if c.w.wrote {
				return
			}
			err = errors.New("error.go returned nil without writing a response")
		}
		if err == nil {
			node, err = a.wrapRoot(c, node)
		}
		if err == nil {
			if werr := a.writeHTML(c, http.StatusInternalServerError, node); werr == nil {
				return
			}
		}
		a.log.Error("error page failed", "err", err)
	}
	detail := ""
	if a.cfg.Env == Dev {
		detail = cause.Error()
		if st, ok := cause.(interface{ Stack() string }); ok {
			detail += "\n\n" + st.Stack()
		}
	}
	_ = a.writeHTML(c, http.StatusInternalServerError, simplePage("500 Internal Server Error", "Algo deu errado.", detail))
}

// simplePage is the framework's fallback page for 404/500.
func simplePage(title, msg, detail string) h.Node {
	return h.Html(h.Lang("pt-BR"),
		h.Head(h.Meta(h.Charset("utf-8")), h.Title(h.Text(title)),
			h.Style(h.Raw("body{font:16px/1.5 system-ui,sans-serif;max-width:48rem;margin:4rem auto;padding:0 1rem;color:#222}pre{background:#f4f4f4;padding:1rem;overflow:auto;white-space:pre-wrap}"))),
		h.Body(h.H1(h.Text(title)), h.P(h.Text(msg)),
			h.If(detail != "", h.Pre(h.Text(detail))),
			h.P(h.Small(h.Text("trilha")))))
}

// panicError wraps a recovered panic with its stack.
type panicError struct {
	value any
	stack string
}

func (p *panicError) Error() string { return fmt.Sprintf("panic: %v", p.value) }
func (p *panicError) Stack() string { return p.stack }

// devScript reconnects to the dev events endpoint and reloads on demand.
const devScript = `<script{nonce}>(function(){var d=false;function c(){var e=new EventSource('/_trilha/events');e.onmessage=function(m){if(m.data==='reload'){location.reload()}};e.onopen=function(){if(d){location.reload()}};e.onerror=function(){d=true;e.close();setTimeout(c,300)}}c()})();</script>`

func injectDevScript(out []byte, nonce string) []byte {
	if bytes.Contains(out, []byte("/_trilha/events")) {
		return out
	}
	script := strings.ReplaceAll(devScript, "{nonce}", ` nonce="`+nonce+`"`)
	i := bytes.LastIndex(bytes.ToLower(out), []byte("</body>"))
	if i < 0 {
		return append(out, []byte(script)...)
	}
	res := make([]byte, 0, len(out)+len(script))
	res = append(res, out[:i]...)
	res = append(res, []byte(script)...)
	res = append(res, out[i:]...)
	return res
}

// compileErrorPage is used by the CLI dev server; exported for reuse.
func CompileErrorPage(output string) string {
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>Erro de compilação</title>" +
		"<style>body{font:15px/1.5 system-ui,sans-serif;max-width:60rem;margin:3rem auto;padding:0 1rem;color:#222}pre{background:#2b1d1d;color:#ffd9d9;padding:1rem;overflow:auto;white-space:pre-wrap;border-radius:6px}</style></head>" +
		"<body><h1>Erro de compilação</h1><p>Corrija o código e salve: esta página recarrega sozinha.</p><pre>" +
		html.EscapeString(strings.TrimSpace(output)) + "</pre>" + strings.ReplaceAll(devScript, "{nonce}", "") + "</body></html>"
}
