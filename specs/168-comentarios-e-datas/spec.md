# Spec 168 — Comentários e datas

**Feature Branch**: `168-comentarios-e-datas` | **Created**: 2026-09-26 | **Status**: Entregue (0.149.0)

- **Issues**: #287 (bug, `trilha migrate next`) e #288 (enhancement, `ui`) — as issues são a
  fonte do escopo; as duas vieram da migração do Prosa e não se tocam.
- **Versão**: 0.149.0

## Por quê

- **#287.** O `scan` do `migrate next` rodava as regexes dos sinais no texto bruto do arquivo.
  Um JSDoc dizendo "a transcrição do SpeechRecognition" num módulo puro fazia **C — media
  capture** toda tela que importasse o módulo, mesmo só um tipo. É o falso positivo simétrico
  ao falso negativo da #182, e vale para todo sinal de texto, não só mídia.
- **#288.** `ui.APIKeysTable` e `ui.WebhooksPanel` chamam `ui.Date` por dentro, com o layout do
  kit. Uma tela que escreve data no formato do produto ficava com dois formatos na mesma página,
  sem outra saída além de desenhar a tabela à mão e perder revogar, reativar e o segredo.

## O que muda

1. `scan` procura os sinais num texto com os comentários `//` e `/* */` trocados por espaços
   (`stripComments`, léxico: conhece strings e template literals, preserva quebras de linha e o
   tamanho, então o número de linha do motivo continua certo). Strings ficam: `type="file"` e
   `role="dialog"` são sinais.
2. `APIKeysOpts.DateFormat` e `WebhooksOpts.DateFormat`, do tipo
   `func(*trilha.Ctx, time.Time) h.Node`, desenham as colunas de data no lugar do `ui.Date`.
   Nil mantém o kit; data zero continua sendo "nunca" ou o traço.

```go
ui.WebhooksPanel(c, subs, entregas, ui.WebhooksOpts{Action: "/webhooks", CSRF: trilha.CSRFInput(c),
	DateFormat: diaCurto}) // "11 de set. de 2026"
```

## Fora de escopo

- Tirar strings do texto antes dos sinais: vários sinais moram em atributos JSX entre aspas.
- Mudar o layout padrão pt-BR do `ui.Date`: quebraria quem conta com `02/01/2006` (a #288
  descarta pelo mesmo motivo).
- Contagem de hooks e a diretiva `'use server'` continuam lendo o texto bruto; nenhuma issue
  mostrou erro ali.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo no `go.mod`; `stripComments` é um laço sobre bytes |
| IV — API pública | dois campos novos em structs de opção; aditivo, `api/current.txt` atualizado |
| VI — teste primeiro | os três testes novos falharam antes do código (ver registro) |

## Segurança e privacidade (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras**: o `migrate next` lê código-fonte local, sem executar nada, sem rede — a mudança
  só reduz o que casa. `DateFormat` é código da própria aplicação rodando no render; o kit não
  passa a ela nada além do `Ctx` e do instante que já desenhava.
- **Impacto**: o formatador devolve `h.Node`, então o escape do `h` continua valendo para o que
  ele escrever com `h.Text` (ASVS V1.2 — codificação de saída).
- **Exceções**: nenhuma.

## Tarefas

- [x] **T01** — `testdata/media` ganha `lib/speaking.ts` (JSDoc com `SpeechRecognition`, URL numa
  string) e as telas `resumo` (import de namespace) e `curva` (import de tipo);
  `TestCapturaDeMidiaEhIlha` exige A nas duas; `TestStripCommentsKeepsStringsAndLines`.
- [x] **T02** — `stripComments` no `scan`.
- [x] **T03** — testes de `DateFormat` nas duas peças, depois os campos e o `dateCell`.
- [x] **T04** — `examples/blog/app/webhooks` usa `DateFormat`; `TestCadastrarPelaTelaEReceberOEvento`
  exige a data no formato do produto.
- [x] **T05** — docs bilíngues (`cli`, `webhook`, `auth`), `api/current.txt`, catálogo do `uidoc`,
  CHANGELOG, ROADMAP, versão.

## Aceitação

- **SC-001** — `resumo` e `curva` saem A; `licao` e `ditado` continuam C, com a linha do hook.
- **SC-002** — com `DateFormat`, nenhuma célula de data das duas peças sai como `<time>` do kit;
  sem ele, a saída é a de antes.
- **SC-003** — `make test` verde.

## Registro da implementação

- Antes do código: `curva = C — media capture (lib/speaking.ts:3, whole module)` e o mesmo para
  `resumo`; os testes de `DateFormat` não compilavam; o teste do `blog` falhava com a
  `page.go` antiga.
- `internal/uidoc/catalog.json` foi regravado: o catálogo lista os campos das opções.
- Evidência: `make test` verde.
