package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/midia"
)

// #201 — o episódio vem de outro serviço, num corpo que não sabe buscar
// posição. A primeira resposta ensina o tamanho ao navegador; a segunda é o
// trecho que ele pediu, que é o que faz o <audio> do iPhone posicionar.
func TestEpisodioRespondeRangeMesmoSemBuscarPosicao(t *testing.T) {
	c := newClient(t, "prod")
	_, tamanho := midia.Episodio()

	inteiro := c.Get("/midia/audio")
	inteiro.WantStatus(200).
		WantHeader("Accept-Ranges", "bytes").
		WantHeader("Content-Type", "audio/wav")
	if int64(inteiro.Body.Len()) != tamanho {
		t.Fatalf("%d bytes, queria %d", inteiro.Body.Len(), tamanho)
	}

	trecho := c.Request("GET", "/midia/audio", trilha.WithHeader("Range", "bytes=44-1043"))
	trecho.WantStatus(206).
		WantHeader("Content-Length", "1000").
		WantHeader("Content-Range", "bytes 44-1043/"+strconv.FormatInt(tamanho, 10))
	if trecho.Body.Len() != 1000 {
		t.Fatalf("o trecho veio com %d bytes", trecho.Body.Len())
	}
	if d := trecho.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "inline; ") {
		t.Fatalf("o 206 saiu fora do envelope: %q", d)
	}
	if trecho.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("206 sem nosniff")
	}

	fora := c.Request("GET", "/midia/audio", trilha.WithHeader("Range", "bytes=99999999-"))
	fora.WantStatus(416).WantHeader("Content-Range", "bytes */"+strconv.FormatInt(tamanho, 10))
}

// #210 — o logotipo em SVG sai inline, com a política que desliga o script; o
// que não é desenho nenhum cai na marca padrão em vez de numa página de erro.
func TestMarcaEmSvgSaiNeutralizadaEOutroTipoCaiNoPadrao(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/midia/marca")
	rec.WantStatus(200).
		WantHeader("Content-Type", "image/svg+xml").
		WantHeader("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'self'").
		WantContains("<svg")
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, `inline; filename="marca.svg"`) {
		t.Fatalf("disposition %q", d)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("SVG neutralizado sem nosniff")
	}

	// A organização que subiu um zip: a recusa é tratada pela rota.
	c.Get("/midia/marca?org=acervo").WantStatus(200).WantContains("<svg")
	c.Get("/midia/marca?org=nao-existe").WantStatus(404)

	// E a tela junta os dois.
	c.Get("/midia").WantStatus(200).WantContains(`src="/midia/marca"`, `<audio controls src="/midia/audio"`)
}
