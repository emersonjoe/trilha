# Plano — spec 053

## Fatos que decidem o desenho

1. **O cookie assinado já existe.** `c.SetSigned`/`c.Signed` fazem HMAC, validade e o aviso
   único quando falta `TRILHA_SECRET`. O flash é um valor pequeno passando por eles — nada
   de formato novo de assinatura.
2. **O valor precisa caber num cookie.** JSON + base64 sem padding: sem `|` (o separador do
   `Signer`), sem espaço, sem acento cru. O limite é 4 KB por cookie, então a lista é
   limitada (5 avisos, 200 runas cada) e o mais antigo é descartado — um aviso perdido é
   melhor do que um `Set-Cookie` que o navegador recusa inteiro.
3. **O fragmento não redireciona.** `c.Fragment()` já diz se a resposta é um pedaço de
   página. Nesse caminho o cookie não serve (ninguém vai navegar), e o cabeçalho serve: o
   `ui.js` já lê `Trilha-Location` na mesma função, então `Trilha-Flash` é uma linha ao lado.
4. **O `ui` importa o `trilha`, e não o contrário.** Por isso `Flash` mora no runtime com
   `kind` como string, e as constantes (`FlashSuccess`) moram no `ui`, onde está o toast que
   entende o nome.
5. **`ui.Toast` já escapa.** O texto vai por `h.Text`; a única regra nova é não aceitar HTML.
6. **O `ui.js` intercepta envio em duas etapas.** O ouvinte de fragmento roda no bubbling; a
   confirmação entra na **captura**, cancela o envio, e depois de confirmado reenvia com
   `requestSubmit()` e uma marca, que o próprio ouvinte de confirmação vê e deixa passar.
   Assim o formulário de fragmento continua sendo de fragmento.
7. **O `<dialog>` do kit já tem CSS.** O diálogo da confirmação é montado com as mesmas
   classes (`ui-dialog`, `ui-dialog-title`, `ui-dialog-footer`), então não entra CSS novo.

## Corte

- `c.Flash` não devolve erro: o handler que acabou de apagar um documento não tem o que
  fazer com "o aviso não foi gravado", e o `SetSigned` já registra a causa uma vez.
- `c.Flashes()` apaga o cookie ao ler. Ler duas vezes devolve o mesmo, porque a lista fica no
  `Ctx` — um layout não sabe se alguém leu antes dele.
- A confirmação é do formulário, não do botão: é o formulário que envia, e é nele que a CSP
  não estorva.

## Fato que só apareceu implementando

8. **O aviso não pode ser escrito na hora.** Se o `c.Flash` gravasse o cookie na chamada, o
   `ui.Flashes(c)` do layout — que roda depois, na mesma requisição — apagaria com o
   `ClearCookie` o que acabara de ser gravado, e a mensagem sumiria sem ninguém ver. Então o
   `Flash` só enfileira; quem escreve é um gancho no `responseWriter`, uma vez, logo antes de
   o cabeçalho sair. Quem renderizou os avisos esvazia a fila e nada é levado adiante. O
   mesmo gancho decide entre cookie e cabeçalho, e por isso enxerga o `Trilha-Location`: um
   fragmento que manda navegar precisa do cookie, não do cabeçalho.

## Arquivos

| Arquivo | Mudança |
| --- | --- |
| `flash.go`, `flash_test.go` | o tipo, os dois métodos, o cabeçalho e os limites |
| `ctx.go` | campos do `Ctx` (pendentes e lidos) |
| `h/attrs.go`, `h/h_test.go` | quatro atributos |
| `ui/ui.go`, `ui/ui_test.go` | `Flashes`, `Confirm`, constantes |
| `ui/assets/ui.js` | confirmação na captura; `Trilha-Flash` no `ask` |
| `internal/scaffold/templates` | layout do `new` com `ui.Flashes(c)` |
| `examples/blog/app/blog/slug_/page.go` | `ui.Confirm` no lugar do diálogo à mão |
| `examples/blog/app/layout.go` | `ui.Flashes(c)` |
| docs `ui`, `ctx`, `h` + receita "ações com feedback" nas duas línguas | referência e receita |
| `CHANGELOG.md` | entradas na 0.39.0 |
