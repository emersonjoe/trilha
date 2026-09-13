// Package midia guarda os dois binários que a tela de mídia serve: o episódio,
// que chega como um leitor que não sabe buscar posição, e a marca, que é um
// SVG — o tipo que o navegador trata como documento com script.
package midia

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
)

// Marca é o logotipo que uma organização subiu. O <script> está aqui de
// propósito: é o que um SVG pode carregar, e é por isso que ele só sai com a
// política que o kit impõe no SendOpts.NeutralizeScript.
const Marca = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64">` +
	`<style>.t{fill:#111}</style>` +
	`<rect width="64" height="64" rx="12" fill="#f5f5f5"/>` +
	`<path class="t" d="M14 44 L32 18 L50 44 Z"/>` +
	`<script>document.title = "marca"</script>` +
	`</svg>`

// MarcaTipo é o tipo do logotipo, e é o que faz o c.Inline recusá-lo.
const MarcaTipo = "image/svg+xml"

// Logo é o logotipo de uma organização: o arquivo como ele foi subido, com o
// nome e o tipo que o upload leu no conteúdo.
type Logo struct {
	Nome  string
	Tipo  string
	Bytes []byte
}

// logos é o que as organizações subiram. A segunda mandou um arquivo que não é
// desenho nenhum — acontece —, e é ela que exercita o outro lado do contrato:
// a recusa que a tela trata.
var logos = map[string]Logo{
	"trilha": {Nome: "marca.svg", Tipo: MarcaTipo, Bytes: []byte(Marca)},
	"acervo": {Nome: "marca.zip", Tipo: "application/zip", Bytes: []byte("PK\x03\x04")},
}

// LogoDe devolve o logotipo de uma organização; vazio é a do próprio blog.
func LogoDe(org string) (Logo, bool) {
	if org == "" {
		return LogoPadrao(), true
	}
	l, ok := logos[org]
	return l, ok
}

// LogoPadrao é a marca que a tela mostra quando não há outra.
func LogoPadrao() Logo { return logos["trilha"] }

// episodio é o áudio: um WAV de um segundo, montado uma vez.
var episodio = wav()

// Episodio devolve o corpo do episódio e o tamanho dele.
//
// Num app de verdade estes dois valores são o `res.Body` e o
// `res.ContentLength` da resposta de outro serviço — o backend que guarda o
// áudio, atrás do Config.Upstreams. Aqui o corpo é embrulhado num io.Reader
// que esconde o Seek, porque é isso que importa para a rota: um corpo que não
// sabe buscar posição não passa pelo http.ServeContent, e sem o Size do
// SendOpts sairia inteiro, com 200, a cada vez que o <audio> do iPhone pula
// para o meio da peça.
func Episodio() (io.Reader, int64) {
	return semBusca{bytes.NewReader(episodio)}, int64(len(episodio))
}

// EpisodioTipo é o tipo do áudio.
const EpisodioTipo = "audio/wav"

// semBusca é um corpo que só sabe ler, como o de toda resposta HTTP.
type semBusca struct{ io.Reader }

// wav monta um segundo de um lá de 440 Hz em PCM 8 bits, 8 kHz mono: o
// cabeçalho de 44 bytes que todo navegador conhece e as amostras depois dele.
func wav() []byte {
	const taxa = 8000
	amostras := make([]byte, taxa)
	for i := range amostras {
		amostras[i] = byte(128 + 40*math.Sin(2*math.Pi*440*float64(i)/taxa))
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(amostras)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))   // tamanho do fmt
	binary.Write(&b, binary.LittleEndian, uint16(1))    // PCM
	binary.Write(&b, binary.LittleEndian, uint16(1))    // mono
	binary.Write(&b, binary.LittleEndian, uint32(taxa)) // amostras por segundo
	binary.Write(&b, binary.LittleEndian, uint32(taxa)) // bytes por segundo
	binary.Write(&b, binary.LittleEndian, uint16(1))    // alinhamento do bloco
	binary.Write(&b, binary.LittleEndian, uint16(8))    // bits por amostra
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(amostras)))
	b.Write(amostras)
	return b.Bytes()
}
