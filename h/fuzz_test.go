package h

import (
	"html"
	"strings"
	"testing"
)

// FuzzRenderEscapes: o h é a última coisa entre o texto de terceiro e o
// navegador. O alvo afirma as duas metades do contrato de uma vez — o valor
// chega inteiro (desescapar volta ao original) e não chega solto (nenhum
// delimitador de HTML sobrevive fora de entidade).
func FuzzRenderEscapes(f *testing.F) {
	for _, s := range []string{
		"", "olá", "a & b", "<script>alert(1)</script>", `" onload="alerta`,
		"&amp;", "&#34;", "'", "</div>", "\x00", "\n\t", "😀", strings.Repeat("<", 64),
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, err := Render(Div(Attr("data-x", s), Text(s)))
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		const abre = `<div data-x="`
		if !strings.HasPrefix(out, abre) || !strings.HasSuffix(out, "</div>") {
			t.Fatalf("estrutura perdida: %q", out)
		}
		resto := out[len(abre) : len(out)-len("</div>")]
		fim := strings.Index(resto, `">`)
		if fim < 0 {
			t.Fatalf("atributo sem fim: %q", out)
		}
		atributo, texto := resto[:fim], resto[fim+2:]

		// Nenhum delimitador escapa: nem para fora do atributo, nem para fora
		// do texto.
		for _, ch := range []string{`"`, "<", ">"} {
			if strings.Contains(atributo, ch) {
				t.Errorf("atributo trouxe %s solto: %q", ch, out)
			}
		}
		for _, ch := range []string{"<", ">"} {
			if strings.Contains(texto, ch) {
				t.Errorf("texto trouxe %s solto: %q", ch, out)
			}
		}

		// E o valor não se perdeu no caminho.
		if got := html.UnescapeString(atributo); got != s {
			t.Errorf("atributo desescapado = %q, queria %q", got, s)
		}
		if got := html.UnescapeString(texto); got != s {
			t.Errorf("texto desescapado = %q, queria %q", got, s)
		}
	})
}
