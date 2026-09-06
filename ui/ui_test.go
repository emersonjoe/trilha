package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func render(t *testing.T, n h.Node) string {
	t.Helper()
	s, err := h.Render(n)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestButtonVariants(t *testing.T) {
	got := render(t, Button(Outline(), Sm(), h.ID("b"), h.Text("Ok")))
	if got != `<button class="ui-btn ui-btn-outline ui-btn-sm" id="b" type="button">Ok</button>` {
		t.Fatal(got)
	}
	if s := render(t, Badge(Destructive(), h.Text("x"))); !strings.Contains(s, `class="ui-badge ui-badge-destructive"`) {
		t.Fatal(s)
	}
	if s := render(t, Submit(h.Text("Enviar"))); !strings.HasPrefix(s, `<button type="submit" class="ui-btn"`) {
		t.Fatal(s)
	}
	if s := render(t, ButtonLink("/x", Ghost(), h.Text("a"))); !strings.Contains(s, `class="ui-btn ui-btn-ghost" href="/x"`) {
		t.Fatal(s)
	}
}

func TestFieldAndShowWhen(t *testing.T) {
	got := render(t, Field("email", "E-mail", Input(h.ID("email"), h.Name("email"), Invalid()), Help("Nunca compartilhado"), Error("inválido"), With(ShowWhen("tipo", "pf", "pj"))))
	for _, want := range []string{`class="ui-field"`, `<label class="ui-label" for="email">E-mail</label>`, `aria-invalid="true"`, `id="email-help"`, `role="alert"`, `data-ui-show-when="tipo=pf|pj"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if s := render(t, ShowWhen("cep")); s != ` data-ui-show-when="cep"` {
		t.Fatal(s)
	}
}

func TestTabsDialogToast(t *testing.T) {
	got := render(t, Tabs("t", Tab{"A", h.Text("a")}, Tab{"B", h.Text("b")}))
	for _, want := range []string{`data-ui-tabs`, `role="tablist"`, `id="t-tab-0" aria-selected="true" aria-controls="t-panel-0" tabindex="0"`, `id="t-panel-1" aria-labelledby="t-tab-1" hidden>b</div>`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	d := render(t, Dialog("dlg", "Título", DialogFooter(DialogClose(h.Text("Fechar")))))
	for _, want := range []string{`<dialog class="ui-dialog" id="dlg" aria-labelledby="dlg-title"`, `data-ui-dialog-close`, `<h2 class="ui-dialog-title" id="dlg-title">Título</h2>`} {
		if !strings.Contains(d, want) {
			t.Fatalf("missing %q in %s", want, d)
		}
	}
	if s := render(t, DialogTrigger("dlg", h.Text("Abrir"))); !strings.Contains(s, `data-ui-dialog-open="dlg"`) {
		t.Fatal(s)
	}
	tst := render(t, Toast("success", "Salvo <b>", 3000))
	if !strings.Contains(tst, `class="ui-toast ui-toast-success"`) || !strings.Contains(tst, `data-ui-fade="3000"`) || !strings.Contains(tst, "Salvo &lt;b&gt;") {
		t.Fatal(tst)
	}
}

func TestMisc(t *testing.T) {
	if s := render(t, Progress(150, 100)); !strings.Contains(s, `width:100%`) || !strings.Contains(s, `aria-valuenow="150"`) {
		t.Fatal(s)
	}
	if s := render(t, Progress(1, 0)); !strings.Contains(s, `width:0%`) {
		t.Fatal(s)
	}
	b := render(t, Breadcrumb(Crumb{"Início", "/"}, Crumb{"Contas", "/contas"}, Crumb{"Receitas", ""}))
	if !strings.Contains(b, `<li><a href="/">Início</a></li>`) || !strings.Contains(b, `<li aria-current="page">Receitas</li>`) || strings.Count(b, "chevron") != 0 && !strings.Contains(b, `aria-hidden="true"`) {
		t.Fatal(b)
	}
	if s := render(t, Avatar("EO", "")); !strings.Contains(s, `aria-label="EO"`) {
		t.Fatal(s)
	}
	if s := render(t, Icon("check")); !strings.HasPrefix(s, `<svg class="ui-icon" viewBox="0 0 24 24"`) || !strings.Contains(s, `<path d="M20 6 9 17l-5-5"/>`) {
		t.Fatal(s)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("unknown icon must panic")
		}
	}()
	Icon("nope")
}

func TestHeadAndAssets(t *testing.T) {
	a := trilha.New(trilha.Config{BasePath: "/app", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, Head(c))
		return h.Div(), nil
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	for _, want := range []string{`href="/app/ui.theme.css"`, `href="/app/ui.css"`, `src="/app/ui.js" defer`, ` nonce="`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	// Theme contract: every shadcn/ui v4 variable must be defined in both blocks.
	theme := string(Asset("ui.theme.css"))
	for _, v := range []string{"--background", "--foreground", "--card", "--card-foreground", "--popover", "--popover-foreground", "--primary", "--primary-foreground", "--secondary", "--secondary-foreground", "--muted", "--muted-foreground", "--accent", "--accent-foreground", "--destructive", "--border", "--input", "--ring", "--chart-1", "--chart-5", "--sidebar", "--sidebar-ring", "--radius"} {
		if n := strings.Count(theme, v+":"); n < 2 && v != "--radius" {
			t.Fatalf("%s defined %d times", v, n)
		}
	}
	// FR-007 of spec 006, with the ui.js budget raised to 12 KB in 0.30.0: the
	// tooltip is the first component since the kit shipped to need script of
	// its own, and a hint that cannot be dismissed is not accessible.
	if len(Asset("ui.css")) > 25<<10 || len(Asset("ui.js")) > 12<<10 {
		t.Fatal("assets too large (FR-007)")
	}
	if len(Icons()) < 30 || Icons()[0] != "arrow-left" {
		t.Fatal(Icons())
	}
}

func TestValidationHelpers(t *testing.T) {
	errs := map[string]string{"cnpj": "inválido"}
	got := render(t, Field("cnpj", "CNPJ", Input(h.ID("cnpj"), h.Value("1"), InvalidIf(errs, "cnpj")), Errors(errs, "cnpj")))
	if !strings.Contains(got, `aria-invalid="true"`) || !strings.Contains(got, `role="alert">inválido<`) {
		t.Fatal(got)
	}
	ok := render(t, Field("cpf", "CPF", Input(h.ID("cpf"), InvalidIf(errs, "cpf")), Errors(errs, "cpf")))
	if strings.Contains(ok, "aria-invalid") || strings.Contains(ok, "ui-field-error") {
		t.Fatal(ok)
	}
	sel := render(t, Select(h.Name("uf"), SelectOptions([]Option{{"", "Escolha…"}, {"SP", "São Paulo"}, {"RJ", "Rio"}}, "RJ")))
	if !strings.Contains(sel, `<option value="" disabled="">Escolha…</option>`) || !strings.Contains(sel, `<option value="RJ" selected>Rio</option>`) {
		t.Fatal(sel)
	}
	if fb := render(t, Select(SelectOptions([]Option{{"", "Escolha…"}, {"SP", "SP"}}, "XX"))); !strings.Contains(fb, `<option value="" selected disabled="">`) || strings.Contains(fb, `value="SP" selected`) {
		t.Fatal("placeholder must be selected when nothing matches:", fb)
	}
	if render(t, Checked(true)) != " checked" || render(t, Checked(false)) != "" {
		t.Fatal("Checked")
	}
}

// Issue #23: client navigation is opt-in — an attribute marks the region and a
// separate file carries the behavior, so an app that does not want it does not
// download it.
func TestNavigateIsOptIn(t *testing.T) {
	if got := render(t, h.Main(h.ID("conteudo"), Navigate(""))); !strings.Contains(got, `data-trilha-nav=""`) {
		t.Fatalf("Navigate(\"\") = %s", got)
	}
	if got := render(t, h.Div(Navigate("conteudo"))); !strings.Contains(got, `data-trilha-nav="conteudo"`) {
		t.Fatalf("Navigate = %s", got)
	}
	if got := render(t, h.A(h.Href("/relatorio.pdf"), NoNavigate())); !strings.Contains(got, `data-trilha-nav="false"`) {
		t.Fatalf("NoNavigate = %s", got)
	}
	for _, name := range Files {
		if name == "ui.nav.js" {
			goto found
		}
	}
	t.Fatal("ui.nav.js is not in Files: trilha ui would not write it")
found:
	if n := len(Asset("ui.nav.js")); n == 0 || n > 4<<10 {
		t.Fatalf("ui.nav.js is %d bytes", n)
	}
	// The behavior does not ride in ui.js: an app without client navigation
	// pays nothing for it.
	if strings.Contains(string(Asset("ui.js")), "data-trilha-nav") {
		t.Fatal("client navigation leaked into ui.js")
	}
}

// The script goes through Asset, like the rest of the kit: the URL carries the
// content hash, and BasePath is respected.
func TestNavigateScript(t *testing.T) {
	a := trilha.New(trilha.Config{BasePath: "/app", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, NavigateScript(c))
		return h.Div(), nil
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if want := `<script src="/app/ui.nav.js" defer></script>`; out != want {
		t.Fatalf("NavigateScript = %s, want %s", out, want)
	}
}

// Issue #24: upload with progress is opt-in the same way navigation is — an
// attribute on the form and a file of its own.
func TestUploadIsOptIn(t *testing.T) {
	got := render(t, h.Form(h.Method("post"), UploadTo("lista"), UploadBar()))
	for _, want := range []string{`data-trilha-upload="lista"`, `<progress`, `data-trilha-progress=""`, ` hidden`, `max="100"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	// The fragment handler in ui.js listens on [data-trilha-target]; the upload
	// form must not carry it, or both would submit the same form.
	if strings.Contains(got, "data-trilha-target") {
		t.Fatal("upload must not reuse data-trilha-target:", got)
	}
	for _, name := range Files {
		if name == "ui.upload.js" {
			goto found
		}
	}
	t.Fatal("ui.upload.js is not in Files: trilha ui would not write it")
found:
	if n := len(Asset("ui.upload.js")); n == 0 || n > 4<<10 {
		t.Fatalf("ui.upload.js is %d bytes", n)
	}
	if strings.Contains(string(Asset("ui.js")), "data-trilha-upload") {
		t.Fatal("upload leaked into ui.js")
	}
}

func TestUploadScript(t *testing.T) {
	a := trilha.New(trilha.Config{BasePath: "/app", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, UploadScript(c))
		return h.Div(), nil
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if want := `<script src="/app/ui.upload.js" defer></script>`; out != want {
		t.Fatalf("UploadScript = %s, want %s", out, want)
	}
}

func TestPagination(t *testing.T) {
	href := func(n int) string { return "/blog?page=" + strconv.Itoa(n) }

	// One page is no navigation at all: nothing is rendered.
	if s := render(t, Pagination(Pages{Page: 1, Total: 1, Href: href})); s != "" {
		t.Fatalf("one page should render nothing, got %q", s)
	}

	first := render(t, Pagination(Pages{Page: 1, Total: 3, Href: href}))
	for _, want := range []string{
		`<nav class="ui-pagination" aria-label="Pagination">`,
		`<span aria-current="page">1</span>`,
		`<a href="/blog?page=2">2</a>`,
		`<a rel="next" href="/blog?page=2">Next</a>`,
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("missing %q in %s", want, first)
		}
	}
	if strings.Contains(first, `rel="prev"`) {
		t.Fatalf("no previous page on the first one: %s", first)
	}
	if strings.Contains(first, `<a href="/blog?page=1">`) {
		t.Fatalf("the current page is not a link: %s", first)
	}

	last := render(t, Pagination(Pages{Page: 3, Total: 3, Href: href, Prev: "Anterior", Next: "Próxima", Label: "Paginação"}))
	for _, want := range []string{
		`aria-label="Paginação"`,
		`<a rel="prev" href="/blog?page=2">Anterior</a>`,
		`<span aria-current="page">3</span>`,
	} {
		if !strings.Contains(last, want) {
			t.Fatalf("missing %q in %s", want, last)
		}
	}
	if strings.Contains(last, `rel="next"`) {
		t.Fatalf("no next page on the last one: %s", last)
	}

	// A long list keeps the ends and a window around the current page, so the
	// footer does not grow with the table.
	mid := render(t, Pagination(Pages{Page: 10, Total: 20, Href: href}))
	if n := strings.Count(mid, "<li"); n != 9 { // 7 slots + prev + next
		t.Fatalf("expected 9 items, got %d in %s", n, mid)
	}
	for _, want := range []string{
		`<a href="/blog?page=1">1</a>`,
		`<li aria-hidden="true">…</li>`,
		`<a href="/blog?page=9">9</a>`,
		`<span aria-current="page">10</span>`,
		`<a href="/blog?page=20">20</a>`,
	} {
		if !strings.Contains(mid, want) {
			t.Fatalf("missing %q in %s", want, mid)
		}
	}
	if strings.Contains(mid, ">15<") {
		t.Fatalf("a page far from the current one is not in the window: %s", mid)
	}

	// Near an edge there is only one gap, and the window slides instead of
	// leaving three numbers on screen.
	near := render(t, Pagination(Pages{Page: 2, Total: 20, Href: href}))
	if n := strings.Count(near, `aria-hidden="true"`); n != 1 {
		t.Fatalf("expected one ellipsis, got %d in %s", n, near)
	}
	if !strings.Contains(near, `<a href="/blog?page=5">5</a>`) {
		t.Fatal(near)
	}
}

func TestTooltip(t *testing.T) {
	got := render(t, Tooltip(`Só você vê "isto"`, Button(h.Text("Ok"))))
	for _, want := range []string{
		`<span class="ui-tooltip"`,
		`data-ui-tooltip="Só você vê &#34;isto&#34;"`,
		`title="Só você vê &#34;isto&#34;"`,
		`<button class="ui-btn" type="button">Ok</button>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}
