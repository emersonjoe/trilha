// Command s8-conserto-base is the Go-pure twin of the ruler's s8-conserto
// scenario: the same register, the same three defects, standard library only.
// handler is what the hidden test drives.
package main

import (
	"log"
	"net/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", handler()))
}

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("GET /contas", form)
	mux.HandleFunc("POST /contas", salvar)
	registrarExtras(mux)
	return mux
}
