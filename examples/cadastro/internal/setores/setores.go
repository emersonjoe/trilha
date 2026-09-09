// Package setores is the hierarchy this example picks from: a small
// organisation chart, in memory, with the shape a real classification plan has
// — a code, a name, and children under it.
package setores

import "strings"

// Setor is one node.
type Setor struct {
	Codigo string
	Nome   string
	Pai    string
}

// Todos is the chart, flat: the parent is a field, which is how it comes out of
// a table and how a real one is stored.
var Todos = []Setor{
	{Codigo: "100", Nome: "Administração"},
	{Codigo: "100.1", Nome: "Financeiro", Pai: "100"},
	{Codigo: "100.1.1", Nome: "Contas a pagar", Pai: "100.1"},
	{Codigo: "100.1.2", Nome: "Contas a receber", Pai: "100.1"},
	{Codigo: "100.2", Nome: "Pessoas", Pai: "100"},
	{Codigo: "100.2.1", Nome: "Recrutamento", Pai: "100.2"},
	{Codigo: "200", Nome: "Operações"},
	{Codigo: "200.1", Nome: "Atendimento", Pai: "200"},
	{Codigo: "200.2", Nome: "Logística", Pai: "200"},
	{Codigo: "200.2.1", Nome: "Expedição", Pai: "200.2"},
}

// Filhos are the nodes directly under pai; an empty pai gives the roots.
func Filhos(pai string) []Setor {
	var out []Setor
	for _, s := range Todos {
		if s.Pai == pai {
			out = append(out, s)
		}
	}
	return out
}

// Folha reports whether nothing hangs under this node, which is what decides
// whether it draws an arrow at all.
func Folha(codigo string) bool { return len(Filhos(codigo)) == 0 }

// Existe is the check a form needs: a code that came from a request is only a
// choice if it is one of these.
func Existe(codigo string) bool {
	for _, s := range Todos {
		if s.Codigo == codigo {
			return true
		}
	}
	return false
}

// Por finds one node.
func Por(codigo string) (Setor, bool) {
	for _, s := range Todos {
		if s.Codigo == codigo {
			return s, true
		}
	}
	return Setor{}, false
}

// Caminho is the ancestry of a node, "Administração › Financeiro", which is
// what makes a search result out of context mean something.
func Caminho(codigo string) string {
	s, ok := Por(codigo)
	if !ok {
		return ""
	}
	var partes []string
	for p := s.Pai; p != ""; {
		pai, ok := Por(p)
		if !ok {
			break
		}
		partes = append([]string{pai.Nome}, partes...)
		p = pai.Pai
	}
	return strings.Join(partes, " › ")
}

// Buscar finds by code or name, anywhere in the tree — the other way into a
// hierarchy, and the only one that works when somebody knows the code and not
// where it lives.
func Buscar(q string) []Setor {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []Setor
	for _, s := range Todos {
		if strings.Contains(strings.ToLower(s.Codigo), q) || strings.Contains(strings.ToLower(s.Nome), q) {
			out = append(out, s)
		}
	}
	return out
}

// Rotulo is what a person reads: "100.1 Financeiro".
func Rotulo(s Setor) string { return s.Codigo + " " + s.Nome }

// Abertos are the roots with the path down to escolhido already open, which is
// how a form that came back with an error comes back showing the choice
// instead of a collapsed tree.
func Abertos(escolhido string) []string {
	var abrir []string
	for p := escolhido; p != ""; {
		s, ok := Por(p)
		if !ok {
			break
		}
		if s.Pai != "" {
			abrir = append(abrir, s.Pai)
		}
		p = s.Pai
	}
	return abrir
}
