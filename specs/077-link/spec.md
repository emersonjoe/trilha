# Spec 077 — O link que funciona sem login

- **Issue**: #106 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `077-link`
- **Versão**: 0.59.0

## Por quê

Três fluxos de toda aplicação interna acontecem sem login: alguém de fora preenche um
formulário, alguém confere um documento por um código, alguém responde a um pedido que chegou
por e-mail. O framework tinha o primitivo — `Signer`, `c.Signed` — e faltava o padrão. O que se
escreve sem ele é uma string aleatória numa tabela, em claro, sem prazo.

## O que muda

`c.Link`, `c.Claim`, `Link.Consume`, `LinkOpts`, `LinkStore`, `Config.Links`. O contrato está na
referência do `Ctx` e na receita nova, nas duas línguas. O que vale registrar:

**Com `Uses: 0` não há estado nenhum.** Nem linha, nem consulta, nem limpeza: conferir é checar
uma assinatura. É o caso do código de verificação de um documento, e é o que faz o fluxo comum
não precisar de tabela — o oposto do que a primeira versão de qualquer app faz.

**Ler não é gastar.** O `Claim` confere que sobrou uso; só o `Consume` tira um, e depois do
trabalho. Senão um recarregar queimaria o link de quem ainda está preenchendo, e um erro de
validação custaria o convite. Foi a primeira coisa que escrevi errado — tinha um "peek" que
gastava um uso a cada abertura — e o teste do GET que abre três vezes é o que fixa isso.

**Uma resposta só para as quatro falhas.** Assinatura, fim, prazo e usos respondem o mesmo 404.
Dizer a um estranho qual das quatro aconteceu é dizer o quão perto ele está.

**O fim faz parte do token.** Link de formulário não abre verificação, mesmo assinado pela mesma
app com a mesma chave.

**O que vai no link é assinado e não é secreto**, dito no doc, na referência e na receita — e com
teste, para ninguém descobrir isso de outro jeito.

## Desvios da issue

- **O bloqueio por tentativa é um orçamento que se recompõe, não uma hora de bloqueio.** Bloquear
  por endereço transforma uma pessoa desastrada atrás do NAT de um escritório numa queda para
  todo mundo atrás dele. A propriedade que importa — adivinhar ficar inviável — é a mesma, e o
  mecanismo é o `trilha.Limiter` que já existe, em vez de um segundo com estado próprio.
- **`E_CLAIM_BEHIND_LOGIN` no `gen`** — é leitura estática de middleware por rota, a mesma dívida
  da [#124](https://github.com/emersonjoe/trilha/issues/124), e entra com ela.
- **`Config.Links` não é a `auth.Store`** que a issue sugeriu: `auth` importa `trilha`, então a
  interface tem de morar aqui. São dois métodos, e o difícil (`Consume`) é atômico de propósito.

## O que o teste corrigiu no desenho

`TTL` negativo virava uma hora em silêncio. Um chamador que calcula "até o fim do dia" depois da
meia-noite ganharia um link válido a partir de uma conta errada; agora é erro. Zero continua
sendo o padrão de uma hora.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `crypto/rand`, `encoding/json`, e o `Signer` que já existia. |
| III — rota no exemplo | `/convite/{token}` no `cadastro`, aberto sem sessão, com e2e do fluxo inteiro. |
| IV — superfície pequena | Dois métodos no `Ctx`, um no `Link`, uma interface de dois. |
| V — inglês no código, pt-BR junto | Receita nova e referência nas duas línguas no mesmo commit. |
| VI — teste primeiro | Abre sem sessão; não serve para outro fim; as cinco formas de inválido respondendo igual; um uso só gasto no `Consume`; sem limite não guarda estado; errar muito custa; o token cabe num segmento de URL; dado grande demais e prazo negativo recusados. |
| VII — segurança por padrão | Assinatura com rotação, prazo obrigatório, resposta uniforme, orçamento contra força bruta, e o "não é secreto" escrito nos três lugares. |

## Tarefas

1. `link.go`: `Link`, `Claim`, `Consume`, `LinkOpts`, `LinkStore`, memória. ✅
2. `Config.Links` e o orçamento de tentativas no `App`. ✅
3. `examples/cadastro`: o convite na lista e a rota pública. ✅
4. Receita "Link público" en e pt; referência do `Ctx`. ✅
5. `api/current.txt`. ✅
