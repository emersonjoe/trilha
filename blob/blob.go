// Package blob is where uploaded files live. The framework already knew how to
// receive a file (Ctx.File) and how to hand one back (Ctx.AttachmentFile,
// Ctx.InlineFile); what it never said was where to keep it — and what gets
// written instead is os.WriteFile into ./uploads with the name the client
// sent, which is both classic mistakes at once: a path that walks out of the
// directory, and two people uploading "nota.pdf".
//
// The key is never the name. It is the hash of the content, split into two
// levels, with the extension of the type that was sniffed from the bytes:
//
//	ab/cd/abcd…ef.pdf
//
// So a path cannot traverse (nothing from outside reaches it), the same file
// twice is the same key (deduplication, free), and a directory does not end up
// with a hundred thousand entries.
package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// ErrNotFound is what a store answers for a key it does not have.
var ErrNotFound = errors.New("blob: not found")

// Ref is what an application stores: the key it will ask for later, and enough
// about the file to show a line about it without opening it.
type Ref struct {
	// Key is the address inside the store, and the only part an application
	// needs to keep.
	Key string
	// Name is what the person called it. It is for showing and for the
	// download's filename, and it is never part of the key.
	Name string
	// Type is what the bytes are, sniffed — never what the client announced.
	Type string
	Size int64
	// SHA256 is the content's digest, hex. It is what makes the key, and it
	// is here because deduplication is the application's decision: two rows
	// pointing at one key is a choice, and so is what Delete means then.
	SHA256 string
}

// Store is a place files live. Four methods and a listing, because that is
// what a file store does — and an application that needs another backend
// implements this instead of the framework growing a driver for it.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, ctype string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Stat(ctx context.Context, key string) (Info, error)
	Delete(ctx context.Context, key string) error
	// Keys walks everything in the store. It is how the orphan sweep works,
	// and it is a callback rather than a slice because a bucket does not fit
	// in memory. Returning an error from fn stops the walk.
	Keys(ctx context.Context, fn func(key string) error) error
}

// Info is what a store knows about a key without opening it.
type Info struct {
	Key     string
	Size    int64
	Type    string
	ModTime time.Time
}

// Presigner is implemented by a store that can hand out a URL the client
// fetches directly. S3 can; disk cannot, and does not pretend to.
type Presigner interface {
	Presign(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// keyOf builds the address of a piece of content: two levels of directory from
// the digest, the digest itself, and the extension of the sniffed type.
//
// Nothing from the request is in it. That is not a precaution against a
// careless caller — it is the reason path traversal cannot happen here at all.
func keyOf(sum, ctype string) string {
	ext := extOf(ctype)
	return sum[:2] + "/" + sum[2:4] + "/" + sum + ext
}

// extOf is the extension of a media type, for the few that matter to somebody
// looking at the store with a file manager. It is cosmetic: what decides how a
// file is served is the type recorded with it, never this.
func extOf(ctype string) string {
	kind, _, _ := strings.Cut(ctype, ";")
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "application/zip":
		return ".zip"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	}
	return ".bin"
}

// safeKey refuses anything that is not a key this package made. A store's Get
// takes a string, and a string that came from a request is the one way a
// traversal could get in — so it does not get in.
func safeKey(key string) error {
	if len(key) < 7 || strings.Contains(key, "..") || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return fmt.Errorf("blob: %q is not a key this package made", key)
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '/', r == '.':
		default:
			return fmt.Errorf("blob: %q is not a key this package made", key)
		}
	}
	return nil
}
