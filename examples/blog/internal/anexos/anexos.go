// Package anexos guarda o que se sabe sobre cada anexo. Os bytes não moram
// aqui: eles vão para o trilha/blob, e o que fica é a chave.
//
// É a migração que todo exemplo acaba fazendo: a primeira versão guardava o
// conteúdo ao lado do nome, o que funciona até o primeiro arquivo de 50 MB e
// até a primeira reinicialização.
package anexos

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Anexo é o que se sabe sobre um arquivo recebido. A Chave é o endereço dele
// no blob — o digest do conteúdo, nunca o nome que veio do cliente.
type Anexo struct {
	Nome   string
	Bytes  int64
	Tipo   string // o tipo lido no conteúdo pelo c.File, não a extensão
	Quando time.Time
	Chave  string
}

var (
	mu    sync.Mutex
	lista []Anexo
)

// Add registra um anexo recebido, com a chave que o blob devolveu.
func Add(nome string, n int64, tipo, chave string) Anexo {
	mu.Lock()
	defer mu.Unlock()
	a := Anexo{Nome: nome, Bytes: n, Tipo: tipo, Quando: time.Now(), Chave: chave}
	lista = append(lista, a)
	sort.SliceStable(lista, func(i, j int) bool { return lista[i].Quando.After(lista[j].Quando) })
	return a
}

// All devolve os anexos, do mais recente para o mais antigo.
func All() []Anexo {
	mu.Lock()
	defer mu.Unlock()
	return append([]Anexo(nil), lista...)
}

// Por devolve o anexo com este nome, se houver. O nome veio do c.File, que
// já o saneou: não é caminho, e não vira caminho aqui.
func Por(nome string) (Anexo, bool) {
	mu.Lock()
	defer mu.Unlock()
	for _, a := range lista {
		if a.Nome == nome {
			return a, true
		}
	}
	return Anexo{}, false
}

// Reset limpa a lista (usado pelos testes).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	lista = nil
}

// Tamanho formata os bytes do jeito que uma pessoa daqui lê (vírgula decimal).
func (a Anexo) Tamanho() string {
	switch {
	case a.Bytes >= 1<<20:
		return decimal(float64(a.Bytes)/(1<<20), "MB")
	case a.Bytes >= 1<<10:
		return decimal(float64(a.Bytes)/(1<<10), "kB")
	default:
		return fmt.Sprintf("%d B", a.Bytes)
	}
}

// decimal escreve o número com vírgula, como se escreve em português.
func decimal(v float64, unit string) string {
	return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1) + " " + unit
}
