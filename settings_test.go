package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type pipeline struct {
	OCR      string `json:"ocr"      form:"ocr"      validate:"required,oneof=auto sempre nunca" label:"OCR" help:"auto detecta PDF sem texto"`
	Modelo   string `json:"modelo"   form:"modelo"   validate:"required" label:"Modelo"`
	Paralelo int    `json:"paralelo" form:"paralelo" validate:"min=1,max=8" label:"Documentos em paralelo"`
	Resumir  bool   `json:"resumir"  form:"resumir"  label:"Resumir"`
	interno  string
}

func padrao() pipeline { return pipeline{OCR: "auto", Modelo: "pequeno", Paralelo: 2} }

type memSettings struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memSettings) Load(k string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[k]
	return b, ok
}

func (s *memSettings) Save(k string, d []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string][]byte{}
	}
	s.m[k] = d
	return nil
}

// Antes de alguém gravar qualquer coisa, a resposta é o padrão — e não o zero
// da struct, que seria uma configuração que nunca ninguém escolheu.
func TestSettingsSemNadaGravadoUsaOPadrao(t *testing.T) {
	cfg := NewSettings("pipeline", padrao())
	if err := cfg.Bind(New(Config{Logger: quiet()}), &memSettings{}); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Get(); got != padrao() {
		t.Fatalf("%+v", got)
	}
}

func TestSettingsGravaELe(t *testing.T) {
	store := &memSettings{}
	cfg := NewSettings("pipeline", padrao())
	cfg.Bind(New(Config{Logger: quiet()}), store)

	novo := padrao()
	novo.Paralelo = 5
	if err := cfg.Set(novo); err != nil {
		t.Fatal(err)
	}
	if cfg.Get().Paralelo != 5 {
		t.Fatal("o Get não viu o Set")
	}
	// E outra instância, subindo depois, lê o que ficou gravado.
	outra := NewSettings("pipeline", padrao())
	outra.Bind(New(Config{Logger: quiet()}), store)
	if outra.Get().Paralelo != 5 {
		t.Fatalf("%+v", outra.Get())
	}
}

