package ui

import (
	"sort"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// ConnectionsOpts is what the panel needs to post back.
type ConnectionsOpts struct {
	// Path is where every button posts: save, test and delete, each with a
	// hidden "_action" and the "id". One route, three actions, because the
	// page that owns the list is the one that owns its changes.
	Path string
	// CSRF is the hidden token field, trilha.CSRFInput(c). It is an option
	// so the panel stays drawable from a test, and so that leaving it out is
	// a visible omission rather than a silent one.
	CSRF h.Node
	// Editing is the connection the form shows, or nil for a new one. A form
	// re-rendered after a 422 passes back what was typed, with Errors.
	Editing *trilha.Connection
	// Errors are the field errors of the last save, by field name.
	Errors map[string]string
}

// ConnectionsPanel is the screen of trilha.Connections: the list, grouped by
// kind, with the last test as a badge and the buttons; and the form of one
// connection, new or being edited.
//
//	ui.ConnectionsPanel(c, conexoes.Conexoes, ui.ConnectionsOpts{
//		Path: "/admin/conexoes", CSRF: trilha.CSRFInput(c), Editing: editing, Errors: errs,
//	})
//
// The secret is never in the HTML: the field is ui.SecretField, which renders
// empty and says "leave blank to keep". Everything works without script; the
// auth-specific fields (user, header name) show and hide with ShowWhen when
// it is there.
func ConnectionsPanel(c *trilha.Ctx, x *trilha.Connections, o ConnectionsOpts) h.Node {
	pt := langOf(c) == "pt-BR"
	list, err := x.List(c)
	if err != nil {
		return EmptyError(c, word(pt, "Connections could not be loaded", "As conexões não puderam ser carregadas"), err, nil)
	}
	return h.Div(h.Class("ui-connections"),
		connectionsList(c, x, list, o, pt),
		connectionForm(c, x, o, pt))
}

func connectionsList(c *trilha.Ctx, x *trilha.Connections, list []trilha.Connection, o ConnectionsOpts, pt bool) h.Node {
	if len(list) == 0 {
		return Empty(EmptyOpts{
			Icon:  "info",
			Title: word(pt, "No connections yet", "Nenhuma conexão ainda"),
			Hint: word(pt, "A connection is an external service this application talks to: an API, an MCP server, a provider.",
				"Uma conexão é um serviço externo com que esta aplicação fala: uma API, um servidor MCP, um provedor."),
		})
	}
	byKind := map[string][]trilha.Connection{}
	for _, conn := range list {
		byKind[conn.Kind] = append(byKind[conn.Kind], conn)
	}
	var groups []h.Node
	for _, k := range x.Kinds() {
		rows := byKind[k.Key]
		if len(rows) == 0 {
			continue
		}
		delete(byKind, k.Key)
		groups = append(groups, connectionGroup(c, k.Label, k.Test != nil, rows, o, pt))
	}
	// A kind that was declared once and is not any more still has rows; they
	// are listed under their key so nobody loses a record to a rename.
	var orphans []string
	for key := range byKind {
		orphans = append(orphans, key)
	}
	sort.Strings(orphans)
	for _, key := range orphans {
		groups = append(groups, connectionGroup(c, key, false, byKind[key], o, pt))
	}
	return h.Div(append([]h.Node{h.Class("ui-connections-list")}, groups...)...)
}

func connectionGroup(c *trilha.Ctx, label string, testable bool, rows []trilha.Connection, o ConnectionsOpts, pt bool) h.Node {
	head := h.Thead(h.Tr(
		h.Th(h.Text(word(pt, "Name", "Nome"))),
		h.Th(h.Text("URL")),
		h.Th(h.Text(word(pt, "Auth", "Autenticação"))),
		h.Th(h.Text(word(pt, "Last test", "Último teste"))),
		h.Th(h.Text("")),
	))
	body := make([]h.Node, 0, len(rows))
	for _, conn := range rows {
		body = append(body, h.Tr(
			h.Td(h.Text(conn.Name)),
			h.Td(Code(conn.URL)),
			h.Td(h.Text(trilha.ConnectionAuths.Label(conn.Auth))),
			h.Td(ConnectionStatus(c, conn.LastTest)),
			h.Td(h.Class("ui-connections-actions"),
				ButtonLink(o.Path+"?edit="+conn.ID, Outline(), Sm(), h.Text(word(pt, "Edit", "Editar"))),
				h.If(testable && o.Path != "", connectionAction(o, conn.ID, "test", Button(Outline(), Sm(), h.Type("submit"), h.Text(word(pt, "Test", "Testar"))))),
				h.If(o.Path != "", connectionAction(o, conn.ID, "delete", h.Fragment(
					Confirm(word(pt, "Delete this connection?", "Apagar esta conexão?"),
						word(pt, "Anything using it stops working immediately.", "O que estiver usando ela para de funcionar na hora.")),
					Button(Destructive(), Sm(), h.Type("submit"), h.Text(word(pt, "Delete", "Apagar")))))),
			),
		))
	}
	return h.Section(h.Class("ui-connections-group"),
		h.H3(h.Class("ui-connections-kind"), h.Text(label)),
		Table(head, h.Tbody(body...)))
}

func connectionAction(o ConnectionsOpts, id, action string, button h.Node) h.Node {
	return h.Form(h.Method("post"), h.Action(o.Path), h.Class("ui-inline"), o.CSRF,
		h.Input(h.Type("hidden"), h.Name("_action"), h.Value(action)),
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(id)),
		button)
}

