# Plano — Spec 031

**Branch**: `031-openapi` · **Spec**: [spec.md](./spec.md) · uma rodada de `make test` por bloco.

## Os fatos que decidem o desenho

1. **O scanner já entrega metade do documento.** `scan.Route` tem `Pattern` (já no formato
   `{id}` do `net/http`), `Kind`, `Methods` e `Dir`. Caminho, método e parâmetro de rota saem
   daí sem uma linha de análise nova — o gerador de OpenAPI é um segundo consumidor do mesmo
   `scan.Result`, como o `trilha gen` e o `trilha routes`.
2. **O que falta é o corpo, e ele mora no AST do handler.** Não há como saber o tipo de `p`
   em `c.JSON(200, p)` olhando só o nome da função. Mas `p` vem de `posts.Get(…)` três linhas
   acima, e a assinatura de `posts.Get` está num arquivo do mesmo projeto. Então: um índice de
   tipos e funções exportadas de todos os pacotes do projeto, e uma inferência **sintática**
   sobre o corpo do handler. Não é `go/types`; é o suficiente para o app de referência.
3. **A inferência precisa saber parar.** Três formas de dar tipo a uma variável local
   (`v := pkg.Fn(…)`, `v, _ := pkg.Fn(…)`, `var v struct{…}` / `v := pkg.Tipo{…}`) e nada
   mais. Uma expressão que não caiu numa delas não tem schema — a resposta aparece no
   documento sem `schema`, e a diretiva `openapi:response` põe o tipo à mão. **Adivinhar
   errado é pior do que não adivinhar**: um schema falso quebra o cliente que confiou nele.
4. **A tag `validate` é metade de um JSON Schema.** `required` → lista `required`; `max=80`
   num texto → `maxLength`; num número → `maximum`; `oneof=a b` → `enum`; `email`/`url` →
   `format`. Isso vem de graça da 0.18.0 e é o argumento mais forte a favor de gerar o
   documento em vez de escrevê-lo: o schema não pode divergir da validação porque é a mesma
   tag.
5. **O erro já tem forma fixa desde a 0.21.0.** `application/problem+json` com `type`,
   `title`, `status`, `detail`, `instance`, `fields`. Então `components.schemas.Problem` é
   constante, e toda operação ganha `default` apontando para ele — o cliente sabe o que vem
   num erro sem que ninguém escreva nada.
6. **Determinismo é requisito, não zelo.** `encoding/json` ordena chave de mapa
   alfabeticamente, e ordem alfabética é estável; a única ordenação própria é a de `required`
   e a das listas de parâmetro. Golden file dos dois exemplos, como o `trilha_gen.go`.
7. **`--check` existe porque o arquivo é commitado.** É o mesmo raciocínio do `gen --check`:
   uma rota nova sem regerar vira um contrato mentiroso publicado, e uma linha no CI evita.
8. **Um tipo desconhecido numa diretiva é erro do comando.** O contrário — emitir schema
   vazio quando o nome está errado — publica um documento que parece certo. Falha com o
   arquivo e o nome.

## Fases

1. **Índice e schema** — `internal/openapi/schema.go`: varredura dos `.go` do projeto (fora de
   `_test.go`, `testdata/`, diretórios ocultos) montando `pacote.Tipo` → `*ast.StructType` e
   `pacote.Func` → tipos de resultado; conversão de tipo Go em JSON Schema, com as tags `json`
   e `validate`; `$ref` para struct nomeada e coleta em `components.schemas`.
2. **Inferência do handler** — `internal/openapi/infer.go`: leitura do corpo da função
   exportada (`GET`, `POST`, …) atrás de `Bind`/`BindJSON`, `c.JSON`, `WriteHeader`,
   `c.Header("Content-Type", …)`, `trilha.ErrNotFound`, `trilha.Errorf`, `&trilha.Problem{}`,
   mais as diretivas `openapi:` do doc comment.
3. **Documento** — `internal/openapi/openapi.go`: `Generate(root, res, Options)`, as structs do
   documento com `json` tags, `Problem` fixo, tag padrão, `operationId`, parâmetros de caminho.
4. **Comando** — `cmd/trilha/openapi.go`: flags `-o`, `--title`, `--version`, `--server`,
   `--check`; mensagens no `i18n.go` (inglês e pt-BR); linha no `usage`.
5. **Exemplos** — diretivas no `examples/blog` (`openapi:response`, `openapi:tag`) e no
   `examples/orcamento` (`openapi:query mes`); golden dos dois em `testdata/golden`.
6. **Documentação e fechamento** — capítulo nas duas locales, referência do CLI, CHANGELOG,
   `version`, ROADMAP, release.

## Riscos

- **Inferência que erra em silêncio.** Mitigada pela regra 3 (três formas, nada mais) e pelo
  golden: se a inferência mudar de resposta, o diff aparece no teste.
- **Índice de pacotes caro em projeto grande.** É um `parser.ParseFile` com
  `SkipObjectResolution` por arquivo, só na chamada do comando, e o `trilha gen` já faz o
  mesmo em `app/`. Sem cache, sem watch.
- **Nome de schema com ponto** (`posts.Post`). É válido no OpenAPI (`[a-zA-Z0-9.\-_]+`) e
  mantém a origem visível; o alternativa (`Post`) colide entre pacotes.
- **Documento incompleto passando por completo.** Uma resposta sem `schema` é honesta e
  visível; o capítulo de documentação diz explicitamente quando escrever a diretiva.
