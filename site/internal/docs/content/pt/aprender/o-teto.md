---
title: O teto
description: Onde a troca de pedaço deixa de ser a ferramenta certa, e como atravessar sem virar SPA.
---

Todo framework tem um teto. Os que merecem confiança dizem onde ele fica.

O modelo do Trilha — o servidor renderiza HTML, um link ou um formulário pede um pedaço, o
navegador troca aquele elemento — cobre a maior parte de uma aplicação. Não toda. Esta página
é sobre a parte que ele não cobre, como reconhecê-la antes de você estar três dias brigando
com ela, e o que fazer quando chegar lá.

## O que a troca de pedaço cobre

Se a tela muda porque **algo aconteceu no servidor**, o fragmento é a resposta inteira: uma
lista filtrada, um formulário que valida, uma linha apagada, um painel que abre já com dados,
uma tabela que pagina. O estado mora em um lugar só, a URL continua significando o que diz, e
não há store para manter em dia com o DOM.

Esses são os 90%, e vale defendê-los: uma troca não tem etapa de hidratação, então não tem bug
de hidratação.

## Onde o teto fica

O teto não é sobre o quanto a tela parece **complicada**. É sobre **quem é dono da verdade
enquanto a pessoa interage**.

Você está acima do teto quando, mesmo por um instante, só o navegador sabe o que está na tela:

- **Um gesto contínuo.** Arrastar para reordenar, redimensionar um painel, desenhar num
  canvas, recortar uma imagem. Entre apertar e soltar o ponteiro não cabe ida ao servidor.
- **Estado que precisa sobreviver a uma troca.** Um comentário digitado pela metade, a posição
  de rolagem dentro de uma lista virtualizada, um combobox aberto com filtro digitado.
- **Duas pessoas ao mesmo tempo.** Edição colaborativa, cursores ao vivo, qualquer coisa em
  que a mudança de outro cliente precise chegar sem pisar no que você está fazendo.
- **Um ritmo que a rede não acompanha.** Sessenta quadros por segundo, áudio, laço de jogo.

Um teste curto: *se eu desligasse a rede agora, a tela ainda teria que responder?* Se sim,
aquela parte é do cliente.

## Atravessando: uma ilha

Uma ilha é um pedaço da página que um módulo JavaScript assume, com props que o servidor
calculou e o HTML que o servidor já renderizou como recuo. Tudo em volta continua vindo do
servidor.

```go
c.Island("/ilha-ordem.js", map[string]any{"ordem": slugs},
    h.Input(h.Type("hidden"), h.Name("ordem"), h.Value(strings.Join(slugs, ","))),
    h.Ol(h.Data("lista", ""), linhas...),
)
```

```js
// public/ilha-ordem.js — um módulo, sem bundler, sem etapa de build
export default function (el, props) {
  const lista = el.querySelector("[data-lista]");
  const campo = el.querySelector('input[name="ordem"]');
  // o arrastar acontece aqui, no navegador, que é onde ele tem que acontecer
  const grava = () => { campo.value = linhas().map((li) => li.dataset.slug).join(","); };
}
```

O exemplo inteiro — o arrastar, o caminho pelo teclado, o handler que salva — está em
[`examples/blog`, rota `/blog/ordem`](https://github.com/emersonjoe/trilha/tree/main/examples/blog/app/blog/ordem).

## As regras que impedem a ilha de virar uma SPA

**O servidor continua dono dos dados.** A ilha escreve num campo do formulário e para por aí.
Quem salva a ordem é o mesmo `POST → redirect → GET` do resto do app — não um `fetch` privado
para um endpoint privado com um formato privado. No instante em que uma ilha começa a
persistir por conta própria, você tem duas fontes de verdade e um bug de sincronização à
espera.

**As props são a verdade inicial.** O servidor renderiza a ordem; a ilha a recebe. Um módulo
que carrega tarde, ou que não carrega, não pode deixar a página mostrando algo que nunca foi
verdade.

**O recuo é HTML de verdade, não um spinner.** Os filhos de `c.Island` são a cara da página
sem o módulo. Se aquilo é uma caixa vazia, a ilha não é aprimoramento: é exigência com passos
a mais.

**Não tire o teclado.** Arrastar não é alcançável pelo teclado. O exemplo mantém botões ↑ ↓
que enviam um passo pelo mesmo handler, e a ilha não mexe neles. Ilha que troca acessibilidade
por acabamento é piora.

**Uma ilha, um trabalho.** Três ilhas que cuidam de uma coisa pequena cada são mais fáceis de
apagar do que uma que cuida da tela inteira.

## Ilha e troca compõem

Uma ilha monta ao entrar no documento, tendo vindo com a página inteira ou dentro de um
fragmento trocado. Você pode trocar um painel que contém uma ilha e ela começa; pode trocar o
painel fora e o módulo vai junto.

> Antes da 0.40.0 isso só valia quando a página já tinha alguma ilha. A ilha que chegava num
> fragmento numa página sem nenhuma ficava parada e muda — o loader vinha junto como
> `<script>`, e o DOM não executa script inserido assim.

## Quando nem a ilha basta

Seja honesto com o formato do seu app. Se a **maioria** das telas está acima do teto — um
editor de diagramas, uma planilha, uma estação de áudio —, então você não está escrevendo um
app com algumas partes interativas: está escrevendo uma aplicação de cliente, e um framework
de cliente vai servir melhor que um monte de ilhas.

O Trilha continua útil ali: sirva a casca, a autenticação, as páginas de configuração e a API
pelo Trilha, e dê ao app de cliente a rota dele. O que não vale é fazer uma ilha crescer até
virar um framework que ninguém escolheu.

## O que isso custa, sem maquiagem

Uma ilha é JavaScript seu: sem tipos compartilhados com o servidor, sem compilador conferindo
que `props.ordem` existe, e com um segundo lugar onde um bug pode morar. É o preço de
atravessar o teto, e é por isso que vale nomear o teto — para você pagar esse preço nas telas
que precisam, e não nas outras noventa.

## Desafio

A ilha da ordem escreve a ordem inteira num campo escondido, então nada é salvo até a pessoa
apertar o botão — e, se ela sair da página, o arrastar se perde. Faça a ordem salvar sozinha,
um instante depois que o arrastar para, sem dar à ilha um endpoint próprio e sem quebrar o
botão para quem não tem JavaScript.

:::solution
```js
let t;
const salvaSozinho = () => {
  clearTimeout(t);
  t = setTimeout(() => campo.form.requestSubmit(), 800);
};
lista.addEventListener("dragend", salvaSozinho);
```
O `requestSubmit()` envia o mesmo formulário, pelo mesmo handler, com o token de CSRF que já
está nele — então não há segundo endpoint nem segundo formato para manter em dia. Ponha
`ui.Swap("lista")` no formulário e a resposta volta como fragmento em vez de redirect, que é o
que impede a página de pular debaixo do ponteiro.

O atraso importa pelo mesmo motivo que o limiar do indicador: o `dragend` dispara a cada
solta, e quem reordena cinco linhas solta cinco vezes. Esperar a pessoa parar transforma cinco
pedidos em um.
:::
