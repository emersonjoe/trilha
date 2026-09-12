---
title: Arquivos
description: Onde um upload mora, como ele volta, e a varredura dos objetos que ninguém aponta.
---

O framework sabia receber um arquivo e sabia devolver. Onde guardar nunca foi dito — e o que se
escreve no lugar é `os.WriteFile` em `./uploads` com o nome que o cliente mandou, que são os dois
erros clássicos de uma vez: um caminho que sai do diretório, e duas pessoas subindo `nota.pdf`.

O `trilha/blob` é um módulo opcional: disco por padrão, S3-compatível quando uma variável diz, e
nenhum SDK nos dois casos.

## A loja

```go
// Arquivos is the store, wired once. FromEnv is a directory on a laptop and a
// bucket in production, decided by one variable — and a URL it cannot read is
// a panic at boot, because an application that starts with the wrong storage
// loses files quietly.
var Arquivos = blob.New(blob.FromEnv())
```

O `TRILHA_BLOB_URL` decide:

| Valor | Loja |
|---|---|
| sem valor | `./data/blob` em disco |
| `file:///var/lib/app/blob` | esse diretório |
| `s3://balde?region=sa-east-1` | AWS |
| `s3://balde?region=us-east-1&endpoint=https://minio.local` | MinIO, Garage, R2 — path style por padrão |

As credenciais vêm de `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` e `AWS_SESSION_TOKEN`, que é
onde toda outra ferramenta já procura. Uma URL que ele não entende é **pânico no boot**, nunca um
padrão silencioso: aplicação que sobe com o storage errado perde arquivo calada.

O pacote inteiro, para quando você preferir ligar na mão em vez de por variável:

| Símbolo | Papel |
|---|---|
| `blob.New(loja) *Files` | a porta de entrada: `Put`, `PutBytes`, `Get`, `Stat`, `Serve`, `Delete`, `Orphans` |
| `blob.NewDisk(dir) *Disk` | um diretório — o que um notebook e um servidor só querem |
| `blob.NewMemory() *Memory` | um mapa, para um teste que não pode tocar no disco |
| `blob.S3{Bucket, Region, Endpoint, Key, Secret, Session, PathStyle, HTTP}` | o balde, assinado com SigV4 e mais nada — sem SDK |
| `Files.PutBytes(ctx, nome, b, contentType)` | um arquivo que a aplicação produziu (um PDF gerado, um CSV), e não um que alguém enviou |
| `Disk.Keys`, `Memory.Keys`, `S3.Keys` | percorre toda chave da loja, uma chamada por chave, que é a metade com que o `Orphans` compara o banco |
| `blob.Presigner`, `S3.Presign(ctx, chave, ttl)` | a loja que sabe entregar uma URL temporária; o `Serve` usa quando existe e transmite quando não, e o `blob.ErrNotFound` é o que uma chave ausente responde nos dois casos |

## Recebendo

```go
// ReceiveFile is the upload. Ctx.File applies the rules and sniffs the real
// type; blob.Put makes the key out of the content's digest, so the name
// somebody typed never becomes a path and the same file twice is one file.
//
// What goes in the database is the key. Everything else in the Ref — name,
// type, size, digest — is there so a screen can show a line about the file
// without opening it.
func ReceiveFile(c *trilha.Ctx) error {
	up, err := c.File("arquivo", trilha.FileRules{
		Accept:  []string{"application/pdf", "image/*"},
		MaxSize: 50 << 20,
	})
	if err != nil {
		return err
	}
	defer up.Close()

	ref, err := Arquivos.Put(c.Context(), up)
	if err != nil {
		return err
	}
	return saveDocument(c, ref.Key, ref.Name, ref.Type, ref.Size)
}
```

**A chave é o conteúdo, nunca o nome.** É o SHA-256 dos bytes, em dois níveis, com a extensão do
tipo farejado:

```
ab/cd/abcd…ef.pdf
```

