// Package anexos guarda os anexos enviados. É memória: o exemplo mostra o
// caminho do arquivo até o handler, não onde guardá-lo.
package anexos

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Anexo é um arquivo recebido.
type Anexo struct {
	Nome   string
	Bytes  int64
	Quando time.Time
}

var (
	mu    sync.Mutex
	lista []Anexo
)

// Add registra um anexo recebido.
func Add(nome string, n int64) Anexo {
	mu.Lock()
	defer mu.Unlock()
	a := Anexo{Nome: nome, Bytes: n, Quando: time.Now()}
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
