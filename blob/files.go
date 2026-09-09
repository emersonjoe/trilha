package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
)

// Files is the store an application talks to: it takes an upload, gives back a
// reference, and hands the file over again later.
type Files struct {
	store Store
	// TTL is how long a presigned URL lasts, when the store can make one.
	// Five minutes: long enough for a browser to follow a redirect and short
	// enough that a copied URL is not a second way in.
	TTL time.Duration
}

// New wires a store.
//
//	var Arquivos = blob.New(blob.FromEnv())
func New(s Store) *Files { return &Files{store: s, TTL: 5 * time.Minute} }

// Put stores what an upload carried and answers what to keep.
//
//	up, err := c.File("arquivo", trilha.FileRules{Accept: []string{"application/pdf"}, MaxSize: 50 << 20})
//	if err != nil {
//		return err
//	}
//	defer up.Close()
//	ref, err := Arquivos.Put(c, up)
//
// The type is the one Ctx.File sniffed from the bytes, never the one the
// client announced, and the key is the content's digest — so the same file
// stored twice is one file, and the name somebody typed never becomes a path.
//
// What Put does not do is decide what a second reference to one key means.
// Ref.SHA256 is there so an application can count references if it wants to;
// Delete removes the object, and whether that is right is the application's
// question.
func (f *Files) Put(ctx context.Context, up *trilha.Upload) (Ref, error) {
	if up == nil {
		return Ref{}, errors.New("blob: Put needs an upload")
	}
	// The digest has to be computed before the write, and the file has to be
	// readable twice. An upload of any size is a ReadSeeker in the standard
	// library — multipart keeps a big one on disk — so this is a seek and not
	// a copy into memory.
	sum := sha256.New()
	if _, err := io.Copy(sum, up.File); err != nil {
		return Ref{}, err
	}
	if _, err := up.File.Seek(0, io.SeekStart); err != nil {
		return Ref{}, fmt.Errorf("blob: the upload could not be read twice: %w", err)
	}
	digest := hex.EncodeToString(sum.Sum(nil))
	key := keyOf(digest, up.MIME)
	if err := f.store.Put(ctx, key, up.File, up.Size, up.MIME); err != nil {
		return Ref{}, err
	}
	return Ref{Key: key, Name: up.Name, Type: up.MIME, Size: up.Size, SHA256: digest}, nil
}

// PutBytes stores content the application produced itself — a generated PDF, a
// thumbnail, an export. ctype is what it is; an empty one is sniffed.
func (f *Files) PutBytes(ctx context.Context, name string, data []byte, ctype string) (Ref, error) {
	if ctype == "" {
		ctype = http.DetectContentType(data)
	}
	digest := sha256.Sum256(data)
	sum := hex.EncodeToString(digest[:])
	key := keyOf(sum, ctype)
	if err := f.store.Put(ctx, key, strings.NewReader(string(data)), int64(len(data)), ctype); err != nil {
		return Ref{}, err
	}
	return Ref{Key: key, Name: name, Type: ctype, Size: int64(len(data)), SHA256: sum}, nil
}

// Get opens the content. The caller closes it.
func (f *Files) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := safeKey(key); err != nil {
		return nil, err
	}
	return f.store.Get(ctx, key)
}

// Stat is what the store knows without opening the file.
func (f *Files) Stat(ctx context.Context, key string) (Info, error) {
	if err := safeKey(key); err != nil {
		return Info{}, err
	}
	return f.store.Stat(ctx, key)
}

// Delete removes the object. A key that is not there is not an error: two
// requests to delete the same thing should not need a lock between them.
func (f *Files) Delete(ctx context.Context, key string) error {
	if err := safeKey(key); err != nil {
		return err
	}
	if err := f.store.Delete(ctx, key); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

// ServeOpts is how a file is handed back.
type ServeOpts struct {
	// Name is the filename the browser sees. Empty uses the key, which is a
	// digest and helps nobody.
	Name string
	// Inline shows it in place instead of downloading — a PDF beside its
	// metadata. It obeys the same rules as Ctx.Inline: what a browser would
	// run instead of render is refused.
	Inline bool
	// Proxy forces the bytes through the application even when the store
	// could hand out a URL. It is the answer when the bucket must not be
	// reachable by anybody who has the link — a presigned URL is a capability,
	// and a capability that leaves your logs is one you cannot revoke.
	Proxy bool
}

// Serve hands the file to the browser.
//
//	return Arquivos.Serve(c, doc.Key, blob.ServeOpts{Name: doc.Nome, Inline: true})
//
// From disk it is http.ServeContent: Range, If-Range, 304 and HEAD come with
// it, which is what makes a large PDF and a video work at all. From a store
// that can presign, it is a redirect to a URL that lasts Files.TTL — the bytes
// never touch the application — unless Proxy says otherwise.
func (f *Files) Serve(c *trilha.Ctx, key string, o ServeOpts) error {
	if err := safeKey(key); err != nil {
		return err
	}
	info, err := f.store.Stat(c.Context(), key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &trilha.HTTPError{Code: http.StatusNotFound}
		}
		return err
	}
	name := o.Name
	if name == "" {
		name = key
	}
	if p, ok := f.store.(Presigner); ok && !o.Proxy {
		url, err := p.Presign(c.Context(), key, f.TTL)
		if err != nil {
			return err
		}
		return trilha.RedirectCode(url, http.StatusFound)
	}
	body, err := f.store.Get(c.Context(), key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &trilha.HTTPError{Code: http.StatusNotFound}
		}
		return err
	}
	defer body.Close()
	// Ctx.Inline and Ctx.Attachment already do the right thing with what they
	// are given: a file on disk arrives as a ReadSeeker and goes out through
	// http.ServeContent, with Range and 304; anything else is a stream that
	// promises nothing. There is no third path to write here.
	if o.Inline {
		return c.Inline(name, body, info.Type)
	}
	return c.Attachment(name, body, info.Type)
}

// Orphans walks the store and answers the keys nothing points at.
//
//	orfaos, err := Arquivos.Orphans(ctx, func(yield func(string) bool) {
//		for _, k := range chavesNoBanco() {
//			if !yield(k) {
//				return
//			}
//		}
//	})
//
// It is the sweep every application eventually needs: a row deleted while the
// object stayed, an upload that failed after the write. Passing the known keys
// as a function and not as a slice is what lets the caller stream them out of
// a database.
//
// (The issue asked for an iter.Seq. This module builds on Go 1.22, where that
// does not exist; the shape is the same and the signature will match it when
// the minimum moves.)
func (f *Files) Orphans(ctx context.Context, known func(yield func(string) bool)) ([]string, error) {
	have := map[string]bool{}
	known(func(k string) bool {
		have[k] = true
		return true
	})
	var out []string
	err := f.store.Keys(ctx, func(key string) error {
		if !have[key] {
			out = append(out, key)
		}
		return nil
	})
	return out, err
}

// osNotExist is the one place this package translates the file system's answer
// into its own, so a caller never has to know which store it is talking to.
func osNotExist(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}
