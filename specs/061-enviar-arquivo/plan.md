# Plano — spec 061

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `send.go` (novo) | `Attachment`, `AttachmentFile`, `Inline`, `InlineFile`, `Pipe` e o cabeçalho |
| `send_test.go` (novo) | nome, `Range`, `HEAD`, tipo recusado, `Pipe`, corpo grande |
| `file.go` | nada: o `safeName` e o sniff já estão lá e são reaproveitados |
| `examples/orcamento/app/api/relatorio.csv` | seis linhas de cabeçalho viram um `c.Attachment` |
| `examples/blog/internal/anexos` | o anexo passa a guardar os bytes recebidos |
| `examples/blog/app/anexos/nome_` (novo) | baixar e abrir o anexo, com `?ver` para o inline |
| `SECURITY-MODEL.md` | download como vetor: tipo errado, nome no cabeçalho, `<iframe>` e CSP |
| docs | `ctx` nas duas línguas ganha a seção **Sending a file** / **Mandando um arquivo** |

## Ordem

`send.go` primeiro, com os testes de cabeçalho e de `Range`, porque tudo depende do formato do
`Content-Disposition`; os exemplos depois, que é onde se vê se a assinatura serve; docs e
modelo de segurança por último, quando não há mais o que mudar de nome.

## O que não entra

- Geração de ZIP, streaming multipart ou "baixar a pasta": é do app.
- Assinar URL de download temporária: pede um segredo e uma política de expiração que são de
  quem hospeda, não do framework.
- Reescrever a CSP no `Inline`: a política é do app; o que entra é a documentação do que pedir.
