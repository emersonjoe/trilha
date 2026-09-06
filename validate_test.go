package trilha

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

// bindForm runs Bind over a form body, the way a handler would.
func bindForm(t *testing.T, v any, body string) FieldErrors {
	t.Helper()
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := &Ctx{r: req}
	err := c.Bind(v)
	if err == nil {
		return FieldErrors{}
	}
	fe, ok := err.(FieldErrors)
	if !ok {
		t.Fatalf("Bind returned %T: %v", err, err)
	}
	return fe
}

func TestRulesCoverAForm(t *testing.T) {
	var in struct {
		Nome  string   `form:"nome" validate:"required,min=3"`
		Email string   `form:"email" validate:"required,email"`
		Senha string   `form:"senha" validate:"required,min=8"`
		Conf  string   `form:"conf" validate:"eqfield=senha"`
		Plano string   `form:"plano" validate:"required,oneof=free pro"`
		Site  string   `form:"site" validate:"url"`
		Idade int      `form:"idade" validate:"required,min=18,max=120"`
		Tags  []string `form:"tags" validate:"min=1,max=3"`
		CEP   string   `form:"cep" validate:"len=8"`
	}
	errs := bindForm(t, &in, "nome=Ed&email=ed@arroba&senha=segredo&conf=outro&plano=gold&site=nao+e+url&idade=17&cep=123")
	want := map[string]string{
		"nome":  "must have at least 3 characters",
		"email": "invalid e-mail",
		"senha": "must have at least 8 characters",
		"conf":  "does not match",
		"plano": "invalid option",
		"site":  "invalid URL",
		"idade": "must be 18 or more",
		"cep":   "must have exactly 8 characters",
	}
	for k, v := range want {
		if errs[k] != v {
			t.Errorf("%s = %q, want %q", k, errs[k], v)
		}
	}
	// tags is empty and optional: nothing to say about it.
	if errs.Has("tags") {
		t.Errorf("tags = %q, want no message", errs["tags"])
	}
	if len(errs) != len(want) {
		t.Errorf("%d messages, want %d: %v", len(errs), len(want), errs)
	}

	ok := bindForm(t, &in, "nome=Emerson&email=ed@trilha.dev&senha=segredo1&conf=segredo1&plano=pro&site=https://trilha.dev&idade=44&tags=a&tags=b&cep=13010000")
	if ok.Any() {
		t.Fatalf("valid form failed: %v", ok)
	}
}

func TestRequiredIsTheZeroValue(t *testing.T) {
	var in struct {
		Quantidade int  `form:"quantidade" validate:"required"`
		Aceito     bool `form:"aceito" validate:"required"`
		Desconto   *int `form:"desconto" validate:"required"`
	}
	errs := bindForm(t, &in, "quantidade=0&aceito=off")
	for _, f := range []string{"quantidade", "aceito", "desconto"} {
		if errs[f] != "required" {
			t.Errorf("%s = %q, want required", f, errs[f])
		}
	}
	// A zero that means something travels in a pointer, which is how Bind
	// already tells "absent" from "sent".
	if errs := bindForm(t, &in, "quantidade=2&aceito=on&desconto=0"); errs.Any() {
		t.Fatalf("%v", errs)
	}
}

func TestEmptySkipsEveryRuleButRequired(t *testing.T) {
	var in struct {
		Site  string `form:"site" validate:"url"`
		Email string `form:"email" validate:"email,min=8"`
	}
	if errs := bindForm(t, &in, "site=&email="); errs.Any() {
		t.Fatalf("optional and empty must pass: %v", errs)
	}
	if errs := bindForm(t, &in, "email=a@b.co"); errs["email"] != "must have at least 8 characters" {
		t.Fatalf("email = %q", errs["email"])
	}
}

// A bad conversion is not a rule failure: telling someone who typed letters
// that the number must be 18 or more helps nobody.
func TestConversionErrorWinsOverTheRule(t *testing.T) {
	var in struct {
		Idade int `form:"idade" validate:"required,min=18"`
	}
	if errs := bindForm(t, &in, "idade=abc"); errs["idade"] != BindInvalid {
		t.Fatalf("idade = %q, want %q", errs["idade"], BindInvalid)
	}
}

type cpf string

func (c cpf) Validate() error {
	if len(c) != 11 {
		return errors.New("CPF tem 11 dígitos")
	}
	return nil
}

type inscricao struct {
	CPF   cpf    `form:"cpf" validate:"required"`
	Senha string `form:"senha"`
	Conf  string `form:"conf"`
}

