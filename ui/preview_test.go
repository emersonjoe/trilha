package ui

import (
	"strings"
	"testing"
)

func TestPreviewEnquadraODocumento(t *testing.T) {
	got := render(t, Preview(nil, "/anexos/nota.pdf?ver=1", PreviewOpts{
		Title: "nota.pdf", Type: "application/pdf", Download: "/anexos/nota.pdf",
	}))
	for _, want := range []string{
		`class="ui-preview"`,
		`<iframe class="ui-preview-frame" src="/anexos/nota.pdf?ver=1"`,
		`title="nota.pdf"`,
		`loading="lazy"`,
		"height: 70vh",
		`href="/anexos/nota.pdf"`,
		">Download<",
		`rel="noopener noreferrer"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
	// O visualizador de PDF do navegador recusa rodar dentro de um frame com
	// sandbox — medido, não suposto. Os dois flags que o trariam de volta são
	// os que deixam um documento de mesma origem tirar o próprio sandbox, então
	// o atributo seria um rótulo e não uma cerca.
	if strings.Contains(got, "sandbox") {
		t.Fatalf("PDF não leva sandbox:\n%s", got)
	}
}

// Onde o sandbox não custa nada, ele fica.
func TestPreviewMantemOSandboxOndeFunciona(t *testing.T) {
	got := render(t, Preview(nil, "/a.txt", PreviewOpts{Title: "a.txt", Type: "text/plain"}))
	if !strings.Contains(got, `sandbox="allow-same-origin"`) {
		t.Fatalf("texto devia manter o sandbox:\n%s", got)
	}
}

// Uma foto dentro de um quadro é uma barra de rolagem em volta de uma foto.
func TestPreviewDeImagemUsaImg(t *testing.T) {
	got := render(t, Preview(nil, "/foto.png", PreviewOpts{Title: "foto.png", Type: "image/png"}))
	if strings.Contains(got, "<iframe") || !strings.Contains(got, `<img src="/foto.png" alt="foto.png"`) {
		t.Fatalf("imagem:\n%s", got)
	}
	// O clique é o zoom, e não custa script nenhum.
	if !strings.Contains(got, `class="ui-preview-image" href="/foto.png"`) && !strings.Contains(got, `href="/foto.png" target="_blank"`) {
		t.Fatalf("faltou o caminho para o tamanho real:\n%s", got)
	}
}

// O que o c.Inline recusa não vira um quadro em branco que não explica nada.
func TestPreviewDizQuandoNaoDaParaMostrar(t *testing.T) {
	got := render(t, Preview(nil, "/pagina.html", PreviewOpts{
		Title: "pagina.html", Type: "text/html", Download: "/pagina.html?baixar=1",
	}))
	if strings.Contains(got, "<iframe") {
		t.Fatalf("HTML não pode ser enquadrado:\n%s", got)
	}
	for _, want := range []string{"cannot be previewed", "Download the file", "ui-empty"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}

func TestPreviewSemTipoDeixaComONavegador(t *testing.T) {
	got := render(t, Preview(nil, "/x", PreviewOpts{Title: "x"}))
	if !strings.Contains(got, "<iframe") || !strings.Contains(got, `sandbox="allow-same-origin"`) {
		t.Fatalf("sem tipo:\n%s", got)
	}
}
