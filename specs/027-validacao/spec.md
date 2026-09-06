# Spec 027 — Validação declarativa no `Bind`

- **Issue**: [#27](https://github.com/emersonjoe/trilha/issues/27) (ROADMAP, Fase 2, item 8)
- **Branch**: `027-validacao`
- **Versão**: 0.18.0

## Por quê

`Bind` já converte o formulário na struct e `FieldErrors` já responde 422 com a mensagem ao
lado do campo. Falta o meio: **decidir se o valor serve**. Hoje cada app escreve isso na mão,
e dá para ver o custo dentro do próprio repositório — o `examples/orcamento` tem uma função
`Validar` de vinte linhas em que metade é "está vazio?" e "tem tamanho?", e o
`examples/blog` responde a um título vazio com um redirecionamento carregando a mensagem na
query string.

O que se repete em todo formulário é sempre o mesmo punhado de perguntas: veio? tem tamanho
mínimo? é e-mail? está na lista? confere com o outro campo? Isso cabe numa tag ao lado do
campo, onde já está o `form:"..."`. O que **não** cabe numa tag é a regra do negócio ("a
conta existe e é analítica") — essa continua em Go, e a spec precisa deixar a fronteira
visível, senão a tag vira uma linguagem de programação ruim escrita dentro de uma string.

## O que muda

```go
var in struct {
	Nome  string `form:"nome"  validate:"required,min=3"`
	Email string `form:"email" validate:"required,email"`
	Senha string `form:"senha" validate:"required,min=8"`
	Conf  string `form:"conf"  validate:"eqfield=senha"`
	Plano string `form:"plano" validate:"required,oneof=free pro"`
	Site  string `form:"site"  validate:"url"`
}
if err := c.Bind(&in); err != nil {
	// FieldErrors, como antes: 422 no JSON, mensagem ao lado do campo no HTML.
	return c.Render(422, formulario(c, in, err.(trilha.FieldErrors)))
}
```

Três coisas entram junto:

1. **Tags**, com um conjunto pequeno e fechado de regras.
2. **Validador próprio por tipo**: qualquer tipo de campo que implemente `Validate() error`
   é chamado depois da conversão — é assim que `CPF`, `CNPJ` ou `Moeda` entram sem o
   framework saber o que é um CPF. A mesma interface na struct inteira vale como verificação
   final entre campos.
3. **Regra própria por nome**: `trilha.AddRule("cep", fn)` põe `validate:"cep"` à disposição
   de todo mundo no app.

### Superfície

| Símbolo | Papel |
|---|---|
| tag `validate:"..."` | regras separadas por vírgula, parâmetro depois de `=` |
| `trilha.Validator` | `interface{ Validate() error }`, num tipo de campo ou na struct |
| `trilha.AddRule(name string, fn func(Field) bool)` | registra uma regra nova (pânico se o nome já existe) |
| `trilha.Field` | o que a regra recebe: `Name`, `Param`, `Text`, `Value`, `Other(name)` |
| `trilha.ValidationMessages` | `map[string]string` de mensagem por regra, em inglês |
| `trilha.UseValidationPTBR()` | troca as mensagens (e o `BindInvalid`) para português |

### Regras

| Regra | Vale para | Falha quando |
|---|---|---|
| `required` | tudo | o valor é o zero do tipo (string vazia depois do `TrimSpace`, slice vazia, ponteiro nulo, `time.Time` zerada, número 0, `false`) |
| `min=N` / `max=N` | número, texto, slice, data | número fora do intervalo; texto ou slice com menos/mais de N; data anterior/posterior a `N` no formato `2006-01-02` |
| `len=N` | texto, slice | tamanho diferente de N |
| `email` | texto | não tem a forma `algo@algo.tld` |
| `url` | texto | não tem esquema `http`/`https` e host |
| `oneof=a b c` | texto, número | não está na lista (separada por espaço) |
| `eqfield=nome` | tudo | o texto do outro campo é diferente |

Toda regra menos `required` **ignora o valor vazio**: um campo opcional só é checado quando
foi preenchido. É o que faz `validate:"email"` (sem `required`) significar "se mandar, mande
um e-mail válido".

## Decisões

- **A tag não vira linguagem.** Sete regras, nenhuma condicional, nenhum `dive`, nenhum
  `omitempty` — quem precisa de "obrigatório só quando o outro campo for X" escreve em Go, no
  `Validate()` da struct, onde dá para ler. A tag serve ao que é sempre igual.
- **`required` é o zero do tipo**, não "veio no formulário". Um `int` com `required` recusa
  o `0`; se `0` é resposta legítima, o campo é `*int` — que é a mesma resposta que o `Bind`
  já dá para "não veio". É a única semântica que não obriga a carregar o formulário cru até
  aqui, e vale igual no JSON.
- **Erro de conversão cancela a regra daquele campo.** Um `idade=abc` vira `BindInvalid` e
  para ali: mandar "precisa ser 18 ou mais" para quem digitou letras é ruído.
- **`Validate()` da struct só roda quando não há erro de campo.** Verificação entre campos
  pressupõe campos válidos; rodar antes disso é convite a `nil` e a mensagem duplicada.
- **A validação vale no JSON também.** `BindJSON` passa pelo mesmo caminho — o formulário e
  a API de uma rota `KindAuto` compartilham a struct, e seria surpreendente que só um lado
  checasse.
- **Mensagem sem o nome do campo.** `"required"`, não `"nome is required"`: o lugar dela é ao
  lado do rótulo, e é assim que o `FieldErrors`/`ui.Errors` já funciona hoje.
- **Inglês por padrão, português numa chamada.** A regra do repositório manda inglês no
  código; a mensagem, porém, é lida pelo usuário final do app. `UseValidationPTBR()` no
  `Setup` resolve, e `ValidationMessages` continua um mapa para quem fala outra língua.

## Requisitos

- **FR-001** `Bind` aplica as regras da tag `validate` depois de converter todos os campos, e
  devolve `FieldErrors` com uma mensagem por campo (a primeira que falhar).
- **FR-002** As sete regras da tabela funcionam nos tipos listados; regra desconhecida é
  pânico no primeiro uso (erro de programação, não de usuário).
- **FR-003** Toda regra além de `required` ignora valor vazio.
- **FR-004** Campo que falhou na conversão recebe `BindInvalid` e não recebe mensagem de
  regra.
- **FR-005** Tipo de campo que implementa `Validator` é chamado depois da conversão; o texto
  do erro vira a mensagem daquele campo.
- **FR-006** Struct que implementa `Validator` é chamada no fim, só quando nenhum campo
  falhou; `FieldErrors` devolvida por ela é fundida no resultado, outro erro sobe como está.
- **FR-007** `AddRule` registra uma regra usável na tag; nome repetido é pânico.
- **FR-008** Struct aninhada (com ou sem prefixo) é validada com o mesmo nome de campo que o
  `Bind` já usa, para a mensagem cair no `name` certo do formulário.
- **FR-009** `BindJSON` valida igual ao `Bind`.
- **FR-010** `UseValidationPTBR()` troca todas as mensagens, inclusive `BindInvalid`.
- **FR-011** Zero dependência externa; struct sem tag `validate` não muda de comportamento.

## Fora de escopo

- **Validação no navegador.** O `ui` já tem `Invalid`/`Errors` e o HTML já tem `required`,
  `minlength` e `type=email`. Gerar atributo a partir da tag parece de graça e não é: exige
  o formulário conhecer a struct, e a checagem que importa é sempre a do servidor.
- **`dive` em slice de struct, mapas, structs recursivas.** Formulário HTML não manda isso
  sem uma convenção de nome (`itens[0].preco`) que esta spec não cria.
- **i18n de verdade** (arquivo de tradução, plural, `Accept-Language`). Duas línguas num
  mapa resolvem o caso real; biblioteca de i18n é dependência.
- **Mensagem por campo na tag** (`msg:"Informe o CPF"`). O app já pode reescrever a
  `FieldErrors` antes de renderizar, e a tag ficaria com duas responsabilidades.
- **CPF, CNPJ, CEP, telefone no núcleo.** São regras de um país; entram pelo `AddRule` ou por
  um tipo com `Validate()`, e o `examples/orcamento` mostra como.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — SSR primeiro | a checagem é do servidor; o HTML não muda |
| II — só biblioteca padrão | `reflect`, `strings`, `strconv`, `net/url`, `time` |
| III — coerência com Go | tag ao lado do `form:`, interface de uma função, erro como valor |
| IV — convenção nova tem uso no exemplo e teste de integração | `examples/blog` (formulário de post) e `examples/orcamento` (`Validar` encolhe para o que é regra de negócio) |
| VI — teste primeiro | `validate_test.go` vermelho antes de `validate.go` |
| VII — compatibilidade | struct sem a tag se comporta como antes; `BindInvalid` continua público e usado |

## Aceitação

- **SC-001** Uma struct com `required,min=3,email,oneof,eqfield` devolve as mensagens certas
  em um só `Bind`, uma por campo.
- **SC-002** Campo opcional vazio passa; o mesmo campo preenchido errado falha.
- **SC-003** Um tipo com `Validate() error` recusa o valor e a mensagem cai no campo dele.
- **SC-004** `AddRule("cep", ...)` passa a valer na tag, no app inteiro.
- **SC-005** No `examples/blog`, publicar sem título devolve 422 com a mensagem ao lado do
  campo (em vez do redirecionamento com a mensagem na URL de hoje).
- **SC-006** No `examples/orcamento`, `plano.Validar` fica só com a regra de negócio, e o
  formulário continua respondendo 422 com as mesmas mensagens em português.
