# Research: Trilha

## R1. Nome de pasta para segmento dinâmico

**Decisão**: `nome_` (um segmento) e `nome__` (catch-all).

**Motivo**: `[slug]`, `{slug}`, `$slug` e `:slug` contêm caracteres proibidos em import
path do Go (`module.CheckImportPath`). `_slug` compila quando importado explicitamente,
mas `go build ./...`, `go vet ./...` e `go test ./...` ignoram pastas iniciadas por `_` —
testes dentro da página não rodariam. `@slug` é rejeitado ("path@version syntax").
`-slug` e `~slug` confundem o shell. Verificado empiricamente em Go 1.25.1: `slug_`
aparece em `go list ./...` e compila.

**Alternativas rejeitadas**: declarar o padrão dentro do arquivo (`var Route = "/blog/{slug}"`)
— perde o roteamento por arquivo, que é a proposta central.

## R2. Descoberta de handlers: `go/ast` em build vs `reflect` em runtime

**Decisão**: varredura com `go/parser` (modo `ParseFuncs`-like: só declarações) em tempo de
geração; arquivo gerado importa cada pacote e registra funções pelo nome.

**Motivo**: o compilador valida assinaturas (`Page` com tipo errado = erro de compilação no
arquivo gerado, apontando o pacote); zero custo em runtime; arquivo legível. `reflect` não
consegue enumerar funções de um pacote, então exigiria registro manual ou `init()` mágico.

## R3. Watcher: polling de mtime vs `fsnotify`

**Decisão**: polling a cada 250 ms de `app/`, `public/` e `*.go` na raiz, comparando um
hash de (caminho, mtime, tamanho).

**Motivo**: mantém zero deps (princípio II); em projetos de até alguns milhares de arquivos
o custo é desprezível; funciona igual em macOS/Linux/Docker volumes (onde `fsnotify` falha
com frequência). Debounce de 100 ms para agrupar saves múltiplos.

## R4. Recarga do navegador: proxy da CLI + SSE

**Decisão**: `trilha dev` escuta na porta pública (`:3000`) e faz proxy reverso
(`httputil.ReverseProxy`, stdlib, `FlushInterval: -1`) para o processo filho, que sobe em
`127.0.0.1:<porta aleatória>` com `TRILHA_ENV=dev`. O runtime em dev injeta antes de
`</body>` um script que abre `EventSource('/_trilha/events')`; a CLI intercepta esse
caminho (nunca encaminha) e envia `reload` quando o filho novo responde na porta, ou
`error` quando o build falha — nesse estado a CLI responde qualquer rota com a saída do
compilador em HTML. O runtime também serve `/_trilha/events` (só pings) para quem roda o
binário em dev sem a CLI.

**Motivo**: a primeira versão desta decisão previa reconexão sem proxy, mas o navegador
recarregaria numa janela em que a porta ainda está vazia (entre o processo velho morrer e o
novo escutar) e a tela de erro de compilação disputaria a mesma porta com o app. Com o proxy
o recarregamento só dispara depois do health check do filho, e a porta pública nunca cai.
`httputil` é biblioteca padrão, então o princípio II se mantém.

## R5. HTML: DSL de funções vs `html/template`

**Decisão**: DSL `h` (`h.Div(h.Class("x"), h.Text(t))`) com interface `Node{ Render(io.Writer) error }`.
Atributos e filhos são ambos `Node`; atributos se identificam por um método marcador e são
escritos na tag de abertura.

**Motivo**: composição em Go puro (layouts recebem `children h.Node`), tipagem, sem parse em
runtime, e o mesmo modelo mental de componentes do JSX. `html/template` exige arquivos
separados e perde a checagem de tipo; adaptador fica para depois.

## R6. `public/` embutido no binário

**Decisão**: o arquivo gerado contém `//go:embed public` (só se a pasta existir) e passa o
`fs.FS` ao runtime; em dev o runtime lê do disco (`os.DirFS("public")`) para refletir
mudanças sem rebuild.

**Motivo**: `embed` só funciona a partir do pacote que contém a diretiva, e `public/` está
na raiz do app, junto do `trilha_gen.go`. Uma flag do gerador decide o modo.

## R7. CSRF

**Decisão**: double-submit cookie: cookie `trilha_csrf` (HttpOnly, SameSite=Lax) com token
aleatório; formulários incluem `<input type=hidden name=_csrf>` via `h.CSRF(c)`; JSON pode
enviar `X-CSRF-Token`. Verificação com `subtle.ConstantTimeCompare` em `POST/PUT/PATCH/DELETE`
de páginas. Rotas de API (`route.go`) não exigem CSRF por padrão (são chamadas por clientes
com token/bearer); configurável em `App`.

## R8. Roteador

**Decisão**: `http.ServeMux` de Go 1.22+, com patterns `"GET /blog/{slug}"`,
`"GET /docs/{path...}"` e `"GET /{$}"` para a raiz. Precedência (mais específico vence) e
405 automático com `Allow` já vêm do mux. Trailing slash: o mux redireciona `/blog/`→`/blog`
apenas para padrões com subárvore; o framework registra explicitamente `/x/{$}` quando
necessário para evitar 404.
