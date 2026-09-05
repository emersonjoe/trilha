package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
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
	if len(Asset("ui.css")) > 25<<10 || len(Asset("ui.js")) > 10<<10 {
		t.Fatal("assets too large (FR-007)")
	}
	if len(Icons()) < 30 || Icons()[0] != "arrow-left" {
		t.Fatal(Icons())
	}
}
