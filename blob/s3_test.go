package blob

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fakeS3 é um servidor que confere a assinatura antes de responder. Ele
// recomputa a partir do que chegou, com a chave secreta — que é o que um
// servidor S3 faz — então uma requisição mal assinada aqui é uma requisição
// que a AWS também recusaria por assinatura.
//
// O que este teste não faz, e vale dizer: conferir contra um vetor oficial da
// AWS. Não tinha como validar um offline, e inventar um "vetor conhecido" seria
// pior que não ter — daria a impressão de uma garantia que não existe. O modo de
// falha, felizmente, é barulhento: assinatura errada é 403 em tudo, na primeira
// chamada, e não um vazamento silencioso.
type fakeS3 struct {
	t       *testing.T
	key     string
	secret  string
	region  string
	objects map[string][]byte
	types   map[string]string
	// visto guarda o último Authorization, para o teste olhar a forma.
	visto string
}

func (f *fakeS3) confere(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	f.visto = auth
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		return false
	}
	var cred, signed, sig string
	for _, parte := range strings.Split(strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 "), ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(parte), "=")
		switch k {
		case "Credential":
			cred = v
		case "SignedHeaders":
			signed = v
		case "Signature":
			sig = v
		}
	}
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" || cred == "" || sig == "" {
		return false
	}
	dia, _, _ := strings.Cut(strings.TrimPrefix(cred, f.key+"/"), "/")
	scope := dia + "/" + f.region + "/s3/aws4_request"

	var canon strings.Builder
	for _, nome := range strings.Split(signed, ";") {
		v := r.Header.Get(nome)
		if nome == "host" {
			v = r.Host
		}
		canon.WriteString(nome + ":" + strings.TrimSpace(v) + "\n")
	}
	canonical := strings.Join([]string{
		r.Method, r.URL.EscapedPath(), r.URL.Query().Encode(),
		canon.String(), signed, r.Header.Get("X-Amz-Content-Sha256"),
	}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	toSign := strings.Join([]string{"AWS4-HMAC-SHA256", amzDate, scope, hex.EncodeToString(sum[:])}, "\n")
	mac := func(k []byte, d string) []byte {
		m := hmac.New(sha256.New, k)
		m.Write([]byte(d))
		return m.Sum(nil)
	}
	k := mac([]byte("AWS4"+f.secret), dia)
	k = mac(k, f.region)
	k = mac(k, "s3")
	k = mac(k, "aws4_request")
	return hmac.Equal([]byte(sig), []byte(hex.EncodeToString(mac(k, toSign))))
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !f.confere(r) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `<Error><Code>SignatureDoesNotMatch</Code><Message>assinatura não confere</Message></Error>`)
		return
	}
	key := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/"), "balde/")
	switch r.Method {
	case http.MethodPut:
		b, _ := io.ReadAll(r.Body)
		f.objects[key] = b
		f.types[key] = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		if r.URL.Query().Get("list-type") == "2" {
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, "<ListBucketResult>")
			for k := range f.objects {
				io.WriteString(w, "<Contents><Key>"+k+"</Key></Contents>")
			}
			io.WriteString(w, "<IsTruncated>false</IsTruncated></ListBucketResult>")
			return
		}
		b, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", f.types[key])
		w.Write(b)
	case http.MethodHead:
		b, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", f.types[key])
		w.Header().Set("Content-Length", string(rune('0'+len(b))))
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		if _, ok := f.objects[key]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	}
}

