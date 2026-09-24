package main

import "net/http"

// registrarExtras was left behind by a rename: it registers the same path the
// handler above already serves, which makes the mux panic on the first call.
func registrarExtras(mux *http.ServeMux) {
	mux.HandleFunc("GET /contas", form)
}
