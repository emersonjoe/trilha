package trilha

import "testing"

func exampleSchema() Schema {
	return Schema{
		{Name: "titulo", Label: "Title", Type: "text", Required: true, Min: "3", Max: "10"},
		{Name: "obs", Label: "Notes", Type: "textarea"},
		{Name: "qtd", Label: "How many", Type: "number", Min: "1", Max: "9"},
		{Name: "prazo", Label: "Due", Type: "date", Max: "2030-01-01"},
		{Name: "tipo", Label: "Kind", Type: "select", Options: []SchemaOption{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}},
		{Name: "ok", Label: "Agreed", Type: "checkbox", Required: true},
		{Name: "cep", Label: "Postcode", Type: "text", Pattern: `^\d{5}-\d{3}$`},
		{Name: "anexo", Label: "File", Type: "file"},
		{Name: "assinatura", Label: "Signature", Type: "signature"},
		{Type: "display", Text: "Fill in what you can."},
	}
}

func bindSchemaForm(t *testing.T, body string) (map[string]string, error) {
	t.Helper()
	var values map[string]string
	var out error
	a := bindApp(t, func(c *Ctx) error {
		v, err := BindSchema(c, exampleSchema())
		values, out = v, err
		if err != nil {
			if _, ok := err.(FieldErrors); !ok {
				return err
			}
		}
		return c.Text(200, "ok")
	})
	if rec := postForm(a, "/api/x", body); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	return values, out
}

func TestBindSchema(t *testing.T) {
	values, err := bindSchemaForm(t, "titulo=Nota&qtd=3&prazo=2029-05-01&tipo=b&ok=on&cep=13000-000&assinatura=Ada")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if values["titulo"] != "Nota" || values["qtd"] != "3" || values["tipo"] != "b" || values["ok"] != "true" || values["assinatura"] != "Ada" {
		t.Fatalf("%v", values)
	}
	// A display field is not a field: it is not read and it never gets a
	// message, or "required" would fire on a paragraph.
	if _, ok := values["anexo"]; ok {
		t.Fatalf("a file was read by BindSchema: %v", values)
	}
}

func TestBindSchemaValidates(t *testing.T) {
	_, err := bindSchemaForm(t, "titulo=No&qtd=99&prazo=2031-01-01&tipo=z&cep=13000")
	errs, ok := err.(FieldErrors)
	if !ok {
		t.Fatalf("%v", err)
	}
	for _, f := range []string{"titulo", "qtd", "prazo", "tipo", "ok", "cep"} {
		if !errs.Has(f) {
			t.Fatalf("%s passed: %v", f, errs)
		}
	}
}

func TestBindSchemaCountsCharactersAndNumbers(t *testing.T) {
	// min=3 on a text is three characters; max=9 on a number is the number
	// itself. Same words in the schema, two sentences, because the value has
	// a type.
	_, err := bindSchemaForm(t, "titulo=No&qtd=99&ok=on")
	errs := err.(FieldErrors)
	if errs["titulo"] != message("minlen", "3") {
		t.Fatalf("titulo: %v", errs)
	}
	if errs["qtd"] != message("max", "9") {
		t.Fatalf("qtd: %v", errs)
	}
	// A number that arrived as words is one message, not two.
	_, err = bindSchemaForm(t, "titulo=Nota&qtd=tres&ok=on")
	if err.(FieldErrors)["qtd"] != BindInvalid {
		t.Fatalf("%v", err)
	}
}

// A schema the app got wrong is the app's problem, and it says so before
// anybody's answer is judged.
func TestSchemaCheck(t *testing.T) {
	for _, s := range []Schema{
		{{Name: "a", Type: "colour"}},
		{{Name: "", Type: "text"}},
		{{Name: "a", Type: "text"}, {Name: "a", Type: "text"}},
		{{Name: "a", Type: "text", Pattern: "("}},
	} {
		if err := s.Check(); err == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	if err := exampleSchema().Check(); err != nil {
		t.Fatal(err)
	}
}
