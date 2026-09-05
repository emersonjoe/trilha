---
title: Exemplos
description: Apps completos em examples/, do básico ao complexo, e o que cada um ensina.
---

Os exemplos são apps de verdade, com testes de integração que rodam no `make test` do
repositório. Cada um tem um `README.md` curto. Rode qualquer um com `trilha dev` dentro da
pasta (ou `go run ../../cmd/trilha dev` a partir do clone).

| Nível | Pasta | O que ensina |
|---|---|---|
| Básico | `examples/blog` | todas as convenções de arquivo, layouts aninhados, grupos de rota, API JSON, middleware, sessão assinada, `tmpl` |
| Médio | `examples/cadastro` | formulário com regras: campos condicionais, validação no servidor com erros por campo, seleção dependente, aviso que some, layout responsivo |
| Complexo | `examples/orcamento` | domínio em árvore (plano de contas), agregação, drill-down por rota dinâmica, componentes aninhados e recursivos, diálogo com formulário, filtro por período, CSV |
| SSO | `examples/sso` | login OpenID Connect com Entra ID ou Keycloak, área protegida, papel exigido, logout federado |
| IA | `examples/assistente` | chat em streaming, agente com ferramentas, handoff, servidor MCP |

## Médio: cadastro

O modelo do formulário é uma struct com tags `form`; `c.Bind(&in)` a preenche (structs
aninhadas são achatadas, com prefixo opcional):

```go
type Cliente struct {
	Tipo     string   `form:"tipo"`
	Nome     string   `form:"nome"`
	Endereco Endereco            // cep, rua, uf, cidade
	Cobranca Endereco `form:"cob_"` // cob_cep, cob_rua...
	Novidades bool    `form:"novidades"`
}
```

A validação é uma função pura que devolve `trilha.FieldErrors`, e o `POST` decide:

```go
func POST(c *trilha.Ctx) error {
	var in clientes.Cliente
	if err := c.Bind(&in); err != nil {
		return err                       // conversão inválida → 422
	}
	clientes.Normalizar(&in)             // ignora o que o tipo não usa
	if errs := clientes.Validar(in); errs.Any() {
		return c.Render(422, tela(c, in, errs)) // mesma página, com layouts
	}
	clientes.Salvar(in)
	return c.Redirect("/?ok=1")          // PRG + aviso que some
}
```

Na tela, cada campo lê o valor e o erro do mesmo lugar:

```go
ui.Field("cnpj", "CNPJ",
	ui.Input(h.ID("cnpj"), h.Name("cnpj"), h.Value(in.CNPJ), ui.InvalidIf(errs, "cnpj")),
	ui.Errors(errs, "cnpj"))
```

Os grupos condicionais usam `ui.ShowWhen("tipo", "pj")`: escondidos ficam desabilitados e
não vão no `POST`; e como alguém pode montar o `POST` à mão, `Normalizar` zera o que o tipo
não usa antes de validar. O `<select>` de cidade é preenchido por `GET /api/cidades?uf=` com
20 linhas de `app.js`; no 422 o servidor já devolve as cidades da UF escolhida, então a
página volta completa sem JavaScript.

## Complexo: orçamento

O plano de contas é uma árvore (`Conta{Codigo, Nome, Filhos}`); orçado e realizado de uma
conta sintética são a soma das filhas, calculados na leitura. Os componentes espelham a
árvore: `Linha` renderiza a conta e chama a si mesma para as filhas, `ui.Depth(n)`
indenta:

```go
func Linha(c *plano.Conta, mes string, nivel, max int) h.Node {
	row := h.Tr(ui.Depth(nivel), h.Td(h.A(h.Href("/contas/"+c.Codigo), h.Text(c.Nome))), ...)
	if nivel >= max || c.Analitica() {
		return row
	}
	return h.Fragment(row, h.Map(c.Filhos, func(f *plano.Conta) h.Node {
		return Linha(f, mes, nivel+1, max)
	}))
}
```

O drill-down é a rota `app/contas/codigo_/page.go`: breadcrumb com `Caminho()`, filhas
(mesma `Tabela`) ou lançamentos (conta analítica). O formulário de lançamento é **um só**
(`FormLancamento`), usado dentro de `ui.Dialog` na visão geral e no drill-down, e sozinho em
`/lancamentos`; o `POST` valida com `c.Bind` + `plano.Validar` e, no 422, `app.js` reabre o
diálogo porque encontrou `.ui-field-error` dentro dele. `voltar` (campo oculto) diz para onde
redirecionar no sucesso. A exportação fica em `app/api/relatorio.csv/route.go`, uma pasta com
ponto no nome.

## SSO: Entra ID e Keycloak

O `examples/sso` é o fluxo de login inteiro em três rotas de duas linhas cada. O pacote
`auth` cuida de PKCE, `state`, `nonce`, troca do código e validação do `id_token`; o app só
encaminha:

```go
// app/entrar/route.go
var Kind = trilha.KindPage
func GET(c *trilha.Ctx) error { return sso.Start(c) }
```

Proteger uma subárvore é um `middleware.go`, como qualquer outro:

```go
// app/painel/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.Require(c, next) }

// app/painel/relatorio/middleware.go — papel, não só sessão
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.RequireAdmin(c, next) }
```

Abaixo do middleware, a página lê `sso.User(c)` sem checar nada. Um navegador anônimo é
mandado para `/entrar?next=…`; uma chamada de `/api` recebe 401 em JSON, porque redirecionar
um cliente HTTP para um formulário só produz um erro de parsing confuso.

Nenhum segredo mora no código: o provedor vem de variáveis de ambiente, e sem elas o app
sobe assim mesmo e diz o que falta.

## O que virou framework

Escrever os dois exemplos mostrou repetição que agora é API: `c.Bind`, `trilha.FieldErrors`,
`c.Render` (página com layouts a partir de um `POST`), `ui.Errors`, `ui.InvalidIf`,
`ui.SelectOptions`, `ui.Checked`. É o critério da constituição: um exemplo que precisa de
código repetitivo indica uma lacuna no Trilha, não no exemplo.

## Desafio

No orçamento, adicione uma coluna "Ano" ao drill-down que some os doze meses da conta.

:::solucao
```go
func Ano(c *plano.Conta, ano string) (orcado, real int64) {
	for m := 1; m <= 12; m++ {
		mes := fmt.Sprintf("%s-%02d", ano, m)
		orcado += plano.Orcado(c, mes)
		real += plano.Realizado(c, mes)
	}
	return
}
```
Chame-o em `Linha` e acrescente as duas células; como a agregação é recursiva, a coluna
já funciona para contas sintéticas.
:::