// ConnectionStatus is the badge of a test result: never run, passed, or
// failed with the message as the title.
func ConnectionStatus(c *trilha.Ctx, t *trilha.ConnectionTest) h.Node {
	pt := langOf(c) == "pt-BR"
	if t == nil {
		return h.Span(h.Class("ui-badge ui-badge-muted"), h.Text(word(pt, "never tested", "nunca testada")))
	}
	if t.OK {
		return h.Span(h.Class("ui-badge ui-badge-success"), h.TitleAttr(when(c, t.At)), h.Text("ok"))
	}
	return h.Span(h.Class("ui-badge ui-badge-danger"), h.TitleAttr(t.Message+" · "+when(c, t.At)), h.Text(word(pt, "failed", "falhou")))
}

// when is the test's moment for a title attribute: relative, in the
// viewer's language.
func when(c *trilha.Ctx, t time.Time) string {
	return relativeText(langOf(c), time.Since(t))
}

func connectionForm(c *trilha.Ctx, x *trilha.Connections, o ConnectionsOpts, pt bool) h.Node {
	var conn trilha.Connection
	if o.Editing != nil {
		conn = *o.Editing
	}
	errs := o.Errors
	if errs == nil {
		errs = map[string]string{}
	}
	kinds := x.Kinds()
	kindOpts := make([]Option, 0, len(kinds))
	auths := map[string]bool{}
	for _, k := range kinds {
		kindOpts = append(kindOpts, Option{Value: k.Key, Label: k.Label})
		if conn.Kind == "" || conn.Kind == k.Key {
			for _, a := range k.Auth {
				auths[a] = true
			}
		}
	}
	if conn.Kind == "" && len(kinds) > 0 {
		conn.Kind = kinds[0].Key
	}
	var authOpts []Option
	for _, v := range trilha.ConnectionAuths {
		if auths[v.Value] {
			authOpts = append(authOpts, Option{Value: v.Value, Label: authLabel(v.Value, pt)})
		}
	}
	title := word(pt, "New connection", "Nova conexão")
	if conn.ID != "" {
		title = word(pt, "Edit connection", "Editar conexão")
	}
	var headers []string
	for k, v := range conn.Headers {
		headers = append(headers, k+": "+v)
	}
	sort.Strings(headers)

	return Card(h.Class("ui-connections-form"),
		CardHeader(CardTitle(title)),
		CardContent(h.Form(h.Method("post"), h.Action(o.Path), o.CSRF,
			h.Input(h.Type("hidden"), h.Name("_action"), h.Value("save")),
			h.If(conn.ID != "", h.Input(h.Type("hidden"), h.Name("id"), h.Value(conn.ID))),
			Field("kind", word(pt, "Kind", "Tipo"),
				Select(h.ID("kind"), h.Name("kind"), InvalidIf(errs, "kind"), SelectOptions(kindOpts, conn.Kind)),
				Errors(errs, "kind")),
			Field("name", word(pt, "Name", "Nome"),
				Input(h.ID("name"), h.Name("name"), h.Value(conn.Name), h.Required(), InvalidIf(errs, "name")),
				Errors(errs, "name")),
			Field("url", "URL",
				Input(h.ID("url"), h.Name("url"), h.Type("url"), h.Value(conn.URL), h.Placeholder("https://"), h.Required(), InvalidIf(errs, "url")),
				Help(word(pt, "The base address. Paths are added by whoever calls it.", "O endereço base. Os caminhos quem chama acrescenta.")),
				Errors(errs, "url")),
			Field("auth", word(pt, "Auth", "Autenticação"),
				Select(h.ID("auth"), h.Name("auth"), InvalidIf(errs, "auth"), SelectOptions(authOpts, conn.Auth)),
				Errors(errs, "auth")),
			Field("username", word(pt, "User", "Usuário"),
				Input(h.ID("username"), h.Name("username"), h.Value(conn.Username), InvalidIf(errs, "username")),
				Errors(errs, "username"), With(ShowWhen("auth", "basic"))),
			Field("header", word(pt, "Header name", "Nome do cabeçalho"),
				Input(h.ID("header"), h.Name("header"), h.Value(conn.Header), h.Placeholder("X-Api-Key"), InvalidIf(errs, "header")),
				Errors(errs, "header"), With(ShowWhen("auth", "header"))),
			SecretField(c, "secret", word(pt, "Secret", "Segredo"), conn.Secret, Errors(errs, "secret"), With(ShowWhen("auth", "bearer", "header", "basic"))),
			Field("headers", word(pt, "Fixed headers", "Cabeçalhos fixos"),
				Textarea(h.ID("headers"), h.Name("headers"), h.Rows("2"), h.Placeholder("X-Tenant: acme"), h.Text(strings.Join(headers, "\n")), InvalidIf(errs, "headers")),
				Help(word(pt, "One per line, Name: value. Not a place for a secret.", "Um por linha, Nome: valor. Não é lugar de segredo.")),
				Errors(errs, "headers")),
			h.Div(h.Class("ui-connections-buttons"),
				Button(h.Type("submit"), h.Text(word(pt, "Save", "Salvar"))),
				h.If(conn.ID != "", ButtonLink(o.Path, Outline(), h.Text(word(pt, "Cancel", "Cancelar"))))),
		)))
}

func authLabel(v string, pt bool) string {
	switch v {
	case "none":
		return word(pt, "None", "Nenhuma")
	case "bearer":
		return word(pt, "Bearer token", "Token Bearer")
	case "header":
		return word(pt, "Header", "Cabeçalho")
	case "basic":
		return word(pt, "Basic (user and password)", "Basic (usuário e senha)")
	}
	return v
}

// ParseConnectionForm reads the panel's form into a Connection, which is
// what the page hands to Connections.Save. The fixed headers come one per
// line as "Name: value".
func ParseConnectionForm(c *trilha.Ctx) trilha.Connection {
	conn := trilha.Connection{
		ID:       c.Form("id"),
		Kind:     c.Form("kind"),
		Name:     c.Form("name"),
		URL:      c.Form("url"),
		Auth:     c.Form("auth"),
		Username: c.Form("username"),
		Header:   c.Form("header"),
		Secret:   trilha.Secret(c.Form("secret")),
	}
	for _, line := range strings.Split(c.Form("headers"), "\n") {
		k, v, ok := strings.Cut(line, ":")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" {
			continue
		}
		if conn.Headers == nil {
			conn.Headers = map[string]string{}
		}
		conn.Headers[k] = v
	}
	return conn
}
