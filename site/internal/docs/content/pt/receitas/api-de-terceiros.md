---
title: Falar com uma API de terceiros
description: Uma conexão que alguém cadastra numa tela — URL, token, Testar — e o cliente que o código pede, que leva a credencial e só fala com aquele host.
---

O token do serviço fiscal está numa variável de ambiente, a URL do servidor MCP em outra, e no
dia em que uma delas muda alguém faz deploy de novo. A alternativa é a tela que toda aplicação de
gestão acaba tendo: a lista das coisas de fora com que esta fala, com o segredo selado e um botão
que diz se ainda funciona.

```
trilha add connections
```

## 1. Os tipos

A receita escreve o `internal/conexoes` com dois tipos. Cada um diz quais autenticações aceita e
como é testado:

```go
var Tipos = []trilha.ConnectionKind{
	{Key: "api", Label: "{{.T.conn_api}}", Auth: []string{"none", "bearer", "header", "basic"},
		// Any answer below 400 passes: the point is "does the credential
		// open the door", not what is behind it.
		Test: trilha.TestHTTP("GET", "/")},
	{Key: "mcp", Label: "{{.T.conn_mcp}}", Auth: []string{"none", "bearer", "header"},
		Test: testarMCP},
}
```

(`{{.T.…}}` é a tabela de palavras da receita; no seu projeto já vem como "APIs" e "Servidores
MCP".)

O teste do MCP é o handshake e a lista de ferramentas, pelo cliente da própria conexão, para o
token ir junto sem ser copiado para um mapa de cabeçalhos:

```go
func testarMCP(ctx context.Context, conn trilha.Connection) error {
	cli, err := mcp.Dial(ctx, mcp.HTTPWith(conn.URL, conn.Client(0), nil))
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.ListTools(ctx)
	return err
}
```

## 2. A tela

`/conexoes` é o [`ui.ConnectionsPanel`](/pt/referencia/ui#connectionspanel): a lista agrupada por
tipo com o badge do último teste, e o formulário. Salvar, Testar e Apagar são três `POST`s para o
mesmo caminho com um `_action` oculto, e a página os lê assim:

```go
	case "test":
		res, err := conns.Test(c, c.Form("id"))
		if err != nil {
			return err
		}
		if res.OK {
			c.Flash(ui.FlashSuccess, "{{.T.conn_test_ok}}")
		} else {
			c.Flash(ui.FlashError, "{{.T.conn_test_failed}} " + res.Message)
		}
```

O segredo é digitado uma vez. Na edição o campo volta vazio com "deixe em branco para manter", e
a trilha de auditoria (`connection.save`, `connection.test`) diz quem mudou o quê sem nunca levar
o valor. Guarde a pasta: quem chega nela pode apontar a sua aplicação para um servidor próprio.

## 3. Usar

Onde a aplicação chama o serviço, ela pede o cliente em vez de ler uma variável:

```go
func Chamar(c *trilha.Ctx, id, path string) (*http.Response, error) {
	conn, err := Conexoes.Get(c, id)
	if err != nil {
		return nil, err
	}
	cli, err := Conexoes.Client(c, id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, conn.URL+path, nil)
	if err != nil {
		return nil, err
	}
	return cli.Do(req)
}
```

O cliente já tem o `Authorization` (ou o cabeçalho nomeado, ou o par basic), os cabeçalhos fixos
e um timeout — e **só fala com o host da conexão**. Uma requisição montada para outro domínio, ou
um redirect que aponte para lá, é recusada antes de qualquer byte sair: o token não consegue
seguir um `Location` que outra pessoa controla.

## No que reparar

- **Em produção a URL tem de ser pública.** `http://127.0.0.1` e nomes `.internal` são recusados
  pelo `Save` com a mensagem no campo; em `dev` passam com aviso, porque é lá que o serviço em
  teste mora.
- **O store é memória** na receita: as conexões duram o processo. Uma tabela é uma linha por
  conexão, `headers` em JSON e `secret` selado pelo `Secret.Value()` — a
  [referência](/pt/referencia/conexoes#o-store) tem a interface.
- **Tipo sem `Test` não tem botão.** Escreva o teste no dia em que souber o que "funciona"
  significa para aquele serviço; um verde falso é pior que badge nenhum.
