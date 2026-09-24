// Command s7-tela-base is the Go-pure twin of the ruler's s7-tela scenario:
// the same data, the same task, standard library only. handler is what the
// hidden test drives.
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
	mux.HandleFunc("GET /{$}", home)
	return mux
}

var homeTmpl = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Início</title></head>
<body><main><h1>Início</h1><p>Um app vazio: a primeira feature começa daqui.</p></main></body></html>
`))

func home(w http.ResponseWriter, _ *http.Request) {
	homeTmpl.Execute(w, nil)
}
