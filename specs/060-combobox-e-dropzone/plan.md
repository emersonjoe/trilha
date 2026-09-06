# Plano — spec 060

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `file.go` | `Ctx.Files`, `FileRules.MaxFiles`, a mensagem `filecount` |
| `ui/combobox.go` (novo) | `ComboboxOpts`, `Combobox`, `ComboboxOptions` |
| `ui/upload.go` (novo) | `DropzoneOpts`, `Dropzone` (o `UploadTo`/`UploadBar` continua onde está) |
| `ui/assets/ui.js` | o combobox: busca com espera, teclado, `aria-activedescendant` |
| `ui/assets/ui.upload.js` | a fila: arrastar, um arquivo por requisição, cancelar |
| `ui/assets/ui.css` | `.ui-combobox`, `.ui-listbox`, `.ui-dropzone`, a linha da fila |
| `examples/cadastro` | cidade por combobox com `With: uf`; `app.js` some |
| `examples/blog/anexos` | dropzone e `c.Files` |
| docs | referência `ui` nas duas línguas, capítulo de formulários, receita de upload |

## Ordem

`c.Files` primeiro, porque a dropzone sem ele é um formulário que perde arquivo. Depois o
combobox inteiro (Go, script, CSS), que não depende de nada. A dropzone em seguida, sobre o
`ui.upload.js` que já existe. Os exemplos no fim, quando os dois campos existem.

## O que não entra

- Combobox de múltipla escolha (chips): a issue pede um valor, e a marcação de vários é outra
  discussão de acessibilidade.
- Criar item novo pela busca ("adicionar 'x'"): é regra do app, e o app já tem o `Source`.
- Retomar upload interrompido: precisa de protocolo no servidor, e não é o que a issue pede.
