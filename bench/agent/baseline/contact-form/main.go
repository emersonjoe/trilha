// Command contact-form-base is the Go-pure twin of the ruler's contact-form
// scenario: the same site, the same task, standard library only. handler is
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
	mux.HandleFunc("GET /{$}", home)
	return mux
}
