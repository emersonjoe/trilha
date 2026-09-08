# Spec 059 — Windows no harness, e o caminho que se lê

- **Issue**: #88 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `059-windows-no-harness`
- **Versão**: nenhuma — vai no `CHANGELOG` sob *Unreleased*.

## Por quê

O job `windows` reprovou no commit da 0.41.0. O produto está certo: os quatro testes e2e novos
é que constroem o `trilha-cli` e o executam sem passar pelo `exeName`. As cinco chamadas
antigas usavam, as quatro novas não — e quatro esquecerem a mesma coisa de uma vez não é
descuido de quem escreveu, é sinal de que o conhecimento estava no lugar errado: espalhado por
cada chamador em vez de morar num só.

Corrigido isso, sobra um segundo, que estava escondido atrás do primeiro: o `migrate next` e o
`client` imprimem caminho com a barra do sistema (`app\page.go`), enquanto o resto da CLI
normaliza (`trilha new` escreve `app/api/hello/route.go`, e o `vendor` chama `ToSlash` na
linha de saída dele). Quem lê um relatório de migração no Windows vê caminho de rota escrito
de um jeito que não é o das rotas.

## O que muda

**`buildCLI(t, repo, tmp)`** no `e2e_test.go`: um lugar constrói o CLI e devolve o caminho que
o executa. Os sete sítios passam por ele; o oitavo teste não tem como errar.

**`filepath.ToSlash` nas duas linhas de saída** do `migrate` e do `client`. O caminho que a
pessoa lê é o mesmo em todo sistema; o que vai para o sistema de arquivos mantém o separador
dele.

Nada da API pública muda.

## Fora de escopo

- **Um teste que proíba `filepath.Join` dentro de `Printf`** — pegaria os casos certos e
  muitos errados; a regra que vale é a saída da CLI ser conferida no `windows`, e ela é.
- **Revisar toda a saída da 0.41.0 atrás de outros caminhos nativos** — o `go test ./...` no
  Windows é quem responde isso, e responde a cada PR.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| VI — teste primeiro | O teste que falha já existia e é o próprio `windows`; a correção é o que o faz passar. A suíte inteira roda no Windows antes do PR. |
| Estilo | A saída da CLI passa a ser a mesma nas três plataformas, que é o que a documentação mostra. |

## Tarefas

- [x] T001 `buildCLI` e os sete sítios.
- [x] T002 `ToSlash` no `migrate.go` e no `client.go`.
- [x] T003 `go test ./...` verde no Windows.
- [ ] T004 `CHANGELOG.md` sob *Unreleased*; CI verde no PR.

## Aceitação

- **SC-001** `go test ./...` passa no Windows, no Linux e no macOS.
- **SC-002** O job `windows` fecha verde na `main` depois do merge.
- **SC-003** `trilha migrate next` e `trilha client` imprimem `app/page.go` e
  `internal/api/client.go` também no Windows.
