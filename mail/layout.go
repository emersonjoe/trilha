package mail

import (
	"os"
	"strings"

	"github.com/emersonjoe/trilha/h"
)

// Layout is the transactional email nobody should have to write twice: a
// header with the brand, the body, and a footer saying why the message
// arrived.
//
// It is built the way email clients still demand rather than the way a page is
// built, and that is the whole point of it existing. Outlook renders with
// Word, which knows no flexbox, no grid and no float; Gmail strips <style> out
// of the head. So: a centred table, widths in pixels, every rule inline. Doing
// that once here is what keeps it out of the application.
func Layout(brand string, body ...h.Node) h.Node {
	return h.Fragment(
		h.Doctype(),
		h.Html(h.Attr("lang", locale()),
			h.Head(
				h.Meta(h.Attr("charset", "utf-8")),
				h.Meta(h.Attr("name", "viewport"), h.Attr("content", "width=device-width")),
				h.Title(h.Text(brand)),
			),
			h.Body(h.Attr("style", "margin:0;padding:0;background:#f4f4f5;"+font),
				// A table and not a div: this is the element Outlook centres.
				h.Table(h.Attr("role", "presentation"), h.Attr("width", "100%"),
					h.Attr("cellpadding", "0"), h.Attr("cellspacing", "0"),
					h.Attr("style", "background:#f4f4f5;padding:24px 12px;"),
					h.Tr(h.Td(h.Attr("align", "center"),
						h.Table(h.Attr("role", "presentation"), h.Attr("width", "600"),
							h.Attr("cellpadding", "0"), h.Attr("cellspacing", "0"),
							h.Attr("style", "width:600px;max-width:100%;background:#ffffff;border:1px solid #e4e4e7;border-radius:8px;"),
							h.Tr(h.Td(h.Attr("style", "padding:20px 28px;border-bottom:1px solid #e4e4e7;font-size:16px;font-weight:600;color:#18181b;"),
								h.Text(brand))),
							h.Tr(h.Td(h.Attr("style", "padding:28px;font-size:15px;line-height:1.55;color:#27272a;"),
								h.Fragment(body...))),
							h.Tr(h.Td(h.Attr("style", "padding:16px 28px;border-top:1px solid #e4e4e7;font-size:12px;line-height:1.5;color:#71717a;"),
								h.Text(footer(brand)))),
						),
					)),
				),
			),
		),
	)
}

// Button is the call to action, as a link that looks like a button. It is an
// <a> and not a <button>: a form control in an email does nothing, and a link
// styled like a button works everywhere including the clients that refuse the
// styling and show it as a plain link.
func Button(label, url string) h.Node {
	return h.P(h.Attr("style", "margin:24px 0;"),
		h.A(h.Href(url), h.Attr("style", "display:inline-block;padding:11px 20px;background:#18181b;color:#ffffff;text-decoration:none;border-radius:6px;font-size:15px;font-weight:500;"),
			h.Text(label)),
	)
}

// Muted is the small print under the button: the address to paste when the
// button does not survive the client, the note about who asked for this.
func Muted(children ...h.Node) h.Node {
	return h.P(h.Attr("style", "margin:16px 0 0;font-size:13px;line-height:1.5;color:#71717a;word-break:break-all;"),
		h.Fragment(children...))
}

const font = "font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;"

// footer says why the message arrived, in the language the app is written in.
// "You received this because" is not a courtesy: it is the line that stops the
// recipient reporting a legitimate message as spam.
func footer(brand string) string {
	if strings.HasPrefix(locale(), "pt") {
		return "Você recebeu este e-mail porque tem uma conta no " + brand + ". Não responda a esta mensagem."
	}
	return "You received this email because you have an account on " + brand + ". Please do not reply to this message."
}

// locale picks the language of the words this package writes on its own — the
// footer, and nothing else. It reads the same variables the CLI reads, because
// a second way of saying which language an app speaks is a second thing to
// get wrong.
//
// An application that wants its own footer, or a third language, writes its
// own layout: it is the function above with the words changed, and copying
// twenty lines is cheaper than a translation mechanism nobody asked for.
func locale() string {
	for _, k := range []string{"TRILHA_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.ToLower(os.Getenv(k)); strings.HasPrefix(v, "pt") {
			return "pt-BR"
		} else if v != "" {
			return "en"
		}
	}
	return "en"
}
