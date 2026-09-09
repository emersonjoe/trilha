package sessao

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
)

// Escopos is what a key of this application may carry, declared once. Require
// refuses anything outside this list at wiring time, so a typo in a route is a
// panic on boot and not a door left open.
var Escopos = []string{"documentos:ler", "documentos:escrever"}

// Chaves are the API keys this application issues. The store is in memory
// because this is an example; a real one writes a table, and the interface has
// five methods precisely so that is the only decision left.
var Chaves = auth.APIKeys(auth.KeyOptions{
	Prefix: "ll",
	Scopes: Escopos,
	// Por chave, e não por endereço: um cliente atrás de uma chave é um
	// orçamento, faça o IP dele o que fizer.
	RateLimit: trilha.RateLimit{RPS: 5, Burst: 20},
})
