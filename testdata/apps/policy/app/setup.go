package app

import "github.com/emersonjoe/trilha"

// Situacoes é a lista de domínio deste projeto sintético: declarada uma vez,
// registrada no Setup, e é isso que o ctx tem de juntar.
var Situacoes = trilha.Enum{
	{Value: "rascunho", Label: "Rascunho"},
	{Value: "enviado", Label: "Enviado", Tone: "info"},
	{Value: "aprovado", Label: "Aprovado", Tone: "success"},
}

// Posicional existe para provar que a leitura entende as duas formas de
// escrever a mesma lista.
var Posicional = trilha.Enum{
	{"sim", "Sim", ""},
	{"nao", "Não", "danger"},
}

// Solta é declarada e nunca registrada: nada a procura por nome, então ela não
// aparece no mapa.
var Solta = trilha.Enum{{Value: "x"}}

func Setup() {
	trilha.RegisterEnum("doc.situacao", Situacoes)
	trilha.RegisterEnum("doc.confirma", Posicional)
}
