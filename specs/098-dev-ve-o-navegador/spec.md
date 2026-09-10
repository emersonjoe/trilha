# Spec 098 — o `trilha dev` vê o que o navegador vê

- **Issue**: [#118](https://github.com/emersonjoe/trilha/issues/118) — a issue é a fonte do escopo.
- **Branch**: `098-dev-ve-o-navegador`
- **Versão**: 0.78.0

## Por quê

A 0.67.0 entregou a metade do servidor: o `trilha.Hint`, a página de erro de dev e as recusas que
eram vulnerabilidade. Ficou o item 2 da issue, e é o que dói mais para quem está começando: **o
erro que só o navegador vê**.

O iframe fica cinza e a explicação está no console do navegador, numa linha que ninguém abriu. A
tabela aparece dentro da tabela e não há erro nenhum — a resposta foi 200. O flash aparece na tela
seguinte, do nada. Em três casos o terminal, onde a pessoa está olhando, não diz nada.

## O que muda

O `trilha dev` passa a receber o que o navegador tem para contar, e a imprimir no terminal com o
conserto junto.

**1. O CSP reporta.** Em `Env: Dev`, a política ganha `report-uri /_trilha/csp` e o cabeçalho
`Reporting-Endpoints`; o `trilha dev` responde nesse endereço e imprime:

```
⚠ CSP recusou frame-src: http://localhost:8080/nota.pdf
  em /documentos/12 — use ui.Preview, ou Security.CSPExtra{"frame-src": {"'self'"}}
```

Em produção nada disso existe: uma política que reporta para um endereço que não existe é ruído
no navegador de quem usa o app.

**2. O fragmento que voltou página inteira.** O `ui.live.js` já lê a resposta do `Poll`, do
`Defer` e do `Swap`. Quando ela traz `<html`, ele conta ao `trilha dev` — só em dev, porque só em
dev existe para quem contar — e o terminal diz:

```
⚠ o Poll de /documentos recebeu a página inteira
  a rota respondeu sem olhar c.Fragment(): devolva só o pedaço quando ele for pedido
```

**3. O flash sem redirect.** Este o servidor sabe sozinho: se a resposta não é um desvio e sobrou
mensagem para a próxima requisição, ela vai aparecer numa tela que ninguém relacionou com o
botão. Em dev, o log diz isso na hora, com a rota.

## Fora de escopo

- **Os itens estáticos do `audit`** (Live em rota pública, string sem `max=`, Audit sem Require).
  São análise de código, não de navegador, e cabem numa spec do `audit`.
- **A pasta `docs/errors/` gerada.** Ela pede que cada mensagem tenha código e tabela; é o item 5
  da issue e vale quando os códigos existirem em número.
- **O cenário "corrija este erro" do `bench`.** Vale quando as mensagens estiverem no lugar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net/http` e o script do kit que já existe |
| VII — segurança por padrão | o canal de report só existe em dev, e não muda a política em produção |
| VI — teste primeiro | cada mensagem tem teste: o cabeçalho, o endpoint e o aviso do flash |

## Tarefas

- [x] T001 Teste que falha: CSP com report em dev e sem ele em prod
- [x] T002 O endpoint `/_trilha/csp` no `trilha dev`, com a frase do conserto
- [x] T003 O `report` do `ui.live.js` e o endpoint `/_trilha/report`
- [x] T004 O aviso de flash sem redirect, em dev
- [x] T005 Documentação (en + pt)
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.78.0`

## Aceitação

- **SC-001** Em dev, a resposta traz `report-uri` e `Reporting-Endpoints`; em prod, nenhum dos dois.
- **SC-002** Um relatório de CSP postado no endpoint vira uma linha no terminal com a diretiva, o
  recurso e o conserto.
- **SC-003** Um fragmento que volta com `<html` vira uma linha no terminal com a rota.
- **SC-004** Um flash que sobra numa resposta sem desvio vira aviso no log, em dev.
- **SC-005** Nada disso existe em produção.