Então travessia de caminho não é evitada, é impossível — nada que veio da requisição chega à
chave. O mesmo arquivo subido duas vezes é um objeto só. E um diretório não termina com cem mil
entradas dentro.

O que o `Put` **não** decide é o que significa uma segunda referência à mesma chave. O
`Ref.SHA256` está ali para a aplicação contar referências se quiser; o `Delete` apaga o objeto, e
se isso está certo é pergunta sua, não do módulo.

## Devolvendo

```go
// SendFile hands it back. From disk this is http.ServeContent — Range, 304 and
// HEAD, which is what makes a large PDF work; from a bucket that can presign,
// it is a redirect and the bytes never touch the application.
func SendFile(c *trilha.Ctx) error {
	doc, err := findDocument(c, c.Param("id"))
	if err != nil {
		return err
	}
	return Arquivos.Serve(c, doc.Key, blob.ServeOpts{Name: doc.Name, Inline: true})
}
```

De disco isto é `http.ServeContent`: `Range`, `If-Range`, `304` e `HEAD`, que é o que faz um PDF
grande e um vídeo funcionarem. De uma loja que sabe pré-assinar, o `Serve` responde um
redirecionamento para uma URL de cinco minutos, e os bytes não passam pela aplicação.

:::warning
URL pré-assinada é uma **capacidade**: quem tem, tem o arquivo até vencer, sem sessão e sem log
seu. É exatamente o que se quer atrás de uma CDN, e exatamente o que não se quer para um
documento que só três pessoas podem ler. O `blob.ServeOpts{Proxy: true}` força os bytes pela
aplicação, onde o seu guarda já está.
:::

## A varredura

```go
// SweepOrphans is the command that runs on a schedule: the objects nothing
// points at. A row deleted while the object stayed, an upload that failed
// after the write — both end up here, and neither shows up anywhere else.
//
// The known keys are streamed out of the database rather than collected into a
// slice, because a bucket does not fit in memory and neither does the table.
func SweepOrphans(ctx context.Context) ([]string, error) {
	return Arquivos.Orphans(ctx, func(yield func(string) bool) {
		for _, key := range documentKeys(ctx) {
			if !yield(key) {
				return
			}
		}
	})
}
```

Toda aplicação acaba precisando disto: a linha apagada com o objeto ficando, o upload que falhou
depois da escrita. As chaves conhecidas saem do banco em fluxo, e não numa fatia, porque nem o
balde nem a tabela cabem em memória.

## O que está testado

Dois níveis, e é o segundo que conta.

O teste de unidade assina contra um servidor que recomputa a assinatura com o mesmo algoritmo que
este pacote escreve — o que prova coerência interna, e nada além disso.

O `TestS3AoVivo` roda tudo contra um **MinIO** de verdade: cria o balde, guarda, lê, stat, lista,
busca uma URL pré-assinada **com um cliente que não assina nada**, vê uma URL vencida ser
recusada, apaga. Ele é pulado a menos que o `TRILHA_S3_TEST` aponte para um servidor, porque
teste que depende de contêiner falha por motivo errado na máquina de outra pessoa:

```bash
docker run -d --name trilha-minio -p 9000:9000 \
  -e MINIO_ROOT_USER=trilha -e MINIO_ROOT_PASSWORD=trilha-secret-123 \
  quay.io/minio/minio:latest server /data

TRILHA_S3_TEST='s3://trilha-teste?region=us-east-1&endpoint=http://localhost:9000&path_style=1' \
AWS_ACCESS_KEY_ID=trilha AWS_SECRET_ACCESS_KEY=trilha-secret-123 \
go test ./blob/ -run TestS3AoVivo -v
```

Rodar uma vez com o segredo errado também vale: o MinIO responde `SignatureDoesNotMatch`, que é o
que diz que o servidor está mesmo conferindo — e que a execução que passou quer dizer alguma
coisa.
