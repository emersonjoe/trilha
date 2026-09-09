# Spec 086 — erros que ensinam: Hint, e recusar cedo o que é sempre errado

- **Issue**: [#118](https://github.com/emersonjoe/trilha/issues/118) — a issue é a fonte do escopo.
- **Branch**: `086-erros-que-ensinam`
- **Versão**: 0.67.0

## Por quê

A issue #118 é uma tabela de treze linhas e cinco propostas. Fazer as cinco de uma vez seria uma
versão que ninguém consegue revisar, e três delas — o relatório de CSP no `trilha dev`, a
detecção de `<html>` em resposta de `Poll`, e o `docs/errors/` gerado — pedem canos que ainda não
existem. Esta spec faz o bloco que não depende de nenhum: **o mecanismo** (`Hint`) e **as recusas
que valem por si**.

Duas linhas da tabela não são só didática, são defeito de segurança, e as duas foram confirmadas
neste repositório antes de escrever qualquer código:

```
REDIRECT status=303 location="https://evil.example/x"   ← c.Redirect com URL absoluta
SECRET CURTO status=200                                 ← Env: Prod com segredo de 5 bytes
```

O primeiro é um open redirect toda vez que o destino vem de um `?next=` — que é exatamente de
onde ele vem, porque é assim que se volta para a página que pediu login. O segundo é uma
aplicação em produção assinando cookie com uma chave que se quebra numa tarde.

Uma terceira linha da tabela já estava feita e vale registrar: o `blob.Files.Serve` de chave
inexistente já responde 404, e não 500.

## O que muda

### `trilha.Hint` — o erro que traz o conserto

```go
return trilha.NewHint("E_REDIRECT_ABSOLUTE", err).
	Fix("Redirect só aceita caminho; para sair do site, RedirectExternal.").
	Doc("/reference/errors")
```

Um `Hint` é um erro comum com três coisas a mais: um **código**, uma frase de **conserto** e um
**link**. Em `Env: Dev` a página de erro mostra as três; em produção mostra o que mostrava —
porque a frase é para quem escreve o código, e quem está do outro lado não escreveu.

O `Hint` embrulha o erro original, então `errors.Is` e `errors.As` continuam funcionando: quem
já tratava um erro não passa a tratar outro.

### `Redirect` recusa endereço absoluto

```go
c.Redirect("/painel")                     // como sempre
c.Redirect(c.Query("next"))               // se vier "https://…", é erro, não redirecionamento
c.RedirectExternal("https://banco.gov.br/pagar")  // quando é de propósito, está escrito
```

Recusar é o único jeito de acertar o caso comum. Uma sanitização — "aceita se o host for o
nosso" — parece mais gentil e é como se escreve um open redirect com mais passos: `//evil.com`,
`https:/\evil.com`, `/\evil.com` e o resto do repertório existem porque cada um passou por uma
checagem que alguém achou suficiente.

O que passa: caminho começando com `/` e que não começa com `//` nem `/\`. Mais nada. O erro é
um `Hint` com o valor recusado, porque a primeira pergunta de quem vê é "recusou o quê?".

### Segredo curto em produção não sobe

`ListenAndServe` recusa antes de escutar, com o tamanho que tem e o comando que gera um bom:

```
trilha: TRILHA_SECRET tem 5 bytes e o mínimo é 32 (E_SECRET_SHORT).
Gere um com: trilha secret
```

É no `ListenAndServe` e não no `New` porque `New` é o que um teste chama: quem sobe um servidor
está subindo, quem monta um `Handler()` numa suíte não está. Em `Env: Dev` é um aviso no log,
uma vez — um segredo de desenvolvimento é um segredo de desenvolvimento.

E `trilha secret` passa a existir: uma linha que imprime 32 bytes aleatórios em base64, porque
"gere um segredo" sem dizer como é meio conselho.

### O `audit` cresce com a linha estática que faltava

- **`Upstream` sem `Timeout`**: o padrão é 30 s, e trinta segundos por requisição pendurada é o
  que derruba o app inteiro quando a API do outro lado fica lenta — e a falha chega como "nosso
  app caiu", que manda todo mundo procurar no lugar errado.

A outra linha que a issue pede aqui — **`ui.Live` em rota sem `Require`** — já existia: o
`liveWithoutAuth` está no `audit` desde a 0.36.0. Vale registrar em vez de escrever de novo; foi
a segunda linha da tabela que se descobriu já feita, junto com o `blob.Files.Serve`.

## Fora de escopo

Fica para specs próprias, e a issue continua aberta para elas:

- **O `trilha dev` observando o browser** — `report-to` de CSP e resposta de `Poll`/`Defer` com
  `<html>`. Precisa de rota nova no servidor de dev e de um canal do browser para o terminal;
  é a metade mais interessante da issue e a que menos cabe junto de outra coisa.
- **`docs/errors/<código>` gerado.** Vale quando houver códigos de runtime suficientes para uma
  página valer a pena; hoje seriam dois.
- **O cenário do `bench/agent`** ("corrija este erro"), que mede se a frase ajuda. Ele mede o que
  esta spec entrega, então vem depois dela e não com ela.
- **`Bind` sem `max=` em campo string.** É heurística: nem todo string vai para o banco, e um
  aviso que erra metade das vezes é um aviso que se aprende a ignorar — o que estraga os outros.
- **`Flash` sem `Redirect`.** É um aviso de runtime que precisa do canal do `dev`, como os dois
  primeiros.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `crypto/rand`, `errors`, `strings` |
| VI — teste primeiro | cada recusa tem teste, e o `//evil.com` e companhia têm um caso cada |
| VII — segurança por padrão | duas vulnerabilidades reais fechadas; a saída deliberada existe e está escrita |
| Mudança incompatível | `Redirect` com URL absoluta passa a ser erro — está no CHANGELOG com o conserto de uma linha |

## Tarefas

- [x] T001 Teste que falha: `Redirect` com URL absoluta e com as variantes de escape
- [x] T002 `Hint`, `RedirectExternal`, e a recusa
- [x] T003 Teste que falha: `ListenAndServe` em prod com segredo curto
- [x] T004 A recusa no boot e o `trilha secret`
- [x] T005 Página de erro de dev mostrando código, conserto e link
- [x] T006 Duas verificações novas no `audit`, com teste
- [x] T007 Documentação: referência de erros en + pt, e a nota de migração
- [x] T008 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.67.0`

## Aceitação

- **SC-001** `c.Redirect("https://evil.example/x")` é erro; `//evil.com`, `/\evil.com` e
  `https:/\evil.com` também.
- **SC-002** `c.RedirectExternal` faz o que o nome diz, e o nome é o registro de que foi de
  propósito.
- **SC-003** `ListenAndServe` com `Env: Prod` e segredo curto devolve erro sem escutar; em dev,
  avisa uma vez e sobe.
- **SC-004** `trilha secret` imprime um segredo que passa na verificação.
- **SC-005** Em dev, a página de erro de um `Hint` mostra código, conserto e link; em produção,
  nada disso vaza.
- **SC-006** O `audit` avisa de `Upstream` sem `Timeout`, e cala para quem não faz proxy.
