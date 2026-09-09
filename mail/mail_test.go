package mail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

func fixaData(t *testing.T) {
	t.Helper()
	antes := now
	now = func() time.Time { return time.Date(2026, 9, 9, 15, 4, 5, 0, time.UTC) }
	t.Cleanup(func() { now = antes })
}

func caixa(t *testing.T) (*Mailer, *Outbox) {
	t.Helper()
	fixaData(t)
	box := &Outbox{}
	return New(Options{From: "Acervo <no-reply@org.br>", Transport: box}), box
}

// O que sai é sempre multipart/alternative, e o texto sai do mesmo h.Node:
// mensagem só em HTML é mensagem que um cliente sem HTML mostra vazia.
func TestSaiComTextoEHTML(t *testing.T) {
	m, box := caixa(t)
	link := "https://acervo.org.br/entrar?t=abc123"
	err := m.Send(context.Background(), Message{
		To:      []string{"Ana <ana@exemplo.com>"},
		Subject: "Você foi convidado",
		Body: Layout("Acervo",
			h.P(h.Text("Clique para entrar:")),
			Button("Entrar", link),
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	sent := box.Last()
	if sent.Subject != "Você foi convidado" {
		t.Fatalf("assunto = %q", sent.Subject)
	}
	if len(sent.To) != 1 || sent.To[0] != "ana@exemplo.com" {
		t.Fatalf("envelope = %v (o nome não vai no envelope)", sent.To)
	}
	if !strings.Contains(sent.HTML, link) {
		t.Fatalf("o link sumiu do HTML: %s", sent.HTML)
	}
	// O texto tem de trazer o link legível, que é a razão de ele existir.
	if !strings.Contains(sent.Text, "Entrar <"+link+">") {
		t.Fatalf("o texto não trouxe o link:\n%s", sent.Text)
	}
	if strings.Contains(sent.Text, "<td") || strings.Contains(sent.Text, "style=") {
		t.Fatalf("o texto veio com marcação:\n%s", sent.Text)
	}
}

// O assunto com acento não pode chegar como "VocÃª": ele viaja em Q-encoding,
// e a linha crua é a que prova isso — o Outbox já decodifica.
func TestAssuntoViajaCodificado(t *testing.T) {
	m, box := caixa(t)
	m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "Ação concluída",
		Body: h.P(h.Text("ok"))})
	raw := string(box.Last().Raw)
	if strings.Contains(raw, "Subject: Ação") {
		t.Fatal("o assunto foi cru para a rede")
	}
	if !strings.Contains(raw, "Subject: =?utf-8?q?") {
		t.Fatalf("sem Q-encoding:\n%s", primeirasLinhas(raw, 12))
	}
}

// Nenhuma linha pode passar de 998 octetos, que é o limite do SMTP — e uma
// linha de HTML gerado passa fácil. É o quoted-printable que resolve.
func TestNenhumaLinhaEstoura(t *testing.T) {
	m, box := caixa(t)
	longo := strings.Repeat("uma frase que se repete e não tem quebra nenhuma. ", 60)
	m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "longo",
		Body: Layout("Acervo", h.P(h.Text(longo)))})
	for i, linha := range strings.Split(string(box.Last().Raw), "\r\n") {
		if len(linha) > 998 {
			t.Fatalf("linha %d tem %d octetos", i, len(linha))
		}
	}
	if !strings.Contains(box.Last().Text, "uma frase que se repete") {
		t.Fatal("o texto não sobreviveu à codificação")
	}
}

// Bcc é destinatário do envelope e de cabeçalho nenhum. Escrevê-lo num
// cabeçalho é mostrar todo mundo para todo mundo, e é uma linha de distância.
func TestBccNaoVaiEmCabecalho(t *testing.T) {
	m, box := caixa(t)
	m.Send(context.Background(), Message{
		To: []string{"ana@exemplo.com"}, Bcc: []string{"chefe@exemplo.com"},
		Subject: "relatório", Body: h.P(h.Text("segue")),
	})
	sent := box.Last()
	if len(sent.To) != 2 {
		t.Fatalf("o envelope precisa do bcc: %v", sent.To)
	}
	if strings.Contains(string(sent.Raw), "chefe@exemplo.com") {
		t.Fatalf("o bcc apareceu na mensagem:\n%s", primeirasLinhas(string(sent.Raw), 12))
	}
}

