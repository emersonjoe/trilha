// Package plano is the domain of the budget example: a chart of accounts
// (tree), monthly budgets on analytic accounts, and entries. Synthetic
// accounts aggregate their children. Everything is in memory with a seed.
package plano

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// Conta is one node of the chart of accounts.
type Conta struct {
	Codigo string // "2.1.1"
	Nome   string
	Tipo   string // receita | despesa
	Filhos []*Conta
	pai    *Conta
}

// Analitica reports whether entries can be posted to the account (leaf).
func (c *Conta) Analitica() bool { return len(c.Filhos) == 0 }

// Nivel is the depth in the tree (1 for "1", 3 for "2.1.1").
func (c *Conta) Nivel() int { return strings.Count(c.Codigo, ".") + 1 }

// Caminho returns the ancestors from the root down to the account.
func (c *Conta) Caminho() []*Conta {
	var out []*Conta
	for n := c; n != nil; n = n.pai {
		out = append([]*Conta{n}, out...)
	}
	return out
}

// Lancamento is one posted entry (cents, positive).
type Lancamento struct {
	ID        int
	Conta     string    `form:"conta"`
	Data      time.Time `form:"data"`
	Valor     int64     `form:"-"` // cents; parsed from ValorTexto
	ValorTxt  string    `form:"valor"`
	Descricao string    `form:"descricao"`
}

// Mes is the month key used everywhere ("2026-09").
func Mes(t time.Time) string { return t.Format("2006-01") }

// Resumo is the month overview.
type Resumo struct {
	Mes                            string
	ReceitaOrcada, ReceitaReal     int64
	DespesaOrcada, DespesaReal     int64
	ResultadoOrcado, ResultadoReal int64
	PctReceita, PctDespesa         float64
}

var (
	mu       sync.Mutex
	raizes   []*Conta
	porCod   = map[string]*Conta{}
	orcado   = map[string]map[string]int64{} // conta → mes → cents
	lancs    []Lancamento
	seq      int
	seeded   bool
	mesAtual = "2026-09"
)

// MesPadrao is the month shown when the URL has none.
func MesPadrao() string { return mesAtual }

func add(pai *Conta, codigo, nome, tipo string) *Conta {
	c := &Conta{Codigo: codigo, Nome: nome, Tipo: tipo, pai: pai}
	if pai == nil {
		raizes = append(raizes, c)
	} else {
		pai.Filhos = append(pai.Filhos, c)
	}
	porCod[codigo] = c
	return c
}

// Seed (re)builds the sample chart, budgets and entries deterministically.
func Seed() {
	mu.Lock()
	defer mu.Unlock()
	raizes, porCod, orcado, lancs, seq = nil, map[string]*Conta{}, map[string]map[string]int64{}, nil, 0
	rec := add(nil, "1", "Receitas", "receita")
	vendas := add(rec, "1.1", "Vendas", "receita")
	add(vendas, "1.1.1", "Produtos", "receita")
	add(vendas, "1.1.2", "Serviços", "receita")
	fin := add(rec, "1.2", "Financeiras", "receita")
	add(fin, "1.2.1", "Juros", "receita")
	desp := add(nil, "2", "Despesas", "despesa")
	pessoal := add(desp, "2.1", "Pessoal", "despesa")
	add(pessoal, "2.1.1", "Salários", "despesa")
	add(pessoal, "2.1.2", "Encargos", "despesa")
	add(pessoal, "2.1.3", "Benefícios", "despesa")
	adm := add(desp, "2.2", "Administrativas", "despesa")
	add(adm, "2.2.1", "Aluguel", "despesa")
	add(adm, "2.2.2", "Energia e água", "despesa")
	add(adm, "2.2.3", "Software", "despesa")
	mkt := add(desp, "2.3", "Marketing", "despesa")
	add(mkt, "2.3.1", "Anúncios", "despesa")
	add(mkt, "2.3.2", "Eventos", "despesa")

	budget := map[string]int64{"1.1.1": 9000000, "1.1.2": 4000000, "1.2.1": 300000,
		"2.1.1": 5000000, "2.1.2": 1800000, "2.1.3": 700000, "2.2.1": 1200000, "2.2.2": 250000, "2.2.3": 400000, "2.3.1": 900000, "2.3.2": 300000}
	// realized as a fraction of budget per month, to make variances interesting
	ratio := map[string][3]float64{"1.1.1": {0.95, 1.02, 0.88}, "1.1.2": {1.1, 1.15, 1.2}, "1.2.1": {1, 1, 0.5},
		"2.1.1": {1, 1, 1}, "2.1.2": {1, 1, 1.03}, "2.1.3": {0.9, 0.95, 1.2}, "2.2.1": {1, 1, 1}, "2.2.2": {1.3, 1.1, 1.25}, "2.2.3": {0.8, 0.85, 1.4}, "2.3.1": {1.2, 0.7, 1.35}, "2.3.2": {0, 2.5, 0.3}}
	meses := []string{"2026-07", "2026-08", "2026-09"}
	for cod, v := range budget {
		orcado[cod] = map[string]int64{}
		for i, m := range meses {
			orcado[cod][m] = v
			real := int64(float64(v) * ratio[cod][i])
			if real == 0 {
				continue
			}
			// two entries per month: 60% on day 5, 40% on day 20
			t, _ := time.Parse("2006-01-02", m+"-05")
			lancs = append(lancs, Lancamento{ID: next(), Conta: cod, Data: t, Valor: real * 6 / 10, Descricao: porCod[cod].Nome + " (1ª quinzena)"})
			t2, _ := time.Parse("2006-01-02", m+"-20")
			lancs = append(lancs, Lancamento{ID: next(), Conta: cod, Data: t2, Valor: real - real*6/10, Descricao: porCod[cod].Nome + " (2ª quinzena)"})
		}
	}
	seeded = true
}

