---
title: Uploads
description: Receive a file with a ceiling, check what it really is, store it outside the served tree, and hand it back without letting it run.
---

An upload is the shortest path from a form to a security incident: a body with no limit, a
type taken from the file name, a path that walks out of the directory, and an HTML file served
back from your own origin. `c.File` closes the first three; the fourth is a decision about how
you serve it.

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
The progress bar and the drag-and-drop area are already in the kit:
`ui.UploadBar`, `ui.UploadTo` and `ui.UploadScript`, in [the ui reference](/reference/ui).
:::
