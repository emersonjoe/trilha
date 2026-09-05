package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
)

// Setup seeds one client so the list is not empty.
func Setup(a *trilha.App) error {
	clientes.Reset()
	clientes.Salvar(clientes.Cliente{Tipo: "pf", Nome: "Ada Lovelace", Email: "ada@example.com", CPF: "52998224725", Nascimento: "1815-12-10",
		Endereco: clientes.Endereco{CEP: "13010000", Rua: "Rua Treze de Maio", Numero: "100", UF: "SP", Cidade: "Campinas"}})
	return nil
}
