package uidoc

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"
	"github.com/emersonjoe/trilha/examples/patterns/approvalinbox"
	"github.com/emersonjoe/trilha/examples/patterns/asyncform"
	"github.com/emersonjoe/trilha/examples/patterns/dashboardchart"
	"github.com/emersonjoe/trilha/examples/patterns/listwithfilter"
	"github.com/emersonjoe/trilha/examples/patterns/masterdetail"
	"github.com/emersonjoe/trilha/examples/patterns/uploadprogress"
	"github.com/emersonjoe/trilha/ui"
)

// fakeData fills every pattern's data source with the same rows every run,
// so a render is a function of the code alone.
func fakeData() {
	listwithfilter.Orders = func(*trilha.Ctx, listwithfilter.Query) ([]listwithfilter.Order, int, error) {
		return []listwithfilter.Order{
			{ID: "o-1", Customer: "Ana Lima", Status: "open", Total: 120.5},
			{ID: "o-2", Customer: "Bruno Reis", Status: "paid", Total: 89},
		}, 2, nil
	}
	approvalinbox.Waiting = func(*trilha.Ctx) ([]approval.Record, error) {
		return []approval.Record{{ID: "ap-1", Kind: "refund", Subject: "Refund of 49.00", Target: "order o-1",
			State: approval.Pending}}, nil
	}
	dashboardchart.Load = func(*trilha.Ctx) (dashboardchart.Numbers, error) {
		return dashboardchart.Numbers{Open: "42", Paid: "1,204",
			ByType: []ui.Datum{{Label: "Retail", Value: 30}, {Label: "Wholesale", Value: 12}},
			Weekly: []float64{3, 5, 4, 8, 6, 9, 7}}, nil
	}
	masterdetail.Customers = func(*trilha.Ctx, trilha.ListParams) ([]masterdetail.Customer, int, error) {
		return []masterdetail.Customer{{ID: "c-1", Name: "Ana Lima", Email: "ana@example.com", Plan: "Pro"},
			{ID: "c-2", Name: "Bruno Reis", Email: "bruno@example.com", Plan: "Basic"}}, 2, nil
	}
	masterdetail.Find = func(_ *trilha.Ctx, id string) (masterdetail.Customer, error) {
		return masterdetail.Customer{ID: id, Name: "Bruno Reis", Email: "bruno@example.com", Plan: "Basic"}, nil
	}
}

// volatile is what changes between two renders of the same code: the CSP
// nonce, the CSRF token, the request id.
var volatile = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`nonce="[^"]*"`), `nonce="{{NONCE}}"`},
	{regexp.MustCompile(`(name="_csrf" value=")[^"]*"`), `${1}{{CSRF}}"`},
	{regexp.MustCompile(`(data-request-id=")[^"]*"`), `${1}{{RID}}"`},
}

// TestUIDocPatternGoldens renders every pattern's page.go with the fake data
// and holds the HTML to testdata/patterns/<name>.html: the markup a pattern
// promises is reviewed as a diff, and a kit change that moves it shows up
// here (make golden rewrites it).
func TestUIDocPatternGoldens(t *testing.T) {
	fakeData()
	pages := map[string]struct {
		page   trilha.PageFunc
		target string
	}{
		"list-with-filter": {listwithfilter.Page, "/p"},
		"async-form":       {asyncform.Page, "/p"},
		"approval-inbox":   {approvalinbox.Page, "/p"},
		"dashboard-chart":  {dashboardchart.Page, "/p"},
		"master-detail":    {masterdetail.Page, "/p?id=c-2"},
		"upload-progress":  {uploadprogress.Page, "/p"},
	}
	if len(pages) != len(ui.Patterns()) {
		t.Fatalf("%d pages for %d patterns", len(pages), len(ui.Patterns()))
	}
	for _, p := range ui.Patterns() {
		pg, ok := pages[p.Name]
		if !ok {
			t.Fatalf("%s has no render here", p.Name)
		}
		rec := trilha.TestRoute(t, trilha.Route{Pattern: "/p", Page: pg.page}, "GET", pg.target)
		if rec.Code != 200 {
			t.Fatalf("%s: %d\n%s", p.Name, rec.Code, rec.Body.String())
		}
		got := rec.Body.String()
		for _, v := range volatile {
			got = v.re.ReplaceAllString(got, v.with)
		}
		golden := filepath.Join("testdata", "patterns", p.Name+".html")
		if *update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%s: %v (run make golden)", p.Name, err)
		}
		if got != string(want) {
			t.Errorf("%s: the render changed; run make golden if on purpose", p.Name)
		}
	}
}
