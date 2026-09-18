package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projetoI18n é um projeto de mentira com o mínimo que estes comandos leem: o
// setup que diz qual é o idioma padrão, código que chama c.T e a pasta i18n.
func projetoI18n(t *testing.T, padrao string, catalogos map[string]string) *project {
	t.Helper()
	raiz := t.TempDir()
	escrever := func(rel, corpo string) {
		abs := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(corpo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escrever("go.mod", "module example.com/posto\n\ngo 1.22\n")
	escrever("app/setup.go", `package app

import "github.com/emersonjoe/trilha"

func Setup(a *trilha.App) error {
	cfg := a.Config()
	cfg.Locales = []string{"`+padrao+`", "ht", "fr"}
	return nil
}
`)
	escrever("app/page.go", `package app

import "github.com/emersonjoe/trilha"

func Page(c *trilha.Ctx) error {
	_ = c.T("atendimento.titulo")
	_ = c.T("atendimento.pendentes", 3)
	return c.Text(200, c.T(`+"`atendimento.crase`"+`))
}
`)
	escrever("internal/relatorio/relatorio.go", `package relatorio

func Linha(c interface{ T(string, ...any) string }) string { return c.T("relatorio.linha") }
`)
	for locale, corpo := range catalogos {
		escrever("app/i18n/"+locale+".json", corpo)
	}
	return &project{Root: raiz, Module: "example.com/posto"}
}

// As quatro chaves do código, em ordem e sem repetição — inclusive a que foi
// escrita com crase e a que está em internal/.
func TestI18nExtractLeAsChavesDoCodigo(t *testing.T) {
	p := projetoI18n(t, "pt-BR", nil)
	keys, err := i18nKeys(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"atendimento.crase", "atendimento.pendentes", "atendimento.titulo", "relatorio.linha"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("chaves = %v, quero %v", keys, want)
	}
	if got := i18nDefaultLocale(p.Root); got != "pt-BR" {
		t.Fatalf("idioma padrão = %q", got)
	}
}

// --write acrescenta o que falta no idioma padrão, com a chave como texto, e
// não mexe no que já estava traduzido.
func TestI18nExtractWriteCompletaOPadrao(t *testing.T) {
	p := projetoI18n(t, "pt-BR", map[string]string{
		"pt-BR": `{"atendimento.titulo": "Atendimento"}`,
	})
	if err := i18nExtract(p.Root, true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(p.Root, "app", "i18n", "pt-BR.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("o arquivo escrito não é JSON: %v\n%s", err, raw)
	}
	if m["atendimento.titulo"] != "Atendimento" {
		t.Errorf("a tradução que já existia foi por cima: %v", m["atendimento.titulo"])
	}
	for _, k := range []string{"atendimento.crase", "atendimento.pendentes", "relatorio.linha"} {
		if m[k] != k {
			t.Errorf("%s = %v, queria a própria chave", k, m[k])
		}
	}
	// Rodar de novo não acrescenta nada.
	antes := string(raw)
	if err := i18nExtract(p.Root, true); err != nil {
		t.Fatal(err)
	}
	depois, _ := os.ReadFile(filepath.Join(p.Root, "app", "i18n", "pt-BR.json"))
	if string(depois) != antes {
		t.Errorf("a segunda passada mudou o arquivo:\n%s", depois)
	}
}

// missing <locale> falha quando falta alguma, e passa quando não falta.
func TestI18nMissing(t *testing.T) {
	p := projetoI18n(t, "pt-BR", map[string]string{
		"pt-BR": `{"atendimento.titulo": "Atendimento", "atendimento.pendentes": {"one": "%d", "other": "%d"},
			"atendimento.crase": "x", "relatorio.linha": "y"}`,
		"ht": `{"atendimento.titulo": "Sèvis"}`,
	})
	if err := i18nMissing(p.Root, "pt-BR"); err != nil {
		t.Fatalf("o padrão está completo e mesmo assim: %v", err)
	}
	err := i18nMissing(p.Root, "ht")
	if err == nil || !strings.Contains(err.Error(), "ht") {
		t.Fatalf("faltando três chaves no crioulo e o comando não reclamou: %v", err)
	}
}

// O passo do check: chave que falta no padrão reprova; no resto é uma linha
// para ler, e o check continua passando.
func TestCheckStepI18n(t *testing.T) {
	// Sem pasta i18n/ o passo nem roda.
	sem := projetoI18n(t, "pt-BR", nil)
	if status, _ := checkStepI18n(sem, false); status != statusSkipped {
		t.Fatalf("sem catálogo: %s", status)
	}

	falta := projetoI18n(t, "pt-BR", map[string]string{
		"pt-BR": `{"atendimento.titulo": "Atendimento"}`,
	})
	status, probs := checkStepI18n(falta, false)
	if status != statusFailed || len(probs) == 0 {
		t.Fatalf("chave usada e não definida no padrão tem que reprovar: %s %+v", status, probs)
	}
	if probs[0].File != "app/i18n/pt-BR.json" || probs[0].Fix == "" {
		t.Errorf("problema sem onde nem conserto: %+v", probs[0])
	}

	completo := map[string]string{
		"pt-BR": `{"atendimento.titulo": "a", "atendimento.pendentes": "b", "atendimento.crase": "c",
			"relatorio.linha": "d"}`,
		"ht": `{"atendimento.titulo": "Sèvis"}`,
	}
	ok := projetoI18n(t, "pt-BR", completo)
	status, probs = checkStepI18n(ok, false)
	if status != statusOK {
		t.Fatalf("o padrão está completo: %s %+v", status, probs)
	}
	if len(probs) != 1 || probs[0].File != "app/i18n/ht.json" {
		t.Fatalf("o aviso do crioulo sumiu: %+v", probs)
	}
	if !strings.HasPrefix(probs[0].Message, "aviso: ") && !strings.HasPrefix(probs[0].Message, "warning: ") {
		t.Errorf("o aviso não se diz aviso: %q", probs[0].Message)
	}
}

// A pasta também pode estar na raiz do projeto: o embed é que manda onde ela
// fica, e a CLI aceita as duas.
func TestI18nAceitaCatalogoNaRaiz(t *testing.T) {
	p := projetoI18n(t, "pt-BR", nil)
	if err := os.MkdirAll(filepath.Join(p.Root, "i18n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "i18n", "pt-BR.json"), []byte(`{"x": "y"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := i18nDir(p.Root); got != "i18n" {
		t.Fatalf("pasta = %q", got)
	}
	if status, probs := checkStepI18n(p, false); status != statusFailed || len(probs) == 0 {
		t.Fatalf("o catálogo da raiz não foi conferido: %s %+v", status, probs)
	}
}
