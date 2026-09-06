package trilha

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

type dependente struct {
	Nome  string `form:"nome" validate:"required,min=2"`
	Idade int    `form:"idade" validate:"min=0,max=130"`
}

type pedido struct {
	Deps []dependente   `form:"deps" validate:"minitems=1,maxitems=3"`
	Perm map[string]int `form:"perm" validate:"maxitems=4"`
	Nome string         `form:"nome"`
}

// bindErrs runs one form through Bind and returns what the handler saw.
func bindErrs(t *testing.T, body string, v any) FieldErrors {
	t.Helper()
	var out FieldErrors
	a := bindApp(t, func(c *Ctx) error {
		err := c.Bind(v)
		if fe, ok := err.(FieldErrors); ok {
			out = fe
			return c.Text(200, "ok")
		}
		if err != nil {
			return err
		}
		return c.Text(200, "ok")
	})
	if rec := postForm(a, "/api/x", body); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	return out
}

func TestBindListOfStructs(t *testing.T) {
	var got pedido
	errs := bindErrs(t, "nome=Ada&deps[0].nome=Bia&deps[0].idade=7&deps[1].nome=Caio&deps[1].idade=9", &got)
	if len(errs) != 0 {
		t.Fatalf("%v", errs)
	}
	if got.Nome != "Ada" || len(got.Deps) != 2 {
		t.Fatalf("%+v", got)
	}
	if got.Deps[0].Nome != "Bia" || got.Deps[0].Idade != 7 || got.Deps[1].Nome != "Caio" || got.Deps[1].Idade != 9 {
		t.Fatalf("%+v", got.Deps)
	}
}

// A browser that removed the second of three rows sends 0 and 2. The row the
// server is about to draw is the second one, so that is the number in the key.
func TestBindListCompactsASparseIndex(t *testing.T) {
	var got pedido
	errs := bindErrs(t, "deps[0].nome=Bia&deps[0].idade=7&deps[2].nome=C&deps[2].idade=9", &got)
	if len(got.Deps) != 2 || got.Deps[1].Nome != "C" {
		t.Fatalf("%+v", got.Deps)
	}
	if errs["deps[1].nome"] == "" {
		t.Fatalf("expected the message on the second row: %v", errs)
	}
	if errs["deps[2].nome"] != "" {
		t.Fatalf("the key of a row nobody will draw: %v", errs)
	}
}

func TestBindListReportsTheRowThatIsMissing(t *testing.T) {
	var got pedido
	errs := bindErrs(t, "deps[0].nome=Bia&deps[1].idade=9", &got)
	if errs["deps[1].nome"] == "" {
		t.Fatalf("required did not fire on the empty row: %v", errs)
	}
	if errs["deps[0].nome"] != "" {
		t.Fatalf("the row that was filled in got a message: %v", errs)
	}
}

func TestBindListCountsItems(t *testing.T) {
	var empty pedido
	if errs := bindErrs(t, "nome=Ada", &empty); errs["deps"] == "" {
		t.Fatalf("minitems did not fire on a list nobody sent: %v", errs)
	}
	var many pedido
	body := ""
	for i := 0; i < 6; i++ {
		body += "deps[" + string(rune('0'+i)) + "].nome=X" + string(rune('0'+i)) + "&"
	}
	errs := bindErrs(t, body, &many)
	if errs["deps"] == "" {
		t.Fatalf("maxitems did not fire: %v", errs)
	}
	// maxitems=3 reads four rows: enough to know the rule broke, not enough to
	// let the form decide how much the server allocates.
	if len(many.Deps) != 4 {
		t.Fatalf("read %d rows", len(many.Deps))
	}
}

// An index nobody could have drawn costs one row, not a billion.
func TestBindListDoesNotAllocateByIndex(t *testing.T) {
	var got pedido
	bindErrs(t, "deps[999999999].nome=Bia&deps[-1].nome=X&deps[].nome=Y&deps[0x1].nome=Z", &got)
	if len(got.Deps) != 1 || got.Deps[0].Nome != "Bia" {
		t.Fatalf("%+v", got.Deps)
	}
}

func TestBindMap(t *testing.T) {
	var got pedido
	errs := bindErrs(t, "deps[0].nome=Bia&perm[docs]=2&perm[fluxos]=1&perm[a.b c]=3", &got)
	if len(errs) != 0 {
		t.Fatalf("%v", errs)
	}
	if got.Perm["docs"] != 2 || got.Perm["fluxos"] != 1 || got.Perm["a.b c"] != 3 {
		t.Fatalf("%+v", got.Perm)
	}
}

func TestBindMapReportsTheKeyThatDidNotConvert(t *testing.T) {
	var got pedido
	errs := bindErrs(t, "deps[0].nome=Bia&perm[docs]=dois", &got)
	if errs["perm[docs]"] != BindInvalid {
		t.Fatalf("%v", errs)
	}
}

func TestBindMapIgnoresAKeyWithABracket(t *testing.T) {
	var got pedido
	bindErrs(t, "deps[0].nome=Bia&perm[a]b]=2&perm[]=3", &got)
	if len(got.Perm) != 0 {
		t.Fatalf("%+v", got.Perm)
	}
}

// The screen that renders the error has one name for the field, whether the
// values arrived as a form or as JSON.
func TestBindJSONUsesTheSameKey(t *testing.T) {
	var got struct {
		Deps []dependente `json:"deps"`
	}
	a := bindApp(t, func(c *Ctx) error {
		if err := c.Bind(&got); err != nil {
			return err
		}
		return c.Text(200, "ok")
	})
	req := httptest.NewRequest("POST", "/api/x", strings.NewReader(`{"deps":[{"nome":"Bia","idade":7},{"nome":"","idade":9}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var body struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Fields["deps[1].nome"] == "" {
		t.Fatalf("%v", body.Fields)
	}
}
