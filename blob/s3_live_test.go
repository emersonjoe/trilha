package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// O servidor de teste do s3_test.go recomputa a assinatura com o mesmo
// algoritmo que este pacote escreve, então ele prova que o cliente é coerente
// consigo mesmo — e não que ele fala com um S3 de verdade. Este teste fecha
// essa lacuna contra um MinIO real.
//
// Ele não roda na CI: precisa de um servidor, e um teste que depende de um
// contêiner é um teste que falha por motivo errado na máquina de outra pessoa.
// Para rodar:
//
//	docker run -d --name trilha-minio -p 9000:9000 \
//	  -e MINIO_ROOT_USER=trilha -e MINIO_ROOT_PASSWORD=trilha-secret-123 \
//	  quay.io/minio/minio:latest server /data
//
//	TRILHA_S3_TEST='s3://trilha-teste?region=us-east-1&endpoint=http://localhost:9000&path_style=1' \
//	AWS_ACCESS_KEY_ID=trilha AWS_SECRET_ACCESS_KEY=trilha-secret-123 \
//	go test ./blob/ -run TestS3AoVivo -v
func TestS3AoVivo(t *testing.T) {
	raw := os.Getenv("TRILHA_S3_TEST")
	if raw == "" {
		t.Skip("sem TRILHA_S3_TEST: este teste quer um S3 de verdade (veja o comentário)")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	s := &S3{
		Bucket: u.Host, Region: pick(q.Get("region"), "us-east-1"),
		Endpoint: q.Get("endpoint"), PathStyle: q.Get("path_style") != "",
		Key: os.Getenv("AWS_ACCESS_KEY_ID"), Secret: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}
	ctx := context.Background()
	if err := criaBalde(ctx, s); err != nil {
		t.Fatalf("criar o balde: %v", err)
	}

	conteudo := []byte("um arquivo de verdade, num S3 de verdade\n")
	digest := sha256.Sum256(conteudo)
	key := keyOf(hex.EncodeToString(digest[:]), "text/plain")

	// Guardar.
	if err := s.Put(ctx, key, bytes.NewReader(conteudo), int64(len(conteudo)), "text/plain"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Ler de volta, byte a byte.
	r, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	volta, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(volta, conteudo) {
		t.Fatalf("voltou %q", volta)
	}

	// O que o servidor sabe sem abrir.
	info, err := s.Stat(ctx, key)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != int64(len(conteudo)) || !strings.HasPrefix(info.Type, "text/plain") {
		t.Fatalf("Stat = %+v", info)
	}

	// Listar: é o que a varredura de órfãos usa, e é onde a paginação erra.
	achou := false
	if err := s.Keys(ctx, func(k string) error {
		if k == key {
			achou = true
		}
		return nil
	}); err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if !achou {
		t.Fatal("a listagem não trouxe a chave que acabou de ser gravada")
	}

	// A URL pré-assinada tem de funcionar num cliente que não assina nada: é
	// esse o ponto dela, e é a parte que o servidor falso não conseguia provar.
	link, err := s.Presign(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("Presign: %v", err)
	}
	res, err := http.Get(link)
	if err != nil {
		t.Fatalf("buscar a URL assinada: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		corpo, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		t.Fatalf("a URL assinada respondeu %d: %s", res.StatusCode, corpo)
	}
	semAssinar, _ := io.ReadAll(res.Body)
	if !bytes.Equal(semAssinar, conteudo) {
		t.Fatalf("a URL assinada devolveu %q", semAssinar)
	}

	// E uma URL vencida não vale: o prazo é a metade da promessa.
	s.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	velha, _ := s.Presign(ctx, key, time.Minute)
	s.now = nil
	if res, err := http.Get(velha); err == nil {
		defer res.Body.Close()
		if res.StatusCode == 200 {
			t.Fatal("uma URL vencida continuou abrindo o arquivo")
		}
	}

	// Apagar, e conferir que sumiu.
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, key); err != ErrNotFound {
		t.Fatalf("depois de apagar: %v", err)
	}
}

// criaBalde faz o PUT que cria o balde. Não é parte da API do módulo — uma
// aplicação não cria o próprio bucket —, mas o teste precisa de um, e assiná-lo
// aqui é mais honesto que pedir para alguém preparar o servidor à mão.
func criaBalde(ctx context.Context, s *S3) error {
	endpoint := strings.TrimSuffix(s.Endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint+"/"+s.Bucket, nil)
	if err != nil {
		return err
	}
	if err := s.sign(req, emptyHash); err != nil {
		return err
	}
	res, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// 409 é "já existe", que é sucesso para o que este teste quer.
	if res.StatusCode == http.StatusConflict {
		return nil
	}
	return s3Error(res)
}

func pick(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
