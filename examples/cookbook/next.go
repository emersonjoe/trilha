package cookbook

import (
	"context"
	"io/fs"
	"os"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// The declarations below are the Trilha half of "From Next.js to Trilha":
// each one answers a React pattern from the same page. The app being moved
// is a dashboard of articles — a list with a filter, a queue that refreshes
// itself, a confirmation modal and a PDF.

// DashQuery is the whole state the dashboard used to keep in React: the page,
// the ordering, the search box and the type filter. Embedding ListParams is
// what makes them live in the address instead of in memory — the visitor can
// bookmark the screen, reload it, or send it to somebody else.
type DashQuery struct {
	trilha.ListParams
	Status string `form:"status"`
}

// Dashboard is app/dashboard/page.go. There is no useEffect, no loading flag
// and no error state: the function runs on the server, so it reads the
// database the way any Go function does and returns the page already filled.
// What React needed three renders for happens here before the first byte.
func Dashboard(c *trilha.Ctx) (h.Node, error) {
	var q DashQuery
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	rows, total, err := dashRows(c.Context(), q)
	if err != nil {
		return nil, err
	}
	return ui.DataTable(c, DashColumns, rows, ui.ListState{
		Params: q.ListParams,
		Total:  total,
		ID:     "articles",
		Search: "Search articles",
		Empty:  h.Text("No article matches this filter."),
		RowHref: func(i int) string {
			return "/dashboard/" + rows[i].Slug
		},
	}), nil
}

// DashColumns replaces the <thead> written by hand and the onClick that
// re-sorted the array in the browser. Sort: true turns the header into a real
// link — the ordering is a GET, so it works with the script blocked and it is
// still there after a reload.
var DashColumns = ui.Columns[Article]{
	{Key: "title", Label: "Title", Sort: true, Cell: func(a Article) h.Node {
		return h.Text(a.Title)
	}},
	{Key: "published", Label: "Published", Sort: true, Cell: func(a Article) h.Node {
		return h.Text(a.Published.Format("2006-01-02"))
	}},
}

// dashRows is the query behind the screen. The ordering comes from the URL,
// so it is checked before it reaches SQL: Restrict drops a column nobody
// declared, and what is left is one of two names this function knows.
func dashRows(ctx context.Context, q DashQuery) ([]Article, int, error) {
	q.Restrict("title", "published")
	order := `published_at DESC`
	if q.Sort == "title" {
		order = `title`
		if !q.Asc() {
			order = `title DESC`
		}
	}
	rows, err := DB.QueryContext(ctx,
		`SELECT id, slug, title, published_at FROM articles WHERE ($1 = '' OR title ILIKE '%' || $1 || '%') AND ($2 = '' OR status = $2) ORDER BY `+order+` LIMIT $3 OFFSET $4`,
		q.Q, q.Status, q.Limit(), q.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	err = DB.QueryRowContext(ctx, `SELECT count(*) FROM articles WHERE ($1 = '' OR status = $1)`, q.Status).Scan(&total)
	return out, total, err
}

// Queue is the block that used to be a useEffect with a setInterval and a
// cleanup nobody remembered to write. ui.Poll asks this same route for the
// fragment every six seconds; when the work is done the handler stops the
// clock from the server side and the browser stops asking.
func Queue(c *trilha.Ctx) (h.Node, error) {
	left, err := pendingArticles(c.Context())
	if err != nil {
		return nil, err
	}
	if left == 0 {
		c.PollStop()
		return h.Div(h.ID("queue"), h.Text("Everything processed.")), nil
	}
	return h.Div(h.ID("queue"), ui.Poll("6s", "#queue"),
		h.Text(strconv.Itoa(left)+" articles in the queue"),
	), nil
}

// pendingArticles counts what is left to process.
func pendingArticles(ctx context.Context) (int, error) {
	var n int
	err := DB.QueryRowContext(ctx, `SELECT count(*) FROM articles WHERE status = 'pending'`).Scan(&n)
	return n, err
}

// Publish is the POST behind the button. The toast() of the React app is a
// flash here: the message is written before the redirect and shown by the
// page that comes next, so a refresh does not repeat the action and the
// message survives the navigation without any state to carry.
func Publish(c *trilha.Ctx) error {
	if _, err := DB.ExecContext(c.Context(), `UPDATE articles SET status = 'published' WHERE slug = $1`, c.Param("slug")); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "Article published")
	return c.Redirect("/dashboard")
}

// ConfirmDelete is the modal that used to be a useState(false), a portal and
// an effect closing it on Escape. The dialog is in the HTML from the start,
// closed; the browser opens it, traps the focus and closes it on Escape by
// itself, because <dialog> already does all of that.
func ConfirmDelete(slug string) h.Node {
	return h.Div(
		ui.DialogTrigger("delete", h.Text("Delete")),
		ui.Dialog("delete", "Delete this article?",
			h.P(h.Text("This cannot be undone.")),
			h.Form(h.Method("post"), h.Action("/dashboard/"+slug+"/delete"),
				ui.Button(ui.Destructive(), h.Text("Delete")),
			),
		),
	)
}

// Preview answers the PDF the <iframe> shows. In the Next.js app this was a
// route handler returning a blob, a createObjectURL and a revoke that leaked
// when the component unmounted early: here the browser asks for a URL and
// gets a document.
//
// The frame still has to be allowed: the default policy is frame-ancestors
// 'none', so the page that frames it adds "frame-src": {"'self'"} to
// Security.CSPExtra.
func Preview(c *trilha.Ctx) error {
	f, err := os.Open("var/previews/" + c.Param("slug") + ".pdf")
	if err != nil {
		return trilha.ErrNotFound
	}
	defer f.Close()
	return c.Inline("preview.pdf", f, "application/pdf")
}

// Panel is the session of the migrated app. It is set in app/setup.go; the
// pages never touch it.
var Panel *auth.Auth

// Middleware is the whole of app/dashboard/middleware.go, and it replaces the
// `if (!user) router.replace("/login")` that every page repeated. The check
// runs before the handler, so a route added tomorrow is covered by having been
// put in this folder — not by remembering the first line.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return Panel.Require()(c, next)
}

