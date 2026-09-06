package apisurface

import (
	"os"
	"path/filepath"
	"testing"
)

const fonteExemplo = `package exemplo

// Config carrega as opções.
type Config struct {
	Nome   string
	oculto int
	Limite int
	Embutido
}

type Embutido struct{ A string }

type Leitor interface {
	Ler(p []byte) (int, error)
	fechar()
}

type Manipulador func(x int) error

type Apelido = Config

const Padrao = "p"

type Nivel int

const (
	A Nivel = iota
	B
)

var Global *Config

func Novo(c Config) *Config { return &c }

func naoExportada() {}

// Antiga não faz nada.
//
// Deprecated: use Novo.
func Antiga() {}

func (c *Config) Aplicar(n int) error { return nil }

func (c Config) Nome2() string { return "" }

func (c *Config) interno() {}
`

const testeExemplo = `package exemplo

func Ruido() {}
`

const querido = `pkg exemplo, const A Nivel
pkg exemplo, const B Nivel
pkg exemplo, const Padrao
pkg exemplo, func Antiga() // deprecated
pkg exemplo, func Novo(Config) *Config
pkg exemplo, method (*Config) Aplicar(int) error
pkg exemplo, method (Config) Nome2() string
pkg exemplo, type Apelido = Config
pkg exemplo, type Config struct
pkg exemplo, type Config struct, Limite int
pkg exemplo, type Config struct, Nome string
pkg exemplo, type Config struct, embedded Embutido
pkg exemplo, type Embutido struct
pkg exemplo, type Embutido struct, A string
pkg exemplo, type Leitor interface
pkg exemplo, type Leitor interface, Ler([]byte) (int, error)
pkg exemplo, type Manipulador func(int) error
pkg exemplo, type Nivel int
pkg exemplo, var Global *Config
`

func TestRender(t *testing.T) {
	raiz := t.TempDir()
	dir := filepath.Join(raiz, "exemplo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	escrever := func(nome, corpo string) {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(corpo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escrever("exemplo.go", fonteExemplo)
	escrever("exemplo_test.go", testeExemplo)

	got, err := Render(raiz, []Package{{Dir: "exemplo", Name: "exemplo"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != querido {
		t.Fatalf("superfície diferente:\n--- querido ---\n%s\n--- obtido ---\n%s", querido, got)
	}
}

func TestRenderDiretorioInexistente(t *testing.T) {
	if _, err := Render(t.TempDir(), []Package{{Dir: "nada", Name: "nada"}}); err == nil {
		t.Fatal("diretório inexistente deveria falhar")
	}
}
