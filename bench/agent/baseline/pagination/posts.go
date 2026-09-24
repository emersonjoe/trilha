package main

import (
	"fmt"
	"html/template"
	"net/http"
)

// post is one row of the blog; twelve of them are seeded, which is what makes
// paging a piece of work and not a no-op.
type post struct {
	ID, Title string
}

var posts = func() (out []post) {
	for i := 1; i <= 12; i++ {
		out = append(out, post{fmt.Sprintf("post-%02d", i), fmt.Sprintf("Post %02d", i)})
	}
	return out
}()

var blogTmpl = template.Must(template.New("blog").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Blog</title></head>
<body><main><h1>Blog</h1><ul>{{range .}}<li><a href="/blog/{{.ID}}">{{.Title}}</a></li>{{end}}</ul></main></body></html>
`))

// blog lists every post on one page — which is exactly what the task asks to
// change.
func blog(w http.ResponseWriter, _ *http.Request) {
	blogTmpl.Execute(w, posts)
}
