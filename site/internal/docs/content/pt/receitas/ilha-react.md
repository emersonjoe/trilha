---
title: Um componente React como ilha
description: Fixe preact e htm, monte um componente que já existe dentro do c.Island e deixe ele falar de volta com island.post.
---

Alguém chega com um componente que já funciona — um seletor de data, um editor, um gráfico — e
a pergunta é onde ele entra. Entra dentro de uma ilha: a página continua vindo do servidor, um
elemento dela é React, e o resto do site não paga por isso.

## O módulo desce uma vez

Não há bundler nem passo de instalação, então o runtime é um arquivo no repositório como
qualquer outro estático:

```bash
trilha vendor preact@10.19.3
trilha vendor htm@3.1.1
```

Isso grava `public/vendor/preact.js` e `public/vendor/htm.js` e anota versão, sha256 e URL no
`vendor.lock`, que é commitado. O `trilha vendor --check` reconfere o hash na CI, e o
`trilha audit` fala alguma coisa se aparecer em `public/vendor/` um arquivo que o lock não
nomeia.

Preact com `htm` é o par que não precisa de build: o `htm` é JSX como template marcado, então o
componente é escrito do jeito que o time escreve e o browser lê direto. Um time em React mesmo
fixa `react` e `react-dom` do mesmo jeito e monta com `createRoot`.

## A página

Nada na página sabe que ali tem React. Ela renderiza o conteúdo de origem e entrega as props ao
componente, como struct para os tipos saírem do outro lado:

```go
// EditorProps is what the editor island gets. It is a struct, not a map, so
// `trilha gen` can write public/islands.d.ts and the editor on the JavaScript
// side knows the same names the page does.
type EditorProps struct {
	// PalavrasPorMinuto is the reading speed the counter divides by.
	PalavrasPorMinuto int `json:"palavrasPorMinuto"`
	// Rascunho is where the island saves without leaving the page.
	Rascunho string `json:"rascunho"`
}
```

Os filhos do `c.Island` são o conteúdo de origem: o formulário que o visitante recebe com o
script bloqueado, ainda a caminho, ou quebrado. Essa é a parte que um app React normalmente não
tem.

## A ilha monta o componente

```js
// public/ilha-editor.js
import { h, render } from "/vendor/preact.js";
import htm from "/vendor/htm.js";

const html = htm.bind(h);

function Editor({ props, island }) {
  const [aviso, setAviso] = useState("");
  const salva = async (texto) => {
    try {
      const r = await island.post(props.rascunho, { corpo: texto });
      setAviso(`salvo · ${r.palavras} palavras`);
    } catch (e) {
      setAviso(e.name === "IslandInvalid" ? e.fields.corpo : "não deu para salvar");
    }
  };
  return html`<${Campo} onSave=${salva} aviso=${aviso} ppm=${props.palavrasPorMinuto} />`;
}

/** @type {import("/islands.d.ts").IslandMount<"/ilha-editor.js">} */
export default function (el, props, island) {
  render(html`<${Editor} props=${props} island=${island} />`, el);
}
```

O `island` é o terceiro argumento que o carregador passa: o `island.post` manda JSON com o
token do CSRF junto, um `422` volta como `IslandInvalid` cujo `.fields` é o mesmo objeto que o
formulário mostraria, e o `island.signal` é abortado quando o elemento sai da página — que é
justamente onde a limpeza de um `useEffect` vazaria.

O tipo `IslandMount` vem do `public/islands.d.ts`, escrito pelo `trilha gen` a partir das
chamadas `c.Island` em `app/`. O editor então confere que `props.rascunho` existe e que
`props.palavrasPorMinuto` é número, sem ninguém escrever os tipos duas vezes.

## A rota do outro lado

Do outro lado tem um `route.go` como qualquer outro. Ele é API — os erros dele são
problem+json, que é o que a ilha lê — mas o cliente dele é a página, com os cookies da página,
então ele pede o token:

```go
func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error {
	return trilha.RequireCSRF(c, next)
}
```

O `c.BindJSON` valida o corpo com as mesmas tags `validate` do formulário, e o `FieldErrors`
que ele devolve vira o `422` que a ilha já sabe mostrar.

## O que isso não é

Não é um SPA com backend em Go. Roteamento, layouts, navegação e todas as outras telas
continuam no servidor; React é um elemento de uma página, e a página funciona sem ele. Se a
resposta para "quais partes são ilhas?" for "todas", o que está sendo construído é um SPA, e a
Trilha é a ferramenta errada — isso é uma resposta de verdade, não uma funcionalidade que
falta.
