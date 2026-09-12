---
title: Uploads
description: Receive a file with a ceiling, check what it really is, store it outside the served tree, and hand it back without letting it run.
---

An upload is the shortest path from a form to a security incident: a body with no limit, a
type taken from the file name, a path that walks out of the directory, and an HTML file served
back from your own origin. `c.File` closes the first three; the fourth is a decision about how
you serve it.

## Or: `trilha add blob`

`c.File` is the primitive; the `blob` recipe is a whole feature built on it — upload, a
listing screen and a serve route, with the key as the file's own digest:

```bash
trilha add blob --dry-run
```

```text
  + internal/arquivos/arquivos.go
  + internal/arquivos/arquivos_test.go
  + app/arquivos/page.go
  + app/arquivos/chave__/route.go
  + arquivos_test.go
  ~ app/setup.go (one line added)

--dry-run: nothing was written
```

That is a store, a page that lists what was received and the route that serves one back by
its key — memory by default, one line from `blob.FromEnv()` away from disk or S3. What this
page teaches instead is the primitive underneath any of that: `MaxSize`, `Accept`, `c.Files`
for more than one field, and the headers that keep served content from acting as if it were
yours. Reach for the recipe when the answer is "a screen that keeps files"; reach for this
page when the answer is "one field, validated" and nothing more — an avatar, an attachment,
a form that does not need its own listing.

## Receiving

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

`c.File` does four things before your code sees the file:

| Check | What it prevents |
|---|---|
| `MaxSize` | one file bigger than the ceiling, refused as a field error |
| `Accept` | a type that is not on the list, sniffed from the content, not the name |
| the name | `../../etc/passwd` and the like: `Save` writes a name it made up |
| `Optional` | telling "no file" apart from "a broken file" |

The sniffing matters more than it looks. A browser sends whatever `Content-Type` it likes and
a script sends whatever it wants; the only thing that says what a file is, is the file.

```go
// MaxAvatar is the ceiling for one file. A limit that lives in a constant
// is a limit somebody can find; a limit spread over three handlers is not.
const MaxAvatar = 2 << 20 // 2 MiB
```

`c.AllowBody` is the other half of the limit. `MaxSize` refuses a file that is too big after
reading it; the body limit stops the request from getting that far — a multipart upload with
no end is a slow way to fill a disk.

The name that goes in the database is the one `Save` returned, never the one the browser sent:

```go
// SetAvatar records the name on disk, not the name the browser sent.
func SetAvatar(ctx context.Context, user int64, file string) error {
	_, err := DB.ExecContext(ctx, `UPDATE users SET avatar = $1 WHERE id = $2`, file, user)
	return err
}
```

## Several at once

A field can carry more than one file, and `c.Files` reads all of them under the same rules:

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

Two things change from the single-file version. `MaxFiles` is a ceiling on the count, not on
the bytes — a request with eleven files is refused before the first one is read:

```go
// MaxAttachments is how many files one request may carry. The queue in the
// browser sends one at a time, so this ceiling is for the request that
// arrives without JavaScript — every file in one multipart body.
const MaxAttachments = 10
```

And an error is named by position. `c.File` puts its message under `files`; `c.Files` puts it
under `files[2]`, so a form that draws one line per file can show the message on the line that
earned it, and the other files still went through.

```go
// AddAttachment records what was saved: the name on disk, the size and the
// type that was sniffed, never the three the browser offered.
func AddAttachment(ctx context.Context, file string, size int64, mime string) error {
	_, err := DB.ExecContext(ctx,
		`INSERT INTO attachments (file, size, mime) VALUES ($1, $2, $3)`, file, size, mime)
	return err
}
```

## Handing it back

Serving user content from the same origin as your app is how a stored XSS gets a session
cookie. The mount plus three headers is the whole answer:

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

`os.DirFS` answers only for what is under the directory, so `..` in a URL reaches nothing. The
headers say the rest: do not guess the type, do not render it, download it.

:::warning
The strong version of this is a different host — `uploads.example.com`, or a bucket with its
own domain. Same-origin content is only ever as safe as the headers you remembered; another
origin is safe because the browser will not let it touch your site.
:::

## Showing it

A download is not a preview: the screen every document application has puts the file beside its
metadata. `ui.Preview` draws it, and `c.Inline` is what allows it.

```go
// ShowFile is the other half of handing a file back: the screen that shows it
// beside what is known about it, instead of a download and a guess.
//
// c.Inline is what makes this possible, and it is the half people miss: the
// framed answer is what refuses to be framed. Every response carries
// X-Frame-Options: DENY and frame-ancestors 'none', and Inline relaxes that
// pair to same-origin on this one response. A page that adds frame-src to its
// own policy while the file still says DENY gets a blank frame and a console
// message about a policy it did not write.
func ShowFile(c *trilha.Ctx, name, ctype string) error {
	f, err := os.Open(filepath.Join(UploadDir, name))
	if err != nil {
		return trilha.Errorf(http.StatusNotFound, "file not found")
	}
	defer f.Close()
	if c.Query("raw") != "" {
		return c.Inline(name, f, ctype)
	}
	return c.Render(http.StatusOK, ui.Preview(c, "?raw=1", ui.PreviewOpts{
		Title:    name,
		Type:     ctype,
		Download: "?download=1",
	}))
}
```

The half people miss is which side refuses. Every response carries `X-Frame-Options: DENY` and
`frame-ancestors 'none'`; `Inline` relaxes that pair to same-origin **on that one response**.
Adding `frame-src 'self'` to the framing page changes nothing while the file it frames still
says DENY — the browser refuses on behalf of the framed answer, and the console names its
policy, not yours.

`ui.Preview` also decides what a browser can do with the type: an image is an `<img>` (clicking
it opens the full size), a PDF is a frame, and anything `Inline` refuses — HTML, SVG, XML — is a
card saying it cannot be previewed, with the download button, instead of a blank frame that
explains nothing.


## Where the files live

`UploadDir` is a directory outside `public/`, and outside the binary's tree:

```go
// UploadDir is where saved files land — a directory outside the tree the
// binary serves, so a file can never be reached by guessing its path.
var UploadDir = "var/uploads"
```

On one machine that is a volume. On more than one it has to be shared storage or object
storage, because the instance that received the file is not the one that will be asked for it.
That is the moment `Save` moves to an S3 client — the handler above does not change, only what
`Save` writes to.

:::tip
The queue, the progress bar and the drag-and-drop area are already in the kit: `ui.Dropzone`,
`ui.UploadBar`, `ui.UploadTo` and `ui.UploadScript`, in [the ui reference](/reference/ui). The
queue sends one file per request, so each one gets its own answer; without JavaScript the same
form posts all of them at once and `c.Files` reads them from the same handler.
:::
