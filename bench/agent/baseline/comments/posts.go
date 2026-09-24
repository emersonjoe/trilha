package main

// post is one row of the blog. The store is memory here, which is the honest
// starting point: the rows last as long as the process.
type post struct {
	ID, Title, Body string
}

var posts = []post{
	{"ola-trilha", "Olá, Trilha", "O primeiro post deste blog."},
	{"segundo-post", "Segundo post", "Mais um pouco de conteúdo."},
	{"terceiro-post", "Terceiro post", "E o último por enquanto."},
}

// postByID finds one post, or nil.
func postByID(id string) *post {
	for i := range posts {
		if posts[i].ID == id {
			return &posts[i]
		}
	}
	return nil
}
