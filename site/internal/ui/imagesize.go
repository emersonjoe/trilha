package ui

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/jpeg" // DecodeConfig of the documentation's images
	_ "image/png"
	"io"
	"io/fs"
	"strings"
	"sync"
)

// imageSize is what imageSizes remembers of one file.
type imageSize struct {
	w, h int
	ok   bool
}

var (
	sizesMu sync.Mutex
	sizes   = map[string]imageSize{}
)

// imageSizes answers the dimensions of an image in public/ by the path the
// Markdown wrote ("/docs/x.webp"). Each file is read once: only its header,
// and the answer is kept for the life of the process, since public/ is
// embedded in the binary.
func imageSizes(public fs.FS) func(string) (int, int, bool) {
	return func(src string) (int, int, bool) {
		if public == nil || !strings.HasPrefix(src, "/") {
			return 0, 0, false
		}
		name := strings.TrimPrefix(src, "/")
		sizesMu.Lock()
		s, seen := sizes[name]
		sizesMu.Unlock()
		if !seen {
			s = readImageSize(public, name)
			sizesMu.Lock()
			sizes[name] = s
			sizesMu.Unlock()
		}
		return s.w, s.h, s.ok
	}
}

func readImageSize(public fs.FS, name string) imageSize {
	f, err := public.Open(name)
	if err != nil {
		return imageSize{}
	}
	defer f.Close()
	head := make([]byte, 32)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if w, h, ok := webpSize(head); ok {
		return imageSize{w, h, true}
	}
	rest, err := io.ReadAll(f)
	if err != nil {
		return imageSize{}
	}
	cfg, _, err := image.DecodeConfig(io.MultiReader(bytes.NewReader(head), bytes.NewReader(rest)))
	if err != nil {
		return imageSize{}
	}
	return imageSize{cfg.Width, cfg.Height, true}
}

// webpSize reads the canvas of a WebP from its first 30 bytes. The standard
// library has no WebP decoder, and the size is all a figure needs: the three
// kinds of first chunk — lossy (VP8), lossless (VP8L) and extended (VP8X) —
// each keep it at a fixed place.
func webpSize(b []byte) (int, int, bool) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(b[12:16]) {
	case "VP8 ":
		if b[23] != 0x9d || b[24] != 0x01 || b[25] != 0x2a {
			return 0, 0, false
		}
		return int(binary.LittleEndian.Uint16(b[26:]) & 0x3fff), int(binary.LittleEndian.Uint16(b[28:]) & 0x3fff), true
	case "VP8L":
		if b[20] != 0x2f {
			return 0, 0, false
		}
		bits := binary.LittleEndian.Uint32(b[21:])
		return int(bits&0x3fff) + 1, int(bits>>14&0x3fff) + 1, true
	case "VP8X":
		le24 := func(p []byte) int { return int(p[0]) | int(p[1])<<8 | int(p[2])<<16 }
		return le24(b[24:]) + 1, le24(b[27:]) + 1, true
	}
	return 0, 0, false
}
