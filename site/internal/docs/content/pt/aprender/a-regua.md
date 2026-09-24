---
title: A régua — quanto custa uma feature para um agente
description: O Trilha mede o que um agente de IA gasta para criar uma feature, contra Go puro, com prompts congelados e baselines commitados. Os números vivem em /pt/custos.
---

O Trilha faz uma promessa sobre agentes: criar uma feature custa **menos da metade** do que a
mesma tarefa custa em Go puro. Uma promessa assim vale o que vale a prova, então a prova faz
parte do repositório — [a régua](https://github.com/emersonjoe/trilha/tree/main/bench/agent)
(`bench/agent`) e a série publicada em **[/pt/custos](/pt/custos)**.

## O que é medido

Oito cenários, cada um uma tarefa que um agente faz de verdade — adicionar uma rota de API, um
formulário, paginação, um login; consertar três erros plantados. Cada cenário é medido **dos
dois lados**: como o Trilha pede, e como a mesma tarefa em Go puro (`net/http` +
`html/template`), com o baseline commitado no repositório. A economia de um cenário é
`1 − tokens(Trilha)/tokens(Go puro)`; a página mostra a média e cada cenário, com rodadas e
contagem de tokens.

## As regras que mantêm a régua honesta

- **Os prompts são congelados** antes da primeira medição, um por lado, com o SHA-256
  registrado em `CHECKSUMS.txt`. Mudar um prompt reabre a série.
- **Só número medido é publicado.** Medianas das rodadas que saíram verdes, três execuções por
  lado; estimativa leva o rótulo `est.` e esta página não tem nenhuma.
- **A economia nunca é armazenada** — ela é derivada dos dois lados onde quer que se leia,
  inclusive em [/pt/custos](/pt/custos).
- **Regressão é bug**: uma release falha no gate quando qualquer cenário cai mais de cinco
  pontos, qualquer cenário fica abaixo de 60%, ou a média perde a meta do marco.

A metodologia, a tabela por cenário e os números atuais estão em
**[/pt/custos](/pt/custos)**; o capítulo de [desenvolvimento agéntico](/pt/aprender/agentico-cloud)
trata de rodar agentes em projetos Trilha depois que a feature existe.

## Desafio

A série de hoje é honesta por estar vazia: a página diz que a primeira medição está pendente.
Leia [o executor da régua](https://github.com/emersonjoe/trilha/tree/main/bench/agent) e
responda sem rodar nada: qual dos dois lados de um cenário deve gastar mais tokens, e o que
significaria os dois lados saírem iguais?

:::solution
O baseline deve gastar mais — esse é o propósito da régua: a mesma tarefa, pedida a um
framework cujas convenções, receitas e kit existem para o agente não escrever o scaffolding à
mão. Gastos iguais significariam que o framework não acrescentou nada que o agente alcançasse:
nem uma falha (a tarefa continuaria feita), nem uma economia, e o cenário estaria medindo Go
contra Go. Essa leitura — igual é o alarme, não um empate — é por que um cenário precisa de
baseline, e por que o `bench-agent-verify` trata regressão como bug e não como ruído.
