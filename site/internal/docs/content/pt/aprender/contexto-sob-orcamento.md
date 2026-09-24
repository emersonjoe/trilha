---
title: Contexto sob orçamento
description: trilha ctx responde o que o projeto tem em uma leitura, fatiado por trabalho e com preço em tokens estimados — para o agente parar de abrir arquivos para descobrir.
---

Um agente paga cada arquivo que abre para descobrir o que o projeto já tem. O Trilha
responde a pergunta em uma leitura: o [`trilha ctx`](/pt/referencia/ctx) imprime o mapa —
rotas, contratos de API, tipos, o que o setup provê, as receitas instaladas, as convenções
que a árvore realmente usa — e se cobra no rodapé, em tokens estimados.

    trilha ctx
    trilha ctx --json          # o mesmo mapa, para máquina
    trilha ctx --pack login    # o que a receita de login toca, e nada mais
    trilha ctx --pack app --budget 1500
    trilha ctx --pack app --budget 1500 --strict

## O que é um pack

`--pack` fatia o mapa para um trabalho. `app` é o mapa inteiro; qualquer outro nome é de uma
receita instalada, e a fatia guarda só as rotas que ela responde e os arquivos que ela trouxe.
Cada fatia termina com um rodapé que dá o preço da página — `tokens est.`, a quatro
caracteres por token, a mesma taxa de que a [régua](/pt/aprender/a-regua) cobra. É orçamento,
não fatura: estimativa leva `est.`, e nenhum número de release sai delas.

## A ordem em que se corta

Um orçamento corta o mapa na ordem em que a informação fica barata — e nunca corta rotas,
que são o mínimo útil:

1. **Contratos** — os caminhos de API, o que cada handler recebe e responde.
2. **Receitas** — o que está instalado e quais arquivos trouxe.
3. **Convenções** — quais convenções a árvore realmente usa.

O que não coube aparece nomeado na resposta (`cut: contracts, recipes`), e com `--strict` o
corte é falha: `E_CTX_BUDGET` lista o que ficou de fora e manda subir o `--budget`. Seção
vazia é ausência, não corte — um projeto sem receitas não paga nada pela seção.

## A mesma fatia por MCP e llms.txt

Um agente sem shell lê as mesmas fatias do site: o servidor MCP de documentação responde
`get_context(pack)` com as páginas que uma receita toca, cada uma com preço, e
`search_code(query)` com janelas `caminho:linha` das fontes Go das receitas — nunca o arquivo
inteiro. Cada receita tem também um `llms.txt` próprio, linkado no selo da página dela, com
o pacote precificado e a página na íntegra.

## O AGENTS.md tem tamanho vigiado

O arquivo que o `trilha new` escreve para agentes tem quatro seções fixas — o mapa primeiro,
as receitas instaladas, os portões, as regras de leitura estreita — e é testado para ficar
abaixo de 2.500 tokens estimados. Ele aponta para o `trilha ctx` em vez de copiar o mapa,
para não envelhecer como cópia.

## Desafio

`trilha ctx --pack app --budget 10` responde com as rotas e sai 0 sem `--strict`. Por que as
rotas sobrevivem a um orçamento que corta todo o resto, e o que daria errado se um orçamento
apertado pudesse cortá-las também?

:::solution
As rotas são a resposta mínima útil — a única coisa para a qual todas as outras seções
existem. Um mapa sem rotas não é um mapa menor; é nenhuma resposta, e um agente com ele
abriria arquivos do mesmo jeito, que é exatamente o custo que o comando existe para evitar.
Por isso a ordem de corte tem piso: contratos, receitas e convenções podem ir, as rotas não —
e o `--strict` transforma o corte em `E_CTX_BUDGET` para quem chama decidir entre um orçamento
maior e uma resposta parcial, em vez de receber um silêncio que vai custar tokens preencher.
