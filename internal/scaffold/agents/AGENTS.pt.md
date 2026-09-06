# AGENTS.md

Instruções para agentes de código que trabalham em `{{.Name}}`, uma aplicação web feita com
[Trilha](https://github.com/emersonjoe/trilha) — um framework Go com roteamento por arquivos e
sem dependências externas.

## As três convenções

- **Uma pasta dentro de `app/` é uma URL.** `app/blog/page.go` responde `/blog`.
- **O nome do arquivo diz o que ele é.** `page.go` renderiza uma página (`func Page(c
  *trilha.Ctx) (h.Node, error)`), `route.go` é uma API (`func GET`, `func POST`, ...),
  `layout.go` envolve tudo que está abaixo dele, `middleware.go` roda antes de tudo que está
  abaixo dele.
- **Uma pasta chamada `slug_` é um parâmetro.** `app/blog/slug_/page.go` responde
  `/blog/{slug}`, lido com `c.Param("slug")`. Já uma pasta com ponto no nome é caminho fixo
  (`app/api/relatorio.csv/route.go` responde `/api/relatorio.csv`).

O HTML é escrito em Go com o pacote `h`, não com template:
`h.Div(h.Class("card"), h.H1(nil, h.Text(titulo)))`. Tudo que ele renderiza sai escapado.

## Comandos

| Comando | O que faz |
|---|---|
| `trilha dev` | servidor de desenvolvimento com recarga; deixe rodando enquanto trabalha |
| `trilha gen` | regrava `trilha_gen.go` a partir de `app/`; rode depois de criar ou remover rota |
| `trilha gen --check` | falha se `trilha_gen.go` estiver desatualizado — é o que a CI roda |
| `trilha generate page /caminho` | grava o esqueleto na pasta certa (também `route`, `component`) |
| `trilha routes` | lista as rotas encontradas e o arquivo de onde vieram |
| `trilha audit` | verifica segurança e configuração: segredos, CSP, cookies, dependências |
| `trilha build` | gera e compila um binário único |
| `trilha export` | grava as páginas estáticas em HTML |
| `trilha openapi` | escreve o documento OpenAPI das rotas de API |
| `trilha ui` | regrava o kit ui em `public/` |
| `trilha agents` | regrava este arquivo |
| `trilha new` | cria outro projeto |
| `trilha version` | a versão do framework |
| `make test` | a suíte do próprio projeto; rode antes de dizer que terminou |

Rota que responde 404 quase sempre é `trilha gen` que faltou.

## O que não fazer

- **Não edite `trilha_gen.go`.** Ele é gerado e commitado, e o próximo `trilha gen` sobrescreve
  o que você escrever ali. Mexa em `app/`.
- **Não acrescente dependência.** O framework roda só com a biblioteca padrão; a resposta
  costuma estar em `net/http`, `database/sql` ou no próprio framework.
- **Não ponha segredo no código.** Leia do ambiente. O `trilha audit` falha em literal com cara
  de chave.
- **Não escreva seu próprio CSRF, assinatura de sessão ou escape de HTML.** Os três já existem e
  já vêm ligados.

## Onde procurar

- Receitas para os problemas de sempre (banco, sessões, uploads, paginação, email, Docker):
  <https://emersonjoe.github.io/trilha/pt/receitas>
- Cada função e cada tipo: <https://emersonjoe.github.io/trilha/pt/referencia>
- A documentação inteira em texto puro, mais barata de ler de uma vez:
  <https://emersonjoe.github.io/trilha/pt/llms.txt>
