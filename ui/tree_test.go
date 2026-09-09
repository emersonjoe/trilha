package ui

import (
	"strings"
	"testing"
)

func arvore() []TreeNode {
	return []TreeNode{
		{Value: "100", Label: "100 Administração", Open: true, Children: []TreeNode{
			{Value: "100.1", Label: "100.1 Financeiro"},
			{Value: "100.2", Label: "100.2 Pessoas", Leaf: true},
		}},
		{Value: "200", Label: "200 Operações"},
	}
}

// Os papéis são os de verdade, e o ramo aberto diz que está aberto: uma árvore
// sem aria-expanded é uma lista com recuo.
func TestTreeTemOsPapeisCertos(t *testing.T) {
	got := render(t, Tree(TreeOpts{Nodes: arvore(), Source: "/nos", Label: "Setores"}))
	for _, want := range []string{
		`role="tree"`, `aria-label="Setores"`, `data-ui-tree-src="/nos"`,
		`role="treeitem"`, `role="group"`, `aria-expanded="true"`, `aria-expanded="false"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
	// Um só role por elemento: dois seria uma promessa inválida, não uma
	// promessa mais forte.
	if strings.Contains(got, `role="tree" data-ui-tree="" role=`) {
		t.Fatalf("role duplicado:\n%s", got)
	}
}

// Folha não tem seta, e ramo sem filhos carregados não vem vazio: vem com o
// lugar onde eles entram.
func TestTreeDistingueFolhaDeRamo(t *testing.T) {
	got := render(t, Tree(TreeOpts{Nodes: arvore()}))
	// 100, 100.1 e 200 são ramos; 100.2 é folha e não vira <details> nenhum.
	if n := strings.Count(got, "<details"); n != 3 {
		t.Fatalf("ramos = %d:\n%s", n, got)
	}
	if !strings.Contains(got, `data-ui-tree-pending=""`) {
		t.Fatalf("o ramo fechado precisa do lugar dos filhos:\n%s", got)
	}
	// A folha não é <details> nenhum.
	if !strings.Contains(got, `data-value="100.2" tabindex="-1"`) {
		t.Fatalf("folha:\n%s", got)
	}
}

// O caminho aberto vem pronto do servidor: nada é buscado para mostrar onde a
// pessoa estava.
func TestTreeTrazOCaminhoAberto(t *testing.T) {
	got := render(t, Tree(TreeOpts{Nodes: arvore(), Current: "100.1"}))
	if !strings.Contains(got, "open>") {
		t.Fatalf("o ramo do caminho devia vir aberto:\n%s", got)
	}
	if !strings.Contains(got, `aria-current="true"`) {
		t.Fatalf("o nó atual devia estar marcado:\n%s", got)
	}
}

// O seletor posta um radio: é isso que faz a coisa inteira funcionar sem
// JavaScript nenhum.
func TestTreePickerPostaUmRadio(t *testing.T) {
	got := render(t, TreePicker(TreePickerOpts{
		Name: "setor", Value: "100.2", Nodes: arvore(),
		Source: "/nos", Search: "/busca", Label: "Setores", Placeholder: "Buscar",
	}))
	for _, want := range []string{
		`type="radio"`, `name="setor"`, `value="100.2" class="ui-tree-radio" checked`,
		`name="setor_q"`, `data-ui-tree-search="/busca"`, `class="ui-tree" role="group"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
	// Sem hidden: o valor é o radio, e dois campos com o mesmo nome mandariam
	// dois valores.
	if strings.Contains(got, `type="hidden" name="setor"`) {
		t.Fatalf("o seletor não pode ter um hidden do mesmo nome:\n%s", got)
	}
}

// Rótulo vazio não vira aria-label vazio: nomear o campo com nada é pior que
// não nomear, porque esconde o que o <label> em volta diria.
func TestTreePickerSemRotuloNaoInventaAria(t *testing.T) {
	got := render(t, TreePicker(TreePickerOpts{Name: "x", Nodes: arvore(), Search: "/b"}))
	if strings.Contains(got, `aria-label=""`) {
		t.Fatalf("aria-label vazio:\n%s", got)
	}
}

// O resultado de busca vem com a ancestralidade: um código achado fora de
// contexto não diz onde mora.
func TestTreeItemsMostraOCaminhoDaBusca(t *testing.T) {
	got := render(t, TreeItems([]TreeNode{
		{Value: "100.1.1", Label: "100.1.1 Contas a pagar", Leaf: true, Path: "Administração › Financeiro"},
	}, TreeOpts{Name: "setor"}))
	if !strings.Contains(got, "Administração › Financeiro") || !strings.Contains(got, "ui-tree-path") {
		t.Fatalf("caminho:\n%s", got)
	}
}
