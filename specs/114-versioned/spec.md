# Spec 114 — `Versioned[T]`: histórico numerado, publicar e voltar

- **Issue**: [#148](https://github.com/emersonjoe/trilha/issues/148) — a issue é a fonte do escopo.
- **Branch**: `114-versioned`
- **Versão**: 0.93.0

## Por quê

Medido no Acervo: dois lugares diferentes reimplementam "guardar versões e publicar uma" — os
documentos e as definições de workflow, cada um com a sua tabela `..._versions` e as suas três
telas. O padrão é sempre o mesmo: histórico numerado, uma versão corrente, quem e quando, e um
botão para voltar à versão N.

## O que muda

```go
var Modelos = trilha.NewVersioned[Modelo]("modelos", trilha.VersionedOpts{})

n, err := Modelos.Draft(c, id)             // abre o rascunho n+1, cópia da corrente
n, err = Modelos.Save(c, id, m, "ajuste")  // grava no rascunho aberto
err = Modelos.Publish(c, id, n)            // congela n e passa a ser a corrente

m, v, err := Modelos.Current(ctx, id)      // a publicada, ou o rascunho quando não há publicada
m, v, err = Modelos.At(ctx, id, 3)
hist, err := Modelos.History(ctx, id)
n, err = Modelos.Restore(c, id, 3)         // cria n+1 igual à 3
```

- **Publicada é imutável.** `Save` numa versão publicada devolve `ErrVersionFrozen`, com o
  `trilha.Hint` dizendo o que fazer — abrir um rascunho. É a regra que os dois lugares do Acervo
  escreveram, cada um do seu jeito.
- **Cada `Save`, `Publish` e `Restore` audita sozinho.** Quem publicou o quê é a pergunta que
  chega uma semana depois.
- **`ui.VersionList` e `ui.VersionBadge`**: o histórico com quem, quando e a marca da publicada,
  o botão de voltar com `ui.Confirm`, e o diff dos campos que mudaram entre uma versão e a
  anterior — no servidor, sem JavaScript.

## Fora de escopo

- **Anexo em `blob`.** O `SaveFile` da issue precisaria de o pacote raiz conhecer o `blob`, que é
  o contrário da direção das dependências aqui (o `blob` importa o `trilha`). O caminho é o app
  guardar a chave do blob **dentro** do valor versionado, que é uma linha e não uma dependência.
- **Store em SQL.** Memória aqui, tabela na receita do cookbook, como em todos os módulos.
- **`generate crud --versioned` e a coluna do `migrate next`.** Ficam nas issues delas.
- **Aprovação antes de publicar.** É o `approval.On` chamando `Publish`, e é uma linha do app.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `encoding/json`, `sync`, `time` |
| VI — teste primeiro | rascunho, publicar, congelar, restaurar e o diff |
| Uma fonte | o valor é o struct do app; o histórico guarda o JSON dele |

## Tarefas

- [x] T001 Teste que falha: rascunho, gravar, publicar, congelar, restaurar
- [x] T002 `Versioned[T]`, o store em memória e o `ErrVersionFrozen` com `Hint`
- [x] T003 `ui.VersionList` e `ui.VersionBadge`, com o diff no servidor
- [x] T004 Documentação (en + pt) e superfície de API
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.93.0`

## Aceitação

- **SC-001** `Save` sem rascunho aberto numa versão publicada é `ErrVersionFrozen` com o conserto.
- **SC-002** `Publish` congela: a versão publicada não muda mais.
- **SC-003** `Restore(3)` cria uma versão nova igual à 3, e não apaga nada.
- **SC-004** `History` traz quem, quando e qual é a publicada.
- **SC-005** O diff mostra só os campos que mudaram.
