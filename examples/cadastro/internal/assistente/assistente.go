// Package assistente is the model of the three-screen form: one struct per
// step, and a draft that carries what the earlier steps already validated.
//
// One struct per step is the point. A single struct with every field and every
// validate tag cannot be checked halfway: step one would fail on the address
// nobody has typed yet, and the way out people find is to drop the rules
// until the last screen — where a message about a field three screens back is
// useless.
package assistente

// Passo1 is who is being registered.
type Passo1 struct {
	Nome  string `form:"nome"  validate:"required,max=80"`
	Email string `form:"email" validate:"required,email"`
}

// Passo2 is where they are.
type Passo2 struct {
	CEP    string `form:"cep"    validate:"required,len=8"`
	Cidade string `form:"cidade" validate:"required,max=60"`
}

// Rascunho is what the framework keeps between one screen and the next. It is
// small on purpose: a draft travels in the person's own cookie, signed but not
// secret, so what goes in it is what they typed and nothing else.
type Rascunho struct {
	Dados    Passo1 `json:"dados"`
	Endereco Passo2 `json:"endereco"`
}

// Nome é o nome do rascunho, não da pessoa: é a chave do c.Draft, e vale a
// pena existir uma vez só para as três telas não escreverem a mesma string.
const Nome = "cadastro"
