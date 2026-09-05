# Trilha — framework web para Go com roteamento por arquivos

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
<!-- SPECKIT END -->

## Comandos

- `make test` — gofmt (fora de testdata) + `go vet ./...` + `go test ./...` (inclui e2e da CLI).
- `make golden` — regrava os golden files do gerador após mudar `internal/gen`.
- `make dev-example` / `make reload` — dev server no exemplo e medição do ciclo de recarga.
- `cd examples/blog && go run ../../cmd/trilha gen` — regenerar `trilha_gen.go` do exemplo (commitado).
- `make release VERSION=X.Y.Z ISSUES="20 21"` — fecha a spec do branch atual: testa, funde na
  `main`, marca a tag, publica a release com as notas do `CHANGELOG.md` e fecha as issues
  (`DRY_RUN=1` mostra sem executar).

## Estrutura

- Raiz (`package trilha`): runtime — `App`, `Ctx`, erros, render, serve, csrf, static.
- `h/`: DSL de HTML (elementos gerados em `elements.go`).
- `internal/scan` (app/ → rotas), `internal/gen` (rotas → trilha_gen.go), `internal/dev` (watch + build + proxy + SSE), `internal/scaffold` (templates do `new`).
- `cmd/trilha`: CLI. `examples/blog`: app de referência com todas as convenções. `testdata/`: árvores sintéticas e goldens.

## Regras (constituição em `.specify/memory/constitution.md`)

- Zero dependências externas no runtime e na CLI (`TestNoExternalDeps`).
- Toda convenção nova precisa de: teste no scanner, rota no `examples/blog` e teste de integração.
- Gerador determinístico; arquivo gerado é commitado.
- Código, identificadores e mensagens de erro em inglês. Público (site, README, comunidade, CLI, scaffold) em inglês por padrão com tradução pt-BR no mesmo commit (site em `/` e `/pt`, `README.pt-BR.md`, `docs/pt-BR/`, `TRILHA_LANG`). Specs e constituição em pt-BR.
- Commits sem trailer de coautoria.

## Fluxo de trabalho

- **Uma spec por sessão.** Fechada a versão, recomece com um resumo curto: cada chamada de
  ferramenta reenvia a conversa inteira, então o custo cresce com o histórico acumulado.
- **Spec curta** para mudança pequena (`.specify/templates/spec-curta-template.md`, arquivo
  único); spec → plan → tasks para o resto. Critério na constituição.
- **A issue é a fonte do escopo**: aponte para ela, não recopie a lista de implementação.
  Fato verificado fora do repositório vai para a issue, para não ser conferido duas vezes.
- **Um dono da `main` por vez**; duas frentes só com divisão de arquivos combinada antes.
- **Ler estreito**: `grep -n -A5` ou `sed -n 'X,Yp'` em vez de ler arquivo grande inteiro.
- **`make test` por bloco de tarefas**, não por arquivo, e não repita a suíte sem ter mudado
  código.
