package scan

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func scanApp(t *testing.T, name string) (*Result, Errors) {
	t.Helper()
	res, err := Scan(filepath.Join("..", "..", "testdata", "apps", name), "example.com/"+name)
	var errs Errors
	if err != nil && !errors.As(err, &errs) {
		t.Fatalf("unexpected error type: %v", err)
	}
	return res, errs
}

func TestFullApp(t *testing.T) {
	res, errs := scanApp(t, "full")
	if errs != nil {
		t.Fatal(errs)
	}
	var got []string
	for _, r := range res.Routes {
		got = append(got, r.Kind+" "+r.Pattern+" "+strings.Join(r.Methods, ","))
	}
	want := []string{
		"page / ",
		"page /admin ",
		"api /api/posts DELETE,GET,POST",
		"api /api/posts/{id} GET",
		"page /blog ",
		"page /blog/novo POST",
		"page /blog/{slug} ",
		"page /docs/{path...} ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("routes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	byPat := map[string]Route{}
	for _, r := range res.Routes {
		byPat[r.Pattern] = r
	}
	slug := byPat["/blog/{slug}"]
	if l := refs(slug.Layouts); l != "app_blog.Layout app.Layout" {
		t.Fatalf("layouts innermost first: %s", l)
	}
	if m := refs(slug.Middlewares); m != "app.Middleware" {
		t.Fatal(m)
	}
	if slug.ImportPath != "example.com/full/app/blog/slug_" || slug.Alias != "app_blog_slug_" {
		t.Fatalf("%+v", slug)
	}
	admin := byPat["/admin"]
	if m := refs(admin.Middlewares); m != "app.Middleware app_admin.Middleware" {
		t.Fatalf("middlewares outermost first: %s", m)
	}
	if api := byPat["/api/posts"]; api.Layouts != nil || refs(api.Middlewares) != "app.Middleware" {
		t.Fatalf("api routes have no layouts: %+v", api)
	}
	if res.RootLayout == nil || res.NotFound == nil || res.ErrorPage == nil || res.Setup == nil || !res.HasPublic {
		t.Fatalf("root refs: %+v", res)
	}
	if res.NotFound.Func != "NotFound" || res.Setup.Alias != "app" {
		t.Fatalf("%+v %+v", res.NotFound, res.Setup)
	}
	var imps []string
	for _, i := range res.Imports {
		imps = append(imps, i.Alias)
	}
	if strings.Join(imps, " ") != "app app_admin app_api_posts app_api_posts_id_ app_blog app_blog_novo app_blog_slug_ app_docs_path__" {
		t.Fatal(strings.Join(imps, " "))
	}
}

func refs(rs []Ref) string {
	var s []string
	for _, r := range rs {
		s = append(s, r.Alias+"."+r.Func)
	}
	return strings.Join(s, " ")
}

func TestMinimal(t *testing.T) {
	res, errs := scanApp(t, "minimal")
	if errs != nil {
		t.Fatal(errs)
	}
	if len(res.Routes) != 1 || res.Routes[0].Pattern != "/" || res.RootLayout != nil || res.HasPublic {
		t.Fatalf("%+v", res)
	}
}

func TestEmptyPublicIsIgnored(t *testing.T) {
	res, _ := scanApp(t, "empty_public")
	if res.HasPublic {
		t.Fatal("public with only dotfiles must not be embedded")
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		app  string
		code string
		file string
	}{
		{"err_page_and_route", ErrPageAndRoute, "app/x"},
		{"err_no_page_func", ErrNoPageFunc, "app/page.go"},
		{"err_no_method", ErrNoMethod, "app/api/route.go"},
		{"err_ambiguous", ErrAmbiguousSegment, "app"},
		{"err_catchall", ErrCatchAllNotLeaf, "app/docs/rest__/x"},
		{"err_bad_segment", ErrBadSegment, "app/1abc_"},
		{"err_parse", ErrParse, "app/page.go"},
		{"err_layout", ErrNoLayoutFunc, "app/layout.go"},
		{"err_layout", ErrNoMiddlewareFunc, "app/middleware.go"},
		{"err_unused_method_mw", ErrUnusedMethodMW, "app/middleware.go"},
	}
	for _, c := range cases {
		_, errs := scanApp(t, c.app)
		found := false
		for _, e := range errs {
			if e.Code == c.code && e.File == c.file {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s at %s, got %v", c.app, c.code, c.file, errs)
		}
	}
	if _, err := Scan(t.TempDir(), "x"); err == nil || !strings.Contains(err.Error(), ErrNoApp) {
		t.Errorf("missing app dir: %v", err)
	}
}

func TestErrorFormat(t *testing.T) {
	_, errs := scanApp(t, "err_no_page_func")
	if got := errs.Error(); got != "app/page.go:3: E_NO_PAGE_FUNC: page.go must export func Page(c *trilha.Ctx) (h.Node, error); found func Render" {
		t.Fatal(got)
	}
}

func TestGroups(t *testing.T) {
	res, errs := scanApp(t, "groups")
	if errs != nil {
		t.Fatal(errs)
	}
	var got []string
	for _, r := range res.Routes {
		got = append(got, r.Pattern+" <- "+r.Dir+" ["+refs(r.Layouts)+"] {"+refs(r.Middlewares)+"}")
	}
	want := []string{
		"/ <- app [app.Layout] {}",
		"/api/stats <- app/painel-/api/stats [] {app_painel_.Middleware}",
		"/painel <- app/painel-/painel [app_painel_.Layout app.Layout] {app_painel_.Middleware}",
		"/precos <- app/marketing-/precos [app_marketing_.Layout app.Layout] {}",
		"/sobre <- app/marketing-/sobre [app_marketing_.Layout app.Layout] {}",
		"/x <- app/a-/b-/x [app_a__b_.Layout app_a_.Layout app.Layout] {}",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestGroupErrors(t *testing.T) {
	_, errs := scanApp(t, "err_group_dup")
	var dups []string
	for _, e := range errs {
		if e.Code == ErrDuplicateRoute {
			dups = append(dups, e.Error())
		}
	}
	if len(dups) != 2 {
		t.Fatalf("want 2 duplicate errors (/x and /), got %v", errs)
	}
	all := strings.Join(dups, "\n")
	for _, want := range []string{"app/c-: E_DUPLICATE_ROUTE: pattern / is already served by app", "app/b-/x: E_DUPLICATE_ROUTE: pattern /x is already served by app/a-/x"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in:\n%s", want, all)
		}
	}
	_, errs = scanApp(t, "err_group_dynamic")
	if len(errs) != 1 || errs[0].Code != ErrBadSegment || errs[0].File != "app/slug_-" {
		t.Fatal(errs)
	}
}

func TestCustomMainAndShutdown(t *testing.T) {
	res, errs := scanApp(t, "custom_main")
	if errs != nil {
		t.Fatal(errs)
	}
	if !res.HasMain {
		t.Fatal("main.go with func main must set HasMain")
	}
	if res.ShutdownFunc == nil || res.ShutdownFunc.Func != "Shutdown" {
		t.Fatalf("Shutdown not detected: %+v", res.ShutdownFunc)
	}
	if full, _ := scanApp(t, "full"); full.HasMain {
		t.Fatal("full has no hand-written main")
	}
}

// #15 — Config may return an error: reading the app's own configuration is
// what fails on boot, and it has to fail where it happens.
func TestConfigMayReturnError(t *testing.T) {
	res, errs := scanApp(t, "custom_main")
	if errs != nil {
		t.Fatal(errs)
	}
	if res.ConfigFunc == nil || !res.ConfigReturnsError {
		t.Fatalf("func Config(cfg) error not detected: %+v", res.ConfigFunc)
	}
	full, _ := scanApp(t, "full")
	if full.ConfigFunc == nil || full.ConfigReturnsError {
		t.Fatalf("the form without a return must keep working: %+v", full.ConfigFunc)
	}
}

func TestDotFolders(t *testing.T) {
	res, errs := scanApp(t, "dotdir")
	if errs != nil {
		t.Fatal(errs)
	}
	var got []string
	for _, r := range res.Routes {
		got = append(got, r.Pattern+" "+r.Alias+" "+r.ImportPath+" kind="+strconv.FormatBool(r.HasKind))
	}
	want := []string{
		"/ app example.com/dotdir/app kind=false",
		"/app.css app_app_css example.com/dotdir/app/app.css kind=true",
		"/manifest.webmanifest app_manifest_webmanifest example.com/dotdir/app/manifest.webmanifest kind=false",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestMethodMiddleware locks the inheritance of MiddlewareX: it follows the
// subtree like Middleware, it lands only on the methods the route actually
// serves, and it lands after the route-wide chain.
func TestMethodMiddleware(t *testing.T) {
	res, errs := scanApp(t, "methodmw")
	if errs != nil {
		t.Fatal(errs)
	}
	byPat := map[string]Route{}
	for _, r := range res.Routes {
		byPat[r.Pattern] = r
	}
	// The root page serves GET only: the root MiddlewarePOST does not reach it.
	if got := byPat["/"]; len(got.MiddlewaresByMethod) != 0 {
		t.Errorf("/: %v", got.MiddlewaresByMethod)
	}
	// A page with a POST form: root chain first, then the directory's own.
	seg := byPat["/segmentos"]
	if got := refs(seg.MiddlewaresByMethod["POST"]); got != "app.MiddlewarePOST app_segmentos.MiddlewarePOST" {
		t.Errorf("/segmentos POST: %q", got)
	}
	if got := refs(seg.MiddlewaresByMethod["GET"]); got != "app_segmentos.MiddlewareGET" {
		t.Errorf("/segmentos GET: %q", got)
	}
	if got := refs(seg.Middlewares); got != "app.Middleware" {
		t.Errorf("/segmentos route chain: %q", got)
	}
	// An API route below the root inherits the root's POST chain, not the
	// sibling directory's.
	api := byPat["/api/funis"]
	if got := refs(api.MiddlewaresByMethod["POST"]); got != "app.MiddlewarePOST" {
		t.Errorf("/api/funis POST: %q", got)
	}
	if _, ok := api.MiddlewaresByMethod["GET"]; ok {
		t.Errorf("/api/funis GET inherited a chain it was never given: %v", api.MiddlewaresByMethod)
	}
}

// TestErrorsCarryLineAndFix is the contract the check command depends on
// (spec 047): a violation says where it is and what resolves it.
func TestErrorsCarryLineAndFix(t *testing.T) {
	cases := []struct {
		app, code, file string
		line            int
		msg, fix        string
	}{
		{"err_no_page_func", ErrNoPageFunc, "app/page.go", 3, "found func Render", "rename"},
		{"err_no_method", ErrNoMethod, "app/api/route.go", 3, "found func Handle", "HTTP method"},
		{"err_lowercase_method", ErrNoMethod, "app/api/route.go", 3, "found func get", "upper case"},
		{"err_dup_param", ErrDuplicateParam, "app/a/id_/b/id_", 0, "id", "rename"},
	}
	for _, c := range cases {
		_, errs := scanApp(t, c.app)
		var got *Error
		for _, e := range errs {
			if e.Code == c.code && e.File == c.file {
				got = e
			}
		}
		if got == nil {
			t.Errorf("%s: want %s at %s, got %v", c.app, c.code, c.file, errs)
			continue
		}
		if got.Line != c.line {
			t.Errorf("%s: line %d, want %d", c.app, got.Line, c.line)
		}
		if !strings.Contains(got.Msg, c.msg) {
			t.Errorf("%s: message %q does not say %q", c.app, got.Msg, c.msg)
		}
		if !strings.Contains(got.Fix, c.fix) {
			t.Errorf("%s: fix %q does not say %q", c.app, got.Fix, c.fix)
		}
	}
}

// TestEveryCodeHasFix keeps the promise above whole: a code without a fix is
// a round trip the reader pays for.
func TestEveryCodeHasFix(t *testing.T) {
	codes := []string{
		ErrPageAndRoute, ErrNoPageFunc, ErrNoMethod, ErrNoLayoutFunc, ErrNoMiddlewareFunc,
		ErrUnusedMethodMW, ErrNoNotFoundFunc, ErrNoErrorFunc, ErrNoSetupFunc,
		ErrAmbiguousSegment, ErrCatchAllNotLeaf, ErrBadSegment, ErrDuplicateRoute,
		ErrDuplicateParam, ErrParse, ErrNoApp,
	}
	for _, c := range codes {
		if fixes[c] == "" {
			t.Errorf("%s has no fix", c)
		}
	}
}

// #75 — a folder that starts with a dot is skipped, and that silence turned a
// URL an RFC requires into a 404 nobody could explain. /.well-known/ is now the
// single exception, and every other skipped dot folder says what it swallowed.
func TestWellKnownIsTheException(t *testing.T) {
	res, errs := scanApp(t, "wellknown")
	if errs != nil {
		t.Fatal(errs)
	}
	var got []string
	for _, r := range res.Routes {
		got = append(got, r.Pattern+" "+r.Alias+" "+r.ImportPath)
	}
	want := []string{
		"/ app example.com/wellknown/app",
		"/.well-known/security.txt app__well_known_security_txt example.com/wellknown/app/.well-known/security.txt",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHiddenRouteIsReported(t *testing.T) {
	_, errs := scanApp(t, "err_hidden_route")
	if len(errs) != 1 || errs[0].Code != ErrHiddenRoute || errs[0].File != "app/.oauth/route.go" {
		t.Fatalf("want one %s about app/.oauth/route.go, got %v", ErrHiddenRoute, errs)
	}
	if errs[0].Fix == "" || !strings.Contains(errs[0].Msg, ".well-known") {
		t.Fatalf("the message has to name the exception and carry a fix: %+v", errs[0])
	}
}

func TestOptionsIsARoutableMethod(t *testing.T) {
	res, errs := scanApp(t, "cors")
	if errs != nil {
		t.Fatal(errs)
	}
	byPat := map[string]Route{}
	for _, r := range res.Routes {
		byPat[r.Pattern] = r
	}
	api := byPat["/api"]
	if strings.Join(api.Methods, ",") != "GET,OPTIONS" {
		t.Fatalf("a preflight written by hand is a handler like the others: %v", api.Methods)
	}
	if refs(api.MiddlewaresByMethod["OPTIONS"]) != "app_api.MiddlewareOPTIONS" {
		t.Fatalf("MiddlewareOPTIONS guards the method: %v", api.MiddlewaresByMethod)
	}
	if api.HasCORS {
		t.Fatal("this route declares no var CORS")
	}
	doc := byPat["/.well-known/oauth-protected-resource"]
	if !doc.HasCORS {
		t.Fatal("var CORS in route.go is the route's own policy and has to reach the generator")
	}
}

func TestUnroutableMethodIsReported(t *testing.T) {
	_, errs := scanApp(t, "err_unroutable_method")
	if len(errs) != 1 || errs[0].Code != ErrUnroutableMethod {
		t.Fatalf("want one %s, got %v", ErrUnroutableMethod, errs)
	}
	e := errs[0]
	if e.File != "app/arquivos/route.go" || e.Line == 0 || !strings.Contains(e.Msg, "GET already answers HEAD") {
		t.Fatalf("the message points at the line and says what answers instead: %+v", e)
	}
}

func TestCORSOnPageIsReported(t *testing.T) {
	_, errs := scanApp(t, "err_cors_on_page")
	if len(errs) != 1 || errs[0].Code != ErrCORSOnPage || errs[0].File != "app/page.go" {
		t.Fatalf("want one %s about app/page.go, got %v", ErrCORSOnPage, errs)
	}
}
