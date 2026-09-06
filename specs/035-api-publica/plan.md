# Plano — API pública e política de depreciação

## Fatos que decidem o desenho

1. **`golang.org/x/exp/apidiff` está fora.** Princípio II: nem runtime nem CLI importam nada de
   fora. O `deps_test.go` reprovaria, e um `tools.go` com build tag continuaria pondo a
   dependência no `go.mod`. A biblioteca padrão resolve: `go/parser` lê o pacote, `go/ast` dá as
   declarações, `go/printer` imprime o tipo.

2. **O Go faz exatamente isso consigo mesmo.** `api/go1.txt` é uma lista de linhas de texto,
   comparada por um teste. Copiar o formato dá três coisas de graça: o diff é legível por
   humanos, a ordenação torna o arquivo estável, e a granularidade por símbolo faz a linha que
   sumiu aparecer sozinha no diff.

3. **`go/parser` basta, `go/types` não é preciso.** Comparar assinaturas *sintáticas* pega
   remoção, renomeação, mudança de parâmetro e campo perdido — tudo que é quebra de compilação
   escrita no arquivo. Resolver tipos exigiria carregar o pacote com `go/packages` (externo) ou
   `go/importer` (frágil, precisa de export data compilada). Fica sintático, e o `API.md` diz
   que é sintático.

4. **`ParseDir` com `ast.FileFilter` que descarta `_test.go`** evita listar helpers de teste.
   Arquivos com build tag de outro SO não existem no repo, então não há caso a tratar.

5. **A superfície tem que incluir os campos exportados de struct.** É onde a quebra silenciosa
   mora: `Config` tem dezenas de campos e tirar um compila do lado de dentro. Interface entra
   com os métodos, pela mesma razão.

6. **Alias e tipo com corpo grande viram uma linha por membro**, não uma linha com o corpo
   inteiro impresso: o objetivo é diff legível, não serializar a AST.

7. **`internal/apisurface` é interno**, então ele próprio não entra na superfície — e não
   precisa se auto-excluir por regra especial: a lista de pacotes públicos é explícita.

8. **A lista de pacotes é explícita, e um teste garante que ela não fica para trás.** Um pacote
   público novo que ninguém acrescentou à lista sairia sem cobertura. `go list` daria a lista
   sozinho, mas é um subprocesso dentro do teste; ler os diretórios do módulo e cruzar com a
   lista dá o mesmo alarme sem sair do processo.

## Fases

### Fase 1 — renderizador (`internal/apisurface`)

`Render(root string, pkgs []Package) (string, error)`, onde `Package{Dir, Name}`. Para cada
pacote, parse do diretório, coleta das declarações exportadas, uma linha por símbolo:

```text
pkg trilha, const Dev Env
pkg trilha, func New(cfg Config) *App
pkg trilha, method (*App) Handler() http.Handler
pkg trilha, type Config struct
pkg trilha, field Config.AllowedHosts []string
pkg trilha, type HandlerFunc func(*Ctx) error
pkg h, func Text(s string) Node // deprecated
```

Ordenação por texto da linha dentro do pacote, pacotes na ordem dada. `// deprecated` no fim
da linha quando o doc comment tem `Deprecated:`.

### Fase 2 — golden e `make api`

`api/current.txt` versionado; `TestSuperficiePublica` compara e, na diferença, imprime as
linhas que entraram (`+`) e saíram (`-`). `make api` regrava via `go run ./internal/apisurface/cmd`
— não: sem binário novo. Regrava com `go test -run TestSuperficiePublica -update`, o mesmo
padrão dos outros goldens do repo (`make golden`).

### Fase 3 — documentos

`API.md` e `docs/pt-BR/API.md` com o cabeçalho de troca de língua igual ao do
`SECURITY-MODEL.md`; ponteiro em `GOVERNANCE.md` nas duas línguas.

### Fase 4 — constituição

Emenda no princípio que trata de contrato público (IV) ou seção própria, registrando fronteira
e ciclo de depreciação; versão 1.4.0, `Last Amended` de hoje.

### Fase 5 — fechamento

Site (referência de governança/CLI não cobre isto; a página certa é a de contribuição do
site, se existir — senão fica nos arquivos da raiz), CHANGELOG 0.26.0, `version`, ROADMAP,
release.

## Riscos

- **Golden ruidoso.** Se cada mexida em comentário mudasse o arquivo, ninguém olharia o diff.
  Por isso a linha carrega assinatura, não doc — o único bit de documentação que entra é o
  `Deprecated:`, que é exatamente o que se quer ver no diff.
- **Falso senso de garantia.** O teste pega quebra de compilação, não de comportamento. O
  `API.md` diz isso em uma frase, para ninguém achar que o golden verde significa "nada mudou
  para quem usa".
