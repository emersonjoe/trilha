package trilha

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FileRules is what a route accepts in one form field. The zero value accepts
// any type and any size the body limit allows, and requires the field.
type FileRules struct {
	// MaxSize is the limit for this file, in bytes, apart from
	// Config.MaxBodyBytes. Zero leaves the body limit doing the work.
	MaxSize int64
	// Accept lists the media types allowed, matched against the type detected
	// in the content: "image/png", "application/pdf", or "image/*". Empty
	// accepts anything.
	Accept []string
	// Optional makes an absent field return (nil, nil) instead of an error.
	Optional bool
}

// Upload is a file that already passed the rules.
type Upload struct {
	// Name is the file name, sanitised: no directory, no separator, no
	// control character, at most 100 characters, never empty. It still comes
	// from the client, so it says nothing about the content — Ext does.
	Name string
	// MIME is the media type detected in the first 512 bytes of the file,
	// never what the client announced.
	MIME string
	// Ext is the extension that matches MIME (".pdf"), which may disagree
	// with the one in Name.
	Ext string
	// Size is the file size in bytes.
	Size int64
	// File is the file itself, positioned at the start. Close it, or call
	// Upload.Close.
	File multipart.File
}

// Close closes the underlying file.
func (u *Upload) Close() error { return u.File.Close() }

// Save writes the file inside dir, creating the directory if needed, with mode
// 0600 and a name that is free: nota.pdf, then nota-1.pdf, and so on. It
// returns the path written. The name cannot escape dir — that is the whole
// reason this exists instead of a filepath.Join in the handler.
func (u *Upload) Save(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	base := u.Name
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 0; ; i++ {
		name := base
		if i > 0 {
			name = stem + "-" + strconv.Itoa(i) + ext
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := io.Copy(f, u.File); err != nil {
			return "", err
		}
		return path, f.Sync()
	}
}

// File reads one file from a multipart form and answers with it only if it
// passes the rules: size, media type detected in the content (never the
// extension, never what the client announced) and a name that cannot walk out
// of a directory.
//
//	up, err := c.File("file", trilha.FileRules{
//		MaxSize: 2 << 20,
//		Accept:  []string{"image/png", "image/jpeg"},
//	})
//
// A rule that fails is a FieldErrors under the field's name, the same answer
// Bind gives, so the form shows the message where the person is looking. Any
// other error (a broken body, a full disk) comes back as itself.
func (c *Ctx) File(field string, rules FileRules) (*Upload, error) {
	if err := c.parseForm(); err != nil {
		return nil, err
	}
	f, hdr, err := c.r.FormFile(field)
	if err != nil || hdr.Filename == "" && hdr.Size == 0 {
		if f != nil {
			f.Close()
		}
		if rules.Optional {
			return nil, nil
		}
		return nil, FieldErrors{field: message("required", "")}
	}
	if rules.MaxSize > 0 && hdr.Size > rules.MaxSize {
		f.Close()
		return nil, FieldErrors{field: message("filemax", humanSize(rules.MaxSize))}
	}
	kind, err := sniff(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if !accepted(kind, rules.Accept) {
		f.Close()
		return nil, FieldErrors{field: message("filetype", "")}
	}
	name := safeName(hdr.Filename)
	return &Upload{Name: name, MIME: kind, Ext: extensionOf(kind, name), Size: hdr.Size, File: f}, nil
}

// sniff reads the first 512 bytes to know what the file is, then puts it back
// at the start: the handler gets a file nobody has read yet.
func sniff(f multipart.File) (string, error) {
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	kind, _, err := mime.ParseMediaType(http.DetectContentType(buf[:n]))
	if err != nil {
		return "application/octet-stream", nil
	}
	return kind, nil
}

func accepted(kind string, accept []string) bool {
	if len(accept) == 0 {
		return true
	}
	for _, a := range accept {
		a = strings.TrimSpace(a)
		if a == kind || a == "*/*" {
			return true
		}
		if prefix, ok := strings.CutSuffix(a, "/*"); ok && strings.HasPrefix(kind, prefix+"/") {
			return true
		}
	}
	return false
}

// extensionOf answers with the extension the content deserves. The one the
// client wrote is kept when it matches the type — a JPEG stays .jpg instead of
// becoming the first entry of an alphabetical list.
func extensionOf(kind, name string) string {
	exts, _ := mime.ExtensionsByType(kind)
	if len(exts) == 0 {
		return ""
	}
	if got := strings.ToLower(filepath.Ext(name)); got != "" {
		for _, e := range exts {
			if e == got {
				return got
			}
		}
	}
	return exts[0]
}

// safeName turns what the client sent into a name that can only be a file, in
// the directory somebody chose: no path, no separator (of either platform), no
// control character, never "..", never empty, never longer than 100 characters.
func safeName(sent string) string {
	sent = strings.ReplaceAll(sent, "\\", "/")
	if i := strings.LastIndex(sent, "/"); i >= 0 {
		sent = sent[i+1:]
	}
	name := strings.Map(func(r rune) rune {
		if r == 0 || unicode.IsControl(r) || r == '/' {
			return -1
		}
		return r
	}, sent)
	name = strings.Trim(strings.TrimSpace(name), ".")
	if name == "" {
		return "file"
	}
	if utf8.RuneCountInString(name) > 100 {
		ext := filepath.Ext(name)
		if utf8.RuneCountInString(ext) > 10 {
			ext = ""
		}
		stem := []rune(strings.TrimSuffix(name, ext))
		name = string(stem[:100-utf8.RuneCountInString(ext)]) + ext
	}
	return name
}

// humanSize is the limit as the person reads it, not as the computer stores it.
func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return trimZero(float64(n)/(1<<30)) + " GB"
	case n >= 1<<20:
		return trimZero(float64(n)/(1<<20)) + " MB"
	case n >= 1<<10:
		return trimZero(float64(n)/(1<<10)) + " KB"
	}
	return strconv.FormatInt(n, 10) + " bytes"
}

func trimZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}
