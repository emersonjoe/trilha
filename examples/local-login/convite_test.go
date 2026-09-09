package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/examples/local-login/internal/correio"
	"github.com/emersonjoe/trilha/mail"
)

// caixa põe um Outbox no lugar do remetente do app, que é como um aplicativo
// testa e-mail: sem rede, sem contêiner, sem servidor SMTP falso.
func caixa(t *testing.T) *mail.Outbox {
	t.Helper()
	box := &mail.Outbox{}
	antes := correio.Mailer
	correio.Mailer = mail.New(mail.Options{From: "Acervo <no-reply@exemplo.com>", Transport: box})
	t.Cleanup(func() { correio.Mailer = antes })
	return box
}

// O ciclo inteiro do convite: quem administra convida, a pessoa recebe o link
// por e-mail, abre sem sessão nenhuma, escolhe a senha e entra.
func TestConviteDaTelaAteOLogin(t *testing.T) {
	box := caixa(t)
	c := cliente(t, "")

	// Bia administra usuários; Ana não.
	entrar(t, c, "bia@exemplo.com", "segredo-da-bia").WantStatus(http.StatusSeeOther)
	c.PostForm("/convites", url.Values{"nome": {"Caio"}, "email": {"caio@exemplo.com"}}).
		WantStatus(http.StatusSeeOther)

	if len(box.Messages()) != 1 {
		t.Fatalf("mensagens = %d", len(box.Messages()))
	}
	msg := box.Last()
	if len(msg.To) != 1 || msg.To[0] != "caio@exemplo.com" {
		t.Fatalf("destinatário = %v", msg.To)
	}
	if !strings.Contains(msg.HTML, "Bia convidou você") {
		t.Fatalf("o convite não diz quem convidou:\n%s", msg.Text)
	}

	// O link vive no texto tanto quanto no HTML — é o que faz a mensagem
	// funcionar num cliente que não mostra HTML.
	link := extraiURL(t, msg.Text, "/convite/")
	if !strings.Contains(msg.HTML, link) {
		t.Fatal("o link do texto não é o mesmo do HTML")
	}

	// Quem recebe abre sem sessão: o link é a autorização.
	anon := cliente(t, "")
	anon.Get(soOCaminho(link), navegador()).WantStatus(200).WantContains("Bem-vindo, Caio")
	anon.PostForm(soOCaminho(link), url.Values{"senha": {"uma-senha-comprida"}}).
		WantStatus(http.StatusSeeOther).WantHeader("Location", "/entrar")

	// A confirmação sai, e a conta funciona.
	if !strings.Contains(box.Last().Subject, "está pronta") {
		t.Fatalf("sem boas-vindas: %q", box.Last().Subject)
	}
	entrar(t, anon, "caio@exemplo.com", "uma-senha-comprida").WantStatus(http.StatusSeeOther)
	anon.Get("/painel").WantStatus(200).WantContains("Olá, Caio")
}

// Um uso só, e é o Consume que garante: o segundo POST no mesmo link não pode
// criar uma segunda conta.
func TestConviteValeUmaVez(t *testing.T) {
	box := caixa(t)
	c := cliente(t, "")
	entrar(t, c, "bia@exemplo.com", "segredo-da-bia")
	c.PostForm("/convites", url.Values{"nome": {"Dora"}, "email": {"dora@exemplo.com"}})
	link := soOCaminho(extraiURL(t, box.Last().Text, "/convite/"))

	anon := cliente(t, "")
	anon.PostForm(link, url.Values{"senha": {"uma-senha-comprida"}}).WantStatus(http.StatusSeeOther)
	if rec := anon.PostForm(link, url.Values{"senha": {"outra-senha-comprida"}}); rec.Code == http.StatusSeeOther {
		t.Fatal("o convite foi usado duas vezes")
	}
}

// Convidar é administrar usuários. Quem não administra não convida, e não
// descobre a tela pelo endereço.
func TestConvidarPedePermissao(t *testing.T) {
	caixa(t)
	c := cliente(t, "")
	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(http.StatusSeeOther)
	c.Get("/convites", navegador()).WantStatus(http.StatusForbidden)
}

// Token que não existe responde a mesma coisa que token vencido: 404. Dizer
// qual dos dois é dizer para um estranho o quanto ele chegou perto.
func TestTokenInventadoNaoAbre(t *testing.T) {
	caixa(t)
	c := cliente(t, "")
	c.Get("/convite/inventado", navegador()).WantStatus(http.StatusNotFound)
}

// O envio que falha não pode virar "convite enviado": a pessoa esperaria um
// e-mail que ninguém mandou.
func TestFalhaDeEnvioApareceNaTela(t *testing.T) {
	antes := correio.Mailer
	correio.Mailer = mail.New(mail.Options{From: "a@b.com"}) // sem transporte
	t.Cleanup(func() { correio.Mailer = antes })

	c := cliente(t, "")
	entrar(t, c, "bia@exemplo.com", "segredo-da-bia")
	c.PostForm("/convites", url.Values{"nome": {"Eva"}, "email": {"eva@exemplo.com"}}).
		WantStatus(http.StatusBadGateway).WantContains("não pôde ser enviado")
}

// extraiURL pega a primeira URL do texto que contém o trecho pedido.
func extraiURL(t *testing.T, texto, trecho string) string {
	t.Helper()
	for _, campo := range strings.FieldsFunc(texto, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '<' || r == '>'
	}) {
		if strings.Contains(campo, trecho) && strings.HasPrefix(campo, "http") {
			return campo
		}
	}
	t.Fatalf("não achei uma URL com %q em:\n%s", trecho, texto)
	return ""
}

// soOCaminho descarta o esquema e o host: o TestClient fala com o app, não com a
// rede.
func soOCaminho(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	return u.Path
}
