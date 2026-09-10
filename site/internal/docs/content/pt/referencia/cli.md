---
title: CLI
description: Os comandos de trilha e suas opções.
---

```text
trilha new <dir> [--module caminho] [--template blog|app] [--with receitas] [--lang en|pt] [--agents]
    [--trilha-dir ../trilha] [--no-tidy]
trilha gen [--check] [--package nome]
trilha generate page|route|test <url> | component <Nome>
    [--methods GET,POST] [--bind Tipo] [--form Tipo] [--layout arquivo] [--force] [--dir caminho] [--lang en|pt]
trilha dev [--addr :3000]
trilha build [-o bin/<nome>]
trilha export [-o out] [--base /prefixo]
trilha openapi [-o arquivo] [--title T] [--version V] [--server URL] [--check]
trilha routes
trilha check [--json] [--fix]
trilha ctx [--json] [--routes|--types|--all]
trilha audit [--no-vuln]
trilha ui [--force] [--css-only|--js-only]
trilha ui describe [Nome] [--json]
trilha agents [--force] [--lang en|pt]
trilha mcp [--write]
trilha version
```

| Comando | O que faz |
|---|---|
| `new` | cria um projeto com `go.mod`, layout, página inicial, 404, uma rota de API, `public/style.css` e `.gitignore`; roda `go mod tidy` e `gen` |
| `gen` | varre `app/` e escreve `trilha_gen.go`; falha com uma linha por convenção violada |
| `generate` | grava um esqueleto — página, rota de API ou componente — na pasta que a convenção pede |
| `generate crud` | lê um struct e escreve tudo: lista, criar, editar, excluir, um store e um teste |
| `add` | escreve uma receita do framework no projeto: a trilha de auditoria, chaves de API, uma seção de configurações |
| `dev` | `gen` + `go build` + executa o app em uma porta interna + proxy em `--addr` + recarga por SSE + inspetor de rotas em `/_trilha/routes` |
| `build` | `gen` + `go build -trimpath -ldflags="-s -w"` com `CGO_ENABLED=0` |
| `export` | `gen` + `go build` + executa com `TRILHA_EXPORT` para gerar HTML estático |
| `openapi` | escreve o documento OpenAPI 3.1 das rotas de API (`-o -` na saída padrão) |
| `routes` | imprime `MÉTODOS PADRÃO ORIGEM` para cada rota |
| `check` | o portão único: `gen`, `gofmt`, `vet`, `test`, `audit` e `openapi`, nesta ordem, parando na primeira falha |
| `ctx` | o mapa do projeto — rotas, API, tipos, setup — numa leitura só, em Markdown ou JSON |
| `audit` | checklist de segurança antes de publicar (veja [Segurança](/pt/referencia/seguranca)) |
| `agents` | grava `AGENTS.md` e `CLAUDE.md` para um agente de código achar as convenções |
| `mcp` | servidor MCP por stdio, para agente sem shell; somente leitura até passar `--write` |

Os comandos rodam na pasta que contém `app/`. O caminho de import do projeto vem do
`go.mod` mais próximo, mais a subpasta, então um app pode viver dentro de um módulo maior.

## trilha new --template

O `--template` escolhe o formato do projeto:

| Formato | O que grava |
|---|---|
| `blog` (padrão) | uma home, uma rota de API e um 404: a menor coisa que roda |
| `app` | o app de gestão — login, [shell](/pt/referencia/shell), painel com [gráficos](/pt/referencia/graficos), uma [listagem](/pt/referencia/listagens) e um formulário, com testes |

O projeto `app` já nasce verde: compila, o `trilha check` passa e o `go test ./...` passa
sem nenhuma edição. A conta de exemplo aparece na própria tela de login, e toda rota
abaixo de `app/` está atrás de uma sessão porque o middleware está na raiz da pasta —
inclusive as rotas que você escrever amanhã.

### O que vem no `app`