// Cabeçalho que este pacote escreve não pode ser trocado por Headers: duas
// linhas From é mensagem que um servidor recusa e outro entrega errado.
func TestHeadersNaoTrocamOsNossos(t *testing.T) {
	m, box := caixa(t)
	m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "x",
		Body:    h.P(h.Text("x")),
		Headers: map[string]string{"From": "outro@invasor.com", "List-Unsubscribe": "<https://org.br/sair>"},
	})
	raw := string(box.Last().Raw)
	if strings.Count(raw, "From:") != 1 || strings.Contains(raw, "invasor") {
		t.Fatalf("From foi trocado:\n%s", primeirasLinhas(raw, 12))
	}
	if !strings.Contains(raw, "List-Unsubscribe: <https://org.br/sair>") {
		t.Fatal("o cabeçalho extra não passou")
	}
}

// Endereço inválido falha aqui, com o endereço na mensagem — e não num 501
// três saltos adiante.
func TestEnderecoInvalidoFalhaComOEndereco(t *testing.T) {
	m, _ := caixa(t)
	err := m.Send(context.Background(), Message{To: []string{"ana arroba exemplo"},
		Subject: "x", Body: h.P(h.Text("x"))})
	if err == nil || !strings.Contains(err.Error(), "ana arroba exemplo") {
		t.Fatalf("erro = %v", err)
	}
}

func TestSemDestinatarioOuSemCorpoEErro(t *testing.T) {
	m, _ := caixa(t)
	if err := m.Send(context.Background(), Message{Subject: "x", Body: h.P(h.Text("x"))}); err == nil {
		t.Fatal("passou sem destinatário")
	}
	if err := m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "x"}); err == nil {
		t.Fatal("passou sem corpo")
	}
}

// Sem servidor em produção o Send falha, e não finge. É a diferença entre um
// convite que não chegou e um convite que ninguém sabe que não chegou.
func TestSemServidorEErroENaoSilencio(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_MAIL_URL", "")
	m := New(FromEnv())
	err := m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "x",
		Body: h.P(h.Text("x"))})
	if err != ErrNotConfigured {
		t.Fatalf("erro = %v", err)
	}
}

// Em dev, sem servidor, o e-mail vira um .eml que se abre com dois cliques.
func TestEmDevEscreveEml(t *testing.T) {
	fixaData(t)
	dir := t.TempDir()
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_MAIL_URL", "")
	t.Setenv("TRILHA_MAIL_DIR", dir)
	m := New(FromEnv())
	if err := m.Send(context.Background(), Message{To: []string{"a@b.com"},
		Subject: "Você foi convidado", Body: Layout("Acervo", h.P(h.Text("oi")))}); err != nil {
		t.Fatal(err)
	}
	arquivos, _ := filepath.Glob(filepath.Join(dir, "*.eml"))
	if len(arquivos) != 1 {
		t.Fatalf("arquivos = %v", arquivos)
	}
	if base := filepath.Base(arquivos[0]); base != "2026-09-09T15-04-05-voc-foi-convidado.eml" {
		t.Fatalf("nome = %q", base)
	}
	b, _ := os.ReadFile(arquivos[0])
	if !strings.Contains(string(b), "Subject:") {
		t.Fatal("o .eml não tem cabeçalho")
	}
}

func TestFromEnvLeAURL(t *testing.T) {
	t.Setenv("TRILHA_MAIL_URL", "smtp://ana:senha@smtp.org.br:587?from=Acervo+%3Cno-reply@org.br%3E")
	t.Setenv("TRILHA_MAIL_FROM", "")
	o := FromEnv()
	s, ok := o.Transport.(*SMTP)
	if !ok {
		t.Fatalf("transporte = %T", o.Transport)
	}
	if s.Addr != "smtp.org.br:587" || s.User != "ana" || s.Pass != "senha" {
		t.Fatalf("%+v", s)
	}
	if o.From != "Acervo <no-reply@org.br>" {
		t.Fatalf("from = %q", o.From)
	}
	// smtps sem porta é 465, que é onde o TLS é implícito.
	t.Setenv("TRILHA_MAIL_URL", "smtps://smtp.org.br?from=a@b.com")
	if s := FromEnv().Transport.(*SMTP); s.Addr != "smtp.org.br:465" {
		t.Fatalf("addr = %q", s.Addr)
	}
}

func primeirasLinhas(s string, n int) string {
	linhas := strings.Split(s, "\r\n")
	if len(linhas) > n {
		linhas = linhas[:n]
	}
	return strings.Join(linhas, "\n")
}
