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
