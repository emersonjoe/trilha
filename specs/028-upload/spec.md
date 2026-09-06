# Spec 028 — Upload com limite, tipo verificado e nome seguro

- **Issue**: [#28](https://github.com/emersonjoe/trilha/issues/28) (ROADMAP, Fase 2, item 9)
- **Branch**: `028-upload`
- **Versão**: 0.19.0

## Por quê

Hoje o app que recebe arquivo chama `c.Request().FormFile("arquivo")` e fica com três
problemas na mão, todos de segurança e todos fáceis de errar em silêncio:

1. **Tamanho.** O `MaxBodyBytes` é do corpo inteiro; não existe limite por arquivo. Quem
   quer aceitar um PDF de 2 MB precisa deixar o corpo inteiro passar de 2 MB e torcer.
2. **Tipo.** `hdr.Header.Get("Content-Type")` é o que o cliente disse, e a extensão é o que
   o cliente escreveu. Aceitar imagem olhando para `.png` é aceitar qualquer coisa renomeada.
3. **Nome.** `hdr.Filename` vem do cliente inteiro, com `../`, com `/`, com caractere de
   controle, com 300 caracteres. Salvar isso com `filepath.Join` é travessia de diretório.

Nada disso é difícil — é só repetitivo, e é o tipo de código que cada projeto reescreve pior.
O `examples/blog` já faz upload com progresso (spec 024) e é exatamente onde falta isto.

## O que muda

`c.File` devolve um arquivo já conferido, ou `FieldErrors` — a mesma resposta do `Bind`, para
o formulário mostrar a mensagem no lugar do campo (spec 027) em vez de responder 500.

```go
func POST(c *trilha.Ctx) error {
	c.AllowBody(8 << 20) // o corpo; o limite por arquivo é outro
	up, err := c.File("arquivo", trilha.FileRules{
		MaxSize: 2 << 20,
		Accept:  []string{"image/png", "image/jpeg", "application/pdf"},
	})
	if err != nil {
		if errs, ok := err.(trilha.FieldErrors); ok {
			return c.Render(http.StatusUnprocessableEntity, pagina(c, errs))
		}
		return err
	}
	defer up.Close()

	caminho, err := up.Save("uploads") // nunca sai de "uploads"
	if err != nil {
		return err
	}
	anexos.Add(up.Name, up.Size, up.MIME)
	return c.Redirect("/anexos")
}
```

| Símbolo | Papel |
|---|---|
| `c.File(field string, rules FileRules) (*Upload, error)` | lê um arquivo do formulário e aplica as regras |
| `FileRules{MaxSize int64, Accept []string, Optional bool}` | limite por arquivo, tipos aceitos (`"image/*"` vale), e se a ausência é erro |
| `Upload{Name, MIME, Ext string, Size int64, File multipart.File}` | nome já sanitizado, tipo **detectado no conteúdo**, extensão que corresponde ao tipo |
| `up.Save(dir string) (string, error)` | grava dentro de `dir` com nome livre (`nota.pdf`, `nota-1.pdf`…), 0600 |
| `up.Close() error` | fecha o arquivo temporário |
| `ValidationMessages["filemax"]`, `["filetype"]` | mensagens novas, traduzidas por `UseValidationPTBR` |

O tipo sai do `http.DetectContentType` (os primeiros 512 bytes), nunca do cabeçalho nem da
extensão; `Upload.File` volta posicionado no começo. O nome é `filepath.Base` sem caractere
de controle, sem separador, sem `..`, limitado a 100 caracteres e nunca vazio.

## Fora de escopo

- **Vários arquivos num campo** (`c.Files`) — a superfície mínima primeiro; quem precisa
  ainda tem `c.Request().MultipartForm`.
- **Armazenamento remoto (S3 e afins)** — `Upload.File` é um `io.Reader`; o destino é do app.
- **Antivírus, redimensionar imagem, ler EXIF** — cada um é uma dependência ou um pacote.
- **Detecção além do que a biblioteca padrão sabe.** `DetectContentType` erra em formatos
  que são zip por dentro (`.docx`, `.xlsx`) e chama CSV de texto; a documentação diz isso, e
  quem precisa de mais confere o conteúdo no app.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `mime`, `multipart`, `net/http`, `path/filepath` |
| III — coerência com Go | erro é valor (`FieldErrors`), `Upload` é `io.Reader`, regras numa struct como o `Config` |
| IV — convenção nova tem uso no exemplo | `examples/blog/app/anexos` passa a usar `c.File` |
| VI — teste primeiro | `file_test.go` vermelho antes de `file.go` |
| VII — segurança por padrão | tipo pelo conteúdo, nome sanitizado, `Save` que não escapa do diretório, arquivo 0600 |

## Tarefas

- [ ] T001 Teste que falha em `file_test.go`: arquivo grande demais; tipo mentido (PDF com
      nome `.png`); nome com `../`, com `/` e com caractere de controle; campo ausente com e
      sem `Optional`; `Accept` com `image/*`; `Save` que não sobrescreve e não escapa do
      diretório; mensagens trocadas por `UseValidationPTBR`.
- [ ] T002 `file.go`: `FileRules`, `Upload`, `Ctx.File`, `safeName`, `Save`, e as duas
      mensagens novas nos dois mapas.
- [ ] T003 `examples/blog`: `POST /anexos` com `c.File`, mostrando a mensagem no formulário;
      teste de integração no `blog_test.go` com arquivo grande demais e tipo mentido.
- [ ] T004 Documentação nas duas locales: seção no capítulo de segurança e a tabela em
      `reference/ctx`.
- [ ] T005 `CHANGELOG.md` (0.19.0), `version`, ROADMAP (Fase 2, item 9).
- [ ] T006 `make test` verde e `make release VERSION=0.19.0 ISSUES="28"`.

## Aceitação

- **SC-001** Arquivo acima de `MaxSize` vira `FieldErrors` com a mensagem do campo, sem 500 e
  sem gravar nada.
- **SC-002** PDF enviado como `foto.png` com `Accept: ["image/png"]` é recusado; o mesmo PDF
  com `Accept: ["application/pdf"]` passa, e `up.Ext` é `.pdf`.
- **SC-003** `hdr.Filename = "../../etc/passwd"` vira `passwd`, e `Save("uploads")` grava
  dentro de `uploads`.
- **SC-004** Dois envios do mesmo nome não se sobrescrevem.
- **SC-005** No `examples/blog`, enviar um arquivo grande demais responde 422 com a mensagem
  em português, e a página continua servindo o upload com progresso.
