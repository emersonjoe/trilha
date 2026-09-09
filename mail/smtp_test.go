package mail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// servidorSMTP é um servidor de verdade num socket de verdade, com TLS de
// verdade: é a única forma de provar que o STARTTLS foi negociado e que a
// senha não saiu antes dele. Um transporte falso provaria que este pacote
// chama os próprios métodos.
type servidorSMTP struct {
	t          *testing.T
	addr       string
	tlsConf    *tls.Config
	semTLS     bool     // não anuncia STARTTLS
	auth       []string // métodos anunciados
	implicit   bool     // TLS já na conexão, como na 465
	authAberto bool     // anuncia AUTH mesmo sem TLS, como um relay velho

	mu       sync.Mutex
	comandos []string
	dados    string
	usuario  string
	senha    string
	claro    bool // algum AUTH chegou fora de TLS
}

func novoSMTP(t *testing.T, ajusta func(*servidorSMTP)) *servidorSMTP {
	t.Helper()
	s := &servidorSMTP{t: t, auth: []string{"PLAIN"}, tlsConf: certificado(t)}
	if ajusta != nil {
		ajusta(s)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.addr = ln.Addr().String()
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.atende(conn)
		}
	}()
	return s
}

func (s *servidorSMTP) atende(conn net.Conn) {
	defer conn.Close()
	cifrado := false
	if s.implicit {
		tc := tls.Server(conn, s.tlsConf)
		if err := tc.Handshake(); err != nil {
			return
		}
		conn = tc
		cifrado = true
	}
	r := bufio.NewReader(conn)
	escreve := func(linha string) { conn.Write([]byte(linha + "\r\n")) }
	escreve("220 teste ESMTP")

	for {
		linha, err := r.ReadString('\n')
		if err != nil {
			return
		}
		linha = strings.TrimRight(linha, "\r\n")
		s.mu.Lock()
		s.comandos = append(s.comandos, linha)
		s.mu.Unlock()
		cmd := strings.ToUpper(strings.Fields(linha + " ")[0])

		switch cmd {
		case "EHLO":
			escreve("250-teste")
			if !cifrado && !s.semTLS {
				escreve("250-STARTTLS")
			}
			if (cifrado || s.authAberto) && len(s.auth) > 0 {
				escreve("250-AUTH " + strings.Join(s.auth, " "))
			}
			escreve("250 SIZE 35882577")
		case "STARTTLS":
			escreve("220 vamos")
			tc := tls.Server(conn, s.tlsConf)
			if err := tc.Handshake(); err != nil {
				return
			}
			conn, cifrado = tc, true
			r = bufio.NewReader(conn)
			escreve = func(linha string) { conn.Write([]byte(linha + "\r\n")) }
		case "AUTH":
			if !cifrado {
				s.mu.Lock()
				s.claro = true
				s.mu.Unlock()
			}
			partes := strings.Fields(linha)
			if len(partes) > 1 && strings.EqualFold(partes[1], "PLAIN") {
				if len(partes) > 2 {
					s.guardaPlain(partes[2])
				}
				escreve("235 ok")
				continue
			}
			// LOGIN: usuário e senha em duas rodadas.
			escreve("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			u, _ := r.ReadString('\n')
			escreve("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
			p, _ := r.ReadString('\n')
			s.mu.Lock()
			s.usuario, s.senha = deb64(u), deb64(p)
			s.mu.Unlock()
			escreve("235 ok")
		case "MAIL", "RCPT":
			escreve("250 ok")
		case "DATA":
			escreve("354 manda")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == "."+"\r\n" {
					break
				}
				b.WriteString(l)
			}
			s.mu.Lock()
			s.dados = b.String()
			s.mu.Unlock()
			escreve("250 aceito")
		case "QUIT":
			escreve("221 tchau")
			return
		default:
			escreve("250 ok")
		}
	}
}

func (s *servidorSMTP) guardaPlain(arg string) {
	b, err := base64.StdEncoding.DecodeString(arg)
	if err != nil {
		return
	}
	partes := strings.Split(string(b), "\x00")
	if len(partes) == 3 {
		s.mu.Lock()
		s.usuario, s.senha = partes[1], partes[2]
		s.mu.Unlock()
	}
}

func deb64(s string) string {
	b, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	return string(b)
}

func (s *servidorSMTP) leu() (comandos []string, dados, usuario, senha string, claro bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.comandos...), s.dados, s.usuario, s.senha, s.claro
}

// certificado gera um par para 127.0.0.1 e devolve a configuração dos dois
// lados: o cliente confia nele porque o teste o pôs na raiz, e não porque a
// verificação foi desligada.
func certificado(t *testing.T) *tls.Config {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modelo := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &modelo, &modelo, &chave.PublicKey, chave)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	raiz := x509.NewCertPool()
	raiz.AddCert(cert)
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: chave, Leaf: cert}},
		RootCAs:      raiz,
	}
}

func envia(t *testing.T, s *SMTP) error {
	t.Helper()
	fixaData(t)
	m := New(Options{From: "Acervo <no-reply@org.br>", Transport: s, Timeout: 5 * time.Second})
	return m.Send(context.Background(), Message{To: []string{"ana@exemplo.com"},
		Subject: "Você foi convidado", Body: Layout("Acervo", h.P(h.Text("oi")))})
}

