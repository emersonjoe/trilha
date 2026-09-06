// Package eventos é o outro lado do ui.Live: uma rota que só diz o nome do que
// mudou. O middleware do /painel vale aqui também, então o stream é da sessão
// — não é um cano aberto para quem passar.
package eventos

import (
	"time"

	"github.com/emersonjoe/trilha"
)

// GET mantém a conexão aberta e anuncia o relógio. Num app de verdade quem
// chama o Notify é o worker, o webhook, o SLA — o barramento é do app, não do
// framework; o que o framework leva é o nome.
func GET(c *trilha.Ctx) error {
	s := c.Stream()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := 0; i < 3; i++ {
		select {
		case <-s.Done():
			return nil
		case <-t.C:
			if err := s.Notify("painel:agora"); err != nil {
				return err
			}
		}
	}
	return nil
}