Além do esqueleto, ele pede ao [`trilha add`](#trilha-add) as três telas que toda aplicação
interna ganha no primeiro mês, e as põe sob `app/admin/`:

| Tela | O que é |
|---|---|
| `/admin/auditoria` | a trilha de quem fez o quê, com o `ui.AuditTable` |
| `/admin/chaves` | chaves de API: emitir, revogar, e a chave mostrada uma vez |
| `/admin/config` | uma seção de configurações, desenhada do struct que a declara |

São **as receitas, e não uma segunda cópia** — uma fonte só, para o template não envelhecer
separado do que o `trilha add` escreve. O `app/admin/` exige o papel `admin`, e quem está logado
sem ele recebe **403, e não um redirecionamento para o login**: a pessoa é conhecida, só não
autorizada, e mandá-la de volta a um login que ela já passou é um laço sem saída.

O `--with` decide com quais receitas um projeto novo começa, em qualquer template:

```bash
trilha new minha-app --template app --with ""              # só o esqueleto
trilha new meu-site --with audit                           # no template blog também
trilha new minha-app --template app --with audit,settings  # escolha a sua
```

Um `--with` vazio é uma escolha, e não uma ausência: quem digita isso está pedindo o esqueleto, e
recebe o esqueleto.

## Idioma

As mensagens da CLI seguem `TRILHA_LANG`, depois `LC_ALL`, `LC_MESSAGES` e `LANG`: um
valor começando com `pt` (em qualquer caixa) seleciona português; qualquer outro, inclusive
variável indefinida, seleciona inglês. As mensagens do runtime, do scanner e do gerador (as
que acabam no seu código e nos seus logs) são sempre em inglês.

`trilha new --lang en|pt` escolhe o idioma dos textos gerados (página inicial, 404,
`<html lang>`); o padrão é o idioma da CLI.

## trilha dev

Além do proxy e da recarga, o supervisor serve o inspetor de rotas em `/_trilha/routes`: a
tabela de rotas em ordem de precedência com layouts e middlewares de cada uma, e uma caixa que
responde qual padrão atenderia um caminho. A página é do supervisor, não do app, então ela não
existe no binário que o `trilha build` produz — veja
[Dev e produção](/pt/aprender/dev-e-producao#o-inspetor-de-rotas).

## trilha build

O `-o` dá o nome do binário; sem ele, `bin/<pasta do projeto>`. No Windows a saída recebe o
`.exe` que o sistema exige para executá-la, tenha o nome vindo do `-o` ou do padrão —
`trilha build -o bin/app` escreve `bin\app.exe`, porque um arquivo chamado `app` é um que o
`exec.LookPath` recusa executar. Nome que já termina em `.exe` fica como está, e em Linux e
macOS nada muda.

## trilha generate

A convenção é o que custa lembrar: que `/blog/{slug}` mora em `app/blog/slug_/`, que uma pasta
catch-all termina em `__`, que um grupo termina em `-`. O `generate` recebe a URL e faz a
tradução:

```bash
trilha generate page /blog/{slug}     # app/blog/slug_/page.go
trilha generate route /api/itens/{id} # app/api/itens/id_/route.go
trilha generate component Aviso       # internal/components/aviso.go
```

A página e a rota saem compilando, com `c.Param` já lendo cada parâmetro, e o `trilha_gen.go`
é regerado no fim — a URL responde antes de você abrir o editor. Um componente é uma função
que devolve `h.Node`, então compõe como qualquer outra; `--dir` põe em outro lugar
(`internal/icones`, por exemplo).

O nome do pacote é o que já está declarado na pasta, quando existe; senão vem do nome da pasta
(`slug_` → `slug`, `relatorio.csv` → `relatoriocsv`, `type` → `type_`).

Um arquivo existente não é sobrescrito sem `--force`, e o `--force` não cobre a única recusa
que é convenção: uma pasta responde ou uma página ou uma rota, nunca as duas.

### O contrato, não só a pasta

Sem flags o esqueleto é genérico, e o que sobra para escrever — a struct, o `Bind`, a
validação, a resposta, o teste — é justamente onde se erra assinatura. As flags escrevem essa
parte:

```bash
trilha generate route /api/posts/{id}/comments --methods GET,POST --bind Comment
trilha generate page /contato --form Contact --layout app/layout.go
trilha generate test /api/posts
```

- `--methods` escreve um handler por método, na assinatura que o scanner lê, com o
  `c.Param("id")` já pronto para cada parâmetro do caminho.
- `--bind Tipo` faz os métodos com corpo chamarem `c.BindJSON(&in)`: devolver esse erro já é o
  422 com os campos, não sobra tratamento. Um tipo que o projeto já declara é importado de
  onde está; um que não existe nasce no pacote da rota com tags `json` e `validate` de
  exemplo. Um nome declarado em dois pacotes é recusa, e a mensagem manda escrever
  `posts.Comment`.
- `--form Tipo` numa página escreve a ida e volta inteira: `trilha.CSRFInput`, um `ui.Field`
  por campo com a mensagem ao lado, 422 com `trilha.FieldErrors` quando o `Bind` recusa e
  `POST → redirect → GET` quando ele aceita.
- `--layout <arquivo>` grava o `layout.go` que falta acima da página. Um caminho que não
  envolve a página é recusado: o scanner nunca o aplicaria, e descobrir isso custa uma ida e
  volta.
- `generate test <url>` escreve o teste ao lado da rota, no pacote dela, com um caso por
  método que o scanner encontra — e um corpo montado das tags quando dá para ler o tipo que o
  handler lê. Logo depois de gerar, o `trilha check` fica verde sem ninguém editar nada.

O `--lang en|pt` escolhe o idioma dos comentários do esqueleto; identificadores, nomes de
campo e mensagens de erro seguem em inglês.

## trilha ui

Grava ou atualiza o kit de interface em `public/`: `ui.theme.css` (só criado; é o seu
tema), `ui.css` e `ui.js` (atualizados; se editados localmente, só com `--force`).
`--css-only` e `--js-only` limitam o que é tocado. `trilha new` roda o mesmo passo. Veja
[Interface com ui](/pt/aprender/interface-com-ui).

### trilha ui describe

Imprime o catálogo do kit: sem argumento, todos os componentes agrupados e resumidos em uma
linha cada; com um nome, a assinatura, para que serve, os campos da struct de opções, um
exemplo e os símbolos que a documentação cita.

```bash
trilha ui describe            # tudo, agrupado
trilha ui describe Field      # um componente
trilha ui describe ui.field   # o mesmo: o prefixo e a caixa são opcionais
trilha ui describe --json     # o catálogo inteiro em JSON
```

O catálogo é montado a partir dos próprios comentários do kit e viaja dentro do binário,
então responde com ou sem projeto por perto e não tem como divergir do código que descreve.
Nome desconhecido sai com status diferente de zero e a lista dos nomes mais próximos. O
`--json` é para o agente que escreve a tela: uma chamada e ele sabe o que existe e como cada
coisa se chama, em vez de chutar um nome e descobrir na hora de compilar.

## trilha client

Gera o cliente Go de uma API que já existe, a partir do documento OpenAPI que essa API
publica. É o sentido contrário do `trilha openapi`, que escreve o documento das *suas* rotas.

```bash
trilha client openapi.json                       # grava internal/api/client.go
trilha client https://api.example.com/openapi.json --out internal/acervo
trilha client openapi.json --check               # falha quando o arquivo está desatualizado
```

Um arquivo só, determinístico, commitado como o `trilha_gen.go`. Dentro dele: uma struct por
esquema de `components/schemas`, com tags `json` e — onde o documento diz `required`,
`minLength`, `format: email`, `enum` — as tags `validate` da [Validação](/pt/referencia/validacao),
para o mesmo tipo ser a resposta da API e o `Bind` de um formulário. Um método por operação,
agrupado por tag: parâmetro de caminho na assinatura, parâmetros de query numa struct, corpo
JSON com o tipo do esquema, e resposta binária como o `*http.Response`, que passa em stream
para o `c.Pipe`.

Um corpo `multipart/form-data` é lido como o formulário que ele é. Um único campo binário e
mais nada continua sendo dois argumentos — `file io.Reader, filename string`. Qualquer outra
forma vira um struct tipado, para nenhum campo do formulário sumir em silêncio:

```go
_, err := c.Certificates().Upload(ctx, api.CertificatesUploadForm{
	File:  api.FilePart{Filename: "cert.pfx", Content: f}, // io.Reader: vai em stream
	Senha: "…",                                            // obrigatório, então sempre viaja
})

_, err = c.Documents().Batch(ctx, api.DocumentsBatchForm{
	Files: []api.FilePart{a, b, c}, // três partes com o mesmo nome, nesta ordem
})
```

Um `[]FilePart` vira várias partes com um nome só, na ordem do slice, que é o que um
`list[UploadFile]` do outro lado lê. Escalar opcional deixado no valor zero não viaja — a mesma
regra da query. Nada fica em memória: o corpo é um `io.Pipe`, então um arquivo maior que a
memória do processo atravessa.

`New(base, WithHeader(...), WithClient(...))` é toda a superfície do construtor: o cliente não
guarda credencial nenhuma, e o `WithHeader` roda por requisição, que é onde o token da sessão
entra. Status fora de 2xx vira um `*Error` com `Status`, `Body` e o `Detail` tirado do
`problem+json` ou do `{"detail": ...}` que uma FastAPI escreve. O arquivo gerado importa a
biblioteca padrão e mais nada — nem a Trilha — então serve também num job, num teste, num
binário que não é web.

O que ele não vai adivinhar, ele diz: `oneOf`, `anyOf` e esquema sem tipo chegam como
`json.RawMessage`, e cada um é uma linha do relatório que o comando imprime. `allOf` é achatado
numa struct só, um `$ref` que fecha ciclo vira ponteiro, e operação sem `operationId` ganha o
nome do método e do caminho — também uma linha do relatório.

A receita [Um app na frente de uma API que já existe](/pt/receitas/api-existente) tem as duas
metades lado a lado: a página lendo a API por este cliente, as ilhas lendo pelo
`Config.Upstreams`, um token de sessão para as duas.

## trilha vendor

Põe um módulo JavaScript no repositório. A ilha que precisa de um ajudante — uma biblioteca de
template, um gráfico, o runtime de um componente — recebe ele como arquivo em `public/vendor/`,
baixado uma vez e fixado:

```bash
trilha vendor preact@10.19.3           # public/vendor/preact.js + uma linha no vendor.lock
trilha vendor htm@3.1.1
trilha vendor                          # o que está fixado
trilha vendor --check                  # confere o hash dos arquivos com o vendor.lock
trilha vendor preact@10.19.3 --from https://cdn.exemplo.com
```

Ele baixa exatamente o que foi pedido e não resolve nada: não há árvore de dependências, não há
`node_modules`, não há passo de instalação, e um módulo que precisa de resolvedor é o módulo
errado para uma ilha. O `vendor.lock` guarda nome, versão, sha256 e URL, e é commitado — então
subir de versão é um diff, e arquivo que mudou sob uma versão que não mudou é o que o `--check`
pega. O `trilha audit` diz a mesma coisa de um arquivo em `public/vendor/` que o `vendor.lock`
não nomeia.

A origem padrão é o `https://esm.sh`, que serve pacotes npm como módulos ES; `--from` ou
`TRILHA_VENDOR_BASE` aponta para qualquer outro lugar. Nada disso é dependência do framework: o
arquivo é servido como qualquer outro estático, e a ilha importa ele pelo caminho.

## trilha migrate

Lê um projeto Next.js e grava as duas coisas mecânicas de uma migração: a árvore de pastas do
`app/`, com um arquivo Go por tela, e um relatório de tudo o que não é mecânico.

```bash
trilha migrate next ../web --dry-run   # imprime o relatório e não grava nada
trilha migrate next ../web             # grava o app/ e o MIGRATION.md
trilha migrate next ../web --out app --report MIGRATION.md --force
```

O que ele grava compila: cada página é uma função `Page` com título e os parâmetros da rota,
cada `route.ts` vira os handlers que exportava devolvendo `501`, e um `trilha gen` na sequência
deixa o projeto verde. Nada é sobrescrito sem `--force`, então rodar em um projeto que já tem
telas acrescenta o que falta e mantém o que está lá — a contagem no fim diz quantos foram
gravados e quantos foram mantidos.

O comentário acima de cada função é a parte que importa: diz de qual arquivo veio, quantas
linhas tinha, quais hooks usava, quais endpoints chamava e qual dos três formatos a tela
provavelmente é — **A** formulário ou lista sem ilha, **B** página com uma ilha, **C** app que
é cliente de verdade. É uma sugestão impressa com o motivo, não um veredito. O `MIGRATION.md`
junta isso numa tabela só, mais a lista do que não tem equivalente aqui: estados de
carregamento, templates, rotas paralelas e interceptadoras, middleware e rewrites, cada um com
a frase que explica o que ocupa o lugar.

A sugestão é sobre a tela, e só sobre ela. Um componente que a página importa conta para ela —
um `page.tsx` de cinquenta linhas na frente de um componente de trezentas não é uma porta de
cinquenta linhas —, mas a moldura não é a tela: o que um `layout.tsx` alcança é listado uma vez,
em **Dependências globais**, e não promove as páginas que envolve. O barril também não:
`import { DataTable } from "@/components"` segue a linha do `index.ts` que reexporta `DataTable`
e mais nenhuma, porque reexportar não é usar. Sem essas duas regras, um shell que carrega um chat
com `<svg>` classificou todas as telas da aplicação como **C** — uma ordem de trabalho que manda
começar por qualquer lugar.

As telas em si não são traduzidas. O corpo de uma página é regra de negócio, e máquina
chutando isso custa mais para revisar do que para escrever — o guia
[Do Next.js para a Trilha](/pt/receitas/do-next) tem o padrão de React ao lado da linha que
toma o lugar dele.

## trilha agents

Grava dois arquivos na raiz do projeto, e só quando é pedido: suporte a agentes de código é
opt-in, então `trilha new` sozinho não deixa nenhum dos dois. `trilha new --agents` já os cria
junto com o projeto.

| Arquivo | De quem é |
|---|---|
| `AGENTS.md` | do framework: as convenções, os comandos e o que não fazer |
| `CLAUDE.md` | seu: três linhas apontando para o `AGENTS.md`, mais o que este repositório pedir |

O `AGENTS.md` leva um carimbo com o hash do próprio corpo, a mesma regra do kit ui. Uma cópia
intocada de uma versão anterior é atualizada em silêncio na próxima rodada; uma que você editou
só é sobrescrita com `--force`, e sem ele o comando para e avisa. O `CLAUDE.md` nunca é
sobrescrito.

`--lang en|pt` escolhe a língua dos dois arquivos e por padrão é a da CLI.

Rode de novo depois de atualizar a CLI: o `AGENTS.md` nomeia os comandos da versão que o
gravou, então uma cópia de uma release anterior continua mandando o agente para comandos que
foram substituídos. A sequência inteira para um projeto vindo de versão anterior está na
[receita de migração](/pt/receitas/migracao#ligar-os-arquivos-de-agente-num-projeto-que-ja-existe).

## trilha openapi

Lê `app/`, deduz o documento a partir dos handlers e escreve `openapi.json`. `-o -` escreve na
saída padrão; `--title`, `--version` e `--server` preenchem o que o código não tem como saber
(o padrão é o nome do módulo, `0.0.0` e nenhum servidor). `--check` compara com o arquivo no
disco e sai com `1` quando divergem — a mesma linha que o `gen --check` é, pelo mesmo motivo:

```yaml
- run: trilha openapi --check
```

O que é deduzido e as diretivas `openapi:` estão em [APIs](/pt/aprender/api#documento-openapi).

## trilha check

Seis portões num comando, na ordem que falha mais barato primeiro: `gen`, `gofmt`, `vet`,
`test`, `audit` (sem a varredura de vulnerabilidades, que precisa de rede) e `openapi` (só se o
projeto guarda o documento). Ele para na primeira falha — o que vem depois de uma compilação
quebrada não diz nada sobre o projeto — e os passos que não rodaram dizem isso:

```text
✓ gen
✗ gofmt (failed)
    app/blog/page.go: not gofmt'd
    → run gofmt -w (or trilha check --fix)
- vet (not run)
- test (not run)
- audit (not run)
- openapi (not run)
```

Todo problema vem com o arquivo, a linha e a frase que resolve. O `--fix` regrava o
`trilha_gen.go` e a formatação antes de julgá-los, e aí o passo reporta `fixed`. O `--json`
escreve o relatório que uma ferramenta lê, com os mesmos campos:

```json
{
  "ok": false,
  "steps": [{ "tool": "gen", "status": "failed" }],
  "problems": [
    {
      "tool": "gen",
      "file": "app/page.go",
      "line": 3,
      "message": "page.go must export func Page(c *trilha.Ctx) (h.Node, error); found func Render",
      "fix": "rename the function to Page, or delete page.go if this directory is not a page"
    }
  ]
}
```

Sai com `1` quando algo falhou, então no CI é a linha única:

```yaml
- run: trilha check
```

## trilha generate crud

Entre o `trilha new --template app`, que traz **um** CRUD pronto de exemplo, e o
`trilha generate page`, que escreve uma página vazia, mora a tarefa que mais se repete: *tenho um
struct, quero a tela.* Dez vezes no mesmo projeto, cada uma com o mesmo esqueleto e um errinho
diferente — uma não confirma o excluir, outra não mostra o erro de validação no campo, outra
perde a página ao voltar.

```bash
trilha generate crud docs.Tipo --at app/admin/tipos
```

```text
  + internal/docs/tipo_store.go       TipoStore: List/Get/Create/Update/Delete, e a memória atrás
  + app/admin/tipos/page.go           lista: ui.DataTable, busca, ordenação, paginação, excluir
  + app/admin/tipos/new/page.go       criar: ui.Field por campo, 422 volta com as mensagens
  + app/admin/tipos/id_/page.go       editar: o mesmo formulário, preenchido
  + tipo_crud_test.go                 lista vazia, cria, edita, 422, exclui
  + app/setup.go                      o store, onde as páginas acham
```

O que sai é código que alguém lê e edita, na forma que o `templates/app` já provou ser
idiomática — e não um runtime que esconde as telas atrás de uma chamada. Ele existe para a décima
tela do projeto sair como a primeira.

### Como os campos viram tela

O struct é lido por **análise estática**, do mesmo jeito que o `--form` já lê: sem compilador e
sem baixar módulo.

- **O `ID` é a chave**, e um struct sem ela é recusado com essa frase. O gerador não escolhe um
  campo: a chave errada só aparece no primeiro `Update`, e aí já há cinco telas escritas em cima.
- **`CriadoEm`/`CreatedAt`/`AtualizadoEm`/`UpdatedAt` e `json:"-"` ficam fora do formulário.** São
  do sistema, e uma data preenchida à mão é um bug esperando. O store carimba: o "criado em"
  continua o mesmo num update, porque é a única data que as pessoas vão procurar.
- **`validate:"required,max=80"`** vira `required` e `maxlength` no controle, e a mensagem no
  campo quando falha.
- O rótulo vem do nome do campo em título, e trocar isso é editar uma string no arquivo que
  acabou de ser gerado — que é onde ela deve estar.
- Os quatro primeiros campos viram as colunas da tabela. Quatro é o que uma tabela mostra antes de
  começar a rolar para o lado.

### Nada é sobrescrito, e não há --force

Arquivo que já existe é recusa nomeando-o. É de propósito: gerador que sobrescreve é gerador que
ninguém roda duas vezes, e um CRUD é justamente o que se gera **depois** de já ter editado um.

A única exceção é o `app/setup.go`, que o gerador edita em vez de recusar — uma linha, dentro de
uma função cuja forma o framework define. Sem ela o CRUD compila e responde 500 na primeira
requisição, que é o pior resultado que um gerador pode ter, porque parece que funcionou. O comando
nomeia esse arquivo na saída, porque um gerador que mexe no que você não pediu deve essa frase.

### O store é interface com memória atrás

A mesma escolha de todo store deste framework, e aqui ela vale duas vezes: **o que sai do gerador
roda** — a tela abre, o formulário grava, o teste gerado passa — e trocar a memória por um banco é
implementar cinco métodos cujas assinaturas já estão escritas. A
[receita de banco de dados](/pt/receitas/banco-de-dados) é a outra metade.

### O que ainda não está aqui

`--store sqlite|postgres` com migração gerada, `--tenant`, `--policy`, a versão compacta com
`ui.SchemaForm`, e rodar de novo para imprimir o diff dos campos que você acrescentou ao struct
depois. Estão na [#115](https://github.com/emersonjoe/trilha/issues/115); o store em SQL, em
particular, teria de escolher dialeto de placeholder e ser dono de um DDL, que é a coisa que este
framework não faz em nenhum outro lugar.

## trilha add

Metade do que um app de gestão precisa é um **padrão**, e não um primitivo: a trilha de quem fez o
quê, a tela que emite chaves de API, a seção de configurações que alguém edita em vez de fazer um
deploy. No framework isso fica rígido; como documentação, vira trabalho de copiar. O `trilha add`
escreve no projeto, como arquivos que ele passa a possuir.

```bash
trilha add              # o que existe, uma linha cada
trilha add audit
trilha add audit --dry-run
trilha add --list --json    # para o servidor MCP e o agente do editor
```

```text
  + internal/auditoria/store.go
  + app/auditoria/page.go
  ~ app/setup.go (uma linha acrescentada)

Rode `trilha dev` e abra /auditoria. Guarde a rota: a trilha diz quem fez o quê, e isso
não é para todo mundo.
Doc: https://trilha.dev/reference/observability
```

A diferença para o `generate` é a direção: **o `generate crud` escreve a partir do seu código** —
um struct vira tela — e o **`add` escreve a partir de uma receita do framework**. Os dois terminam
em `gen`, e os dois deixam o `trilha check` verde.

### Rodar duas vezes acrescenta; não recomeça

Arquivo que já está lá é pulado com um aviso — não é sobrescrito nem vira recusa. Na segunda vez o
arquivo é de quem o recebeu, e provavelmente já foi editado.

O único arquivo que uma receita edita é o `app/setup.go`, e a inserção é **marcada**:

```go
func Setup(a *trilha.App) error {
	// trilha:add audit
	a.Config().Audit = auditoria.Store
	return nil
}
```

É essa marca que faz a segunda execução reconhecer a própria linha, em vez de acrescentar uma
segunda cópia de um store que ninguém queria duas vezes. Três receitas deixam um bloco de imports,
e não três grupos de uma linha — o arquivo é de alguém, e essa pessoa vai lê-lo.

O `--dry-run` imprime tudo isso e não escreve nada, que é o que se roda antes de deixar um comando
mexer num projeto que já tem código.

### As receitas

| Receita | O que escreve |
|---|---|
| `audit` | o destino que o `Config.Audit` recebe, e a tela que o lê com o `ui.AuditTable` |
| `api-keys` | o emissor, a tela que cria e revoga, e a chave mostrada uma vez com o `ui.SecretOnce` |
| `settings` | uma seção declarada como struct, e a tela que o `ui.SettingsForm` desenha a partir dela |

Cada uma vem com memória atrás, para a tela funcionar desde a primeira requisição, e um comentário
dizendo onde entra um banco. Cada uma também diz, dentro do arquivo, que a pasta precisa ser
guardada: uma trilha de auditoria nomeia pessoas, e uma tela de chaves emite credencial.

Faltam outras — `login`, `share-link`, `webhooks`, `mail`, `blob`, `tasks`, `permissions`,
`tenant` — na [#116](https://github.com/emersonjoe/trilha/issues/116).

### Por que a CI aplica todas elas

Uma receita que quebrou em silêncio é pior que nenhuma receita, porque quem a rodou já está com o
código dela dentro do projeto. Então o teste não lê os templates: ele cria um projeto, aplica todas
as receitas nele e roda o `trilha check` — compilar, vet, testar, auditar — sem ninguém editar
nada.

## trilha ctx

O mapa do projeto numa leitura só: o módulo, se o `trilha_gen.go` está em dia, cada rota com
seu arquivo, métodos, parâmetros, layouts e middlewares, cada operação de API com sua query,
corpo e respostas, os tipos que essas operações trocam e o que o `app/setup.go` provê:

```text
# example.com/loja

- trilha 0.37.0 · 8 routes (6 pages, 2 APIs)
- trilha_gen.go: up to date
- app/setup.go: Setup, Config

## Routes

- `GET /` — app/page.go · layouts: app/layout.go
...
```

Duas seções a mais aparecem só quando o projeto as tem, então quem não usa nenhuma das duas não
paga nada — nem linha, nem token:

```text
- `GET POST /docs` — app/docs/page.go · middleware: app/docs/middleware.go · needs docs:ver · POST needs docs:editar

## Enums

- `doc.situacao` — `rascunho` (Rascunho), `enviado` (Enviado · info) · app/setup.go
```

**O que uma rota exige** é lido do `middleware.go` que declara, e herdado do jeito que a cadeia
de middleware é herdada: a pasta mais funda ganha, e um `MiddlewarePOST` aparece só naquele
método. A leitura resolve as duas formas que a documentação ensina — o guarda chamado na hora e
a var de pacote que o `Middleware` devolve —, inclusive um embrulho seu que entrega duas strings
ao `RequirePolicy`, que é o idioma do `examples/local-login`. O que ela não consegue ler não
aparece: um módulo que vem de variável precisaria de um compilador, e um mapa que chuta é pior
que um mapa calado sobre uma pasta.

**Os enums** são as listas de domínio que o `trilha.RegisterEnum` registrou, casadas com a
declaração de onde vieram. Estão aqui porque quem não sabe que uma lista existe inventa uma
segunda, e aí duas telas escrevem "em análise" de dois jeitos. As mesmas listas são legíveis em
tempo de execução com o `trilha.LookupEnum` e o `trilha.RegisteredEnums`.

O padrão é Markdown compacto, para ler. `--routes` e `--types` imprimem uma seção sozinha,
`--all` não elide nada (os middlewares por método, todas as respostas de erro, o tipo
`Problem`) e `--json` escreve o mesmo modelo como documento, ordenado e sem relógio nem caminho
absoluto, de modo que duas execuções na mesma árvore dão os mesmos bytes.

A seção de API e os tipos saem da mesma inferência que está por trás do `trilha openapi`, então
o mapa e o documento não têm como divergir. Como o `openapi.json`, a saída é um documento de
máquina e não é traduzida.

## trilha gen --check

Gera em memória, compara com o `trilha_gen.go` commitado e sai com `1` mostrando as linhas
que divergem — uma linha no CI, e uma pasta nova em `app/` sem `trilha gen` depois deixa de
ser um 404 que ninguém explica:

```yaml
- run: trilha gen --check
```

O `trilha check` faz essa mesma comparação no primeiro portão, e é por isso que um projeto que
usa o `check` não precisa de uma linha `gen --check` à parte. O `trilha audit` faz a comparação
como aviso, e ainda compara a versão da CLI com a da
biblioteca no `go.mod`: uma CLI mais nova escreve código que a biblioteca pode ainda não ter,
e o erro aparece dentro de código gerado — o pior lugar para procurar.

## Arquivo gerado

`trilha_gen.go` é determinístico (mesma árvore, mesmos bytes), tem o cabeçalho
`// Code generated by trilha. DO NOT EDIT.`, mais a diretiva `//go:generate trilha gen`
(para `go generate ./...` funcionar sem ninguém precisar saber o nome da ferramenta), e deve
ser commitado: `go build ./...` funciona
sem a CLI instalada. Ele define `newApp() *trilha.App` e `main()`; se outro arquivo do
pacote já tem `func main()`, o gerador omite o dele (veja [App](/pt/referencia/app)).

### Um app dentro de um binário que já existe

O arquivo gerado adota o pacote que a pasta declara, então um app Trilha pode ser um pacote
comum, importável, dentro de um servidor `net/http` que você já roda:

```go
// internal/crm/crm.go — package crm, escrito à mão
// internal/crm/app/…    — as rotas
// internal/crm/trilha_gen.go — package crm, func NewApp() *trilha.App

mux.Handle("/", crm.NewApp().Handler())
```

A precedência, do mais explícito ao menos: `--package <nome>`; o pacote declarado pelos `.go`
escritos à mão da pasta; o pacote declarado por um `trilha_gen.go` que já esteja lá; `main`.
O terceiro passo é o que faz a bandeira valer uma vez só — o arquivo gerado lembra a escolha,
e o `trilha gen --check` do CI não precisa dela.

Fora do `package main` o construtor é exportado (`NewApp`, porque quem chama mora em outro
pacote) e nenhum `func main()` é escrito. O `trilha dev` e o `trilha build` recusam um app
assim e dizem quem o roda: não há binário aqui, o hospedeiro é que tem um.

## Códigos de saída

`0` sucesso; `1` erro de geração, compilação ou execução; `2` uso incorreto.
