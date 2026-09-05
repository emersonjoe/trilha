# Spec 021 — Os cinco atritos do app que já está rodando

- **Issues**: [#15](https://github.com/emersonjoe/trilha/issues/15),
  [#16](https://github.com/emersonjoe/trilha/issues/16),
  [#17](https://github.com/emersonjoe/trilha/issues/17),
  [#18](https://github.com/emersonjoe/trilha/issues/18),
  [#19](https://github.com/emersonjoe/trilha/issues/19) — cada uma traz a medição e o
  código real; aqui só a decisão e o contrato.
- **Branch**: `021-atritos-partiu`
- **Versão**: 0.12.0

## Por quê

As cinco issues mais antigas são a mesma leva: um app com 76 rotas **já rodando** na Trilha
relatando o que dói *depois* da adoção. As #5–#14 tiraram o que travava a entrada; estas são
o atrito de quem ficou, e todas têm a mesma forma — a Trilha decide sozinha algo que o app
precisa decidir (onde a configuração falha, o que vai para o log, como o disco vira URL,
quando um aviso é aviso). Foram numa release só porque o custo do ritual de release é o
mesmo para uma ou cinco.

## O que muda

```go
func Config(cfg *trilha.Config) error       // #15: a forma sem retorno continua válida

cfg.Mounts = map[string]fs.FS{"/icones/": icones}                    // #17
cfg.LogRequest = func(c *trilha.Ctx, status int, d time.Duration) bool { ... }  // #16
```

- **#15** — o scanner aceita as duas assinaturas de `Config` (`FuncDecl.Type.Results` diz
  qual é) e o gerador emite `if err := app.Config(&cfg); err != nil { trilha.Fatal(err) }`
  para a com erro. É o mesmo par que `Setup` (com erro) e `Layout` (sem) já formavam.
- **#16** — `LogRequest` decide por requisição, com a resposta pronta. `nil` loga todas.
- **#17** — `Mounts` casa antes de `Public`, do prefixo mais longo para o mais curto, e
  **cai para a próxima montagem** quando o prefixo casa mas o arquivo não está lá; sem isso
  toda montagem teria de ser exaustiva. `StaticCacheControl`, `StaticHeaders` e `Asset`
  valem igual, e o `name` entregue ao `StaticHeaders` passa a ser o da URL — é o que
  distingue uma montagem da outra.
- **#18** — `trilha gen --check` gera em memória, compara e sai com 1 mostrando as linhas que
  divergem; o arquivo gerado ganha `//go:generate trilha gen`.
- **#19** — o aviso de `TRILHA_SECRET` sai da subida e vai para o `SetSigned`, uma vez por
  cookie, dizendo qual cookie e qual rota.

## Fora de escopo

- **Carimbar a versão da CLI no arquivo gerado**, que a #18 sugeria. Ele faria toda troca de
  versão sujar o arquivo commitado de todo projeto e faria o `--check` falhar por uma
  diferença que não mudou uma linha de comportamento. O que a #18 queria mesmo — descobrir
  que a CLI está adiante da biblioteca — o `trilha audit` passa a fazer comparando a versão
  da CLI com a do `go.mod`, sem carimbo e sem ruído.
- **`Config.Secret = trilha.Off`**, a alternativa menor da #19: com o aviso no uso, declarar
  o que o app não faz deixou de ter função.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `fs.FS`, `ast`, `sort`, `sync`; nada novo no `go.mod` |
| IV — convenção nova tem teste, uso no exemplo e integração | `examples/blog` usa as três formas novas (`Config` com erro, `Mounts` de `internal/icones`, `LogRequest`) e `TestMontagemServeArvoreForaDePublic` cobre |
| V — gerador determinístico e commitado | goldens regravados; `--check` é a garantia de que ele continua em dia |
| VI — teste primeiro | os testes das cinco issues foram escritos antes e falharam por símbolo inexistente |
| VII — compatibilidade | nenhuma assinatura pública mudou; `Config` sem retorno, `Public` sozinho e log sem filtro seguem idênticos |

## Tarefas

- [x] T001 Testes vermelhos: filtro do log, montagem por prefixo com fallback, aviso no uso,
      `Config` com erro no scanner e no gerador, `gen --check` no e2e da CLI
- [x] T002 `Config.Mounts`/`Config.LogRequest`, `staticFile`, `warnOnce`, aviso fora do `New`
- [x] T003 `pkgInfo.results` no scanner, ramo novo no gerador, `//go:generate`, goldens
- [x] T004 `trilha gen --check` com diff, verificação CLI × `go.mod` no `audit`, i18n
- [x] T005 `examples/blog`: `internal/icones`, `Config` com erro, montagem e filtro de log
- [x] T006 Referência (`app`, `cli`, convenções) e capítulo de segurança nas duas locales
- [x] T007 `CHANGELOG`, `version`, `ROADMAP`, `make release VERSION=0.12.0`

## Aceitação

- **SC-001** `Config` devolvendo erro interrompe a subida com a mensagem do app; a forma sem
  retorno gera exatamente o mesmo código de antes.
- **SC-002** Com `LogRequest`, o arquivo filtrado não aparece no log e o 4xx aparece.
- **SC-003** `/icones/x` vem da montagem, `/js/only.js` cai para o `Public`, e um arquivo que
  não existe em nenhuma das duas é 404.
- **SC-004** `trilha gen --check` sai com 0 no projeto em dia e com 1 mostrando a rota que
  falta quando uma pasta é criada sem `trilha gen`.
- **SC-005** Um app em produção que nunca chama `SetSigned` sobe sem nenhum aviso; quem chama
  recebe `ErrNoSecret` e **um** aviso nomeando o cookie.

## Fora da lista: um defeito da 0.11.0

O `trilha audit` aprendeu na 0.11.0 onde fica o segredo em `Cognito(...)`, mas o `authCalls`
continuou procurando só `OIDC`, `EntraID` e `Keycloak` — a checagem nunca rodava para o
Cognito. Corrigido aqui, com teste.
