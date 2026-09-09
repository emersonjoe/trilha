package mail

import (
	"context"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// O alternativo em texto é o que um cliente sem HTML mostra, e é o que um
// filtro de spam lê. Estes são os casos em que uma conversão ingênua perde
// informação.
func TestPlainText(t *testing.T) {
	casos := []struct {
		nome  string
		no    h.Node
		quero string
	}{
		{"parágrafos viram linhas separadas",
			h.Fragment(h.P(h.Text("um")), h.P(h.Text("dois"))),
			"um\n\ndois\n"},
		{"o link aparece, senão a mensagem não serve para nada",
			h.P(h.Text("Clique "), h.A(h.Href("https://org.br/x"), h.Text("aqui"))),
			"Clique aqui <https://org.br/x>\n"},
		{"link cujo texto já é a URL não vira eco",
			h.P(h.A(h.Href("https://org.br/x"), h.Text("https://org.br/x"))),
			"https://org.br/x\n"},
		{"lista é uma linha por item",
			h.Ul(h.Li(h.Text("um")), h.Li(h.Text("dois"))),
			"- um\n- dois\n"},
		{"entidade volta a ser caractere",
			h.P(h.Text("Ação & cia <n>")),
			"Ação & cia <n>\n"},
		{"o CSS não é conteúdo",
			h.Fragment(h.Style(h.Raw("p{color:red}")), h.P(h.Text("oi"))),
			"oi\n"},
		{"quebra explícita é quebra",
			h.P(h.Text("um"), h.Br(), h.Text("dois")),
			"um\ndois\n"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := PlainText(c.no); got != c.quero {
				t.Fatalf("veio %q, queria %q", got, c.quero)
			}
		})
	}
}

// O texto escrito à mão manda: quem escreveu o alternativo tem um motivo.
func TestTextEscritoAMaoGanha(t *testing.T) {
	m, box := caixa(t)
	m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "x",
		Body: h.P(h.Text("gerado")), Text: "escrito à mão"})
	if got := box.Last().Text; !strings.Contains(got, "escrito à mão") || strings.Contains(got, "gerado") {
		t.Fatalf("texto = %q", got)
	}
}

// Contexto nulo vem de um job que nunca teve requisição de onde tirar um. Ele
// é engano, mas engano não merece um stack trace apontando para context.go.
func TestContextoNuloNaoEstoura(t *testing.T) {
	m, box := caixa(t)
	if err := m.Send(nil, Message{To: []string{"a@b.com"}, Subject: "x", Body: h.P(h.Text("x"))}); err != nil {
		t.Fatal(err)
	}
	if len(box.Messages()) != 1 {
		t.Fatal("não enviou")
	}
}
