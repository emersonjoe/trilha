package ui

import (
	"strings"
	"testing"
	"time"
)

type cidade struct{ ID, Nome string }

func TestComboboxDrawsTheTwoInputs(t *testing.T) {
	got := render(t, Combobox(ComboboxOpts{
		Name: "cidade_id", Value: "3509502", Label: "Campinas",
		Source: "/cidades/busca", With: []string{"uf"}, MinChars: 2, Debounce: 400 * time.Millisecond,
		Placeholder: "Busque a cidade",
	}, Invalid()))
	for _, want := range []string{
		`data-ui-combo=""`, `data-ui-combo-min="2"`, `data-ui-combo-wait="400"`,
		`data-ui-combo-src="/cidades/busca"`, `data-ui-combo-with="uf"`,
		`role="combobox"`, `aria-expanded="false"`, `aria-controls="cidade_id-list"`,
		`name="cidade_id_q"`, `value="Campinas"`, `placeholder="Busque a cidade"`,
		`<input type="hidden" name="cidade_id" value="3509502">`,
		`<ul id="cidade_id-list" class="ui-listbox" role="listbox" hidden>`,
		`aria-invalid="true"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %s em:\n%s", want, got)
		}
	}
}

// A short list needs no route: the options come with the page and the browser
// filters them.
func TestComboboxCarriesAStaticList(t *testing.T) {
	got := render(t, Combobox(ComboboxOpts{Name: "uf", Options: []Option{{Value: "SP", Label: "São Paulo"}}}))
	if strings.Contains(got, "data-ui-combo-src") {
		t.Error("lista estática não devia ter rota")
	}
	if !strings.Contains(got, `<li class="ui-option" role="option" aria-selected="false" data-value="SP">São Paulo</li>`) {
		t.Error(got)
	}
	if !strings.Contains(got, `data-ui-combo-min="1"`) || !strings.Contains(got, `data-ui-combo-wait="250"`) {
		t.Error("faltou o padrão de MinChars/Debounce: " + got)
	}
}

func TestComboboxOptionsIsJustTheOptions(t *testing.T) {
	got := render(t, ComboboxOptions([]cidade{{"1", "Santos"}, {"2", "Niterói"}}, func(c cidade) (string, string) {
		return c.ID, c.Nome
	}))
	want := `<li class="ui-option" role="option" aria-selected="false" data-value="1">Santos</li>` +
		`<li class="ui-option" role="option" aria-selected="false" data-value="2">Niterói</li>`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
