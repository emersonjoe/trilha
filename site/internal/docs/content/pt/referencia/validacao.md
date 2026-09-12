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
| `enum=<nome>` | um valor de um `trilha.Enum` registrado; a mensagem lista os rótulos |
| `eqfield=outro` | igual ao valor do outro campo, pelo nome de formulário | igual | igual | — |

As regras são separadas por vírgula e aplicadas em ordem; a primeira que falha é a mensagem
daquele campo. Toda regra além de `required` ignora valor vazio, então campo opcional só
responde pelo que alguém digitou. Valor que nem converte (`abc` num `int`) recebe
`trilha.BindInvalid` e nenhuma mensagem de regra — uma mensagem por campo.

**`required` é o valor zero**: `0`, `false`, `""` e a data zero não passam. Onde zero é
resposta de verdade, declare o campo como ponteiro: um `*int` que chegou com `0` está
presente, e só o campo ausente falha.

## Listas e matrizes

Formulário cresce: três dependentes, cinco linhas de pedido, uma permissão por módulo. O
nome carrega a posição — `itens[0].nome`, `itens[1].nome` — ou a chave — `perm[docs]` — e o
`Bind` preenche uma fatia de struct ou um mapa a partir dele:

```go
type Linha struct {
	Nome string `form:"nome" validate:"required,max=40"`
	Qtd  int    `form:"qtd" validate:"min=1"`
}

var in struct {
	Itens []Linha        `form:"itens" validate:"minitems=1,maxitems=50"`
	Perm  map[string]int `form:"perm"`
}
```

| Regra | O que conta |
|---|---|
| `minitems=n` | ao menos `n` linhas ou chaves |
| `maxitems=n` | no máximo `n` linhas ou chaves |
| `lenitems=n` | exatamente `n` linhas ou chaves |

Essas três são a exceção ao "valor vazio pula a regra": "ao menos uma linha" é justamente
uma frase sobre o caso vazio.

O nome do input é também a chave da mensagem, então `ui.Errors(errs, "itens[1].qtd")` acha o
campo que a pessoa está olhando. Índice que ninguém mandou não é linha: `itens[0]` e
`itens[7]` chegam como duas linhas, nessa ordem, e a mensagem da segunda diz `itens[1].qtd`
— a posição depois de compactar, que é a posição que o formulário vai desenhar de novo. Nada
é alocado por índice, então `itens[9999999999]` custa uma linha e não dez bilhões; o teto é
`maxitems` quando a tag tem um e `trilha.MaxItems` (1000) quando não tem. Chave de mapa vale
inteira, com espaço e ponto, e chave com colchete é recusada.

O `BindJSON` fala a mesma chave: `{"itens":[…]}` erra com `itens[1].qtd`, a mesma string que
o formulário HTML produz, então uma tela só serve aos dois.

## Formulário que vem como dado

Tem formulário que não está no código: o passo de um fluxo, o formulário público atrás de um
token, a configuração de um cliente. `trilha.Schema` é esse formulário como dado, e
`trilha.BindSchema` lê pelo motor de cima — as mesmas regras, as mesmas mensagens, o mesmo
`FieldErrors`:

```go
values, err := trilha.BindSchema(c, esquema)
errs, ok := err.(trilha.FieldErrors)
if err != nil && !ok {
	return err
}
if ok {
	return c.Render(http.StatusUnprocessableEntity, pagina(c, values, errs))
}
```

Os valores voltam como texto: esquema que veio de uma tabela não tem tipo Go para preencher,
e converter para `any` só mudaria a conversão de lugar. O `SchemaField` diz o que uma tag
diria — `Required`, `Min`, `Max`, `Pattern`, `Options` (um `[]trilha.SchemaOption`, um
`{Value, Label}` por escolha de um select) — e o `Type` diz qual controle desenha, um dos
`trilha.SchemaTypes`: `text`, `textarea`, `number`, `date`, `datetime`, `select`, `checkbox`,
`file`, `signature`, `display`. Campo `display` é um parágrafo no meio do formulário: não é lido e
nunca recebe mensagem. Arquivo se lê com `c.File`, como qualquer arquivo.

