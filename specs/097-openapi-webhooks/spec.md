# Spec 097 — `trilha openapi` também documenta o que o app **manda**

- **Issue**: [#112](https://github.com/emersonjoe/trilha/issues/112) — a issue é a fonte do escopo.
- **Branch**: `097-openapi-webhooks`
- **Versão**: 0.77.0

## Por quê

O módulo `webhook` entregou a entrega assinada, o retry e a tela. Ficou de fora a última linha do
aceite: *"`trilha openapi` gera a seção `webhooks` do OpenAPI 3.1 com os eventos e o exemplo de
corpo"*.

Quem integra do outro lado precisa de três coisas — quais eventos existem, o que vem no corpo, e
como conferir a assinatura — e hoje as três só existem no código de quem manda. O documento
descreve o que o app **recebe** e não diz nada do que ele **manda**, que é metade do contrato.

## O que muda

O documento ganha a seção `webhooks`, ao lado de `paths`:

```json
"webhooks": {
  "documento.processado": {
    "post": {
      "operationId": "webhook_documento_processado",
      "parameters": [
        {"name": "X-Webhook-Id", "in": "header", "required": true, "description": "…"},
        {"name": "X-Webhook-Event", "in": "header", "required": true, "description": "…"},
        {"name": "X-Webhook-Timestamp", "in": "header", "required": true, "description": "…"},
        {"name": "X-Webhook-Signature", "in": "header", "required": true, "description": "sha256=… — HMAC de \"timestamp.corpo\""}
      ],
      "requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Documento"}}}},
      "responses": {"200": {"description": "…"}}
    }
  }
}
```

De onde saem os eventos: **de cada chamada a `Emit` com o nome escrito na chamada**. O corpo é o
schema do terceiro argumento, lido pela mesma máquina que já lê o corpo de uma rota — então um
struct que já virou componente é reaproveitado como `$ref`, e não copiado.

A regra é mecânica e está na doc, para ser contestada como as outras: um `Emit` cujo nome do
evento vem de uma variável não entra, porque um documento não pode dizer um nome que não existe
até a hora de rodar.

## Fora de escopo

- **Ler o `Subscribe` para dizer quem assina o quê.** Isso é dado do app em produção, não do
  código; o documento descreve o contrato, não a lista de clientes.
- **Um registro de eventos declarado à parte.** Seria uma segunda fonte para envelhecer ao lado da
  chamada que manda de verdade.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `go/ast` e `go/parser`, como o resto do gerador |
| Determinismo | eventos em ordem de nome; o golden prova |
| VI — teste primeiro | fixture com dois `Emit` e o golden regravado |

## Tarefas

- [x] T001 Teste que falha: dois eventos na fixture, com corpo e cabeçalhos
- [x] T002 A varredura dos `Emit` e a seção `webhooks`
- [x] T003 Golden regravado
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.77.0`

## Aceitação

- **SC-001** Um `Emit` com nome literal vira uma entrada em `webhooks`, com o corpo do payload.
- **SC-002** O schema de um struct já usado numa rota é o mesmo componente, por `$ref`.
- **SC-003** Os quatro cabeçalhos da entrega estão descritos.
- **SC-004** Um `Emit` com nome vindo de variável não inventa evento nenhum.
- **SC-005** Duas execuções dão os mesmos bytes.
