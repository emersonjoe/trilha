// Package midiaaudio serve o episódio que vem de outro serviço. O corpo não
// sabe buscar posição, então quem responde o Range é o kit, com o tamanho que
// o outro serviço declarou.
package midiaaudio

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/midia"
)

// GET /midia/audio devolve o episódio inteiro, ou o trecho que o navegador
// pediu.
//
// O Size é o que muda tudo: com ele a primeira resposta traz Accept-Ranges e
// Content-Length — é ela que ensina o tamanho ao navegador — e o pedido
// seguinte, com Range, volta 206 com o Content-Range e só aqueles bytes. Sem
// isso o <audio> do Safari no iOS não posiciona: "pular para o instante X"
// vira baixar o arquivo inteiro.
//
// Quando o serviço de trás sabe responder Range, o melhor é repassar o
// cabeçalho para ele e devolver o 206 dele aqui, que o prefixo não viaja duas
// vezes:
//
//	return c.Send(nome, res.Body, tipo, trilha.SendOpts{
//		Inline:       true,
//		ContentRange: res.Header.Get("Content-Range"),
//	})
//
// openapi:tag midia
func GET(c *trilha.Ctx) error {
	corpo, tamanho := midia.Episodio()
	return c.Send("episodio.wav", corpo, midia.EpisodioTipo, trilha.SendOpts{
		Inline: true,
		Size:   tamanho,
	})
}
