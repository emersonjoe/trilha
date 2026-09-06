package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// SchemaForm renders a form the product defined at runtime: the step of a
// workflow, the public form behind a token, the settings of a tenant. It draws
// the fields and nothing else — the <form>, the CSRF input and the button are
// the app's, as in every other form of the kit, because where the form posts
// and what the button says are not in the schema.
//
//	values, err := trilha.BindSchema(c, schema)
//	errs, _ := err.(trilha.FieldErrors)
//	return h.Form(h.Method("post"), trilha.CSRFInput(c),
//		ui.SchemaForm(schema, values, errs),
//		ui.Button(h.Text("Save")))
//
// values and errs are what BindSchema returned, so a form that came back with
// messages comes back filled in.
func SchemaForm(schema trilha.Schema, values map[string]string, errs map[string]string, children ...h.Node) h.Node {
	fields := make([]h.Node, 0, len(schema)+len(children))
	for _, f := range schema {
		fields = append(fields, schemaField(f, values[f.Name], errs))
	}
	return h.Group(append(fields, children...)...)
}

// schemaField is one field: the control its type asks for, wrapped in the same
// ui.Field every hand-written form uses, so help, message and aria say the
// same things here as there.
func schemaField(f trilha.SchemaField, value string, errs map[string]string) h.Node {
	if f.Type == "display" {
		return h.Div(h.Class("ui-field ui-field-display"), h.P(h.Text(f.Text)))
	}
	id := "f-" + f.Name
	invalid := InvalidIf(errs, f.Name)
	common := []h.Node{h.ID(id), h.Name(f.Name), invalid}
	if f.Required {
		common = append(common, h.Required())
	}
	var control h.Node
	switch f.Type {
	case "textarea":
		rows := f.Rows
		if rows == 0 {
			rows = 3
		}
		control = Textarea(append(common, h.Attr("rows", strconv.Itoa(rows)), h.Text(value))...)
	case "select":
		opts := make([]Option, 0, len(f.Options)+1)
		if !f.Required {
			opts = append(opts, Option{})
		}
		for _, o := range f.Options {
			opts = append(opts, Option{Value: o.Value, Label: o.Label})
		}
		control = Select(append(common, SelectOptions(opts, value))...)
	case "checkbox":
		box := Checkbox(append(common, h.Value("true"), checkedIf(value == "true"))...)
		return h.Div(h.Class("ui-field"), CheckRow(box, f.Label, id), schemaHelp(f, id), schemaError(f, errs, id))
	case "number":
		control = Input(append(append(common, h.Type("number"), h.Attr("step", "any"), h.Value(value)), minmaxAttrs(f)...)...)
	case "date", "datetime":
		t := "date"
		if f.Type == "datetime" {
			t = "datetime-local"
		}
		control = Input(append(append(common, h.Type(t), h.Value(value)), minmaxAttrs(f)...)...)
	case "file":
		control = Input(append(common, h.Type("file"))...)
	default: // text, signature: a signature is a name typed in, and drawing it is the app's
		control = Input(append(append(common, h.Type("text"), h.Value(value), patternAttr(f)), lengthAttrs(f)...)...)
	}
	return Field(id, f.Label, control, Help(f.Help), Errors(errs, f.Name))
}

// minmaxAttrs and lengthAttrs put the schema's own limits on the control, so
// the browser says it before the round trip. The answer is still the server's.
func minmaxAttrs(f trilha.SchemaField) []h.Node {
	var n []h.Node
	if f.Min != "" {
		n = append(n, h.Attr("min", f.Min))
	}
	if f.Max != "" {
		n = append(n, h.Attr("max", f.Max))
	}
	return n
}

func lengthAttrs(f trilha.SchemaField) []h.Node {
	var n []h.Node
	if f.Min != "" {
		n = append(n, h.Attr("minlength", f.Min))
	}
	if f.Max != "" {
		n = append(n, h.Attr("maxlength", f.Max))
	}
	return n
}

func patternAttr(f trilha.SchemaField) h.Node {
	if f.Pattern == "" {
		return h.Nil
	}
	return h.Attr("pattern", f.Pattern)
}

func checkedIf(on bool) h.Node {
	if on {
		return h.Checked()
	}
	return h.Nil
}

// A checkbox carries its own label, so the help and the message are placed by
// hand instead of by ui.Field.
func schemaHelp(f trilha.SchemaField, id string) h.Node {
	if f.Help == "" {
		return h.Nil
	}
	return h.P(h.Class("ui-field-help"), h.ID(id+"-help"), h.Text(f.Help))
}

func schemaError(f trilha.SchemaField, errs map[string]string, id string) h.Node {
	msg := errs[f.Name]
	if msg == "" {
		return h.Nil
	}
	return h.P(h.Class("ui-field-error"), h.ID(id+"-error"), h.Role("alert"), h.Text(msg))
}