// Validate is the check that does not fit a tag: it looks at two fields at
// once and knows what the app means.
func (c inscricao) Validate() error {
	if c.Senha != c.Conf {
		return FieldErrors{"conf": "as senhas não conferem"}
	}
	return nil
}

func TestTypeAndStructValidators(t *testing.T) {
	var in inscricao
	errs := bindForm(t, &in, "cpf=123&senha=a&conf=b")
	if errs["cpf"] != "CPF tem 11 dígitos" {
		t.Fatalf("cpf = %q", errs["cpf"])
	}
	// The struct check did not run: a field is still wrong.
	if errs.Has("conf") {
		t.Fatalf("struct Validate ran too early: %v", errs)
	}
	errs = bindForm(t, &in, "cpf=12345678901&senha=a&conf=b")
	if errs["conf"] != "as senhas não conferem" {
		t.Fatalf("conf = %q", errs["conf"])
	}
	if errs := bindForm(t, &in, "cpf=12345678901&senha=a&conf=a"); errs.Any() {
		t.Fatalf("%v", errs)
	}
}

func TestAddRule(t *testing.T) {
	AddRule("cep8", func(f Field) bool { return len(f.Text) == 8 })
	ValidationMessages["cep8"] = "CEP has 8 digits"
	defer delete(ValidationMessages, "cep8")

	var in struct {
		CEP string `form:"cep" validate:"cep8"`
	}
	if errs := bindForm(t, &in, "cep=1301"); errs["cep"] != "CEP has 8 digits" {
		t.Fatalf("cep = %q", errs["cep"])
	}
	if errs := bindForm(t, &in, "cep=13010000"); errs.Any() {
		t.Fatalf("%v", errs)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("a repeated rule name must panic")
		}
	}()
	AddRule("cep8", func(Field) bool { return true })
}

// A tag nobody registered is a bug in the app, and passing silently would let
// the form accept anything in production.
func TestUnknownRulePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("unknown rule must panic")
		}
	}()
	var in struct {
		X string `form:"x" validate:"nao-existe"`
	}
	bindForm(t, &in, "x=1")
}

func TestUseValidationPTBR(t *testing.T) {
	en, enInvalid := ValidationMessages, BindInvalid
	defer func() { ValidationMessages, BindInvalid = en, enInvalid }()
	UseValidationPTBR()

	var in struct {
		Nome  string `form:"nome" validate:"required"`
		Email string `form:"email" validate:"email"`
		Idade int    `form:"idade" validate:"required,min=18"`
	}
	errs := bindForm(t, &in, "nome=&email=nao&idade=abc")
	for f, want := range map[string]string{"nome": "obrigatório", "email": "e-mail inválido", "idade": "valor inválido"} {
		if errs[f] != want {
			t.Errorf("%s = %q, want %q", f, errs[f], want)
		}
	}
	if got := bindForm(t, &in, "nome=Ed&idade=12")["idade"]; got != "precisa ser 18 ou mais" {
		t.Errorf("idade = %q", got)
	}
}

// Issue #27: the message has to land on the name the form actually posted,
// prefix included, or it shows up next to no field at all.
func TestNestedStructKeepsThePrefix(t *testing.T) {
	var in struct {
		Entrega struct {
			CEP string `form:"cep" validate:"required,len=8"`
		} `form:"ent_"`
		Cobranca struct {
			CEP string `form:"cep" validate:"len=8"`
		}
	}
	errs := bindForm(t, &in, "ent_cep=13&cep=13010000")
	if errs["ent_cep"] != "must have exactly 8 characters" {
		t.Fatalf("ent_cep = %q, all: %v", errs["ent_cep"], errs)
	}
	if len(errs) != 1 {
		t.Fatalf("%v", errs)
	}
}

// A route that answers both the form and the API shares the struct; it would
// be a surprise if only one of them checked.
func TestBindJSONValidates(t *testing.T) {
	var in struct {
		Nome  string `json:"nome" form:"nome" validate:"required"`
		Email string `json:"email" form:"email" validate:"email"`
	}
	req := httptest.NewRequest("POST", "/api/x", strings.NewReader(`{"nome":"","email":"nao"}`))
	req.Header.Set("Content-Type", "application/json")
	c := &Ctx{r: req}
	err := c.Bind(&in)
	fe, ok := err.(FieldErrors)
	if !ok {
		t.Fatalf("%T: %v", err, err)
	}
	if fe["nome"] != "required" || fe["email"] != "invalid e-mail" {
		t.Fatalf("%v", fe)
	}
}
