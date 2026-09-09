package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/setores"
)

// Setup seeds one client so the list is not empty.
func Setup(a *trilha.App) error {
	// The name the validate tag cites. Registering it here means a tag that
	// names an enum nobody declared fails on the first request instead of
	// accepting anything.
	trilha.RegisterEnum("cadastro.Frequencia", clientes.Frequencias)
	// O seletor de árvore manda um código; a regra confere que ele é um nó de
	// verdade. Sem isto, um POST à mão põe qualquer string no cadastro — o
	// campo é escolhido na tela, e o que chega ao servidor não vem da tela.
	trilha.AddRule("setor", func(f trilha.Field) bool { return setores.Existe(f.Text) })
	trilha.ValidationMessages["setor"] = "escolha um setor da árvore"
	clientes.Reset()
	clientes.Salvar(clientes.Cliente{Tipo: "pf", Nome: "Ada Lovelace", Email: "ada@example.com", CPF: "52998224725", Nascimento: "1815-12-10",
		Endereco: clientes.Endereco{CEP: "13010000", Rua: "Rua Treze de Maio", Numero: "100", UF: "SP", Cidade: "Campinas"}})
	return nil
}
