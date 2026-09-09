# Spec 079 — Onde o arquivo mora

- **Issue**: #114 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `079-blob`
- **Versão**: 0.61.0

## Por quê

O framework já sabia receber (`c.File`) e devolver (`c.AttachmentFile`, `c.InlineFile`). Onde
guardar nunca foi dito, e o exemplo gravava em `./uploads` com o nome que veio do cliente — os
dois erros clássicos numa linha só.

## O que muda

O módulo opcional `trilha/blob`: `Files` com `Put`, `PutBytes`, `Get`, `Stat`, `Serve`, `Delete`,
`Orphans`; as lojas `Disk`, `Memory` e `S3`; `FromEnv`. O contrato está na receita nova, nas duas
línguas. O que vale registrar:

**A chave é o conteúdo, nunca o nome.** SHA-256 em dois níveis com a extensão do tipo farejado.
Travessia de caminho não é evitada por checagem: é impossível, porque nada que veio da requisição
chega perto da chave. De brinde, o mesmo arquivo duas vezes é um objeto só, e nenhum diretório
termina com cem mil entradas.

**Deduplicar é do módulo; decidir o que isso significa é da aplicação.** O `Ref.SHA256` está lá
para quem quiser contar referências. O `Delete` apaga; se apagar está certo quando duas linhas
apontam para a mesma chave é pergunta que só a aplicação responde, e responder por ela seria
inventar uma regra de negócio dentro de um pacote de armazenamento.

**S3 sem SDK, cerca de duzentas linhas.** É o que mantém este módulo opcional em vez de o
framework ganhar uma árvore de dependências. Ele faz put, get, head, delete, list e presign —
o que uma loja de arquivos precisa. Mais que isso é motivo para usar o SDK no código da
aplicação, não para este crescer.

**URL pré-assinada é capacidade, e a documentação diz isso.** Quem tem o link tem o arquivo até
vencer, sem sessão e sem log seu. Ótimo atrás de CDN, péssimo para um documento que três pessoas
podem ler — daí o `ServeOpts{Proxy: true}`.

**Loja errada é pânico no boot.** `TRILHA_BLOB_URL` que o módulo não entende para a aplicação em
vez de cair num diretório que ninguém quis: subir com o storage errado é perder arquivo calado.

**Escrita atômica no disco**: arquivo temporário e rename. Uma queda no meio não deixa uma chave
que existe e não abre — e a varredura ignora o parcial em vez de reportá-lo como órfão.

## O que não foi verificado, e está escrito

A assinatura SigV4 é conferida contra um servidor de teste que a **recomputa** com o segredo:
requisição mal assinada falha aqui pelo mesmo motivo que falharia na AWS. Ela **não** foi
conferida contra um vetor oficial da AWS — não havia como validar um offline, e inventar um
"vetor conhecido" sugeriria uma garantia que não existe. Está dito na receita e no doc do teste.

O modo de falha ajuda: assinatura errada é 403 em tudo, na primeira chamada, e não um vazamento
silencioso. Contra um MinIO real é um `docker run` de distância, e vale fazer uma vez antes de
alguém publicar com bucket.

## Desvios da issue

- **`Orphans` recebe uma função e não um `iter.Seq`** — Go 1.22. O formato é o mesmo (`func(yield
  func(string) bool)`), então a assinatura vira `iter.Seq` sem quebrar ninguém quando o mínimo
  subir.
- **A migração do exemplo foi no `blog` e não no `cadastro`.** O `cadastro` não tem upload
  nenhum; o `blog` tem upload, listagem e entrega — a migração ali mostra as três pontas, e é
  onde o `os.WriteFile` com nome do cliente realmente estava.
- **Recusar SVG com `<script>`** já é do `c.File`/`c.Inline` (spec 070): o `Put` recebe o que o
  `c.File` aprovou, e duplicar a checagem aqui seria a segunda cópia que fica para trás.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `crypto/sha256`, `crypto/hmac`, `encoding/xml`, `net/http`. O `TestNoExternalDeps` passa com o S3 dentro. |
| III — rota no exemplo | O `blog` migrou: sobe, lista e entrega pelo blob. |
| IV — superfície pequena | Uma interface de cinco métodos e um tipo com sete. |
| V — inglês no código, pt-BR junto | Receita nova nas duas línguas no mesmo commit. |
| VI — teste primeiro | Chave vinda do conteúdo, dedup, chave de fora recusada, ciclo completo em memória e em disco, órfãos, parcial que não vira chave, `Serve` com nome e tipo, 404; e no S3: o ciclo contra servidor que confere assinatura, segredo errado explicando, presign com prazo e limite, e o endereço nos três estilos. |
| VII — segurança por padrão | Travessia impossível por construção, 0600 no disco, e o aviso sobre a URL pré-assinada onde alguém vai ler. |

## Tarefas

1. `blob/blob.go` e `files.go`: contrato, chave, `Put`/`Serve`/`Delete`/`Orphans`. ✅
2. `blob/disk.go`, `memory.go`: as duas lojas locais, com escrita atômica. ✅
3. `blob/s3.go`: SigV4, put/get/head/delete/list, presign, erro que cita o servidor. ✅
4. `blob/env.go`: `FromEnv` e o pânico no boot. ✅
5. `examples/blog`: anexos migrados, com e2e. ✅
6. Receita "Arquivos" en e pt; `api/current.txt` e a lista de pacotes públicos. ✅
