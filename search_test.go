package trilha

import (
	"strings"
	"testing"
)

func indice(t *testing.T, o SearchOpts) *Search {
	t.Helper()
	s := NewSearch(o).
		Kind("pessoa", KindOpts{Label: "Pessoas", Module: "rh"}).
		Kind("processo", KindOpts{Label: "Processos"})
	docs := []Doc{
		{Kind: "pessoa", ID: "1", Title: "João da Silva", Body: "joao@example.com 111.222.333-44", URL: "/pessoas/1"},
		{Kind: "pessoa", ID: "2", Title: "Maria Souza", Body: "maria@example.com", URL: "/pessoas/2", Tenant: "outra"},
		{Kind: "processo", ID: "9", Title: "Processo 2024/07",
			Body: "Objeto: contratação de serviço para o João <b> da portaria", URL: "/processos/9"},
	}
	if err := s.Put(nil, docs...); err != nil {
		t.Fatal(err)
	}
	return s
}

// #149 — acento e caixa não importam: é o motivo de a busca ser um índice e não
// um LIKE, e é o que todo mundo tenta primeiro.
func TestSearchAcentoECaixa(t *testing.T) {
	s := indice(t, SearchOpts{})
	for _, q := range []string{"joao", "JOÃO", "Joao"} {
		res, err := s.Query(nil, q, SearchQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Total != 2 {
			t.Fatalf("%q achou %d: %+v", q, res.Total, res.Groups)
		}
	}
	// Um termo que não existe não acha nada, e nada é uma resposta e não um erro.
	res, err := s.Query(nil, "helicóptero", SearchQuery{})
	if err != nil || res.Total != 0 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	// Dois termos são um E: a busca de uma caixa só é uma conjunção.
	if res, _ := s.Query(nil, "joao portaria", SearchQuery{}); res.Total != 1 {
		t.Fatalf("dois termos = %d", res.Total)
	}
}

// O resultado sai agrupado na ordem em que os tipos foram declarados, para a
// tela não trocar de lugar entre uma busca e outra.
func TestSearchAgrupaNaOrdemDeclarada(t *testing.T) {
	s := indice(t, SearchOpts{})
	res, _ := s.Query(nil, "joao", SearchQuery{})
	if len(res.Groups) != 2 {
		t.Fatalf("grupos = %+v", res.Groups)
	}
	if res.Groups[0].Kind != "pessoa" || res.Groups[0].Label != "Pessoas" {
		t.Fatalf("primeiro grupo = %+v", res.Groups[0])
	}
	if res.Groups[1].Kind != "processo" || len(res.Groups[1].Hits) != 1 {
		t.Fatalf("segundo grupo = %+v", res.Groups[1])
	}
	// O título pesa mais que o corpo: quem se chama João vem antes do processo
	// que menciona um.
	if res.Groups[0].Hits[0].Score <= res.Groups[1].Hits[0].Score {
		t.Fatalf("o título não pesou mais: %v vs %v",
			res.Groups[0].Hits[0].Score, res.Groups[1].Hits[0].Score)
	}
	// Um tipo pedido de fora da lista não inventa grupo.
	if r, _ := s.Query(nil, "joao", SearchQuery{Kinds: []string{"processo"}}); len(r.Groups) != 1 {
		t.Fatalf("filtro por tipo = %+v", r.Groups)
	}
}

// O trecho vem partido em pedaços, com as partes que casaram marcadas: quem
// escreve o <mark> é a tela, porque uma string com marcação vinda do dado é uma
// injeção esperando o primeiro corpo com <script>.
func TestSearchTrechoNaoEHTML(t *testing.T) {
	s := indice(t, SearchOpts{})
	res, _ := s.Query(nil, "portaria", SearchQuery{})
	hit := res.Groups[0].Hits[0]
	var texto strings.Builder
	marcadas := 0
	for _, p := range hit.Snippet {
		texto.WriteString(p.Text)
		if p.Match {
			marcadas++
			if !strings.EqualFold(p.Text, "portaria") {
				t.Fatalf("parte marcada = %q", p.Text)
			}
		}
	}
	if marcadas != 1 {
		t.Fatalf("%d partes marcadas em %+v", marcadas, hit.Snippet)
	}
	if !strings.Contains(texto.String(), "<b>") {
		t.Fatalf("o corpo foi mexido: %q", texto.String())
	}
}

// Um documento de outro tenant não aparece. É a linha que alguém esquece uma
// vez e vira incidente.
func TestSearchTenant(t *testing.T) {
	s := indice(t, SearchOpts{Tenant: func(*Ctx) string { return "acme" }})
	res, _ := s.Query(nil, "maria", SearchQuery{})
	if res.Total != 0 {
		t.Fatalf("achou o de outro tenant: %+v", res.Groups)
	}
	// E o que foi gravado sem tenant herdou o do contexto, então continua sendo
	// achado por quem o gravou.
	if r, _ := s.Query(nil, "joao", SearchQuery{}); r.Total != 2 {
		t.Fatalf("o próprio tenant sumiu: %+v", r.Groups)
	}
}

// Um tipo cujo módulo é negado não aparece, nem no contador: um "3 pessoas"
// para quem não pode ver pessoas já contou o que não devia.
func TestSearchModuloNegado(t *testing.T) {
	s := indice(t, SearchOpts{
		Allow: func(_ *Ctx, modulo string) bool { return modulo != "rh" },
	})
	res, _ := s.Query(nil, "joao", SearchQuery{})
	if res.Total != 1 || len(res.Groups) != 1 || res.Groups[0].Kind != "processo" {
		t.Fatalf("o módulo negado apareceu: %+v", res)
	}
}

// Apagar apaga, e reindexar troca tudo de um tipo sem tocar nos outros.
func TestSearchApagaEReindexa(t *testing.T) {
	s := indice(t, SearchOpts{})
	if err := s.Delete(nil, "pessoa", "1"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Query(nil, "silva", SearchQuery{}); r.Total != 0 {
		t.Fatalf("continuou achando o apagado: %+v", r.Groups)
	}
	if err := s.Reindex(nil, "processo", []Doc{
		{Kind: "processo", ID: "10", Title: "Processo 2025/01", URL: "/processos/10"},
	}); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Query(nil, "portaria", SearchQuery{}); r.Total != 0 {
		t.Fatalf("o antigo sobreviveu à reindexação: %+v", r.Groups)
	}
	if r, _ := s.Query(nil, "2025", SearchQuery{}); r.Total != 1 {
		t.Fatalf("o novo não entrou: %+v", r.Groups)
	}
}

// Um tipo que ninguém declarou não é indexado: um Kind digitado errado que
// grava em silêncio é um registro que nunca aparece na busca.
func TestSearchTipoNaoDeclarado(t *testing.T) {
	s := NewSearch(SearchOpts{}).Kind("pessoa", KindOpts{})
	if err := s.Put(nil, Doc{Kind: "pesoa", ID: "1", Title: "João"}); err == nil {
		t.Fatal("gravou um tipo que ninguém declarou")
	}
	hint := HintOf(NewSearch(SearchOpts{}).Put(nil, Doc{Kind: "x", ID: "1"}))
	if hint == nil || hint.Repair == "" {
		t.Fatalf("a recusa não ensina: %+v", hint)
	}
}

// SearchTerms é o mesmo tokenizador para quem escrever um store em SQL: dois
// tokenizadores diferentes são um índice que não acha o que gravou.
func TestSearchTerms(t *testing.T) {
	got := SearchTerms("  João  da  SILVA, 111.222!  ")
	quero := []string{"joao", "da", "silva", "111", "222"}
	if len(got) != len(quero) {
		t.Fatalf("termos = %q", got)
	}
	for i := range got {
		if got[i] != quero[i] {
			t.Fatalf("termos = %q", got)
		}
	}
	if len(SearchTerms("   ")) != 0 {
		t.Fatal("espaço virou termo")
	}
}
