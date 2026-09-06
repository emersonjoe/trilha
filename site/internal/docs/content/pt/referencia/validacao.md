---
title: Validação
description: A tag validate, as regras por tipo, as suas próprias regras e as mensagens que o Bind devolve.
---

O `Bind` valida enquanto preenche: depois de converter os valores, aplica a tag `validate`
de cada campo e devolve `FieldErrors` (campo → mensagem) com tudo o que falhou. As mesmas
regras valem para formulário e para JSON — com corpo JSON o campo é nomeado pela tag `json`,
que é o nome que o cliente reconhece.

```go
type entrada struct {
	Nome      string    `form:"nome" validate:"required,min=3,max=80"`
	Email     string    `form:"email" validate:"required,email"`
	Confirma  string    `form:"confirma" validate:"eqfield=email"`
	Data      time.Time `form:"data" validate:"required,min=2026-01-01"`
	Plano     string    `form:"plano" validate:"oneof=gratis pro"`
	Desconto  *int      `form:"desconto" validate:"required,min=0"`
}
```

## Regras

| Regra | Texto | Número | `time.Time` | `[]string` (checkbox, select) |
|---|---|---|---|---|
| `required` | não vazio | qualquer valor; `0` só por ponteiro | não é a data zero | ao menos um |
| `min=n` | ao menos `n` caracteres | valor `>= n` | data não é antes de `n` (`2006-01-02`) | ao menos `n` escolhidos |
| `max=n` | no máximo `n` caracteres | valor `<= n` | data não é depois de `n` | no máximo `n` escolhidos |
| `len=n` | exatamente `n` caracteres | — | — | exatamente `n` escolhidos |
| `email` | um `@`, domínio com ponto | — | — | — |
| `url` | `http`/`https` absoluta | — | — | — |
| `oneof=a b c` | valor é uma das opções, separadas por espaço | igual, como texto | — | — |
| `eqfield=outro` | igual ao valor do outro campo, pelo nome de formulário | igual | igual | — |

As regras são separadas por vírgula e aplicadas em ordem; a primeira que falha é a mensagem
daquele campo. Toda regra além de `required` ignora valor vazio, então campo opcional só
responde pelo que alguém digitou. Valor que nem converte (`abc` num `int`) recebe
`trilha.BindInvalid` e nenhuma mensagem de regra — uma mensagem por campo.

**`required` é o valor zero**: `0`, `false`, `""` e a data zero não passam. Onde zero é
resposta de verdade, declare o campo como ponteiro: um `*int` que chegou com `0` está
presente, e só o campo ausente falha.

## Regras suas

| Símbolo | Papel |
|---|---|
| `trilha.Validator` | `interface{ Validate() error }`: o valor se confere |
| `trilha.AddRule(nome, func(Field) bool)` | registra um nome para a tag; nome repetido entra em pânico |
| `trilha.Field` | o que a regra vê: `Name`, `Param`, `Text`, `Value`, `Other(nome)` |
| `trilha.ValidationMessages` | `map[string]string` das mensagens; `{param}` é substituído |
| `trilha.UseValidationPTBR()` | troca as mensagens, `BindInvalid` incluído, para português |

Campo cujo **tipo** tem `Validate() error` é conferido depois de as regras da tag passarem, e
a mensagem do erro vai para `FieldErrors` como está (receptor por valor ou por ponteiro, os
dois funcionam). A **struct** também pode ter `Validate() error`: roda no fim, só quando
nenhum campo falhou — é o que torna segura uma conferência que lê dois campos. Ela pode
devolver `FieldErrors` para dizer de quem é a culpa; qualquer outro erro volta do `Bind`
intacto.

```go
trilha.AddRule("cep", func(f trilha.Field) bool { return cepValido(f.Text) })
trilha.ValidationMessages["cep"] = "CEP inválido"
```

`Field.Value` é o valor convertido (`string`, `bool`, `int64`, `float64`, `time.Time`,
`[]string`, ou `nil` quando o campo não veio) e `Field.Text` é a mesma coisa como texto, que
é tudo de que a maioria das regras precisa. Regra que compara campos lê o outro com
`f.Other("email")`.

## Onde a validação para

A tag diz o que um **valor** aceita. Se a conta existe, se a sala está livre nessa noite, se
essa pessoa pode fazer isso — essas leem os seus dados e são do seu pacote. Rode depois do
`Bind` e junte no mesmo `FieldErrors`, para todas as mensagens chegarem numa resposta só:

```go
errs := trilha.FieldErrors{}
if err := c.Bind(&in); err != nil {
	fe, ok := err.(trilha.FieldErrors)
	if !ok {
		return err
	}
	errs = fe
}
for campo, msg := range plano.Validar(&in) {
	errs.Add(campo, msg)
}
if errs.Any() {
	return c.Render(http.StatusUnprocessableEntity, pagina(c, in, errs))
}
```

Nome de regra que ninguém registrou entra em pânico na primeira requisição que passa pelo
campo, de propósito: um erro de digitação na tag seria, senão, um formulário que aceita
qualquer coisa em produção.