// Frame is app/dashboard/layout.go: the sidebar, the top bar and the user
// menu that the React app assembled from a component library, a state for the
// collapsed sidebar and a localStorage to remember it. Shell renders the
// frame; the current item comes from the path, so no page has to say which
// link is active.
func Frame(c *trilha.Ctx, children ...h.Node) h.Node {
	return ui.Shell(c, ui.ShellOpts{
		Brand:   h.Text("Editorial"),
		Current: c.Request().URL.Path,
		Nav: []ui.NavGroup{{Label: "Content", Items: []ui.NavItem{
			{Href: "/dashboard", Label: "Articles", Icon: "file"},
			{Href: "/dashboard/queue", Label: "Queue", Icon: "clock"},
		}}},
		User: ui.UserMenu{Name: "Editor", Items: []h.Node{
			ui.MenuLink("/logout", h.Text("Sign out")),
		}},
	}, children...)
}

// Notes renders text written by a person or by a model. It is the answer to
// dangerouslySetInnerHTML: the source becomes a tree of nodes, a tag in the
// source comes out as text, and there is no string on the way that anybody
// could hand to h.Raw.
func Notes(src string) h.Node {
	return ui.Markdown(src, ui.MarkdownOpts{HeadingBase: 3})
}

// Sales is the one place the React answer was right: a chart library that
// runs in the browser. The island loads the module, hands it the data as
// JSON and mounts it on this element — and the children are what a visitor
// with the script blocked reads instead.
func Sales(c *trilha.Ctx, points []float64) h.Node {
	return c.Island("/chart.js", map[string]any{"points": points},
		h.Class("chart"),
		ui.Sparkline(points, ui.SparkOpts{}),
	)
}

// SetupPreviews is the last piece of the move. next.config.js had rewrites,
// headers and a public/ folder; here public/ is served by the framework and
// the rest is this: a mount for the generated files, and the one relaxation
// the <iframe> of Preview needs.
func SetupPreviews(cfg *trilha.Config) {
	cfg.Mounts = map[string]fs.FS{"/previews/": os.DirFS("var/previews")}
	cfg.Security.CSPExtra = map[string][]string{"frame-src": {"'self'"}}
}
