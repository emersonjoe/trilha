// Package clientes holds the domain of the example: the form model, its
// validation rules and an in-memory store.
package clientes

import (
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// Endereco is a postal address (billing may differ from delivery).
type Endereco struct {
	CEP    string `form:"cep"`
	Rua    string `form:"rua"`
	Numero string `form:"numero"`
	UF     string `form:"uf"`
	Cidade string `form:"cidade"`
	// CidadeQ is what was typed in the combobox. With JavaScript the hidden
	// Cidade already holds the choice; without it, this is all the server has,
	// and Normalizar resolves it.
	CidadeQ string `form:"cidade_q"`
}

// Dependente is one row of the dependants list. It is read from
// dependentes[0].nome, dependentes[1].nome…, and the message of a row comes
// back under the very name the input had.
type Dependente struct {
	Nome       string `form:"nome"`
	Nascimento string `form:"nascimento"`
}

// Cliente is the form model. Tags map form fields; conditional fields are
// simply empty when hidden (ui.ShowWhen disables them in the browser and the
// rules below ignore them by Tipo on the server).
type Cliente struct {
	ID          int
	Tipo        string `form:"tipo"` // pf | pj
	Nome        string `form:"nome"`
	Email       string `form:"email"`
	CPF         string `form:"cpf"`
	Nascimento  string `form:"nascimento"`
	CNPJ        string `form:"cnpj"`
	RazaoSocial string `form:"razao_social"`
	Endereco    Endereco
	CobrancaDif bool         `form:"cobranca_diferente"`
	Cobranca    Endereco     `form:"cob_"` // bound as cob_cep, cob_rua...
	Dependentes []Dependente `form:"dependentes" validate:"maxitems=10"`
	Novidades   bool         `form:"novidades"`
	Frequencia  string       `form:"frequencia" validate:"enum=cadastro.Frequencia"`
	Criado      time.Time
}

// Frequencias is the newsletter cadence, declared once. Before this it was
// written four times — two radios, a badge that printed the raw value, a
// comment on the tag and a hand-written check — and the badge was the one that
// showed "mensal" to somebody who had chosen "Mensal".
var Frequencias = trilha.Enum{
	{Value: "semanal", Label: "Semanal", Tone: "info"},
	{Value: "mensal", Label: "Mensal", Tone: "accent"},
}

// Documento returns the identifier shown in lists.
func (c Cliente) Documento() string {
	if c.Tipo == "pj" {
		return "CNPJ " + c.CNPJ
	}
	return "CPF " + c.CPF
}

// Normalizar trims and keeps only what the type allows, so a hand-made POST
// with both CPF and CNPJ never stores contradictory data.
func Normalizar(c *Cliente) {
	c.Nome, c.Email = strings.TrimSpace(c.Nome), strings.TrimSpace(strings.ToLower(c.Email))
	c.CPF, c.CNPJ = digits(c.CPF), digits(c.CNPJ)
	c.RazaoSocial = strings.TrimSpace(c.RazaoSocial)
	switch c.Tipo {
	case "pf":
		c.CNPJ, c.RazaoSocial = "", ""
	case "pj":
		c.CPF, c.Nascimento = "", ""
	}
	resolverCidade(&c.Endereco)
	resolverCidade(&c.Cobranca)
	if !c.CobrancaDif {
		c.Cobranca = Endereco{}
	}
	if !c.Novidades {
		c.Frequencia = ""
	}
	// The form always shows one spare row; a row nobody typed into is not a
	// dependant, and dropping it here keeps the indexes of the messages equal
	// to the indexes the form is about to draw.
	deps := c.Dependentes[:0]
	for _, d := range c.Dependentes {
		d.Nome, d.Nascimento = strings.TrimSpace(d.Nome), strings.TrimSpace(d.Nascimento)
		if d.Nome == "" && d.Nascimento == "" {
			continue
		}
		deps = append(deps, d)
	}
	c.Dependentes = deps
}

// Validar applies the business rules and returns one message per field.
func Validar(c Cliente) trilha.FieldErrors {
	e := trilha.FieldErrors{}
	if c.Tipo != "pf" && c.Tipo != "pj" {
		e.Add("tipo", "Escolha pessoa física ou jurídica")
	}
	if len(c.Nome) < 3 {
		e.Add("nome", "Informe o nome completo")
	}
	if _, err := mail.ParseAddress(c.Email); err != nil || !strings.Contains(c.Email, "@") {
		e.Add("email", "E-mail inválido")
	}
	switch c.Tipo {
	case "pf":
		if !CPFValido(c.CPF) {
			e.Add("cpf", "CPF inválido")
		}
		if t, err := time.Parse("2006-01-02", c.Nascimento); err != nil {
			e.Add("nascimento", "Data inválida")
		} else if t.After(time.Now().AddDate(-18, 0, 0)) {
			e.Add("nascimento", "É preciso ter 18 anos ou mais")
		}
	case "pj":
		if !CNPJValido(c.CNPJ) {
			e.Add("cnpj", "CNPJ inválido")
		}
		if len(c.RazaoSocial) < 3 {
			e.Add("razao_social", "Informe a razão social")
		}
	}
	for i, d := range c.Dependentes {
		row := "dependentes[" + strconv.Itoa(i) + "]."
		if len(d.Nome) < 3 {
			e.Add(row+"nome", "Informe o nome do dependente")
		}
		if _, err := time.Parse("2006-01-02", d.Nascimento); err != nil {
			e.Add(row+"nascimento", "Data inválida")
		}
	}
	validarEndereco(e, "", c.Endereco)
	if c.CobrancaDif {
		validarEndereco(e, "cob_", c.Cobranca)
	}
	// The enum tag already refuses a value that is not on the list; what it
	// cannot know is that this field is required only when the box is ticked.
	if c.Novidades && c.Frequencia == "" {
		e.Add("frequencia", "Escolha a frequência")
	}
	return e
}

func validarEndereco(e trilha.FieldErrors, prefix string, a Endereco) {
	if len(digits(a.CEP)) != 8 {
		e.Add(prefix+"cep", "CEP com 8 dígitos")
	}
	if strings.TrimSpace(a.Rua) == "" {
		e.Add(prefix+"rua", "Informe a rua")
	}
	if strings.TrimSpace(a.Numero) == "" {
		e.Add(prefix+"numero", "Informe o número")
	}
	if _, ok := Cidades[a.UF]; !ok {
		e.Add(prefix+"uf", "Escolha a UF")
	} else if !contains(Cidades[a.UF], a.Cidade) {
		e.Add(prefix+"cidade", "Escolha a cidade")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CPFValido checks the two verifier digits.
func CPFValido(s string) bool {
	s = digits(s)
	if len(s) != 11 || strings.Count(s, s[:1]) == 11 {
		return false
	}
	for _, n := range []int{9, 10} {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(s[i]-'0') * (n + 1 - i)
		}
		d := (sum * 10 % 11) % 10
		if d != int(s[n]-'0') {
			return false
		}
	}
	return true
}

// CNPJValido checks the two verifier digits.
func CNPJValido(s string) bool {
	s = digits(s)
	if len(s) != 14 || strings.Count(s, s[:1]) == 14 {
		return false
	}
	weights := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	for _, n := range []int{12, 13} {
		sum := 0
		w := weights[13-n:]
		for i := 0; i < n; i++ {
			sum += int(s[i]-'0') * w[i]
		}
		d := sum % 11
		if d < 2 {
			d = 0
		} else {
			d = 11 - d
		}
		if d != int(s[n]-'0') {
			return false
		}
	}
	return true
}

// resolverCidade turns what somebody typed into the city itself, which is what
// the combobox does in the browser and the server has to do when there is no
// browser doing it.
func resolverCidade(a *Endereco) {
	if a.Cidade != "" || strings.TrimSpace(a.CidadeQ) == "" {
		return
	}
	typed := strings.ToLower(strings.TrimSpace(a.CidadeQ))
	for _, ci := range Cidades[a.UF] {
		if strings.ToLower(ci) == typed {
			a.Cidade = ci
			return
		}
	}
}

// BuscarCidades lists the cities of a state whose name contains q, at most n.
func BuscarCidades(uf, q string, n int) []string {
	q = strings.ToLower(strings.TrimSpace(q))
	out := []string{}
	for _, ci := range Cidades[uf] {
		if q == "" || strings.Contains(strings.ToLower(ci), q) {
			out = append(out, ci)
		}
		if len(out) == n {
			break
		}
	}
	return out
}

// UFs and Cidades are the dependent-select data (a small sample).
var Cidades = map[string][]string{
	"MG": {"Belo Horizonte", "Uberlândia", "Juiz de Fora"},
	"PR": {"Curitiba", "Londrina", "Maringá"},
	"RJ": {"Rio de Janeiro", "Niterói", "Petrópolis"},
	"SP": {"São Paulo", "Campinas", "Santos", "Ribeirão Preto"},
}

// UFs lists the states, sorted.
func UFs() []string {
	out := make([]string, 0, len(Cidades))
	for uf := range Cidades {
		out = append(out, uf)
	}
	sort.Strings(out)
	return out
}

// ---- store ------------------------------------------------------------------

var (
	mu   sync.Mutex
	seq  int
	list []Cliente
)

// Reset clears the store (tests and Setup).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	seq, list = 0, nil
}

// Salvar stores a validated client.
func Salvar(c Cliente) Cliente {
	mu.Lock()
	defer mu.Unlock()
	seq++
	c.ID, c.Criado = seq, time.Now()
	list = append(list, c)
	return c
}

// Buscar filters by name, e-mail, document or city, ignoring case and
// accents-free punctuation. An empty query returns everyone.
func Buscar(q string) []Cliente {
	q = strings.ToLower(strings.TrimSpace(q))
	todos := Todos()
	if q == "" {
		return todos
	}
	out := todos[:0]
	for _, c := range todos {
		campos := strings.ToLower(strings.Join([]string{c.Nome, c.Email, c.Documento(), c.Endereco.Cidade, c.Endereco.UF}, " "))
		if strings.Contains(campos, q) || strings.Contains(digits(campos), digits(q)) && digits(q) != "" {
			out = append(out, c)
		}
	}
	return out
}

// Todos returns the clients, newest first.
func Todos() []Cliente {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Cliente, len(list))
	for i, c := range list {
		out[len(list)-1-i] = c
	}
	return out
}
