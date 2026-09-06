// Package anexoarquivo devolve um anexo ao navegador: baixar por padrão, abrir
// no visor com ?ver. As duas metades do mesmo arquivo são duas funções
// diferentes porque são duas decisões de segurança diferentes.
package anexoarquivo

import (
	"bytes"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/anexos"
)

// GET manda o anexo. O nome que vai no Content-Disposition é o que o c.File
// saneou na entrada, e o tipo é o que ele leu no conteúdo — nenhum dos dois é
// o que o navegador disse na hora do upload.
//
// openapi:query ver string  qualquer valor abre no visor em vez de baixar
// openapi:tag anexos
func GET(c *trilha.Ctx) error {
	a, ok := anexos.Por(c.Param("nome"))
	if !ok {
		return trilha.Errorf(http.StatusNotFound, "anexo não encontrado")
	}
	corpo := bytes.NewReader(a.Conteudo)
	if c.Query("ver") != "" {
		// O Inline recusa o que o navegador rodaria em vez de mostrar, então
		// um texto ou um PDF abrem e um HTML não chega aqui como inline.
		return c.Inline(a.Nome, corpo, a.Tipo)
	}
	return c.Attachment(a.Nome, corpo, a.Tipo)
}
