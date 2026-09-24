// Command generate-crud-base is the Go-pure twin of the ruler's generate-crud
// scenario: the same task, standard library only. handler is what the hidden
// test drives.
package main

import (
	"html/template"
	"log"
	"net/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", handler()))
}

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /categorias", listar)
	return mux
}

var listaTmpl = template.Must(template.New("lista").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Categorias</title></head>
<body><main><h1>Categorias</h1><p>Nenhuma categoria ainda.</p></main></body></html>
`))

// listar shows the register — empty, because nothing creates yet.
func listar(w http.ResponseWriter, _ *http.Request) {
	listaTmpl.Execute(w, nil)
}
