// /.well-known/ é a única pasta com ponto no começo que o scanner enxerga: é
// onde as RFCs mandam publicar documento. Aqui, a política de segurança da
// RFC 9116; o nome do pacote não pode ser o da pasta, então é declarado.
package security

import (
	"fmt"
	"time"

	"github.com/emersonjoe/trilha"
)

// CORS é a política desta rota, e só dela: o documento é buscado de outra
// origem por quem ainda não tem token nenhum, enquanto as outras rotas do blog
// são cookie e mesma origem. O framework responde o preflight a partir daqui.
var CORS = trilha.CORS{Origins: []string{"*"}, Methods: []string{"GET"}, MaxAge: 24 * time.Hour}

// GET serve a security.txt da RFC 9116, com validade de um ano.
func GET(c *trilha.Ctx) error {
	return c.Text(200, fmt.Sprintf(""+
		"Contact: mailto:seguranca@example.com\n"+
		"Expires: %s\n"+
		"Preferred-Languages: pt-BR, en\n",
		time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339)))
}
