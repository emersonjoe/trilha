package cookbook

import (
	"context"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/blob"
)

// Arquivos is the store, wired once. FromEnv is a directory on a laptop and a
// bucket in production, decided by one variable — and a URL it cannot read is
// a panic at boot, because an application that starts with the wrong storage
// loses files quietly.
var Arquivos = blob.New(blob.FromEnv())

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

// Document is the row this recipe pretends to have.
type Document struct {
	Key  string
	Name string
}

func saveDocument(c *trilha.Ctx, key, name, ctype string, size int64) error { return nil }

func findDocument(c *trilha.Ctx, id string) (Document, error) { return Document{}, nil }

func documentKeys(ctx context.Context) []string { return nil }
