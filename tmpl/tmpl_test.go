package tmpl

import (
	"html/template"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha/h"
)

var fsys = fstest.MapFS{
	"rel.html":      {Data: []byte(`{{define "rel"}}<p>{{.Nome}}</p>{{end}}`)},
	"quebrado.html": {Data: []byte(`{{define "quebrado"}}{{.Nao.Existe}}{{end}}`)},
}

func TestNodeRendersInsideDSL(t *testing.T) {
	tpl := Must(fsys, "*.html")
	got, err := h.Render(h.Div(h.Class("x"), Node(tpl, "rel", map[string]string{"Nome": "<b>Ana</b>"})))
	if err != nil {
		t.Fatal(err)
	}
	if got != `<div class="x"><p>&lt;b&gt;Ana&lt;/b&gt;</p></div>` {
		t.Fatal(got)
	}
}

func TestMissingTemplateIsAnError(t *testing.T) {
	tpl := Must(fsys, "*.html")
	_, err := h.Render(Node(tpl, "nao-existe", nil))
	if err == nil || !strings.Contains(err.Error(), "nao-existe") {
		t.Fatal(err)
	}
	_, err = h.Render(Node(tpl, "quebrado", struct{}{}))
	if err == nil {
		t.Fatal("expected execution error")
	}
	if _, err := h.Render(Node(nil, "x", nil)); err == nil {
		t.Fatal("nil template must error")
	}
}

func TestMustPanicsOnBadPattern(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Must(fsys, "*.nada")
}

func TestPartialOutputIsDiscarded(t *testing.T) {
	tpl := template.Must(template.New("p").Parse(`antes {{.Nao.Existe}} depois`))
	var sb strings.Builder
	err := Node(tpl, "p", struct{}{}).Render(&sb)
	if err == nil || sb.Len() != 0 {
		t.Fatalf("err=%v out=%q", err, sb.String())
	}
}

var cascas = fstest.MapFS{
	"casca.html": {Data: []byte(`{{define "casca"}}<html><head><title>{{.Titulo}}</title></head><body><nav>{{.Menu}}</nav><main>{{template "conteudo" .}}</main></body></html>{{end}}`)},
	"vazia.html": {Data: []byte(`{{define "vazia"}}<html><body>sem slot</body></html>{{end}}`)},
	"duas.html":  {Data: []byte(`{{define "duas"}}<a>{{template "conteudo" .}}</a><b>{{template "conteudo" .}}</b>{{end}}`)},
}

type dadosDaCasca struct{ Titulo, Menu string }

func TestWrapPoeONoDentroDaCasca(t *testing.T) {
	casca := Wrap(Must(cascas, "*.html"), "casca", "conteudo")
	got, err := h.Render(casca.Node(dadosDaCasca{Titulo: "Painel <novo>", Menu: "<b>a</b>"},
		h.Div(h.Class("miolo"), h.Text("olá & tchau"))))
	if err != nil {
		t.Fatal(err)
	}
	// Os dados do app passam pelo escape do html/template; o miolo já veio
	// escapado pelo h e entra inteiro.
	for _, want := range []string{
		`<title>Painel &lt;novo&gt;</title>`,
		`<nav>&lt;b&gt;a&lt;/b&gt;</nav>`,
		`<main><div class="miolo">olá &amp; tchau</div></main>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("falta %q em %s", want, got)
		}
	}
	if strings.Contains(got, "trilha-slot-") {
		t.Fatalf("o marcador vazou: %s", got)
	}
}

func TestWrapSemSlotEUmErro(t *testing.T) {
	casca := Wrap(Must(cascas, "*.html"), "vazia", "conteudo")
	var sb strings.Builder
	err := casca.Node(nil, h.Text("miolo")).Render(&sb)
	if err == nil || !strings.Contains(err.Error(), "vazia") {
		t.Fatalf("erro %v", err)
	}
	if sb.String() != "" {
		t.Fatalf("erro não pode sair pela metade: %q", sb.String())
	}
	// Template que não existe e casca sem filho nenhum.
	if _, err := h.Render(Wrap(Must(cascas, "*.html"), "nao-existe", "conteudo").Node(nil, nil)); err == nil {
		t.Fatal("template inexistente tinha de dar erro")
	}
}

func TestWrapSlotChamadoDuasVezes(t *testing.T) {
	casca := Wrap(Must(cascas, "*.html"), "duas", "conteudo")
	got, err := h.Render(casca.Node(nil, h.Em(h.Text("x"))))
	if err != nil {
		t.Fatal(err)
	}
	if got != `<a><em>x</em></a><b><em>x</em></b>` {
		t.Fatal(got)
	}
}

func TestWrapExigeUmTemplateQueAindaNaoRodou(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "package level") {
			t.Fatalf("pânico %v", r)
		}
	}()
	tpl := Must(cascas, "*.html")
	_, _ = h.Render(Node(tpl, "vazia", nil)) // executou: o clone não é mais possível
	Wrap(tpl, "casca", "conteudo")
}

func TestWrapComTemplateNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("template nil tinha de dar pânico")
		}
	}()
	Wrap(nil, "casca", "conteudo")
}

func TestShellEUsavelEmParalelo(t *testing.T) {
	casca := Wrap(Must(cascas, "*.html"), "casca", "conteudo")
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := h.Render(casca.Node(dadosDaCasca{Titulo: strconv.Itoa(i)}, h.Text(strconv.Itoa(i))))
			if err != nil {
				t.Error(err)
				return
			}
			if !strings.Contains(s, "<main>"+strconv.Itoa(i)+"</main>") {
				t.Errorf("misturou as requisições: %s", s)
			}
		}()
	}
	wg.Wait()
}

func TestHTMLEntregaOValorPronto(t *testing.T) {
	got, err := HTML(h.Div(h.Text("a & b")))
	if err != nil || got != template.HTML(`<div>a &amp; b</div>`) {
		t.Fatalf("%q %v", got, err)
	}
	if got, err := HTML(nil); err != nil || got != "" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := HTML(Node(nil, "x", nil)); err == nil {
		t.Fatal("erro do nó tem de subir")
	}
}
