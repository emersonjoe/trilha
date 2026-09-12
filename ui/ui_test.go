package ui

import (
	"errors"
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

// #174 — the title has to read before the description, with or without an
// icon: variant() appended the H4 after the children, so the description
// (already among children) came first in the DOM.
func TestAlertTitleComesBeforeDescription(t *testing.T) {
	got := render(t, Alert("Desatualizada", AlertDescription(h.Text("Refaça a análise."))))
	title, desc := strings.Index(got, `class="ui-alert-title"`), strings.Index(got, `class="ui-alert-description"`)
	if title < 0 || desc < 0 || title > desc {
		t.Fatalf("título não vem antes da descrição: %s", got)
	}
	if strings.Contains(got, "ui-icon") {
		t.Fatalf("sem Icon, não deveria haver célula de ícone: %s", got)
	}

	// A grade posiciona por CSS (grid-column/grid-row), não pela ordem no DOM,
	// então o que importa com ícone é o mesmo: título antes da descrição.
	withIcon := render(t, Alert("Desatualizada", Icon("triangle-alert"), AlertDescription(h.Text("Refaça a análise."))))
	titleI, descI := strings.Index(withIcon, `class="ui-alert-title"`), strings.Index(withIcon, `class="ui-alert-description"`)
	if titleI < 0 || descI < 0 || titleI > descI {
		t.Fatalf("com ícone, título não vem antes da descrição: %s", withIcon)
	}
	if !strings.Contains(withIcon, "ui-icon") {
		t.Fatalf("com Icon, deveria haver a célula do ícone: %s", withIcon)
	}
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
	// FR-007 of spec 006, with the ui.js budget raised to 12 KB in 0.30.0 (the
	// tooltip is the first component since the kit shipped to need script of
	// its own, and a hint that cannot be dismissed is not accessible), to
	// 16 KB in 0.39.0, where the confirmation dialog is built here so that no
	// app has to write the inline script the CSP forbids, and to 28 KB: the
	// pending threshold, the view transition and the island that arrives inside
	// a fragment are about 3.8 KB, and the combobox — a listbox driven from the
	// keyboard — another two. Every one of them sits on the path a swap already
	// takes, or is a component the app that does not use it pays four hundred
	// bytes of dead listeners for, against shipping its own copy of the same
	// thing; none could move to a file only the apps using it download, the way
	// ui.nav.js and ui.upload.js do. The island channel (spec 066) is another
	// 1.2 KB, and it is the one duplication the kit accepts: an island cannot
	// know whether the loader or the kit mounted it, so both hand it the same
	// object.
	// ui.css went to 30 KB: ui.Markdown needs prose rules (a model writes
	// lists, quotes and code blocks, and unstyled they read as one block of
	// text) and ui.Chat needs the bubbles. Both are paid by every page, which
	// is why they are rules and not a second stylesheet — a chat that has to
	// remember to load its own CSS renders wrong once.
	// ui.css went to 32 KB in 0.52.0: ui.Preview needs the bar, the frame and
	// the image (about 800 bytes), and it was 24 bytes from the old ceiling.
	// The frame carries a background of its own on purpose — a PDF that has
	// not painted yet, over a transparent one, reads as a hole in the page.
	// And to 34 KB in 0.55.0: ui.Tree brings the branch lines, the indent and
	// the picker's box (about 1.3 KB). The arrow is two characters and no
	// animation — a caret that rotates is decoration, and decoration is what a
	// budget is for.
	// 0.64.0 spent about 600 bytes on ui.TaskProgress: the indeterminate bar
	// and its reduced-motion fallback, plus two rules for the table's retry
	// button. That left barely a hundred bytes under the ceiling, and the next
	// rule that did not fit was meant to be a conversation and not an
	// increment — which is what a budget is for.
	//
	// That conversation happened in 0.65.0, and the answer was to spend less
	// rather than to raise the ceiling. ui.WebhooksPanel needed two rules and
	// no more: everything else it draws is a table, a form and buttons the kit
	// already styles. The retry button's rule was not copied for it either —
	// it was renamed .ui-inline-form and both screens use it, which is what a
	// second caller is supposed to do to a rule. The comment that would have
	// explained it lives in the Go doc, where it costs the reader nothing.
	//
	// 0.74.0 raised it to 36 KB, and it is the first raise that buys headroom
	// instead of a component: ui.Assistant is three rules and about 250 bytes
	// — the corner, the panel that scrolls and the log inside it — and there
	// were 47 bytes left. Everything else it draws is a .ui-dialog, a .ui-btn
	// and a ui.Chat.
	//
	// What was tried first: sharing the corner with .ui-toaster (it costs more
	// than it saves, because the toaster keeps four declarations of its own)
	// and dropping the width override (kept dropped — .ui-dialog's width is
	// the right one). What was not done is a second stylesheet: a launcher
	// that has to remember to load its own CSS renders wrong once, which is
	// the same reason the chat's bubbles are here.
	if len(Asset("ui.css")) > 36<<10 || len(Asset("ui.js")) > 28<<10 {
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
	// 8 KB since the dropzone: the queue that sends one file per request, so
	// the message of a file lands on the line of that file, lives here and not
	// in ui.js — the page with no upload still loads nothing.
	if n := len(Asset("ui.upload.js")); n == 0 || n > 8<<10 {
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

func TestConfirmMarcaOFormulario(t *testing.T) {
	got := render(t, h.Form(h.Method("post"),
		Confirm("Apagar este post?", "Não dá para desfazer."),
		Submit(Destructive(), h.Text("Apagar"))))
	for _, want := range []string{
		`data-ui-confirm="Apagar este post?"`,
		`data-ui-confirm-description="Não dá para desfazer."`,
		`<button type="submit" class="ui-btn ui-btn-destructive"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	// Os atributos ficam na tag de abertura, não no corpo do formulário.
	if i, j := strings.Index(got, ">"), strings.Index(got, "data-ui-confirm"); j > i {
		t.Fatalf("atributo fora da tag: %s", got)
	}
	if s := render(t, h.Form(Confirm("Tem certeza?", ""))); strings.Contains(s, "description") {
		t.Fatal(s)
	}
}

func TestFlashesRenderizaOsAvisos(t *testing.T) {
	a := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Secret: []byte(strings.Repeat("k", 32))})
	var out, empty string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		empty = render(t, Flashes(c))
		c.Flash(FlashSuccess, "Post apagado")
		c.Flash(FlashError, "<b>ops</b>")
		out = render(t, Flashes(c))
		return h.Div(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	// Sem avisos o toaster continua lá: é onde o ui.js pendura os que chegam
	// pelo cabeçalho.
	if !strings.Contains(empty, `class="ui-toaster"`) || strings.Contains(empty, "ui-toast ") {
		t.Fatalf("toaster vazio: %s", empty)
	}
	for _, want := range []string{"ui-toast-success", "Post apagado", "ui-toast-error", "&lt;b&gt;ops&lt;/b&gt;", strconv.Itoa(FlashFadeMs)} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

// O ui.js é a outra metade destas duas funções: sem o par de nomes que ele
// procura, o Go escreve atributos que ninguém lê.
func TestUiJSConheceOsAtributosNovos(t *testing.T) {
	js := string(Asset("ui.js"))
	for _, want := range []string{"data-ui-confirm", "data-ui-confirm-description", "data-ui-confirm-cancel", "Trilha-Flash", "requestSubmit"} {
		if !strings.Contains(js, want) {
			t.Fatalf("ui.js não fala de %q", want)
		}
	}
}

// Spec 057: the pending trio is three attributes and no state, so they compose
// on whatever element the page already has.
func TestPendingAttributes(t *testing.T) {
	if got := render(t, Spinner(Indicator("lista"))); !strings.Contains(got, `data-trilha-indicator="lista"`) {
		t.Fatalf("Indicator = %s", got)
	}
	if got := render(t, h.A(h.Href("/x"), Swap("lista"), PendingAfter(200))); !strings.Contains(got, `data-trilha-pending-after="200"`) {
		t.Fatalf("PendingAfter = %s", got)
	}
	// A threshold that makes no sense is the page's mistake, and the page should
	// not have to guard it: zero and negative mean "the default".
	if got := render(t, h.A(PendingAfter(-1))); strings.Contains(got, "pending-after") {
		t.Fatalf("PendingAfter(-1) must not write the attribute: %s", got)
	}
	if got := render(t, h.A(h.Href("/x"), Swap("lista"), NoTransition())); !strings.Contains(got, `data-trilha-transition="false"`) {
		t.Fatalf("NoTransition = %s", got)
	}
	// The indicator composes with Swap on the same element: a link may be both
	// the trigger and the thing that dims.
	if got := render(t, h.A(h.Href("/x"), Swap("lista"), Indicator("lista"))); !strings.Contains(got, `data-trilha-target="lista"`) || !strings.Contains(got, `data-trilha-indicator="lista"`) {
		t.Fatalf("Swap+Indicator = %s", got)
	}
}

// Spinner is only worth a test for the class it carries; the animation is CSS.
func TestSpinner(t *testing.T) {
	if got := render(t, Spinner()); !strings.Contains(got, `class="ui-spinner"`) || !strings.Contains(got, `aria-hidden="true"`) {
		t.Fatalf("Spinner = %s", got)
	}
}

// Spec 057 (#82) said an island arriving inside a swapped fragment must mount
// even on a page that had none, because the DOM does not run a <script> written
// by outerHTML. Spec 060 kept the guarantee and moved where it lives: the
// runtime is a file, and the kit only has to make that one tag run.
//
// The division is the point. Two implementations of mounting had already
// drifted — only one of them restored focus — so the mounting lives in
// ui.island.js and nowhere else.
func TestKitRunsTheIslandRuntimeItDoesNotMount(t *testing.T) {
	rt := string(Asset("ui.island.js"))
	for _, want := range []string{"data-trilha-island", "data-trilha-mounted", "data-trilha-props", "import(src)"} {
		if !strings.Contains(rt, want) {
			t.Errorf("ui.island.js does not mount islands: %q is missing", want)
		}
	}
	js := string(Asset("ui.js"))
	if !strings.Contains(js, `script[data-trilha-islands]`) || !strings.Contains(js, "createElement") {
		t.Error("ui.js no longer re-creates the runtime tag that arrives in a fragment")
	}
	if strings.Contains(js, "data-trilha-mounted") {
		t.Error("ui.js mounts islands again: that belongs to ui.island.js alone")
	}
}

// Spec 065 (#97): the empty state is a component so that thirty-five screens do
// not each invent one. What it must never do is fail on the screen that is
// already empty.
func TestEmptyDrawsWhatItWasGivenAndNothingElse(t *testing.T) {
	full := render(t, Empty(EmptyOpts{
		Icon: "info", Title: "Nenhum documento ainda",
		Hint:   "Envie o primeiro PDF.",
		Action: ButtonLink("/upload", h.Text("Enviar")),
	}))
	for _, want := range []string{"ui-empty-icon", "Nenhum documento ainda", "Envie o primeiro PDF.", `href="/upload"`} {
		if !strings.Contains(full, want) {
			t.Errorf("faltou %q em %s", want, full)
		}
	}

	// Only Title is required, and the rest simply does not appear.
	bare := render(t, Empty(EmptyOpts{Title: "Vazio"}))
	for _, gone := range []string{"ui-empty-icon", "ui-empty-hint", "ui-empty-action"} {
		if strings.Contains(bare, gone) {
			t.Errorf("desenhou %q sem ter recebido nada: %s", gone, bare)
		}
	}

	// An icon the kit does not carry would panic inside Icon. Panicking on the
	// screen that is already empty is the worst place for it: no icon is a fine
	// empty state, a 500 is not.
	safe := render(t, Empty(EmptyOpts{Icon: "nao-existe", Title: "Vazio"}))
	if strings.Contains(safe, "ui-empty-icon") {
		t.Errorf("desenhou um ícone que não existe: %s", safe)
	}
}

// The real error belongs in development and nowhere else: a driver's sentence
// on a production page is an information leak with a friendly font.
func TestEmptyErrorHidesTheErrorOutsideDevelopment(t *testing.T) {
	boom := errors.New(`pq: relation "documentos" does not exist`)

	renderIn := func(env trilha.Env) string {
		t.Helper()
		a := trilha.New(trilha.Config{Env: env, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		var out string
		a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
			out = render(t, EmptyError(c, "Não deu para carregar", boom, nil))
			return h.Div(), nil
		}})
		a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		return out
	}

	if got := renderIn(trilha.Dev); !strings.Contains(got, "relation") {
		t.Errorf("em desenvolvimento o erro real tem que aparecer: %s", got)
	}
	got := renderIn(trilha.Prod)
	if strings.Contains(got, "relation") {
		t.Errorf("o erro real vazou em produção: %s", got)
	}
	if !strings.Contains(got, "Não deu para carregar") || !strings.Contains(got, `role="alert"`) {
		t.Errorf("a mensagem da pessoa sumiu junto: %s", got)
	}
}