func next() int { seq++; return seq }

// Raizes returns the level-1 accounts.
func Raizes() []*Conta {
	mu.Lock()
	defer mu.Unlock()
	return raizes
}

// Get finds an account by code.
func Get(codigo string) (*Conta, bool) {
	mu.Lock()
	defer mu.Unlock()
	c, ok := porCod[codigo]
	return c, ok
}

// Analiticas lists the leaf accounts (for the entry form).
func Analiticas() []*Conta {
	mu.Lock()
	defer mu.Unlock()
	var out []*Conta
	for _, c := range porCod {
		if c.Analitica() {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Codigo < out[j].Codigo })
	return out
}

// Meses lists the months with budget or entries, ascending.
func Meses() []string {
	mu.Lock()
	defer mu.Unlock()
	set := map[string]bool{}
	for _, m := range orcado {
		for k := range m {
			set[k] = true
		}
	}
	for _, l := range lancs {
		set[Mes(l.Data)] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Orcado is the budget of an account in a month; synthetic = sum of children.
func Orcado(c *Conta, mes string) int64 {
	mu.Lock()
	defer mu.Unlock()
	return orcadoLocked(c, mes)
}

func orcadoLocked(c *Conta, mes string) int64 {
	if c.Analitica() {
		return orcado[c.Codigo][mes]
	}
	var s int64
	for _, f := range c.Filhos {
		s += orcadoLocked(f, mes)
	}
	return s
}

// Realizado is the sum of entries of an account (and descendants) in a month.
func Realizado(c *Conta, mes string) int64 {
	mu.Lock()
	defer mu.Unlock()
	return realizadoLocked(c, mes)
}

func realizadoLocked(c *Conta, mes string) int64 {
	var s int64
	if c.Analitica() {
		for _, l := range lancs {
			if l.Conta == c.Codigo && Mes(l.Data) == mes {
				s += l.Valor
			}
		}
		return s
	}
	for _, f := range c.Filhos {
		s += realizadoLocked(f, mes)
	}
	return s
}

// Variacao is (realizado-orcado)/orcado in percent; 0 when there is no budget.
func Variacao(orcado, realizado int64) float64 {
	if orcado == 0 {
		return 0
	}
	return float64(realizado-orcado) * 100 / float64(orcado)
}

// Lancamentos lists the entries of an analytic account in a month, newest first.
func Lancamentos(codigo, mes string) []Lancamento {
	mu.Lock()
	defer mu.Unlock()
	var out []Lancamento
	for _, l := range lancs {
		if l.Conta == codigo && (mes == "" || Mes(l.Data) == mes) {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Data.After(out[j].Data) || (out[i].Data.Equal(out[j].Data) && out[i].ID > out[j].ID)
	})
	return out
}

// Validar checks an entry from the form and fills Valor from ValorTxt.
func Validar(l *Lancamento) trilha.FieldErrors {
	e := trilha.FieldErrors{}
	c, ok := porCodSafe(l.Conta)
	switch {
	case !ok:
		e.Add("conta", "Conta inexistente")
	case !c.Analitica():
		e.Add("conta", "Lance numa conta analítica (sem filhos)")
	}
	if l.Data.IsZero() {
		e.Add("data", "Informe a data")
	}
	v, err := ParseMoney(l.ValorTxt)
	if err != nil || v <= 0 {
		e.Add("valor", "Valor deve ser maior que zero (ex.: 1.234,56)")
	}
	l.Valor = v
	if strings.TrimSpace(l.Descricao) == "" {
		e.Add("descricao", "Descreva o lançamento")
	}
	return e
}

func porCodSafe(cod string) (*Conta, bool) {
	mu.Lock()
	defer mu.Unlock()
	c, ok := porCod[cod]
	return c, ok
}

// Lancar stores a validated entry.
func Lancar(l Lancamento) Lancamento {
	mu.Lock()
	defer mu.Unlock()
	l.ID = next()
	lancs = append(lancs, l)
	return l
}

// Resumir builds the month overview.
func Resumir(mes string) Resumo {
	r := Resumo{Mes: mes}
	for _, c := range Raizes() {
		o, re := Orcado(c, mes), Realizado(c, mes)
		if c.Tipo == "receita" {
			r.ReceitaOrcada, r.ReceitaReal = r.ReceitaOrcada+o, r.ReceitaReal+re
		} else {
			r.DespesaOrcada, r.DespesaReal = r.DespesaOrcada+o, r.DespesaReal+re
		}
	}
	r.ResultadoOrcado = r.ReceitaOrcada - r.DespesaOrcada
	r.ResultadoReal = r.ReceitaReal - r.DespesaReal
	if r.ReceitaOrcada > 0 {
		r.PctReceita = float64(r.ReceitaReal) * 100 / float64(r.ReceitaOrcada)
	}
	if r.DespesaOrcada > 0 {
		r.PctDespesa = float64(r.DespesaReal) * 100 / float64(r.DespesaOrcada)
	}
	return r
}

// Money formats cents as "R$ 1.234,56".
func Money(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	intPart := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	s := fmt.Sprintf("R$ %s,%02d", b.String(), cents%100)
	if neg {
		return "-" + s
	}
	return s
}

// ParseMoney reads "1.234,56", "1234.56" or "1234" into cents.
func ParseMoney(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "R$"))
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f*100 + 0.5), nil
}

// MesLabel formats "2026-09" as "set/2026".
func MesLabel(mes string) string {
	t, err := time.Parse("2006-01", mes)
	if err != nil {
		return mes
	}
	nomes := []string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}
	return nomes[t.Month()-1] + "/" + strconv.Itoa(t.Year())
}
