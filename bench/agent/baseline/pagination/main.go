// Command pagination-base is the Go-pure twin of the ruler's pagination
// scenario: the same blog, the same task, standard library only. handler is
// what the hidden test drives.
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
	mux.HandleFunc("GET /blog", blog)
	return mux
}
