---
title: Desenvolvimento agentic — o control plane
description: Compartilhe a fila entre um time e uma frota de workers com o trilha-cloud, e como é o contrato se você rodar o seu.
---

Um runner numa máquina serve uma pessoa. Um time com várias máquinas, vários projetos e
agentes trabalhando em paralelo precisa de um lugar que responda *o que está na fila*, *quem
está executando o quê*, *que evidência aquela execução deixou* e *quem pediu* — sem que o
código de projeto nenhum saia da máquina que o guarda. Isso é o **trilha-cloud**, e é privado:
protocolo e runner são abertos para que qualquer ferramenta os fale; o control plane é onde
operar uma frota custa alguma coisa.

```text
                    trilha-spec (protocolo)
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
   trilha-runner      Claude Code      outro runner
          │                │                │
          └────────────────┼────────────────┘
                           ▼
                     trilha-cloud
     projetos · fila · frota · evidência · auditoria
```

O trilha-cloud é ele mesmo um app Trilha — rotas por arquivo, `trilha gen`,
[`auth.APIKeys`](/pt/aprender/autenticacao), `c.Audit` —, o que faz deste capítulo também o
maior exemplo do framework em uso.

## O fluxo

Se você tem acesso ao repositório, um processo é o control plane inteiro:

```bash
export TRILHA_SECRET=$(trilha secret)
export TRILHA_CLOUD_ADMIN_TOKEN=troque-me
export TRILHA_CLOUD_DATA=./data/cloud.json
make dev                                  # http://localhost:3000
```

O operador registra um projeto e emite uma chave para os workers. Administração fica atrás do
token do ambiente — a primeira chave tem que vir de algum lugar — e todo o resto atrás de
chaves com escopo:

```bash
curl -X POST localhost:3000/api/admin/projects -H "Authorization: Bearer troque-me" \
     -d '{"name":"agenda","org":"aprender","repo":"git@github.com:voce/agenda"}'
curl -X POST localhost:3000/api/admin/keys -H "Authorization: Bearer troque-me" \
     -d '{"name":"notebook","scopes":["runs:write"]}'
# {"key":"tc_…"}   mostrada uma vez; o segredo é temperado com TRILHA_SECRET e nunca guardado
```

Quem tem chave enfileira uma task — a mesma `TASK-002` dos capítulos anteriores:

```bash
curl -X POST localhost:3000/api/runs -H "Authorization: Bearer tc_…" \
     -d '{"project":"agenda","task_id":"TASK-002"}'
# {"id":"run-000001","status":"queued",…}
```

E um worker — um `trilha-runner` numa máquina com checkout da agenda — pega:

```bash
cd agenda
trilha runner worker --cloud http://localhost:3000 --token tc_… --project agenda --once
# worker notebook on http://localhost:3000, project agenda
# TASK-002 → review (driver exec, 48s)
# branch trilha/task-002 in .trilha/runs/TASK-002/wt
# ✓ go test ./... (exit 0)
```

O worker pediu a próxima execução, executou localmente exatamente como no
[capítulo do runner](/pt/aprender/agentico-runner) e reportou status, branch, commit e os
registros de evidência. Abra `http://localhost:3000`: o painel mostra o projeto, a execução em
`review` com o branch, e a frota — `notebook`, ocioso, visto há segundos. A aprovação do
revisor é `DELETE /api/runs/run-000001` (review → done): a mesma decisão humana que o
protocolo mantém fora das mãos do agente.

Todo passo está na trilha de auditoria, com a chave que o fez como ator:

```bash
curl localhost:3000/api/admin/audit -H "Authorization: Bearer troque-me" | jq '.[].action'
# "run.closed" "run.finished" "run.claimed" "run.enqueued" "apikey.emitiu" "project.created"
```

## O contrato

O worker só precisa de três rotas, então outro control plane — o seu — pode implementá-las:

| Método | Caminho | Corpo | Resposta |
|---|---|---|---|
| POST | `/api/runs/next` | `{worker, project}` | `200 {id, project, task_id}`, ou `204` quando não há nada na fila |
| POST | `/api/runs/{id}/result` | `{passed, status, branch, commit, evidence[], log, error}` | `202` |
| POST | `/api/workers/heartbeat` | `{name, project, status}` | `200` |

Todas com `Authorization: Bearer <chave>`. O corpo do resultado é o `queue.Result` do
`trilha-runner`; `evidence[]` é o registro do protocolo, sem mudar. O código não viaja.

## O que está no MVP e o que não está

| | Entregue | Depois |
|---|---|---|
| Control plane | projetos, fila com claim, resultados com evidência, fechamento pelo revisor | organizações e times, billing |
| Frota | workers com heartbeat; quem faz o quê | agendamento entre projetos, sandboxes remotos |
| Governança | token de admin, chaves com escopo e limite por chave, auditoria de toda ação | SSO, políticas por projeto, evidência assinada |
| Armazenamento | memória, snapshot JSON a cada escrita | SQL |

## Desafio

Escreva o menor control plane que um `trilha-runner worker --once` aceita: um app Trilha com
as três rotas acima, uma fila que é um slice em memória, e ainda sem autenticação. Rode o
worker contra ele com o driver `echo`.

:::solucao
Três arquivos em `app/api/` de um projeto novo (`trilha new fila`):

```go
// app/api/runs/next/route.go
package next

var fila = []string{"TASK-004"} // os ids de task a entregar, em ordem

func POST(c *trilha.Ctx) error {
	var in struct{ Worker, Project string }
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	if len(fila) == 0 {
		c.Status(204)
		return nil
	}
	id := fila[0]
	fila = fila[1:]
	return c.JSON(200, map[string]string{"id": "run-1", "project": in.Project, "task_id": id})
}
```

```go
// app/api/runs/id_/result/route.go
package result

func POST(c *trilha.Ctx) error {
	var in map[string]any
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	c.Log().Info("resultado", "run", c.Param("id"), "status", in["status"], "commit", in["commit"])
	c.Status(202)
	return nil
}
```

```go
// app/api/workers/heartbeat/route.go
package heartbeat

func POST(c *trilha.Ctx) error { return c.JSON(200, map[string]string{"ok": "1"}) }
```

`trilha gen && trilha dev` num terminal; na agenda,
`trilha runner worker --cloud http://localhost:3000 --token x --project agenda --once --driver echo`.
O worker reivindica a `TASK-004`, roda e posta o resultado que aparece no log. Um slice não é
uma fila que dois workers compartilham, e um `Authorization` que ninguém confere não é um
control plane — que é exatamente a lista do que o trilha-cloud acrescenta.
:::
