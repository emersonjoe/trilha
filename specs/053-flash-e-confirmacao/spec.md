# Spec 053 — o aviso que sobrevive ao redirect e a confirmação antes de destruir

- **Issues**: [#66](https://github.com/emersonjoe/trilha/issues/66),
  [#58](https://github.com/emersonjoe/trilha/issues/58)
- **Branch**: `053-flash-e-confirmacao`
- **Versão**: 0.39.0 (com as specs 051 e 052)

## Por quê

As duas issues saíram da mesma tarde: portar a primeira tela de CRUD de um app React
(Acervo) e um CRM (Farol) para a Trilha. Não faltou nada grande — faltaram as três coisas
que todo formulário tem e que o framework deixava para o app inventar.

1. **O aviso não sobrevive ao redirect.** O padrão da Trilha é `POST → Redirect(303) → GET`.
   O `ui.Toast` existe, mas é um nó que a *página seguinte* teria de saber renderizar: quem
   apagou o documento na requisição anterior não tem como contar isso para a próxima. Cada
   app reescreve o mesmo cookie de flash — e o de fragmento, onde não há redirect nenhum.
2. **Confirmar antes de destruir é um diálogo montado à mão por botão.** O próprio
   `examples/blog` prova: a página de um post tem `ui.DialogTrigger` + `ui.Dialog` +
   `ui.DialogFooter` + um `<form>` dentro do diálogo, quinze linhas para perguntar "apagar?".
   `onclick="return confirm()"` não é saída: a CSP com nonce não deixa script inline.
3. **Quatro atributos de formulário só saem por `h.Attr`.** `maxlength`, `minlength`,
   `autocomplete` e `inputmode` aparecem ao lado de `Pattern` e `Required`, que são funções.
   Num formulário em que tudo é função e um atributo é string solta, a string é a que
   ninguém revisa: `h.Attr("maxlenght", "30")` compila e não faz nada.

## O que muda

### `c.Flash(kind, text)` e `c.Flashes()`

```go
func POST(c *trilha.Ctx) error {
	if err := repo.Delete(c.Context(), c.Param("id")); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "Documento excluído")
	return c.Redirect("/docs")
}
```

- Grava num cookie **assinado** de vida curta (a infraestrutura do `SetSigned` já existe),
  lido e apagado na requisição seguinte. Sem `TRILHA_SECRET`, o aviso não sai e o app avisa
  uma vez no log — a mesma decisão do `SetSigned`, e nunca um aviso não assinado.
- Vários flashes por requisição; texto é dado, escapado por quem renderiza; sem HTML.
- Em resposta de **fragmento** (`c.Fragment() != ""`) não há redirect: os avisos vão no
  cabeçalho `Trilha-Flash` e o `ui.js` os mostra. A chamada no handler é a mesma.
- `c.Flashes()` devolve o que veio e apaga o cookie; chamá-la duas vezes na mesma
  requisição devolve a mesma lista, porque um layout não sabe quem já leu.

### `ui.Flashes(c)`

O `ui.Toaster()` que o layout já tem vira `ui.Flashes(c)`: mesmo contêiner, com um
`ui.Toast` por aviso. É uma linha no layout — e é a linha que o `trilha new` passa a
escrever sozinho.

### `ui.Confirm(title, description)`

```go
h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
	ui.Confirm("Apagar este post?", "Não dá para desfazer."),
	ui.Submit(ui.Destructive(), h.Text("Apagar")))
```

- Põe `data-ui-confirm` no formulário; o `ui.js` intercepta o envio, monta o `<dialog>` do
  kit (título, descrição, cancelar/confirmar, `Escape` fecha, foco no cancelar) e só então
  deixa o formulário seguir — inclusive quando ele é um formulário de fragmento.
- Sem JavaScript o formulário envia direto. Isso é dito na documentação em vez de escondido:
  quando o envio direto não serve, a saída é uma página de confirmação (`GET .../apagar`
  que renderiza o formulário), que funciona com e sem script.

### `h.Maxlength`, `h.Minlength`, `h.Autocomplete`, `h.Inputmode`

Quatro linhas em `h/attrs.go`, no formato das vizinhas. Junto vai `h.Attrs(...)`: o
`ui.Confirm` põe dois atributos no mesmo formulário, e um `h.Group` deles renderizaria no
corpo em vez da tag de abertura — o `h` não tinha como um componente devolver mais de um
atributo.

## Superfície

| Onde | O quê |
| --- | --- |
| `flash.go` (runtime) | `Flash`, `c.Flash`, `c.Flashes`, cabeçalho `Trilha-Flash` |
| `ui/ui.go` | `Flashes(c)`, `Confirm(title, desc)`, `FlashSuccess`/`FlashError`/`FlashInfo` |
| `ui/assets/ui.js` | confirmação no envio; `Trilha-Flash` na resposta de fragmento |
| `h/attrs.go`, `h/node.go` | quatro atributos e `h.Attrs`, que junta atributos num nó só |
| `internal/scaffold` | `ui.Flashes(c)` no layout do `trilha new` |
| `examples/blog` | apagar post com `ui.Confirm` e `c.Flash` (quinze linhas viram três) |

## Fora de escopo

- Fila de flash entre usuários ou em servidor: o cookie é do navegador que fez a ação.
- `ui.Confirm` com `Href` de página de confirmação: a página é do app, e a receita mostra
  como escrevê-la. O que entra aqui é o atributo e o diálogo.
- Tradução do texto do diálogo: título e descrição vêm do app, que é onde a língua está. O
  botão que confirma repete o rótulo do que foi apertado; o outro diz `Cancel`, ou o que
  estiver em `h.Data("ui-confirm-cancel", "…")`.

## Constitution Check

- **Convenção nova em `app/`**: nenhuma — não muda o varredor.
- **Zero dependências**; o `ui.js` cresce 2,8 KB (12,0 → 14,8 KB) e o teto do teste sobe de
  12 para 16 KB: o diálogo é montado em JavaScript porque a CSP proíbe script inline.
- **Inglês no código e no público, pt-BR na spec**; docs nas duas línguas no mesmo commit.
- Rota no `examples/blog` e teste de integração: apagar um post.

## Critérios

- **SC-001** `c.Flash` + `Redirect` → a página seguinte traz o toast, e o cookie é apagado.
- **SC-002** Dois flashes na mesma requisição saem os dois, na ordem em que foram escritos.
- **SC-003** Cookie de flash adulterado é ignorado, sem erro para o usuário.
- **SC-004** Sem `TRILHA_SECRET`, `c.Flash` não escreve cookie e avisa uma vez no log.
- **SC-005** Em resposta de fragmento, o aviso sai no cabeçalho `Trilha-Flash` e não no
  cookie.
- **SC-006** `c.Flashes()` chamada duas vezes devolve a mesma lista.
- **SC-007** Texto de flash com `<b>` chega escapado ao HTML.
- **SC-008** `ui.Confirm` põe `data-ui-confirm` e a descrição no formulário; o `examples/blog`
  apaga um post por ele, com e sem JavaScript (o teste de integração é o sem).
- **SC-009** Os quatro atributos do `h` renderizam o par nome/valor certo.
- **SC-010** `make test` verde; `trilha new` nasce com o layout que mostra flashes.
