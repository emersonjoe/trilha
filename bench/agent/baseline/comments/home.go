package main

import (
	"html/template"
	"net/http"
)

var homeTmpl = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Blog</title></head>
<body><main><h1>Blog</h1><ul>{{range .}}<li><a href="/blog/{{.ID}}">{{.Title}}</a></li>{{end}}</ul></main></body></html>
`))

func home(w http.ResponseWriter, _ *http.Request) {
	homeTmpl.Execute(w, posts)
}
