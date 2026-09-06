# Spec 036 — `trilha generate page|route|component`

- **Issue**: [#36](https://github.com/emersonjoe/trilha/issues/36) (ROADMAP, item 18)
- **Branch**: `036-generate`
- **Versão**: 0.27.0

## Por quê

Criar uma página hoje é copiar uma pasta e apagar o que sobrou. O custo não é digitar o
esqueleto — é lembrar da convenção na hora de nomear a pasta. `{slug}` vira `slug_`,
`{path...}` vira `path__`, grupo termina em `-`, e o nome do pacote não é o nome da pasta
quando a pasta tem ponto. Quem erra descobre pelo scanner, com um código de erro, depois de
já ter escrito o handler.

O gerador troca essa ordem: você diz a URL, ele escreve a pasta certa. É o mesmo movimento do
`trilha new` — o valor não é o arquivo, é a convenção ensinada no momento em que ela importa.

## O que muda

### `trilha generate page <url>`

```console
$ trilha generate page /blog/{slug}
  criado  app/blog/slug_/page.go   GET /blog/{slug}
  trilha_gen.go atualizado (12 rotas)
```

O argumento é a **URL**, não o caminho da pasta. A tradução é a convenção:

| No comando | Vira a pasta | Na URL |
|---|---|---|
| `eventos` | `eventos` | `/eventos` |
| `{slug}` | `slug_` | `{slug}`, lido com `c.Param("slug")` |
| `{path...}` | `path__` | `{path...}`, folha obrigatória |
| `marketing-` | `marketing-` | nada: é grupo de rotas |
| `relatorio.csv` | `relatorio.csv` | `/relatorio.csv` |

O esqueleto é o mínimo que compila e roda: `Page` com `c.SetTitle`, e uma linha lendo cada
parâmetro dinâmico do caminho, porque é para isso que a pasta dinâmica existe.

### `trilha generate route <url>`

O mesmo, escrevendo `route.go` com `GET` respondendo `c.JSON`, mais os `c.Param` do caminho.

### `trilha generate component <Nome>`

Escreve `internal/components/<nome>.go` com `package components` e uma função que devolve
`h.Node` — o formato que os exemplos já usam para o que não é rota. `--dir` escolhe outro
lugar.

### Regras comuns

- **Não sobrescreve.** Arquivo existente faz o comando falhar dizendo qual, e `--force`
  autoriza.
- **`page.go` e `route.go` não convivem** na mesma pasta (é `E_PAGE_AND_ROUTE` no scanner):
  o comando recusa antes de escrever, dizendo qual dos dois já está lá.
- **O nome do pacote não é chutado.** Se a pasta já tem `.go`, o novo arquivo usa o pacote
  declarado neles; senão, deriva do nome da pasta como as convenções mandam (`slug_` → `slug`,
  `relatorio.csv` → `relatoriocsv`, raiz → `app`), com sufixo `_` se der numa palavra
  reservada.
- **Depois de escrever, regenera.** Rota nova sem `trilha gen` é um 404 que ninguém explica;
  o comando roda a geração e diz quantas rotas ficaram.
- Segmento inválido (`{1x}`, `{a}-`, `..`, vazio) falha com a mesma mensagem que o scanner
  daria, antes de criar pasta alguma.

## Fora de escopo

- Convenção nova em `app/`: o gerador só escreve o que o scanner já entende, e por isso não
  mexe em `internal/scan`.
- `layout.go`, `middleware.go`, `setup.go`, `not_found.go` — arquivos de raiz ou de subárvore,
  que se escreve uma vez por projeto e o `trilha new` já entrega.
- Editar arquivo existente (acrescentar `POST` a um `route.go` que já existe).
- Templates configuráveis pelo usuário.

## Constitution Check

| Princípio | Como esta spec o respeita |
|---|---|
| I — convenção sobre configuração | O comando não inventa convenção: ele materializa a que já existe, e a tabela acima é a mesma da referência. |
| II — só biblioteca padrão | `text/template` e `go/token` para validar identificador. |
| VI — teste primeiro | Unitário em `internal/scaffold` para cada tradução de URL, e2e da CLI por subcomando. |

## Aceitação

- **SC-001** `trilha generate page /blog/{slug}` cria `app/blog/slug_/page.go` com
  `package slug`, `Page` compilável e a leitura de `c.Param("slug")`; a rota aparece em
  `trilha routes` sem passo extra.
- **SC-002** `trilha generate route /api/relatorio.csv` cria
  `app/api/relatorio.csv/route.go` com `package relatoriocsv` e `GET`.
- **SC-003** `trilha generate component Aviso` cria `internal/components/aviso.go` com
  `package components` e `func Aviso(...h.Node) h.Node`.
- **SC-004** Repetir qualquer um dos três falha citando o arquivo; com `--force`, sobrescreve.
- **SC-005** `trilha generate page /api/hello` (pasta que já tem `route.go`) falha explicando
  o conflito, e não escreve nada.
- **SC-006** As mensagens existem nas duas línguas, e a linha do comando entra no `usage`.
