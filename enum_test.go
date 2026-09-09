package trilha

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

var statusEnum = Enum{
	{Value: "queued", Label: "Na fila"},
	{Value: "processing", Label: "Processando", Tone: "info"},
	{Value: "processed", Label: "Processado", Tone: "success"},
	{Value: "error", Label: "Erro", Tone: "danger"},
}

// A value the list does not know is a row somebody wrote before that value was
// retired. It has to render, not panic and not disappear: the screen showing
// "legacy" raw is a screen somebody can act on.
func TestEnumSurvivesAValueItDoesNotKnow(t *testing.T) {
	if got := statusEnum.Label("legado"); got != "legado" {
		t.Errorf("Label of an unknown value = %q, want it raw", got)
	}
	if got := statusEnum.Tone("legado"); got != "muted" {
		t.Errorf("Tone of an unknown value = %q, want muted", got)
	}
	if statusEnum.Has("legado") {
		t.Error("an unknown value passed Has")
	}
	// A value with no label of its own reads as itself, not as empty.
	bare := Enum{{Value: "x"}}
	if got := bare.Label("x"); got != "x" {
		t.Errorf("Label without a label = %q", got)
	}
	if got := bare.Tone("x"); got != "muted" {
		t.Errorf("Tone without a tone = %q", got)
	}
}

func TestEnumOptionsMarkTheCurrentValue(t *testing.T) {
	got, err := h.Render(statusEnum.Options("processed"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<option value="processed" selected>Processado</option>`) {
		t.Fatalf("the current value is not marked: %s", got)
	}
	if strings.Count(got, "selected") != 1 {
		t.Fatalf("more than one option is selected: %s", got)
	}
	// The placeholder is selected exactly when nothing is chosen yet.
	empty, _ := h.Render(statusEnum.Options("", "— escolha —"))
	if !strings.Contains(empty, `<option value="" selected>— escolha —</option>`) {
		t.Fatalf("the placeholder is not selected on an empty form: %s", empty)
	}
	chosen, _ := h.Render(statusEnum.Options("error", "— escolha —"))
	if strings.Contains(chosen, `value="" selected`) {
		t.Fatalf("the placeholder stayed selected with a value chosen: %s", chosen)
	}
}

// The tag cites the enum by the name it was registered under, and the message
// lists labels: the person filling the form read labels, not values.
func TestEnumTagMessageListsLabels(t *testing.T) {
	RegisterEnum("test.Status", statusEnum)

	var in struct {
		Status string `form:"status" validate:"required,enum=test.Status"`
	}
	res := TestRoute(t, Route{Pattern: "/e", Kind: KindAPI,
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
			if err := c.Bind(&in); err != nil {
				return err
			}
			return c.Text(200, "ok")
		}}}, "POST", "/e",
		WithForm(map[string][]string{"status": {"nao-existe"}}), WithoutCSRF())

	body := res.Body.String()
	if res.Code != 422 {
		t.Fatalf("status = %d, want 422: %s", res.Code, body)
	}
	for _, label := range []string{"Na fila", "Processando", "Processado", "Erro"} {
		if !strings.Contains(body, label) {
			t.Errorf("the message does not list %q: %s", label, body)
		}
	}
	if strings.Contains(body, "nao-existe") && !strings.Contains(body, "Na fila") {
		t.Error("the message echoes the value instead of listing the labels")
	}

	// A value the enum knows passes.
	ok := TestRoute(t, Route{Pattern: "/e", Kind: KindAPI,
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
			if err := c.Bind(&in); err != nil {
				return err
			}
			return c.Text(200, "ok")
		}}}, "POST", "/e",
		WithForm(map[string][]string{"status": {"processed"}}), WithoutCSRF())
	ok.WantStatus(200)
}

// A tag naming an enum nobody registered must not accept anything.
func TestEnumTagRefusesAnUnregisteredName(t *testing.T) {
	var in struct {
		Status string `form:"status" validate:"required,enum=nunca.Registrado"`
	}
	res := TestRoute(t, Route{Pattern: "/u", Kind: KindAPI,
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
			if err := c.Bind(&in); err != nil {
				return err
			}
			return c.Text(200, "ok")
		}}}, "POST", "/u",
		WithForm(map[string][]string{"status": {"qualquer"}}), WithoutCSRF())
	if res.Code == 200 {
		t.Fatal("a tag naming an enum nobody registered accepted the value")
	}
}

// Setup is where RegisterEnum belongs, and a test suite boots the app once per
// test — so registering the same list twice has to be fine. What must not be
// fine is two different lists behind one name: the form would validate against
// one and the select would draw the other.
func TestRegisterEnumTwiceIsFineUnlessItChanged(t *testing.T) {
	RegisterEnum("test.Twice", statusEnum)
	RegisterEnum("test.Twice", statusEnum) // a second boot

	defer func() {
		if recover() == nil {
			t.Fatal("two different lists under one name were accepted")
		}
	}()
	RegisterEnum("test.Twice", Enum{{Value: "outro", Label: "Outro"}})
}

// #124 — os dois símbolos que a 0.46.0 deixou sem exportar por não terem
// consumidor. O `trilha ctx` passou a listar os enums, e uma aplicação que
// quer a lista em tempo de execução — uma tela oferecendo todos os valores de
// um status — passa a ter por onde.
func TestLookupERegisteredEnums(t *testing.T) {
	RegisterEnum("teste.situacao", Enum{
		{Value: "aberto", Label: "Aberto"},
		{Value: "fechado", Label: "Fechado", Tone: "success"},
	})

	got, ok := LookupEnum("teste.situacao")
	if !ok || len(got) != 2 || got[1].Tone != "success" {
		t.Fatalf("lookup = %+v (%v)", got, ok)
	}
	if _, ok := LookupEnum("nao.registrado"); ok {
		t.Fatal("achou um enum que ninguém registrou")
	}

	todos := RegisteredEnums()
	if _, ok := todos["teste.situacao"]; !ok {
		t.Fatalf("o registro não apareceu: %v", todos)
	}
	// É uma cópia: mexer no que voltou não mexe no registro, senão quem lista
	// consegue reescrever a lista de outra pessoa sem querer.
	todos["teste.situacao"][0].Value = "mexido"
	delete(todos, "teste.situacao")
	depois, _ := LookupEnum("teste.situacao")
	if len(depois) != 2 || depois[0].Value != "aberto" {
		t.Fatalf("o registro foi alterado por fora: %+v", depois)
	}
}
