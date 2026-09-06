# Contribuindo com o Trilha

> 🇧🇷 Português · [🇺🇸 English](../../CONTRIBUTING.md)

Obrigado pelo interesse. Este guia explica como o projeto trabalha para que a sua
contribuição seja aceita sem ida e volta.

## Antes de codar

- **Bug**: abra uma issue com a árvore de `app/` que reproduz.
- **Convenção nova ou mudança de API**: abra uma issue de proposta. Mudanças de comportamento
  passam por uma spec em `specs/NNN-nome/` (fluxo [spec-kit](https://github.com/github/spec-kit):
  spec → plan → tasks → implement). O mantenedor abre a spec com você a partir da issue.
  Mudança pequena — um pacote, sem convenção nova em `app/`, sem quebra de API pública —
  usa a forma curta: um único `spec.md` a partir de
  `.specify/templates/spec-curta-template.md`.
- **Dúvida**: use as [Discussões](https://github.com/emersonjoe/trilha/discussions).

## Princípios que não mudam sem emenda

Estão em [`.specify/memory/constitution.md`](../../.specify/memory/constitution.md). Os que mais
afetam contribuições:

1. Runtime e CLI usam **só a biblioteca padrão**. Um PR que adiciona `require` ao `go.mod`
   é fechado, por melhor que seja a biblioteca.
2. Rotas vêm de **convenções de arquivo** e viram **código gerado**, verificado pelo
   compilador. Nada de `reflect` para descobrir handlers.
3. Toda convenção nova precisa de **três coisas**: teste de tabela no scanner
   (`internal/scan`), rota em `examples/blog` e teste de integração em
   `examples/blog/blog_test.go`.
4. Testes primeiro no núcleo. `go vet ./...` e `go test ./...` verdes.

## Ambiente

```bash
git clone https://github.com/emersonjoe/trilha && cd trilha
make test          # gofmt + vet + testes (inclui e2e da CLI)
make dev-example   # examples/blog com recarga
make golden        # regrava goldens do gerador (confira o diff!)
make api           # regrava api/current.txt depois de uma mudança intencional na API
make race          # go test -race ./... (o TestConcorrencia é quem lhe dá concorrência)
make fuzz          # 20s em cada alvo de fuzzing, igual ao CI; FUZZTIME=2m make fuzz para mais
make fuzz-long     # 5 minutos por alvo, antes de uma release
```

O `make test` falha quando a superfície exportada difere do `api/current.txt`. Se a mudança é
intencional, rode `make api` e deixe o diff desse arquivo entrar na revisão: uma linha removida
é quebra para quem usa o framework, e o [`API.md`](API.md) diz o que precisa acontecer antes de
um símbolo sumir.

Falha achada pelo fuzzing cai em `testdata/fuzz/<Alvo>/`. Commite esse arquivo junto com a
correção: é a regressão que impede a volta do defeito.

Go 1.22 ou mais novo. Nenhuma outra ferramenta.

## Estilo

- Código, identificadores, comentários e mensagens de erro em **inglês**. Tudo o que é
  público (site, README, arquivos de comunidade, CLI, scaffold) em **inglês por padrão, com
  tradução para o português do Brasil no mesmo commit** (`/pt` no site, `README.pt-BR.md`,
  `docs/pt-BR/`, tabela em `cmd/trilha/i18n.go`). Specs e constituição em português.
- `gofmt`. Doc comment em todo símbolo exportado.
- Commits pequenos, mensagem no imperativo com prefixo (`feat:`, `fix:`, `docs:`, `chore:`),
  sem trailers de coautoria.

## Pull request

Use o template. Um PR resolve uma issue. Se descobrir outro problema no caminho, abra outra
issue em vez de crescer o PR. Revisão em até uma semana; se passar disso, mencione
`@emersonjoe` na PR.

A `main` é protegida: o PR precisa dos checks de CI verdes, de uma aprovação e de todas as
conversas resolvidas antes de ser integrado (veja [GOVERNANCE.md](GOVERNANCE.md)). Faça
`git rebase main` em vez de merge para manter o histórico linear.

## Documentação

O site em `site/` é um app Trilha; o conteúdo fica em
`site/internal/docs/content/<en|pt>/**.md` (as duas árvores têm as mesmas páginas, por posição). Rode `cd site && go run ../cmd/trilha dev --addr :3010`
para ver as mudanças. Toda mudança visível de comportamento precisa de documentação no mesmo PR.

## Licença

Ao contribuir, você concorda que a sua contribuição é licenciada sob a MIT do projeto.
