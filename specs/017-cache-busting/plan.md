# Plano: 017-cache-busting

## Superfície pública

```go
// Ctx
func (c *Ctx) Asset(path string) string // "/site.css" → "/site.css?v=8f3a1c92"

// App (para quem precisa fora de uma requisição, como ExportPaths ou um e-mail)
func (a *App) Asset(path string) string
```

`Ctx.Asset` aplica `BasePath` como `Ctx.Base()` já faz, então `c.Asset("/ui.css")` é
substituto direto de `c.Base()+"/ui.css"`.

## Arquivos

| Arquivo | Papel |
|---|---|
| `assets.go` | hash por arquivo, cache com invalidação em `dev`, `App.Asset`/`Ctx.Asset` |
| `static.go` | `?v=` correto → `immutable`; `v` ausente ou divergente → regra de hoje |
| `assets_test.go` | hash estável, mudança de conteúdo muda o `v`, arquivo ausente, `dev` recalcula, cabeçalho por caso |
| `ui/ui.go` | `Head` passa a usar `c.Asset` |
| `site/internal/ui/*` | layout do site usa `c.Asset` (é a origem do problema) |
| `examples/blog/app/*` | passa a versionar de verdade o que o comentário já prometia |
| `cmd/trilha/audit.go` | aviso de `immutable` sem `Asset` |
| `site/.../referencia/app.md`, `.../aprender/dev-e-producao.md` | documentação |

## Decisões

1. **Parâmetro de consulta, não nome de arquivo.** Um HTML antigo em cache continua
   achando o arquivo; com nome hasheado ele levaria 404 e ficaria sem estilo. A perda é
   teórica (alguns proxies antigos não cacheiam URL com query), e nenhum CDN atual se
   comporta assim.
2. **Hash do conteúdo, não da hora do build.** Uma publicação que não mudou nada não pode
   invalidar o cache de ninguém. FNV-1a de 64 bits, oito dígitos hexadecimais: não é
   assinatura, é identidade de conteúdo, e `crypto/sha256` custaria mais sem ganho.
3. **Preguiçoso e em cache.** Nada é lido no `New`: um app com 300 arquivos em `public/`
   não paga por 299 que ninguém referencia. Em `prod` cada arquivo é lido uma vez; em `dev`
   o `Stat` (mtime + tamanho) decide se relê.
4. **`immutable` só quando o `v` confere.** Se qualquer `?v=` bastasse, um endereço
   adivinhado congelaria a versão errada no navegador de quem clicou.
5. **Sem opção nova na CLI.** O `trilha export` renderiza as mesmas páginas; se o layout usa
   `Asset`, o HTML exportado sai versionado. Menos superfície, mesmo resultado.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I | não muda roteamento nem convenção de arquivo |
| II | `hash/fnv`, `io/fs` — biblioteca padrão |
| III | sem reflexão, sem geração de código nova |
| IV | uma função no `Ctx`, do lado das que já existem (`Base`, `Title`) |
| V | em `dev` o hash acompanha o arquivo; nada de reiniciar para ver o CSS mudar |
| VI | teste do hash, do cabeçalho, do caminho ausente e do comportamento em `dev` |
| VII | `immutable` só com `v` correto; caminho inválido não sai do `Public` |

## Complexity Tracking

O risco é acrescentar leitura de disco ao caminho quente. Mitigação: o cache é um
`map[string]assetVersion` sob `RWMutex`, consultado só por quem chama `Asset` (o layout,
uma vez por página), e o `serveStatic` compara com o valor já em cache — sem `Stat` extra
fora de `dev`.
