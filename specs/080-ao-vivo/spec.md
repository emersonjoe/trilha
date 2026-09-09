# Spec 080 — testes contra servidores de verdade

- **Issue**: nenhuma — saiu do pedido de rodar os testes contra os componentes externos de
  verdade, e do defeito que isso achou.
- **Branch**: `080-ao-vivo`
- **Versão**: 0.62.0

## Por quê

Dois módulos deste framework falam com servidores que não existem na suíte: o `blob.S3` assina
requisições AWS SigV4 e o `auth` faz login OpenID Connect. Os dois estavam testados contra
servidores escritos aqui — e um servidor escrito aqui confere o que este código faz do jeito
que este código faz.

O `fakeS3` recomputa a assinatura com o mesmo algoritmo que o `blob/s3.go` escreve: se os dois
estiverem errados igual, o teste passa. O `fakeIDP` põe as claims onde o `auth` já procura,
porque foi escrito depois dele. É coerência interna, e coerência interna passa em cima de um
mal-entendido sobre o protocolo sem fazer barulho nenhum.

Foi exatamente o que aconteceu. Um Keycloak de verdade recusou, calado, todo `RequireRole`:
**o Keycloak põe `realm_access` e `resource_access` no access token, e não no `id_token`**. O
login funcionava, o `User.Roles` vinha vazio, e o sintoma — 403 em toda página guardada — se
parece com erro de configuração de permissão, não com claim lida do token errado. Nenhum teste
nosso podia ter mostrado isso.

## O que muda

`(*Provider).rolesFromAccess` passa a completar as claims de papel a partir do access token,
quando ele for um JWT do mesmo emissor, e o `Callback` a chama depois de verificar o
`id_token`. Regras, nesta ordem:

- **identidade vem sempre do `id_token`** — o access token não muda `sub`, `email` nem `name`;
- **o `id_token` ganha**: só as claims de papel que faltam são preenchidas, nunca sobrescritas;
- access token opaco (o que a maioria dos provedores emite), que não confere, ou de outro
  emissor não é erro e não vale papel nenhum: o login já foi provado pelo `id_token`.

Dois testes novos, pulados sem variável de ambiente, porque teste que precisa de contêiner
falha por motivo errado na máquina de outra pessoa:

```bash
# blob/s3_live_test.go — MinIO
TRILHA_S3_TEST='s3://balde?endpoint=http://localhost:9000&path_style=1' go test ./blob/ -run AoVivo

# auth/oidc_live_test.go — Keycloak (cria realm, cliente, usuário e papel pela API de admin)
TRILHA_OIDC_TEST=http://localhost:8080 go test ./auth/ -run AoVivo
```

O do S3 cria o balde, guarda, lê, faz `Stat`, lista, **busca a URL pré-assinada com um cliente
que não assina nada**, vê uma URL vencida ser recusada e apaga. O do OIDC faz o login inteiro
por um navegador com cookies: `/entrar`, o formulário do Keycloak, o retorno, e a página
guardada imprimindo `sub`, e-mail e papéis.

## Fora de escopo

- CI com contêineres. A suíte tem de rodar em qualquer máquina sem Docker; estes testes são
  para rodar à mão quando o assunto for mexido.
- Refresh token e introspecção. O access token é lido aqui como portador de claims de papel e
  nada mais — o `auth` não faz chamadas em nome do usuário.
- Azure, Cognito e Clerk ao vivo. Precisam de conta paga ou de nuvem; o Keycloak fecha o caso
  que importava, que era o de as claims não estarem onde o padrão sugere.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo; os testes ao vivo usam `net/http` e a API dos servidores |
| VI — teste primeiro | o defeito veio de um teste ao vivo, e o conserto tem teste de unidade que falha sem ele |
| VII — segurança por padrão | papel só sai de token verificado; o `id_token` continua sendo a fonte da identidade |

## Tarefas

- [x] T001 `blob/s3_live_test.go` contra o MinIO, e rodar com o segredo errado para ver o servidor recusar
- [x] T002 `auth/oidc_live_test.go` contra o Keycloak, provisionando o realm pela API de admin
- [x] T003 `rolesFromAccess` + a chamada no `Callback`
- [x] T004 Testes de unidade: papel do access token, precedência do `id_token`, token opaco e token inválido
- [x] T005 Documentação nas duas locales (referência do `auth`, receita de arquivos)
- [x] T006 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T007 `make test` verde e `scripts/release.sh 0.62.0`

## Aceitação

- **SC-001** Um login contra um Keycloak de fábrica traz os papéis do realm e os do cliente.
- **SC-002** Papel presente no `id_token` não é trocado pelo do access token.
- **SC-003** Access token opaco ou não verificável não impede o login nem concede papel.
- **SC-004** As duas suítes ao vivo passam contra MinIO e Keycloak, e são puladas sem as
  variáveis de ambiente — `go test ./...` continua sem depender de rede.
