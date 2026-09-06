# Plano — `trilha generate`

## Fatos que decidem o desenho

1. **A tradução URL → pasta é o inverso do `parseSegment`** (`internal/scan/scan.go:168`).
   Escrever o inverso no `scaffold` duplica a regra em dois lugares; a alternativa — exportar
   algo do `scan` — obrigaria o pacote de varredura a saber gerar. O empate se desfaz pelo
   teste: o unitário do gerador confere a pasta produzida *e* o padrão que o scanner extrai
   dela, então as duas metades não podem divergir em silêncio.

2. **O pacote da pasta vem do disco quando existe.** Um `route.go` acrescentado ao lado de um
   `page.go` legado tem que declarar o mesmo pacote, ou o Go recusa o diretório inteiro. Ler o
   primeiro `.go` da pasta é mais barato e mais correto do que derivar de novo.

3. **O esqueleto mora em `internal/scaffold`**, junto dos templates do `new`, e usa
   `text/template` como eles. A lógica fica testável sem compilar a CLI; o comando só traduz
   erro em mensagem.

4. **`Generate` devolve o que aconteceu, não imprime.** O `cmdGenerate` decide o texto, na
   língua do usuário — mesmo desenho do `WriteUI`/`cmdUI`.

5. **Regenerar depois de escrever é parte do comando**, não cortesia: a rota só existe no
   `trilha_gen.go`. Se a geração falhar (o projeto já estava quebrado), o arquivo criado
   continua lá e o erro aparece; escrever e regenerar não é transação.

6. **`--force` só cobre a sobrescrita.** Conflito de convenção (`page.go` onde há `route.go`)
   não é sobrescrita: é um projeto que não compilaria. `--force` não passa por cima disso.

## Fases

### Fase 1 — `internal/scaffold/generate.go`

```go
type GenResult struct {
	File    string // relativo à raiz
	Pattern string // URL respondida; vazio para componente
	Package string
}

type GenOptions struct { Kind, Arg string; Force bool; Dir string }

func Generate(root string, o GenOptions) (GenResult, error)
```

`kind` ∈ `page|route|component`. Sentinelas `ErrGenExists` e `ErrGenConflict` para o comando
distinguir. Tradução da URL, derivação do pacote, `text/template` para o corpo.

### Fase 2 — `cmd/trilha/generate.go` + `i18n.go` + despacho

`flag.NewFlagSet("generate")` com `--force` e `--dir`; sem argumento, imprime a linha de uso do
próprio comando. Mensagens novas nas duas línguas e uma linha no `usage`.

### Fase 3 — testes

Unitário: tabela de URLs → pasta/pacote/padrão, incluindo os erros; e o padrão conferido contra
o que o `scan` devolve para a árvore criada. E2E: os três subcomandos num projeto recém-criado,
mais a recusa sem `--force` e o conflito `page`×`route`.

### Fase 4 — documentação

`reference/cli` nas duas locales (o comando e a tabela de tradução) e uma menção no capítulo de
páginas e rotas, onde a convenção de pasta é ensinada.

## Riscos

- **Esqueleto que envelhece.** Um esqueleto que não compila é pior que nenhum. O e2e compila o
  projeto depois de gerar os três, então a quebra aparece aqui, não no usuário.
- **Adivinhar demais.** Gerar `POST`, formulário e validação junto pareceria útil e daria um
  arquivo que ninguém quer inteiro. O esqueleto fica no mínimo que roda.
