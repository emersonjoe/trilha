package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteUIStamp(t *testing.T) {
	dir := t.TempDir()
	res, err := WriteUI(dir, false, false, false)
	if err != nil || len(res) != 3 || res[1].Action != UICreated {
		t.Fatal(err, res)
	}
	css := filepath.Join(dir, "public", "ui.css")
	b, _ := os.ReadFile(css)
	if !untouched(b) {
		t.Fatal("fresh copy must be untouched")
	}
	// An older, untouched kit version (different body, valid stamp) is updated silently.
	os.WriteFile(css, stamp([]byte(".ui-btn{old:1}")), 0o644)
	res, err = WriteUI(dir, false, false, false)
	if err != nil || res[1].Action != UIUpdated {
		t.Fatal(err, res)
	}
	// A locally edited file is refused without force, even if it keeps the stamp.
	b, _ = os.ReadFile(css)
	os.WriteFile(css, append(b, []byte("\n.mine{}")...), 0o644)
	if _, err = WriteUI(dir, false, false, false); err != ErrUIModified {
		t.Fatal(err)
	}
	if res, err = WriteUI(dir, true, true, false); err != nil || res[1].Action != UIUpdated || len(res) != 2 {
		t.Fatal(err, res)
	}
	// The theme is never overwritten.
	theme := filepath.Join(dir, "public", "ui.theme.css")
	os.WriteFile(theme, []byte(":root{}"), 0o644)
	res, _ = WriteUI(dir, true, false, false)
	if b, _ := os.ReadFile(theme); string(b) != ":root{}" || res[0].Action != UIKeptTheme {
		t.Fatal(string(b), res)
	}
}