// O caminho normal da porta 587: EHLO, STARTTLS, autentica, entrega.
func TestSMTPNegociaSTARTTLSEEntrega(t *testing.T) {
	srv := novoSMTP(t, nil)
	if err := envia(t, &SMTP{Addr: srv.addr, User: "ana", Pass: "senha", TLS: clienteTLS(srv)}); err != nil {
		t.Fatal(err)
	}
	comandos, dados, usuario, senha, claro := srv.leu()
	if !temComando(comandos, "STARTTLS") {
		t.Fatalf("sem STARTTLS: %v", comandos)
	}
	if claro {
		t.Fatal("a autenticação começou antes do TLS")
	}
	if usuario != "ana" || senha != "senha" {
		t.Fatalf("credencial = %q/%q", usuario, senha)
	}
	if !strings.Contains(dados, "Subject: =?utf-8?q?") || !strings.Contains(dados, "multipart/alternative") {
		t.Fatalf("mensagem entregue: %q", dados)
	}
}

// LOGIN não está na biblioteca padrão, e é o que o Office 365 e alguns
// appliances exigem.
func TestSMTPFalaLOGINQuandoEOQueTem(t *testing.T) {
	srv := novoSMTP(t, func(s *servidorSMTP) { s.auth = []string{"LOGIN"} })
	if err := envia(t, &SMTP{Addr: srv.addr, User: "ana", Pass: "senha", TLS: clienteTLS(srv)}); err != nil {
		t.Fatal(err)
	}
	_, _, usuario, senha, _ := srv.leu()
	if usuario != "ana" || senha != "senha" {
		t.Fatalf("credencial = %q/%q", usuario, senha)
	}
}

// Servidor sem STARTTLS e com senha configurada: a senha não sai. Um servidor
// que só oferece texto claro está mal configurado — não é um convite.
func TestSMTPNaoMandaSenhaEmClaro(t *testing.T) {
	srv := novoSMTP(t, func(s *servidorSMTP) { s.semTLS = true })
	err := envia(t, &SMTP{Addr: srv.addr, User: "ana", Pass: "senha"})
	if err == nil {
		t.Fatal("mandou a senha por um canal aberto")
	}
	if !strings.Contains(err.Error(), "AllowInsecureAuth") {
		t.Fatalf("o erro não diz o que fazer: %v", err)
	}
	if _, _, _, senha, _ := srv.leu(); senha != "" {
		t.Fatalf("a senha chegou mesmo assim: %q", senha)
	}
}

// Quem escolheu mandar a senha em claro consegue — o campo existe para a rede
// interna com um relay velho, e ele tem de estar escrito no código.
func TestSMTPComPermissaoExplicitaMandaEmClaro(t *testing.T) {
	srv := novoSMTP(t, func(s *servidorSMTP) { s.semTLS, s.authAberto = true, true })
	if err := envia(t, &SMTP{Addr: srv.addr, User: "ana", Pass: "senha", AllowInsecureAuth: true}); err != nil {
		t.Fatal(err)
	}
	_, _, usuario, senha, claro := srv.leu()
	if usuario != "ana" || senha != "senha" || !claro {
		t.Fatalf("credencial = %q/%q, em claro = %v", usuario, senha, claro)
	}
}

// Sem usuário não há o que proteger: um relay interno na porta 25 entrega.
func TestSMTPSemCredencialEntregaSemTLS(t *testing.T) {
	srv := novoSMTP(t, func(s *servidorSMTP) { s.semTLS = true })
	if err := envia(t, &SMTP{Addr: srv.addr}); err != nil {
		t.Fatal(err)
	}
	if _, dados, _, _, _ := srv.leu(); !strings.Contains(dados, "multipart/alternative") {
		t.Fatal("não entregou")
	}
}

// TLS implícito: a conexão já nasce cifrada, sem STARTTLS. É o que a porta 465
// faz — e a porta de verdade é privilegiada, então quem decide aqui é o campo.
func TestSMTPTLSImplicito(t *testing.T) {
	srv := novoSMTP(t, func(s *servidorSMTP) { s.implicit = true })
	s := &SMTP{Addr: srv.addr, User: "ana", Pass: "senha", TLS: clienteTLS(srv), ImplicitTLS: true}
	if err := envia(t, s); err != nil {
		t.Fatal(err)
	}
	comandos, _, usuario, _, claro := srv.leu()
	if temComando(comandos, "STARTTLS") {
		t.Fatalf("pediu STARTTLS numa conexão já cifrada: %v", comandos)
	}
	if claro || usuario != "ana" {
		t.Fatalf("autenticação = %q, em claro = %v", usuario, claro)
	}
}

// O prazo é do context, e é ele que solta o manipulador quando o servidor
// pendura. Sem isso o request fica girando até o TCP desistir.
func TestSMTPRespeitaOPrazo(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(30 * time.Second) // nunca diz 220
	}()

	m := New(Options{From: "a@b.com", Transport: &SMTP{Addr: ln.Addr().String()},
		Timeout: 300 * time.Millisecond})
	inicio := time.Now()
	err = m.Send(context.Background(), Message{To: []string{"c@d.com"}, Subject: "x", Body: h.P(h.Text("x"))})
	if err == nil {
		t.Fatal("esperou para sempre e disse que deu certo")
	}
	if passou := time.Since(inicio); passou > 3*time.Second {
		t.Fatalf("levou %s: o prazo não valeu", passou)
	}
}

// clienteTLS é a configuração do lado de cá: confia na raiz que o teste gerou
// e verifica o nome, que é o que um cliente de verdade faz.
func clienteTLS(s *servidorSMTP) *tls.Config {
	return &tls.Config{RootCAs: s.tlsConf.RootCAs, ServerName: "127.0.0.1"}
}

func temComando(comandos []string, prefixo string) bool {
	for _, c := range comandos {
		if strings.HasPrefix(strings.ToUpper(c), prefixo) {
			return true
		}
	}
	return false
}
