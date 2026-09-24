package main

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"net/http"
	"strings"

	"example.com/s8-conserto-base/contas"
)

// registros is the app's one table: every request sees the same rows.
var registros contas.Store

var (
	formTmpl = template.Must(template.New("form").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Contas</title></head>
<body><main><h1>Contas</h1>
<form method="post" action="/contas">
<input type="hidden" name="_csrf" value="{{.Token}}">
<label for="nome">Nome</label><input id="nome" name="nome" value="{{.Nome}}">
<label for="email">E-mail</label><input id="email" name="email" type="email" value="{{.Email}}">
<button type="submit">Cadastrar</button>
</form>
<h2>Cadastradas</h2><ul>{{range .Linhas}}<li>{{.Nome}} — {{.Email}}</li>{{end}}</ul>
</main></body></html>
`))
	homeTmpl = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Início</title></head>
<body><main><h1>Início</h1><p>Um app vazio: a primeira feature começa daqui.</p></main></body></html>
`))
)

func home(w http.ResponseWriter, _ *http.Request) {
	homeTmpl.Execute(w, nil)
}

// form renders the register: the form with this visit's token in it, and the
// list of what is already there.
func form(w http.ResponseWriter, r *http.Request) {
	token := novoToken()
	http.SetCookie(w, &http.Cookie{Name: "csrf", Value: token, Path: "/"})
	formTmpl.Execute(w, struct {
		Token, Nome, Email string
		Linhas             []contas.Conta
	}{Token: token, Linhas: registros.Todas()})
}

// salvar handles the POST. What is missing here is the point of the scenario:
// the token the form carries is never checked, and an empty name is stored as
// any other.
func salvar(w http.ResponseWriter, r *http.Request) {
	nome := strings.TrimSpace(r.FormValue("nome"))
	email := strings.TrimSpace(r.FormValue("email"))
	// TODO(csrf): verificar o token do formulário contra o cookie antes de
	// gravar qualquer coisa.
	// TODO(validacao): recusar nome vazio com 422 em vez de gravar.
	registros.Nova(contas.Conta{Nome: nome, Email: email})
	http.Redirect(w, r, "/contas", http.StatusSeeOther)
}

// novoToken mints the per-visit token the form carries back.
func novoToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// tokenDoCookie is what the POST should compare the form's token against.
func tokenDoCookie(r *http.Request) string {
	ck, err := r.Cookie("csrf")
	if err != nil {
		return ""
	}
	return ck.Value
}
