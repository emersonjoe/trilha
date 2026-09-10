# Spec 107 — a receita `share-link`: o link que dá acesso sem dar conta

- **Issue**: [#116](https://github.com/emersonjoe/trilha/issues/116) — a issue é a fonte do escopo.
- **Branch**: `107-receita-share-link`
- **Versão**: 0.87.0

## Por quê

"Manda o link para o cliente ver" é um pedido de todo mês, e a resposta escrita à mão é sempre a
mesma: um id sequencial na URL, sem prazo, sem limite de uso, que continua funcionando dois anos
depois — e que qualquer um adivinha somando um.

O `c.Link`/`c.Claim` existe desde a Fase 6 e resolve isso com assinatura e prazo. Falta a receita:
a rota pública, o botão que cria o link, e a linha que diz por que ela **não** pode estar atrás do
login.

## O que muda

```
$ trilha add share-link
  + app/compartilhado/token_/page.go   a rota pública que recebe o link
  + app/compartilhado/token_/kind.go   Kind = KindPage
  + compartilhado_test.go
```

Quatro decisões, e as quatro são o que se erra:

- **A rota do link não fica atrás de login.** Quem recebe o link é quem não tem conta — é o
  motivo de o link existir. O arquivo diz isso, e o `trilha audit` já reclama de um `Claim` atrás
  de `Require`.
- **O que viaja no token é um id, e não um nome.** O token é assinado, não é secreto: quem tem o
  link lê o conteúdo dele.
- **Prazo sempre.** O padrão é uma hora; um link público sem prazo é uma senha que nunca expira.
- **Uso contado quando importa.** `Uses: 1` para o que só pode ser aberto uma vez, e o arquivo diz
  o que isso custa (guardar o consumo) e o que a alternativa custa (nada, e o link vale até vencer).

## Fora de escopo

- **A tela que lista os links criados.** Um link vivo é um segredo: listá-los é criar uma tela que
  os revela. Quem precisa de revogação guarda o id e recusa por ele, e o `Next` diz a linha.
- **O que o link mostra.** É do app: a receita mostra um exemplo e diz onde trocar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `c.Link` e `c.Claim`, que já são do runtime |
| VI — teste primeiro | a receita escreve um teste, e o e2e da CI aplica ela |
| VII — segurança por padrão | prazo, assinatura, e a rota fora do login por escrito |

## Tarefas

- [x] T001 Teste que falha: `add share-link` escreve os três arquivos
- [x] T002 A receita: rota pública, kind, teste
- [x] T003 O e2e da CI aplicando `share-link` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.87.0`

## Aceitação

- **SC-001** `trilha add share-link` num projeto novo passa `trilha check` sem uma edição.
- **SC-002** Um link criado abre; um token mexido não abre.
- **SC-003** Um link vencido não abre.
- **SC-004** A rota responde sem sessão nenhuma.
