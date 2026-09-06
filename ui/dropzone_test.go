package ui

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

func TestDropzoneIsAFileInput(t *testing.T) {
	got := render(t, Dropzone(DropzoneOpts{Name: "arquivos", Accept: "application/pdf", MaxSize: 4 << 20},
		h.P(h.Text("Solte os PDFs aqui"))))
	for _, want := range []string{
		`class="ui-dropzone"`, `data-ui-dropzone=""`, `data-ui-dropzone-max="4194304"`,
		`<label class="ui-dropzone-area" for="arquivos"><p>Solte os PDFs aqui</p></label>`,
		`<input id="arquivos" name="arquivos" type="file" multiple="" accept="application/pdf">`,
		`<ul class="ui-queue" data-ui-queue=""></ul>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %s em:\n%s", want, got)
		}
	}
}

func TestDropzoneSingle(t *testing.T) {
	got := render(t, Dropzone(DropzoneOpts{Name: "anexo", Single: true}))
	if strings.Contains(got, "multiple") || strings.Contains(got, "accept=") || strings.Contains(got, "data-ui-dropzone-max") {
		t.Error(got)
	}
}
