package ui

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

func schema() trilha.Schema {
	return trilha.Schema{
		{Name: "titulo", Label: "Title", Type: "text", Required: true, Min: "3", Max: "10", Help: "As it appears on the cover"},
		{Name: "obs", Label: "Notes", Type: "textarea", Rows: 5},
		{Name: "qtd", Label: "How many", Type: "number", Min: "1", Max: "9"},
		{Name: "prazo", Label: "Due", Type: "date"},
		{Name: "quando", Label: "When", Type: "datetime"},
		{Name: "tipo", Label: "Kind", Type: "select", Options: []trilha.SchemaOption{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}},
		{Name: "ok", Label: "Agreed", Type: "checkbox"},
		{Name: "cep", Label: "Postcode", Type: "text", Pattern: `^\d{5}-\d{3}$`},
		{Name: "anexo", Label: "File", Type: "file"},
		{Name: "assinatura", Label: "Signature", Type: "signature"},
		{Type: "display", Text: "Fill in what you can."},
	}
}

func TestSchemaFormDrawsEveryType(t *testing.T) {
	got := render(t, SchemaForm(schema(), map[string]string{"titulo": "Nota", "obs": "oi", "tipo": "b", "ok": "true"}, nil))
	for _, want := range []string{
		`<input class="ui-input" id="f-titulo" name="titulo" required type="text" value="Nota"`,
		`minlength="3"`, `maxlength="10"`, `pattern="^\d{5}-\d{3}$"`,
		`<textarea class="ui-textarea" id="f-obs" name="obs" rows="5">oi</textarea>`,
		`type="number"`, `step="any"`, `min="1"`, `max="9"`,
		`type="date"`, `type="datetime-local"`, `type="file"`,
		`<select class="ui-select" id="f-tipo" name="tipo"`,
		`<option value="b" selected>B</option>`,
		`type="checkbox"`, `checked`,
		`Fill in what you can.`,
		`As it appears on the cover`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in\n%s", want, got)
		}
	}
	// A signature is a name typed in; drawing it is the app's business.
	if !strings.Contains(got, `id="f-assinatura" name="assinatura"`) {
		t.Fatalf("no signature field:\n%s", got)
	}
}

// The message lands beside its own field, and the control says it is invalid —
// the same round trip a hand-written form has.
func TestSchemaFormShowsTheMessage(t *testing.T) {
	errs := trilha.FieldErrors{"titulo": "must have at least 3 characters", "ok": "required"}
	got := render(t, SchemaForm(schema(), map[string]string{"titulo": "No"}, errs))
	if !strings.Contains(got, `must have at least 3 characters`) || !strings.Contains(got, `role="alert"`) {
		t.Fatalf("no message:\n%s", got)
	}
	if strings.Count(got, `aria-invalid="true"`) != 2 {
		t.Fatalf("expected two invalid controls:\n%s", got)
	}
	if !strings.Contains(got, `value="No"`) {
		t.Fatalf("the form came back empty:\n%s", got)
	}
}

// An optional select opens with the empty choice, so the browser does not pick
// the first real option for somebody who never touched it.
func TestSchemaFormSelectHasAWayOut(t *testing.T) {
	got := render(t, SchemaForm(trilha.Schema{
		{Name: "tipo", Label: "Kind", Type: "select", Options: []trilha.SchemaOption{{Value: "a", Label: "A"}}},
	}, nil, nil))
	if !strings.Contains(got, `<option value="" selected`) {
		t.Fatalf("%s", got)
	}
	got = render(t, SchemaForm(trilha.Schema{
		{Name: "tipo", Label: "Kind", Type: "select", Required: true, Options: []trilha.SchemaOption{{Value: "a", Label: "A"}}},
	}, nil, nil))
	if strings.Contains(got, `<option value=""`) {
		t.Fatalf("a required select kept the empty choice:\n%s", got)
	}
}