Esquema com tipo desconhecido, campo sem nome ou padrão que não compila é defeito do app, e
não coisa que a pessoa fez ao preencher: o `schema.Check()`, que o `BindSchema` chama antes
de tudo, responde com erro comum e nunca com 422. O `ui.SchemaForm(esquema, values, errs)`
desenha os campos; o `<form>`, o input de CSRF e o botão continuam seus, porque para onde o
formulário posta não está no esquema.

## Enum — uma lista de domínio declarada uma vez

Um status, um tipo de documento, um estágio de pipeline. Escrito à mão, ele existe em quatro
lugares — a badge da tabela, as opções de um select, a validação do formulário e um comentário
na tag — e o quarto é onde o rótulo fica errado.

```go
// internal/docs/status.go
var Status = trilha.Enum{
	{Value: "na-fila",    Label: "Na fila"},
	{Value: "processando", Label: "Processando", Tone: "info"},
	{Value: "processado",  Label: "Processado",  Tone: "success"},
	{Value: "erro",        Label: "Erro",        Tone: "danger"},
}
```

`Tone` é um entre `muted`, `info`, `success`, `warning`, `danger` e `accent` — um nome do tema,
nunca uma classe CSS. Quem declara um status escolhe um significado; escolher uma cor é como
duas telas acabam com dois verdes diferentes. Vazio é `muted`.

| Símbolo | O que faz |
|---|---|
| `Enum.Label(v)` | o que a pessoa lê, ou o valor cru quando a lista não o conhece |
| `Enum.Tone(v)` | o tom, `muted` para valor desconhecido |
| `Enum.Has(v)` | está na lista |
| `Enum.Options(atual, placeholder…)` | a lista de `<option>`, com o atual marcado |
| `Enum.Values()`, `Enum.Labels()` | os dois, na ordem da declaração |
| `trilha.EnumValue{Value, Label, Tone}` | uma entrada da lista: o que o banco guarda, o que a pessoa lê e qual tom do tema ela veste — um `Enum` é uma fatia desses |
| `ui.Status(e, v)` | a badge: o rótulo, no tom |

### Os quatro usos

```go
ui.Status(docs.Status, doc.Status)                       // badge
{Key: "status", Cell: func(d Doc) h.Node {               // coluna do DataTable
	return ui.Status(docs.Status, d.Status)
}}
ui.Field("status", "Situação", ui.Select(docs.Status.Options(form.Status)))  // select
type Form struct {                                       // validação
	Status string `validate:"required,enum=docs.Status"`
}
```

A tag cita o enum pelo nome com que ele foi registrado:

```go
func Setup(a *trilha.App) error {
	trilha.RegisterEnum("docs.Status", docs.Status)
	return nil
}
```

Registrar a mesma lista de novo pode — `Setup` é onde isto mora, e uma suíte de testes sobe o
app uma vez por teste. Duas listas **diferentes** sob um nome só entram em pânico: o formulário
validaria contra uma e o select desenharia a outra.

A mensagem lista os rótulos, não os valores: `valor inválido; aceita Na fila, Processando,
Processado, Erro`. Quem preencheu o formulário leu rótulos.

### Valor que a lista não conhece

Renderiza cru, no tom `muted`, e o `Has` diz não. Uma linha gravada antes de alguém aposentar
aquele valor não pode derrubar a tela — mostrar `legado` em cinza é uma tela em que dá para
agir; um pânico não é.

### O que não está aqui

Tradução dos rótulos. `Label` é uma string, e um app com duas línguas passa dois enums ou monta
um a partir da própria tabela. Uma tabela de tradução aqui dentro seria uma segunda i18n, pior.

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
