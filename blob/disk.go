package blob

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Disk keeps the files in a directory. It is the default, and for most
// applications it is also the answer: a disk with a backup is a storage
// system, and one that is already backed up is one less thing to run.
type Disk struct {
	root string
}

// NewDisk opens (and creates) the directory.
//
// The directory belongs outside the tree the binary serves. Nothing here
// enforces that — it cannot know what you serve — but a store under public/ is
// a store anybody can read by guessing, and the guess is a hash somebody
// already has.
func NewDisk(root string) *Disk { return &Disk{root: root} }

func (d *Disk) path(key string) string {
	return filepath.Join(d.root, filepath.FromSlash(key))
}

func (d *Disk) Put(ctx context.Context, key string, r io.Reader, size int64, ctype string) error {
	dst := d.path(key)
	if _, err := os.Stat(dst); err == nil {
		// The same content is already there: the key is its digest, so this is
		// not a collision, it is the same file. Writing it again would be
		// bytes for nothing.
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	// A temporary file and a rename: a crash halfway through leaves no key
	// that exists and cannot be read.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".partial-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func (d *Disk) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(d.path(key))
	if err != nil {
		return nil, osNotExist(err)
	}
	return f, nil
}

func (d *Disk) Stat(ctx context.Context, key string) (Info, error) {
	st, err := os.Stat(d.path(key))
	if err != nil {
		return Info{}, osNotExist(err)
	}
	return Info{Key: key, Size: st.Size(), Type: typeOfKey(key), ModTime: st.ModTime()}, nil
}

func (d *Disk) Delete(ctx context.Context, key string) error {
	return osNotExist(os.Remove(d.path(key)))
}

func (d *Disk) Keys(ctx context.Context, fn func(string) error) error {
	return filepath.WalkDir(d.root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		// A partial write in progress is not a key.
		if strings.HasPrefix(e.Name(), ".partial-") {
			return nil
		}
		rel, err := filepath.Rel(d.root, path)
		if err != nil {
			return nil
		}
		return fn(filepath.ToSlash(rel))
	})
}

// typeOfKey reads the type back out of the extension the key carries. It is
// the one place the cosmetic extension earns its keep: a disk store has
// nowhere else to write the content type down, and inventing a metadata file
// beside every object would be a second thing to keep in sync.
func typeOfKey(key string) string {
	switch filepath.Ext(key) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".zip":
		return "application/zip"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	}
	return "application/octet-stream"
}

var _ = time.Time{}
