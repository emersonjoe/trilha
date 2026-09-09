package acesso

import "github.com/emersonjoe/trilha"

// Exige é o embrulho que a documentação do auth ensina: dois parâmetros que
// vão direto para o RequirePolicy. Reconhecê-lo é reconhecer o que as pessoas
// escrevem.
func Exige(modulo, nivel string) trilha.MiddlewareFunc {
	return Auth.RequirePolicy(Politica, modulo, nivel)
}
