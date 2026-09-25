package main

import (
	"flag"
	"path/filepath"
	"testing"

	"github.com/emersonjoe/trilha"
)

var update = flag.Bool("update", false, "rewrite the page snapshots in testdata/")

// A home e o login seguram o HTML servido num golden: nonce, token, request id
// e datas saem normalizados, então duas execuções dão o mesmo arquivo e uma
// mudança de marcação aparece como diff (make golden regrava).
func TestPaginasContraOGolden(t *testing.T) {
	for _, p := range []struct{ nome, caminho string }{{"home", "/"}, {"login", "/login"}} {
		c := newClient(t, "prod")
		snap := trilha.CapturePage(t, c.app, "GET", p.caminho)
		if snap.Status != 200 {
			t.Fatalf("%s: %d\n%s", p.caminho, snap.Status, snap.Body)
		}
		for _, err := range []error{
			snap.MatchGolden(filepath.Join("testdata", "snapshots", p.nome+".txt"), *update),
			snap.HasCSPNonce(),
			snap.HasCSRFToken(),
			snap.HasSafeCookies(),
		} {
			if err != nil {
				t.Errorf("%s: %v", p.caminho, err)
			}
		}
	}
}
