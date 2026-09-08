// Package documentos é a lista que toda tela de gestão tem: filtro em cima,
// tabela no meio, paginação embaixo. É memória, e é de propósito: o que o
// exemplo mostra é o caminho da URL até a consulta, não onde guardar.
package documentos

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Documento é uma linha da lista.
type Documento struct {
	ID     string
	Nome   string
	Tipo   string // nota, recibo, contrato
	Bytes  int
	Status string // fila, processando, pronto
}

// Tamanho formata os bytes do jeito que uma pessoa daqui lê.
func (d Documento) Tamanho() string {
	if d.Bytes >= 1<<10 {
		return fmt.Sprintf("%d kB", d.Bytes/(1<<10))
	}
	return fmt.Sprintf("%d B", d.Bytes)
}

var (
	mu    sync.Mutex
	lista = semear()
	tipos = []string{"nota", "recibo", "contrato"}
)

// Tipos são os valores do filtro, para o formulário não inventar os seus.
func Tipos() []string { return append([]string(nil), tipos...) }

func semear() []Documento {
	var out []Documento
	for i := 1; i <= 23; i++ {
		out = append(out, Documento{
			ID:     fmt.Sprintf("%d", i),
			Nome:   fmt.Sprintf("%s-%02d.pdf", []string{"nota", "recibo", "contrato"}[i%3], i),
			Tipo:   []string{"nota", "recibo", "contrato"}[i%3],
			Bytes:  1024 * i,
			Status: []string{"pronto", "pronto", "processando", "fila"}[i%4],
		})
	}
	return out
}

// Consulta é o que a tela pergunta: um texto, um tipo, uma ordem e uma página.
type Consulta struct {
	Q, Tipo, Ordem string
	Asc            bool
	Offset, Limite int
}

// Buscar devolve a página pedida e quantos documentos passaram pelo filtro —
// os dois números que a paginação precisa.
func Buscar(c Consulta) ([]Documento, int) {
	mu.Lock()
	defer mu.Unlock()
	var f []Documento
	for _, d := range lista {
		if c.Q != "" && !strings.Contains(strings.ToLower(d.Nome), strings.ToLower(c.Q)) {
			continue
		}
		if c.Tipo != "" && d.Tipo != c.Tipo {
			continue
		}
		f = append(f, d)
	}
	// A ordem só chega aqui com uma coluna que a tela declarou: o
	// ui.DataTable derruba o resto antes de renderizar.
	sort.SliceStable(f, func(i, j int) bool {
		var menor bool
		switch c.Ordem {
		case "tamanho":
			menor = f[i].Bytes < f[j].Bytes
		case "status":
			menor = f[i].Status < f[j].Status
		default:
			menor = f[i].Nome < f[j].Nome
		}
		if c.Asc {
			return menor
		}
		return !menor
	})
	total := len(f)
	if c.Offset >= total {
		return nil, total
	}
	fim := c.Offset + c.Limite
	if fim > total {
		fim = total
	}
	return f[c.Offset:fim], total
}

// Pendentes conta o que ainda não terminou — o número que o fragmento vivo
// mostra enquanto o processamento anda.
func Pendentes() int {
	mu.Lock()
	defer mu.Unlock()
	n := 0
	for _, d := range lista {
		if d.Status != "pronto" {
			n++
		}
	}
	return n
}

// Andar termina um documento pendente. É o que um worker faria; aqui é o
// tique do polling que faz a fila andar, para a página ter o que mostrar.
func Andar() {
	mu.Lock()
	defer mu.Unlock()
	for i, d := range lista {
		if d.Status == "processando" {
			lista[i].Status = "pronto"
			return
		}
	}
	for i, d := range lista {
		if d.Status == "fila" {
			lista[i].Status = "processando"
			return
		}
	}
}

// Reset devolve a lista ao estado inicial (usado pelos testes).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	lista = semear()
}

// Importar acrescenta o que veio da planilha e devolve quantos entraram. O id
// é gerado aqui: uma planilha editada à mão não tem por que carregar um.
func Importar(novos []Documento) int {
	mu.Lock()
	defer mu.Unlock()
	for _, d := range novos {
		d.ID = fmt.Sprintf("d%d", len(lista)+1)
		if d.Status == "" {
			d.Status = "fila"
		}
		lista = append(lista, d)
	}
	return len(novos)
}
