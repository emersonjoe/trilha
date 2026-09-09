---
title: Files
description: Where an upload lives, how it comes back, and the sweep for the objects nothing points at.
---

The framework knew how to receive a file and how to hand one back. Where to keep it was never
said — and what gets written instead is `os.WriteFile` into `./uploads` with the name the client
sent, which is both classic mistakes at once: a path that walks out of the directory, and two
people uploading `nota.pdf`.

`trilha/blob` is an optional module: disk by default, S3-compatible when a variable says so, and
no SDK in either case.

## The store

```go
// Arquivos is the store, wired once. FromEnv is a directory on a laptop and a
// bucket in production, decided by one variable — and a URL it cannot read is
// a panic at boot, because an application that starts with the wrong storage
// loses files quietly.
var Arquivos = blob.New(blob.FromEnv())
```

`TRILHA_BLOB_URL` decides:

| Value | Store |
|---|---|
| unset | `./data/blob` on disk |
| `file:///var/lib/app/blob` | that directory |
| `s3://bucket?region=sa-east-1` | AWS |
| `s3://bucket?region=us-east-1&endpoint=https://minio.local` | MinIO, Garage, R2 — path style by default |

Credentials come from `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_SESSION_TOKEN`, where
every other tool already looks for them. A URL it cannot read is a **panic at boot**, never a
silent fallback: an application that starts with the wrong storage loses files quietly.

## Receiving

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

**The key is the content, never the name.** It is the SHA-256 of the bytes, split two levels
deep, with the extension of the sniffed type:

```
ab/cd/abcd…ef.pdf
```

So path traversal is not prevented, it is impossible — nothing from the request reaches the key.
The same file uploaded twice is one object. And a directory does not end up with a hundred
thousand entries in it.

What `Put` does **not** decide is what a second reference to one key means. `Ref.SHA256` is there
so an application can count references if it wants to; `Delete` removes the object, and whether
that is right is your question, not the module's.

## Handing it back

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

From disk this is `http.ServeContent`: `Range`, `If-Range`, `304` and `HEAD`, which is what makes
a large PDF and a video work at all. From a store that can presign, `Serve` answers a redirect to
a URL that lasts five minutes and the bytes never touch the application.

:::warning
A presigned URL is a **capability**: whoever holds it has the file until it expires, with no
session and no log of yours. That is exactly what you want behind a CDN, and exactly what you do
not want for a document only three people may read. `blob.ServeOpts{Proxy: true}` forces the
bytes through the application, where your own guard already is.
:::

## The sweep

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

Every application eventually needs this: a row deleted while the object stayed, an upload that
failed after the write. The known keys are streamed out of the database rather than collected,
because neither the bucket nor the table fits in memory.

## What is tested

Two levels, and the second one is the one that counts.

The unit test signs against a server that recomputes the signature with the same algorithm this
package writes — which proves the client is consistent with itself, and nothing more.

`TestS3AoVivo` runs the whole thing against a real **MinIO**: create the bucket, put, get, stat,
list, fetch a presigned URL **with a client that signs nothing**, watch an expired URL be refused,
delete. It is skipped unless `TRILHA_S3_TEST` points at a server, because a test that depends on a
container fails for the wrong reason on somebody else's machine:

```bash
docker run -d --name trilha-minio -p 9000:9000 \
  -e MINIO_ROOT_USER=trilha -e MINIO_ROOT_PASSWORD=trilha-secret-123 \
  quay.io/minio/minio:latest server /data

TRILHA_S3_TEST='s3://trilha-teste?region=us-east-1&endpoint=http://localhost:9000&path_style=1' \
AWS_ACCESS_KEY_ID=trilha AWS_SECRET_ACCESS_KEY=trilha-secret-123 \
go test ./blob/ -run TestS3AoVivo -v
```

Running it with the wrong secret is worth doing once, too: MinIO answers `SignatureDoesNotMatch`,
which is what says the server really is checking and the passing run really means something.
