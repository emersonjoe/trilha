// Package config is what an administrator changes without a deploy: the model
// the assistant answers with, how creative it is, and how long an answer may
// get. The struct is the configuration and the screen comes from it.
package config

import "github.com/emersonjoe/trilha"

// Assistente is the section. The tags carry three jobs at once: json says how
// it is stored, form says what the input is called, and validate is the rule —
// the same rule on the screen and on anything else that saves it.
type Assistente struct {
	Modelo      string  `json:"modelo"      form:"modelo"      validate:"required,max=60" label:"Modelo" help:"o nome que o provedor conhece, ex. gpt-4o-mini ou llama3.1"`
	Temperatura float64 `json:"temperatura" form:"temperatura" validate:"min=0,max=2" label:"Temperatura" help:"0 responde igual sempre; 2 inventa"`
	MaxTokens   int     `json:"max_tokens"  form:"max_tokens"  validate:"min=64,max=4096" label:"Tamanho máximo da resposta"`
	Ferramentas bool    `json:"ferramentas" form:"ferramentas" label:"Deixar o assistente usar as ferramentas"`
	// A chave do provedor é o caso do trilha.Secret: não pode ficar em claro
	// no banco, não pode voltar inteira para a tela, e não pode aparecer num
	// log por descuido. Vazia, o cliente segue com o que veio do ambiente.
	Chave trilha.Secret `json:"chave" form:"chave" label:"Chave do provedor"`
}

// Cfg is the section itself, with the defaults that answer before anybody has
// saved anything. It is a package var because configuration is not built per
// request — it is read on every one.
var Cfg = trilha.NewSettings("assistente", Assistente{
	Temperatura: 0.7,
	MaxTokens:   1024,
	Ferramentas: true,
})
