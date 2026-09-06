package cookbook

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/emersonjoe/trilha"
)

// MaxAvatar is the ceiling for one file. A limit that lives in a constant
// is a limit somebody can find; a limit spread over three handlers is not.
const MaxAvatar = 2 << 20 // 2 MiB

// UploadDir is where saved files land — a directory outside the tree the
// binary serves, so a file can never be reached by guessing its path.
var UploadDir = "var/uploads"

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

// SetAvatar records the name on disk, not the name the browser sent.
func SetAvatar(ctx context.Context, user int64, file string) error {
	_, err := DB.ExecContext(ctx, `UPDATE users SET avatar = $1 WHERE id = $2`, file, user)
	return err
}

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
