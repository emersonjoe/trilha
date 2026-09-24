package main

import (
	"html/template"
	"net/http"
)

// layout is the frame of every page of this site, nav included — a new page
// has to live inside it.
var layout = template.Must(template.New("layout").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body><nav><a href="/">Início</a> · <a href="/precos">Preços</a> · <a href="/blog">Blog</a></nav>
<main>{{template "body" .}}</main></body></html>
`))

type pagina struct {
	Title string
}

func home(w http.ResponseWriter, _ *http.Request) {
	layout.Execute(w, pagina{Title: "Início"})
}
