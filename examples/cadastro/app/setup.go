package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
)

// Setup seeds one client so the list is not empty.
func Setup(a *trilha.App) error {
	// The name the validate tag cites. Registering it here means a tag that
	// names an enum nobody declared fails on the first request instead of
	// accepting anything.
	trilha.RegisterEnum("cadastro.Frequencia", clientes.Frequencias)
	clientes.Reset()
	clientes.Salvar(clientes.Cliente{Tipo: "pf", Nome: "Ada Lovelace", Email: "ada@example.com", CPF: "52998224725", Nascimento: "1815-12-10",
		Endereco: clientes.Endereco{CEP: "13010000", Rua: "Rua Treze de Maio", Numero: "100", UF: "SP", Cidade: "Campinas"}})
	return nil
}