func s3Fake(t *testing.T) (*S3, *fakeS3) {
	t.Helper()
	f := &fakeS3{t: t, key: "AKIAEXEMPLO", secret: "segredo-de-teste", region: "sa-east-1",
		objects: map[string][]byte{}, types: map[string]string{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return &S3{
		Bucket: "balde", Region: f.region, Endpoint: srv.URL, PathStyle: true,
		Key: f.key, Secret: f.secret, HTTP: srv.Client(),
	}, f
}

// O caminho inteiro contra um servidor que confere a assinatura: guardar, ler,
// stat, listar, apagar.
func TestS3AssinaOQueOServidorAceita(t *testing.T) {
	s, fake := s3Fake(t)
	ctx := context.Background()
	key := "ab/cd/abcdef.txt"

	if err := s.Put(ctx, key, strings.NewReader("conteúdo"), 9, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if string(fake.objects[key]) != "conteúdo" {
		t.Fatalf("guardou %q", fake.objects[key])
	}
	r, err := s.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "conteúdo" {
		t.Fatalf("leu %q", b)
	}
	if info, err := s.Stat(ctx, key); err != nil || info.Type != "text/plain" {
		t.Fatalf("%v %+v", err, info)
	}
	var keys []string
	if err := s.Keys(ctx, func(k string) error { keys = append(keys, k); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != key {
		t.Fatalf("listou %v", keys)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, key); err != ErrNotFound {
		t.Fatalf("depois de apagar: %v", err)
	}
}

// Segredo errado é recusado pelo servidor — e a mensagem de erro traz o que ele
// disse, que é a diferença entre "403" e "o relógio desta máquina está torto".
func TestS3ComSegredoErradoFalhaEExplica(t *testing.T) {
	s, _ := s3Fake(t)
	s.Secret = "outro-segredo"
	err := s.Put(context.Background(), "ab/cd/x.txt", strings.NewReader("x"), 1, "text/plain")
	if err == nil {
		t.Fatal("passou com o segredo errado")
	}
	if !strings.Contains(err.Error(), "SignatureDoesNotMatch") {
		t.Fatalf("a mensagem não diz o que o servidor disse: %v", err)
	}
}

// A URL pré-assinada é uma capacidade com prazo: quem tem o link tem o arquivo
// até vencer. O prazo é obrigatório e limitado.
func TestS3PresignTemPrazoELimite(t *testing.T) {
	s, _ := s3Fake(t)
	fixo := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixo }

	u, err := s.Presign(context.Background(), "ab/cd/x.txt", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.Parse(u)
	v := q.Query()
	if v.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || v.Get("X-Amz-Expires") != "300" {
		t.Fatalf("query = %v", v)
	}
	if v.Get("X-Amz-Signature") == "" || v.Get("X-Amz-Credential") == "" {
		t.Fatalf("faltou assinatura: %v", v)
	}
	// Duas assinaturas do mesmo instante são iguais; de instantes diferentes,
	// não — é o que prova que a data entra na conta.
	outra, _ := s.Presign(context.Background(), "ab/cd/x.txt", 5*time.Minute)
	if outra != u {
		t.Fatal("mesma hora devia dar a mesma URL")
	}
	s.now = func() time.Time { return fixo.Add(time.Hour) }
	depois, _ := s.Presign(context.Background(), "ab/cd/x.txt", 5*time.Minute)
	if depois == u {
		t.Fatal("outra hora devia dar outra URL")
	}

	for _, ruim := range []time.Duration{0, -time.Minute, 8 * 24 * time.Hour} {
		if _, err := s.Presign(context.Background(), "ab/cd/x.txt", ruim); err == nil {
			t.Fatalf("aceitou prazo %s", ruim)
		}
	}
}

// O endereço do objeto muda com o estilo, e errar isso é um erro de DNS que
// ninguém liga a esta configuração.
func TestS3MontaOEndereco(t *testing.T) {
	aws := &S3{Bucket: "balde", Region: "sa-east-1"}
	if got := aws.url("ab/cd/x.txt"); got != "https://balde.s3.sa-east-1.amazonaws.com/ab/cd/x.txt" {
		t.Fatalf("aws = %q", got)
	}
	minio := &S3{Bucket: "balde", Region: "us-east-1", Endpoint: "https://minio.local", PathStyle: true}
	if got := minio.url("ab/cd/x.txt"); got != "https://minio.local/balde/ab/cd/x.txt" {
		t.Fatalf("minio = %q", got)
	}
	sub := &S3{Bucket: "balde", Region: "us-east-1", Endpoint: "https://s3.exemplo.com"}
	if got := sub.url("x.txt"); got != "https://balde.s3.exemplo.com/x.txt" {
		t.Fatalf("subdomínio = %q", got)
	}
}
