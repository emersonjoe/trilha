// Package anexoarquivo devolve um anexo ao navegador: baixar por padrão, abrir
// no visor com ?ver. As duas metades do mesmo arquivo são duas funções
// diferentes porque são duas decisões de segurança diferentes.
package anexoarquivo

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/blob"
	"github.com/emersonjoe/trilha/examples/blog/internal/anexos"
	arquivos "github.com/emersonjoe/trilha/examples/blog/internal/arquivos"
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
	// O blob entrega: de disco sai pelo http.ServeContent, com Range e 304;
	// de um bucket que sabe pré-assinar, sai como redirecionamento e os bytes
	// não passam por aqui.
	return arquivos.Arquivos.Serve(c, a.Chave, blob.ServeOpts{
		Name:   a.Nome,
		Inline: c.Query("ver") != "",
	})
}
