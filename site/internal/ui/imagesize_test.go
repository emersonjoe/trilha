package ui

import (
	"encoding/hex"
	"testing"
)

// The first 30 bytes of three WebPs made by ImageMagick (libwebp 1.3.2), one
// per kind of first chunk: the size has to come out of each.
func TestWebPSize(t *testing.T) {
	for _, tc := range []struct {
		kind, head string
		w, h       int
	}{
		{"VP8 (lossy)", "524946463c000000574542505650382030000000d001009d012a07000900", 7, 9},
		{"VP8L (lossless)", "524946461c000000574542505650384c0f0000002f020001000710fd8ffe", 3, 5},
		{"VP8X (alpha)", "524946466000000057454250565038580a00000010000000060000080000", 7, 9},
	} {
		b, _ := hex.DecodeString(tc.head)
		w, h, ok := webpSize(b)
		if !ok || w != tc.w || h != tc.h {
			t.Errorf("%s: %dx%d %v, want %dx%d", tc.kind, w, h, ok, tc.w, tc.h)
		}
	}
	if _, _, ok := webpSize([]byte("\x89PNG\r\n\x1a\n0000000000000000000000")); ok {
		t.Error("a PNG read as WebP")
	}
}
