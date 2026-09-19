package recipes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// #274 — uma chave que só existia na tabela em inglês não saía em branco na
// tela em português: saía como "<no value>", que é o que o text/template
// escreve para uma chave ausente. Três receitas foram entregues assim, e a
// segunda vez que isso acontece é a vez de escrever o teste em vez de
// corrigir as chaves de novo.
//
// Ele adiciona todas as receitas num projeto só, em cada idioma, e lê o que
// foi escrito. É o mesmo caminho do `trilha add`, então uma chave nova
// esquecida amanhã reprova aqui e não na tela de quem rodou o comando.
func TestNenhumaReceitaEscreveValorAusente(t *testing.T) {
	for _, lang := range []string{"en", "pt"} {
		t.Run(lang, func(t *testing.T) {
			raiz := projeto(t)
			for _, r := range emOrdemDeDependencia(t) {
				res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: lang})
				if err != nil {
					t.Fatalf("%s: %v", r.Name, err)
				}
				for _, rel := range res.Written {
					corpo, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
					if err != nil {
						t.Fatal(err)
					}
					if i := strings.Index(string(corpo), "<no value>"); i >= 0 {
						linha := trechoDaLinha(string(corpo), i)
						t.Errorf("%s (%s) escreveu uma chave ausente em %s:\n\t%s",
							r.Name, lang, rel, linha)
					}
				}
			}
		})
	}
}

// emOrdemDeDependencia devolve todas as receitas numa ordem em que cada uma
// encontra o que precisa já escrito.
func emOrdemDeDependencia(t *testing.T) []Recipe {
	t.Helper()
	faltam := All()
	escritas := map[string]bool{}
	var ordem []Recipe
	for len(faltam) > 0 {
		var sobraram []Recipe
		avancou := false
		for _, r := range faltam {
			pronta := true
			for _, n := range r.Needs {
				if !escritas[n.Recipe] {
					pronta = false
					break
				}
			}
			if !pronta {
				sobraram = append(sobraram, r)
				continue
			}
			ordem = append(ordem, r)
			escritas[r.Name] = true
			avancou = true
		}
		if !avancou {
			var nomes []string
			for _, r := range sobraram {
				nomes = append(nomes, r.Name)
			}
			t.Fatalf("dependência circular ou receita faltando: %s", strings.Join(nomes, ", "))
		}
		faltam = sobraram
	}
	return ordem
}

// trechoDaLinha é a linha inteira em volta de uma posição, para a falha dizer
// onde procurar em vez de só dizer que aconteceu.
func trechoDaLinha(s string, i int) string {
	inicio := strings.LastIndexByte(s[:i], '\n') + 1
	fim := strings.IndexByte(s[i:], '\n')
	if fim < 0 {
		return strings.TrimSpace(s[inicio:])
	}
	return strings.TrimSpace(s[inicio : i+fim])
}

// A rede de segurança do fallback tem um custo: se alguém apagar uma tradução,
// o inglês entra no lugar e a tela continua legível — então o teste acima, que
// só procura "<no value>", passa calado. Este fecha essa porta.
//
// A ideia é do PR #276, aberto contra a mesma #274: afirmar que o português
// traduz de fato, em vez de só cair no inglês.
func TestPortuguesTraduzEmVezDeCairNoIngles(t *testing.T) {
	en, pt := wordsEN(), words("pt")

	// O fallback cobre toda chave: é o que a #274 pediu.
	for chave := range en {
		if pt[chave] == "" {
			t.Errorf("pt[%q] ficou vazia; o fallback para o inglês não cobriu", chave)
		}
	}

	// E as dezessete que a #274 encontrou estão na tabela em português, não
	// emprestadas do inglês. Sem isto, apagar uma tradução não reprova nada.
	dezessete := []string{
		"blob_title", "blob_desc", "blob_field", "blob_send", "blob_empty",
		"blob_name", "blob_type", "blob_size", "blob_when",
		"share_title", "share_desc", "share_item",
		"inbox_title", "inbox_desc", "inbox_recent", "inbox_done", "inbox_not_yours",
	}
	tabela := wordsPT()
	for _, chave := range dezessete {
		valor, tem := tabela[chave]
		if !tem || valor == "" {
			t.Errorf("%q saiu da tabela em português; voltaria a ser servida em inglês", chave)
			continue
		}
		// share_item é "Item %s." nos dois idiomas, e é assim mesmo: o verbo
		// do Sprintf é o conteúdo. Os outros têm de diferir.
		if chave != "share_item" && valor == en[chave] {
			t.Errorf("pt[%q] é o texto em inglês", chave)
		}
	}

	// O verbo do Sprintf sobrevive à tradução: uma chave que o perdesse
	// escreveria a tela sem o id.
	for _, chave := range []string{"share_item", "mail_welcome_hi"} {
		if !strings.Contains(tabela[chave], "%s") {
			t.Errorf("pt[%q] perdeu o %%s: %q", chave, tabela[chave])
		}
	}
}
