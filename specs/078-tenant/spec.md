# Spec 078 — Multi-tenant por coluna

- **Issue**: #110 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `078-tenant`
- **Versão**: 0.60.0

## Por quê

Uma coluna é a forma mais comum de multi-tenant, e esquecer essa coluna numa consulta é o bug
mais comum de multi-tenant. O Trilha não tem — nem deve ter — ORM. Mas o tenant é dado de sessão,
e a sessão é do framework.

## O que muda

`auth.User.Tenant`, `auth.Tenant(c)`, `RequireTenant`, `SwitchTenant`, `Options.ChooseTenantPath`,
o tenant na trilha e no registro de acesso, e o item do `trilha audit`. O contrato está na
referência de auth, nas duas línguas. O que vale registrar:

**Campo próprio, não mais uma entrada no `Extra`.** Tudo o que o framework faz com o tenant —
pôr na trilha, pôr no log, recusar sessão sem ele — precisa encontrá-lo no mesmo lugar em toda
aplicação. Uma chave de mapa combinada por convenção seria uma convenção que a metade das apps
escreveria diferente.

**O framework carrega e aponta; a consulta é da aplicação.** Está escrito no doc, na referência
e no aviso da ferramenta. Um `WHERE` gerado por este pacote seria um `WHERE` que ninguém lê numa
revisão, que é o oposto do que um filtro de tenant precisa.

**"Entrou e não escolheu" não é intruso.** É um administrador no primeiro login. Navegador vai
escolher; o resto leva 403, porque redirecionamento para uma tela não é resposta que uma API use.

**Trocar de organização é auditado dos dois lados.** Uma investigação que começa com "viram as
linhas erradas" começa perguntando quando a pessoa trocou.

**Conferir vínculo é da aplicação.** O `SwitchTenant` não sabe o que é pertencer a uma
organização, e fingir que sabe seria uma checagem que parece garantia e não é.

## O item do `audit`, que é a parte que pega o bug

Conta, por tabela, quantas consultas a filtram por tenant, e nomeia as que não filtram — com
arquivo e linha. É heurística de texto, sem parser de SQL, e diz isso na própria mensagem: aponta
um lugar para olhar, não dá veredito. Duas consultas filtradas são o limiar (uma não é padrão), e
`_test.go` não conta — fixture cheia de consulta faria a ferramenta acusar o que não existe em
produção.

O registro de acesso ganhou o campo na **linha que já existia**. A primeira versão que escrevi
emitia uma segunda linha `request.tenant`, o que seria trocar um problema por outro: quem
investiga passaria a precisar juntar duas linhas.

## Fora de escopo

- **`--tenant` no `generate crud` e no template de app** — são a
  [#115](https://github.com/emersonjoe/trilha/issues/115) e a
  [#117](https://github.com/emersonjoe/trilha/issues/117), e entram com elas.
- **Isolamento por schema ou por banco** — outra forma de multi-tenant, com outras decisões
  (migração, conexão, custo). Esta issue é sobre a coluna, e misturar as duas numa API só faria
  as duas piores.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `regexp` e `strings` na ferramenta; nada novo no runtime. |
| III — rota no exemplo | O `auth` tem app de teste próprio com as três telas (listar, escolher, trocar); o `local-login` já usa política e não ganha tenant para não virar duas coisas. |
| IV — superfície pequena | Um campo, uma função, dois middlewares, uma opção. |
| V — inglês no código, pt-BR junto | Referência de auth nas duas línguas; as mensagens da ferramenta também. |
| VI — teste primeiro | O tenant chegando ao handler e à trilha; sem organização indo escolher e levando 403 na API; sem caminho configurado sendo 403; a troca auditada dos dois lados; e a heurística com fixture de três casos, incluindo o que ela **não** deve acusar. |
| VII — segurança por padrão | O aviso da ferramenta é o controle; a documentação diz o que ele é e o que não é. |

## Tarefas

1. `auth.User.Tenant`, `Tenant(c)`, `RequireTenant`, `SwitchTenant`, `ChooseTenantPath`. ✅
2. `Actor.Tenant` na trilha e o campo no registro de acesso. ✅
3. `cmd/trilha/tenant.go`: a contagem por tabela e o aviso com arquivo e linha. ✅
4. Referência de auth en e pt. ✅
5. `api/current.txt`. ✅
