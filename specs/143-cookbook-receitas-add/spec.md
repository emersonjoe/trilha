# Spec 143 — cookbook aponta para as receitas

> Mudança de conteúdo do site (`site/internal/docs/content`), sem convenção nova em `app/` e
> sem mudança de API pública: spec curta.

- **Issue**: #193 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `143-cookbook-receitas-add`
- **Versão**: 0.122.0

## Por quê

Sete páginas do cookbook (`database`, `sessions`, `uploads`, `files`, `email`, `tasks`,
`webhooks`) ensinam, linha por linha, exatamente o que `trilha add store|login|blob|mail|
tasks|webhooks` já escreve no projeto. Quem chega numa dessas páginas por uma busca não
descobre o atalho — a receita só aparece como uma linha na tabela de
`reference/cli.md#trilha-add`. E o que decide *quais* receitas um projeto novo já tem
(`trilha new --with`) e o que acontece quando uma receita depende de outra (`Needs`) não têm
uma linha em lugar nenhum. `reference/store.md` já resolve isso para a receita `store`: a
página de referência abre dizendo o que o comando escreve; falta o mesmo gesto nas sete
páginas de cookbook, com a diferença de que ali o comando é o atalho e a página continua
sendo o caminho de quem quer entender.

## O que muda

Em cada uma das sete páginas de cookbook (en e pt), logo depois da introdução, uma seção nova:

```markdown
## Or: `trilha add <recipe>`

<saída real de `trilha add <recipe> --dry-run`, colada>

<o que a página ensina além do que o comando escreve, em duas ou três frases>
```

O texto explica três coisas: o que o comando grava (arquivos e a linha marcada em
`app/setup.go`), o que a página ensina a mais do que a receita (o raciocínio, a variação, o
caminho que a receita não cobre) e quando vale escrever à mão em vez de rodar o comando.
Mapeamento página → receita: `database` → `store`, `sessions` → `login`, `uploads` e `files`
→ `blob`, `email` → `mail`, `tasks` → `tasks`, `webhooks` → `webhooks`.

`cookbook/index.md` (en e pt) ganha uma coluna "Recipe"/"Receita" na tabela, com o nome da
receita nas linhas que têm uma e vazia nas que não têm.

`reference/cli.md` (en e pt), seção `trilha new --template`: mais um exemplo de `--with` com
mais de uma receita (`trilha new loja --with login,blob,mail`). Seção `trilha add`: um
parágrafo nomeando `Needs` como o mecanismo que faz uma receita recusar quando a que ela
precisa não está no projeto (hoje o texto descreve o efeito caso a caso — `users`, `profile`,
`tenant`, `permissions`, `approvals` — sem nomear o mecanismo comum).

## Fora de escopo

- Novo texto para as dez receitas que não têm página de cookbook equivalente (`audit`,
  `api-keys`, `approvals`, `connections`, `permissions`, `profile`, `search`, `settings`,
  `share-link`, `tenant`, `users`) — nenhuma issue pede isso.
- Mudança de comportamento do `trilha add`, do `Needs` ou do `--with`: só a documentação do
  que já existe.
- Mudança no `trilha add store` / `reference/store.md`, que já serve de molde.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Mudança é só conteúdo Markdown do site; nenhuma dependência nova, `TestNoExternalDeps` intocado. |
| VI — teste primeiro | `TestCookbookLinksRecipe` (site/internal/docs) falha antes das seções existirem — cada uma das sete páginas, nas duas línguas, tem que mencionar `trilha add`; `reference/cli.md` tem que mencionar `Needs`. Escrito antes do conteúdo. |
| VII — segurança por padrão | Não se aplica: nenhum comportamento de runtime muda. |

## Tarefas

- [ ] T001 Teste que falha: `site/internal/docs/cookbook_recipes_test.go` — as sete páginas
      (en+pt) contêm "trilha add", `reference/cli.md` (en+pt) contêm "Needs" e "--with".
- [ ] T002 Rodar `trilha add store|login|blob|mail|tasks|webhooks --dry-run` num app de
      exemplo e colar a saída real na seção de cada página.
- [ ] T003 Escrever a seção "Or: `trilha add <recipe>`" / "Ou: `trilha add <receita>`" nas
      quatorze páginas; coluna de receita em `cookbook/index.md` e `receitas/index.md`;
      parágrafos novos em `reference/cli.md` e `referencia/cli.md`.
- [ ] T004 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha de versão do
      `ROADMAP.md`.
- [ ] T005 `make test` verde e `scripts/release.sh 0.122.0 --issues "193"`.

## Aceitação

- **SC-001**: `go test ./site/...` cobre as quatorze páginas e `reference/cli.md`/
  `referencia/cli.md` mencionando a receita correspondente e `Needs`.
- **SC-002**: `make test` verde (gofmt, vet, todos os pacotes, e2e da CLI).
- **SC-003**: as saídas de comando coladas nas páginas são as mesmas que `trilha add <r>
  --dry-run` produz na versão 0.122.0 (conferido rodando o comando de novo).