// Campo que sumiu entre dois deploys não derruba a seção: o que falta fica no
// padrão, que é melhor que uma configuração vazia em produção.
func TestSettingsDeOutraVersaoNaoDerruba(t *testing.T) {
	store := &memSettings{m: map[string][]byte{"pipeline": []byte(`{"ocr":"nunca","campo_que_nao_existe":1}`)}}
	cfg := NewSettings("pipeline", padrao())
	if err := cfg.Bind(New(Config{Logger: quiet()}), store); err != nil {
		t.Fatal(err)
	}
	got := cfg.Get()
	if got.OCR != "nunca" || got.Modelo != "pequeno" || got.Paralelo != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestSettingsIlegivelCaiNoPadrao(t *testing.T) {
	store := &memSettings{m: map[string][]byte{"pipeline": []byte("isto não é json")}}
	cfg := NewSettings("pipeline", padrao())
	if err := cfg.Bind(New(Config{Logger: quiet()}), store); err != nil {
		t.Fatal(err)
	}
	if cfg.Get() != padrao() {
		t.Fatalf("%+v", cfg.Get())
	}
}

// ---- a tela ----------------------------------------------------------------

func settingsCtx(a *App, method, body string) (*Ctx, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, "/admin/pipeline", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	return newCtx(a, &responseWriter{ResponseWriter: rec, status: http.StatusOK}, req, kindPage), rec
}

// 422 não grava. É a metade que toda app escreve errado: valida na tela, salva
// de qualquer jeito.
func TestSettingsUpdateInvalidoNaoGrava(t *testing.T) {
	store := &memSettings{}
	a := New(Config{Logger: quiet()})
	cfg := NewSettings("pipeline", padrao())
	cfg.Bind(a, store)

	c, _ := settingsCtx(a, "POST", "ocr=talvez&modelo=grande&paralelo=99")
	err := cfg.Update(c)
	fe, ok := err.(FieldErrors)
	if !ok {
		t.Fatalf("err = %v", err)
	}
	if fe["ocr"] == "" || fe["paralelo"] == "" {
		t.Fatalf("mensagens: %+v", fe)
	}
	if cfg.Get() != padrao() {
		t.Fatalf("gravou mesmo com erro: %+v", cfg.Get())
	}
	if len(store.m) != 0 {
		t.Fatal("chegou a escrever no store")
	}
}

func TestSettingsUpdateBomGravaEAudita(t *testing.T) {
	store := &memSettings{}
	var trilhaLog []AuditRecord
	a := New(Config{Logger: quiet(), Audit: AuditFunc(func(r AuditRecord) error {
		trilhaLog = append(trilhaLog, r)
		return nil
	})})
	cfg := NewSettings("pipeline", padrao())
	cfg.Bind(a, store)

	c, _ := settingsCtx(a, "POST", "ocr=sempre&modelo=grande&paralelo=4&resumir=on")
	err := cfg.Update(c)
	if _, ok := err.(*RedirectError); !ok {
		t.Fatalf("err = %v", err)
	}
	got := cfg.Get()
	if got.OCR != "sempre" || got.Modelo != "grande" || got.Paralelo != 4 || !got.Resumir {
		t.Fatalf("%+v", got)
	}
	if len(trilhaLog) != 1 {
		t.Fatalf("auditoria: %+v", trilhaLog)
	}
	campos, _ := trilhaLog[0].Fields["fields"].(string)
	// Os nomes do que mudou, e nenhum valor: uma tela de configuração é onde
	// mora um token, e a trilha não pode virar um segundo lugar de onde ele
	// vaza.
	for _, quer := range []string{"ocr", "modelo", "paralelo", "resumir"} {
		if !strings.Contains(campos, quer) {
			t.Fatalf("faltou %q em %q", quer, campos)
		}
	}
	if strings.Contains(campos, "grande") || strings.Contains(campos, "sempre") {
		t.Fatalf("a trilha copiou os valores: %q", campos)
	}
}

// ---- o formulário que vem da struct ----------------------------------------

func TestSchemaOfLeAsTags(t *testing.T) {
	s := SchemaOf[pipeline]()
	if len(s) != 4 {
		t.Fatalf("campos: %d (%+v)", len(s), s)
	}
	byName := map[string]SchemaField{}
	for _, f := range s {
		byName[f.Name] = f
	}
	ocr := byName["ocr"]
	if ocr.Type != "select" || len(ocr.Options) != 3 || !ocr.Required || ocr.Label != "OCR" || ocr.Help == "" {
		t.Fatalf("ocr = %+v", ocr)
	}
	if p := byName["paralelo"]; p.Type != "number" || p.Min != "1" || p.Max != "8" {
		t.Fatalf("paralelo = %+v", p)
	}
	if r := byName["resumir"]; r.Type != "checkbox" {
		t.Fatalf("resumir = %+v", r)
	}
	// O campo não exportado não vira campo de formulário.
	if _, existe := byName["interno"]; existe {
		t.Fatal("campo não exportado entrou no schema")
	}
	if err := s.Check(); err != nil {
		t.Fatalf("o schema gerado não passa na própria conferência: %v", err)
	}
}

// O que nenhum formulário segura é erro de programação, e explode na hora de
// escrever a tela — não em silêncio, com metade da configuração ineditável.
func TestSchemaOfRecusaOQueNaoCabeNumFormulario(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("devia ter explodido")
		}
	}()
	type mau struct {
		Agentes []string `form:"agentes"`
	}
	_ = SchemaOf[mau]()
}

func TestSettingsValuesSaoOQueOFormularioMostra(t *testing.T) {
	cfg := NewSettings("pipeline", pipeline{OCR: "auto", Modelo: "m", Paralelo: 3, Resumir: true})
	v := cfg.Values()
	if v["ocr"] != "auto" || v["paralelo"] != "3" || v["resumir"] != "on" {
		t.Fatalf("%+v", v)
	}
}
