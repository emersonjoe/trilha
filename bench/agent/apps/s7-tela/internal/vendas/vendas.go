// Package vendas holds this month's sales. It is memory here, which is the
// honest starting point: the rows last as long as the process, and a real
// store arrives behind the same function.
package vendas

// Venda is one row: what was sold, to whom, in which state, for how much.
type Venda struct {
	ID      string
	Cliente string
	Estado  string
	Total   float64
}

// Todas returns every sale of the month, oldest first.
func Todas() []Venda {
	return []Venda{
		{"v-01", "Ana Souza", "SP", 120.50},
		{"v-02", "Ana Souza", "SP", 89.90},
		{"v-03", "Ana Souza", "SP", 310.00},
		{"v-04", "Ana Souza", "SP", 74.20},
		{"v-05", "Bruno Lima", "RJ", 205.10},
		{"v-06", "Bruno Lima", "RJ", 42.00},
		{"v-07", "Bruno Lima", "RJ", 178.35},
		{"v-08", "Bruno Lima", "RJ", 99.99},
		{"v-09", "Carla Dias", "MG", 260.40},
		{"v-10", "Carla Dias", "MG", 55.75},
		{"v-11", "Carla Dias", "MG", 133.10},
		{"v-12", "Carla Dias", "MG", 88.88},
	}
}
