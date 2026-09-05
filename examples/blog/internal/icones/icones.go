// Package icones embute os ícones do site. Eles moram fora de public/ porque
// quem os gera escreve aqui, e a árvore de disco não precisa ter o formato da
// árvore de URLs: Config.Mounts faz a ligação.
package icones

import (
	"embed"
	"io/fs"
)

//go:embed arquivos
var embutidos embed.FS

// FS devolve a árvore servida em /icones/.
func FS() fs.FS {
	sub, err := fs.Sub(embutidos, "arquivos")
	if err != nil {
		panic(err) // embed: o diretório existe em tempo de compilação.
	}
	return sub
}
