# Spec 035 — API pública e política de depreciação

- **Issue**: [#35](https://github.com/emersonjoe/trilha/issues/35) (ROADMAP, §1 e §20)
- **Branch**: `035-api-publica`
- **Versão**: 0.26.0

## Por quê

O projeto está em 0.x e pode quebrar compatibilidade numa versão menor. Isso é legítimo, e
está escrito. O que não está escrito é **o que** pode quebrar: quem usa o Trilha hoje não tem
como saber se `trilha.Ctx` e `ui.Button` têm o mesmo grau de estabilidade, se `auth.Provider`
é para o app chamar ou é encanamento, ou o que acontece quando um símbolo precisa sumir.

Sem essa fronteira escrita, duas coisas ruins acontecem em silêncio. Quem adota se apoia em
algo que era detalhe e leva uma quebra que, do lado de dentro, ninguém chamou de quebra. E do
lado de dentro ninguém percebe que mudou a superfície: renomear um campo de struct exportada é
uma linha de diff que passa por qualquer revisão — o compilador do usuário é que descobre.

A 1.0 não pode ser o momento de descobrir as duas coisas.

## O que muda

### `API.md` (inglês) e `docs/pt-BR/API.md`

O documento que define a fronteira:

- **Coberto pela garantia**: `trilha`, `h`, `ui`, `ai`, `ai/mcp`, `auth`, `cache`, `tmpl` — os
  símbolos exportados, com o comportamento que a documentação descreve.
- **Não coberto**: tudo em `internal/`, o conteúdo do `trilha_gen.go`, a saída exata da CLI, o
  HTML gerado pelo `ui`, o site, os exemplos, os campos que a documentação marca como
  experimentais.
- **O que conta como quebra**: remover ou renomear símbolo exportado, mudar assinatura, tirar
  campo de struct, apertar o que uma função aceita ou afrouxar o que ela garante. Acrescentar
  campo, método ou função não é quebra — e por isso struct de configuração se preenche por
  nome, nunca posicionalmente.
- **Política de depreciação**: `// Deprecated:` no doc comment dizendo o que usar no lugar,
  linha em `Deprecated` no CHANGELOG e **pelo menos uma versão menor de convivência** antes da
  remoção. Em 0.x, remoção sem esse ciclo só com razão de segurança, e escrita como tal.
- **O que a 1.0 muda**: a partir dela, quebra só em versão maior; o ciclo de depreciação passa
  a durar até a próxima maior.

### Teste da superfície exportada

Um arquivo versionado descreve a API pública, uma linha por símbolo:

```text
pkg trilha, type Config struct
pkg trilha, type Config struct, AllowedHosts []string
pkg trilha, func New(Config) *App
pkg trilha, method (*App) Handler() http.Handler
```

- `internal/apisurface` lê os pacotes públicos com `go/parser` e gera esse texto, ordenado e
  determinístico.
- O teste compara com `api/current.txt`; diferença **falha**, mostrando o que entrou e o que
  saiu.
- `make api` regrava o arquivo. O diff dele é a parte da revisão que hoje não existe: quem
  aprova vê a linha que sumiu.
- Símbolo com `// Deprecated:` sai marcado na linha, então a revisão vê também quando algo
  entra em depreciação e quando some sem ter passado por ela.

### Ligações

`GOVERNANCE.md` (duas línguas) aponta para o `API.md`; a constituição ganha a emenda que fixa a
fronteira e o ciclo de depreciação, com nova versão.

## Fora de escopo

- Verificar compatibilidade semântica de verdade (mudança de comportamento sem mudança de
  assinatura) — nenhum comparador faz isso, e prometer que faz é pior que não ter.
- `apidiff` do `golang.org/x/exp`: dependência externa, contra o princípio II.
- Congelar a superfície: em 0.x ela muda; o que a spec entrega é que ela não mude **sem que
  alguém veja**.
- Compatibilidade do arquivo gerado ou do formato do `trilha_gen.go`.

## Constitution Check

| Princípio | Como esta spec o respeita |
|---|---|
| II — só biblioteca padrão | `go/parser`, `go/ast`, `go/printer`; nada de `apidiff`. |
| IV — contrato de handler pequeno e estável | Escrever a fronteira é o que torna "estável" verificável. |
| VI — teste primeiro | A superfície vira golden testado, como o gerador. |

## Aceitação

- **SC-001** `API.md` existe nas duas línguas, listando pacotes cobertos, não cobertos, o que
  conta como quebra e o ciclo de depreciação; `GOVERNANCE.md` aponta para ele nas duas.
- **SC-002** `api/current.txt` descreve a superfície de hoje: funções, tipos, campos exportados
  de struct, métodos, constantes e variáveis dos oito pacotes públicos.
- **SC-003** Renomear um símbolo exportado faz o teste falhar dizendo o que saiu e o que
  entrou; `make api` regrava o arquivo.
- **SC-004** Símbolo com `// Deprecated:` aparece marcado na linha.
- **SC-005** A constituição registra a fronteira e o ciclo, com nova versão semântica.
