---
title: Uploads
description: Receber um arquivo com teto, conferir o que ele é de verdade, guardar fora da árvore servida e devolver sem deixar rodar.
---

Um upload é o caminho mais curto entre um formulário e um incidente de segurança: um corpo sem
limite, um tipo tirado do nome do arquivo, um caminho que sai do diretório e um arquivo HTML
devolvido a partir da sua própria origem. `c.File` fecha os três primeiros; o quarto é uma
decisão sobre como você serve.

## Recebendo

```go
// SaveAvatar takes the file from the form. c.File checks the size, sniffs
// the real type instead of believing the name, and drops a filename that
// tries to walk out of the directory.
func SaveAvatar(c *trilha.Ctx) error {
	// The body limit is the file plus the rest of the form; without it, a
	// multipart request with no end is a slow way to fill the disk.
	c.AllowBody(MaxAvatar + 64<<10)
	up, err := c.File("avatar", trilha.FileRules{
		MaxSize: MaxAvatar,
		Accept:  []string{"image/png", "image/jpeg", "image/webp"},
	})
	if err != nil {
		return err
	}
	defer up.Close()
	name, err := up.Save(UploadDir)
	if err != nil {
		return err
	}
	if err := SetAvatar(c.Context(), CurrentUser(c).ID, name); err != nil {
		return err
	}
	if err := Flash(c, "Photo updated."); err != nil {
		return err
	}
	return c.Redirect("/account")
}
```

`c.File` faz quatro coisas antes de o seu código ver o arquivo:

| Checagem | O que evita |
|---|---|
| `MaxSize` | um arquivo maior que o teto, recusado como erro de campo |
| `Accept` | um tipo fora da lista, farejado no conteúdo, não no nome |
| o nome | `../../etc/passwd` e parecidos: o `Save` escreve um nome inventado por ele |
| `Optional` | separar "nenhum arquivo" de "um arquivo quebrado" |

O farejamento importa mais do que parece. O navegador manda o `Content-Type` que quiser e um
script manda o que bem entender; a única coisa que diz o que um arquivo é, é o arquivo.

```go
// MaxAvatar is the ceiling for one file. A limit that lives in a constant
// is a limit somebody can find; a limit spread over three handlers is not.
const MaxAvatar = 2 << 20 // 2 MiB
```

`c.AllowBody` é a outra metade do limite. O `MaxSize` recusa um arquivo grande demais depois de
lê-lo; o limite de corpo impede a requisição de chegar até lá — um upload multipart sem fim é
um jeito lento de encher um disco.

O nome que vai para o banco é o que o `Save` devolveu, nunca o que o navegador mandou:

```go
// SetAvatar records the name on disk, not the name the browser sent.
func SetAvatar(ctx context.Context, user int64, file string) error {
	_, err := DB.ExecContext(ctx, `UPDATE users SET avatar = $1 WHERE id = $2`, file, user)
	return err
}
```

## Vários de uma vez

Um campo pode carregar mais de um arquivo, e o `c.Files` lê todos sob as mesmas regras:

```go
// SaveAttachments takes every file the field carries. c.Files applies the
// same rules as c.File to each one, and a file that fails comes back under
// its own position — arquivos[2] — so the message lands on the right line
// instead of blaming the whole form.
func SaveAttachments(c *trilha.Ctx) error {
	c.AllowBody(MaxAvatar*MaxAttachments + 64<<10)
	ups, err := c.Files("files", trilha.FileRules{
		MaxSize:  MaxAvatar,
		MaxFiles: MaxAttachments,
		Accept:   []string{"image/png", "image/jpeg", "application/pdf"},
	})
	if err != nil {
		return err
	}
	for _, up := range ups {
		defer up.Close()
		name, err := up.Save(UploadDir)
		if err != nil {
			return err
		}
		if err := AddAttachment(c.Context(), name, up.Size, up.MIME); err != nil {
			return err
		}
	}
	return c.Redirect("/attachments")
}
```

Duas coisas mudam em relação à versão de um arquivo só. O `MaxFiles` é teto de quantidade, não
de bytes — uma requisição com onze arquivos é recusada antes de o primeiro ser lido:

```go
// MaxAttachments is how many files one request may carry. The queue in the
// browser sends one at a time, so this ceiling is for the request that
// arrives without JavaScript — every file in one multipart body.
const MaxAttachments = 10
```

E o erro é nomeado por posição. O `c.File` põe a mensagem em `files`; o `c.Files` põe em
`files[2]`, então um formulário que desenha uma linha por arquivo mostra a mensagem na linha
que a mereceu — e os outros arquivos passaram assim mesmo.

```go
// AddAttachment records what was saved: the name on disk, the size and the
// type that was sniffed, never the three the browser offered.
func AddAttachment(ctx context.Context, file string, size int64, mime string) error {
	_, err := DB.ExecContext(ctx,
		`INSERT INTO attachments (file, size, mime) VALUES ($1, $2, $3)`, file, size, mime)
	return err
}
```

## Devolvendo

Servir conteúdo de usuário da mesma origem do seu app é como um XSS armazenado consegue um
cookie de sessão. O mount mais três cabeçalhos são a resposta inteira:

```go
// ServeUploads is what Config does to hand the files back. os.DirFS answers
// only what is under the directory, and the mount is a URL prefix: nothing
// else on disk becomes reachable by adding ../ to an address.
func ServeUploads(cfg *trilha.Config) {
	cfg.Mounts = map[string]fs.FS{"/uploads/": os.DirFS(UploadDir)}
	cfg.StaticHeaders = func(path string, hdr http.Header) {
		if !strings.HasPrefix(path, "/uploads/") {
			return
		}
		// Content someone else uploaded is never rendered as if it were
		// ours: no sniffing, and the browser downloads instead of running.
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Content-Disposition", "attachment")
		hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	}
}
```

`os.DirFS` só responde pelo que está sob o diretório, então `..` numa URL não alcança nada. Os
cabeçalhos dizem o resto: não adivinhe o tipo, não renderize, baixe.

:::atencao
A versão forte disso é outro host — `uploads.exemplo.com`, ou um bucket com domínio próprio.
Conteúdo na mesma origem é seguro só até onde os cabeçalhos que você lembrou alcançam; outra
origem é segura porque o navegador não deixa ela encostar no seu site.
:::

## Onde os arquivos moram

`UploadDir` é um diretório fora de `public/`, e fora da árvore do binário:

```go
// UploadDir is where saved files land — a directory outside the tree the
// binary serves, so a file can never be reached by guessing its path.
var UploadDir = "var/uploads"
```

Numa máquina só, isso é um volume. Em mais de uma, tem que ser armazenamento compartilhado ou
armazenamento de objetos, porque a instância que recebeu o arquivo não é a que vai ser
perguntada por ele. É esse o momento em que o `Save` vira um cliente de S3 — o handler acima
não muda, só muda para onde o `Save` escreve.

:::dica
A fila, a barra de progresso e a área de arrastar-e-soltar já estão no kit: `ui.Dropzone`,
`ui.UploadBar`, `ui.UploadTo` e `ui.UploadScript`, na [referência de ui](/pt/referencia/ui). A
fila manda um arquivo por requisição, então cada um ganha a sua resposta; sem JavaScript o
mesmo formulário posta todos de uma vez e o `c.Files` os lê do mesmo handler.
:::
