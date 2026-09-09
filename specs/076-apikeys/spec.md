# Spec 076 — A chave que a própria app emite

- **Issue**: #105 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `076-apikeys`
- **Versão**: 0.58.0

## Por quê

O framework tinha sessão por cookie e bearer de terceiros (OIDC). Faltava o terceiro caso: a
chave que a própria aplicação emite. É um monte de regra de segurança que o iniciante não sabe,
e a primeira versão dele guarda a chave em texto claro.

## O que muda

`auth.APIKeys` com `Issue`, `Require`, `Revoke`, `User`, `All`; `KeyStore` e `MemoryKeyStore`;
`trilha.Pepper` e `trilha.Limiter` exportados; `ui.SecretOnce` e `ui.APIKeysTable`. O contrato
está na referência de auth, nas duas línguas. O que vale registrar:

**Só o hash é guardado**, com `trilha.Pepper` — HMAC-SHA256 sob chave derivada do segredo da app,
irmã do `Seal` da spec 075. Sem segredo, o `Issue` **recusa**: gravar um hash sem chave pareceria
ter funcionado e deixaria uma tabela que se ataca offline.

**A ordem das conferências.** Comparação em tempo constante primeiro, revogação e vencimento
depois. Responder mais rápido para uma chave revogada do que para uma errada conta a quem está
tentando qual das duas aconteceu — e as três respostas são o mesmo 401 com `WWW-Authenticate`.

**Chave é ator.** O `Require` põe um `auth.User` com os escopos de papéis e `via: "api_key"`,
então `c.Audit`, o log e a política já funcionam sem a aplicação escrever nada.

**O limite é por chave.** Foi para isso que o `limiter` interno virou `trilha.Limiter` exportado,
em vez de eu escrever um segundo balde de tokens no `auth` — a duplicação que venho evitando
issue após issue.

**Uso gravado uma vez por minuto.** Gravar por requisição transforma um endpoint de leitura numa
escrita por chamada, e a pergunta que isso responde não precisa do segundo.

**Escopo não declarado é pânico ao montar a rota**, não aviso do `trilha audit` como a issue
propunha: o aviso aparece quando alguém roda o comando, e o pânico aparece na primeira vez que a
rota sobe. Para uma rota que deveria estar guardada, essa diferença é a diferença.

## O bug que só aparecia às vezes

O segredo saía em base64url — cujo alfabeto **inclui `_`**, que é o separador do formato
`ak_<identificador>_<segredo>`. O `Split` quebrava no lugar errado sempre que os bytes aleatórios
codificavam um sublinhado: três testes falhando, dois passando, e a cada execução um conjunto
diferente. Agora o alfabeto é base32 minúsculo — letras e dígitos e nada mais — e o corte é
`SplitN`. Cinco execuções seguidas para confirmar.

E o exemplo mostrou dois atributos repetidos no meu markup: `class="ui-input ui-input"` (o
`Input` já põe a classe) e dois `type="button"` no mesmo botão (o `Button` já põe o tipo). Ambos
com teste agora.

## Fora de escopo, com o motivo

- **`trilha openapi` marcando as rotas com `securitySchemes`** — é o gerador de OpenAPI lendo o
  middleware de uma rota, que é a mesma leitura estática que a [#124](https://github.com/emersonjoe/trilha/issues/124)
  registra como dívida do scanner. Entra junto com ela.
- **Store em SQL** — mesma razão de sempre: sem dependência de banco no framework.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `crypto/rand`, `crypto/subtle`, `encoding/base32`, `encoding/hex`. |
| III — rota no exemplo | `/api/v1/documentos` por chave e a tela `/chaves` no `local-login`, as duas guardadas. |
| IV — superfície pequena | Um construtor, cinco métodos, uma interface de cinco, dois componentes. |
| V — inglês no código, pt-BR junto | Referência de auth nas duas línguas no mesmo commit. |
| VI — teste primeiro | Guarda só o hash; autentica e aplica escopo; as cinco formas de chave inválida respondendo igual; revogada e vencida; limite por chave com o mesmo IP; uso uma vez por minuto; escopo inexistente explodindo; a chave virando ator na trilha; e o e2e do exemplo. |
| VII — segurança por padrão | Hash com pimenta, tempo constante, 401 uniforme, `WWW-Authenticate`, limite por chave, escopo obrigatório, emissão e revogação auditadas. |

## Tarefas

1. `trilha.Pepper` e `trilha.Limiter` exportados. ✅
2. `auth/apikeys.go`: chave, store, emissão, middleware, revogação, memória. ✅
3. `ui/apikeys.go`: `SecretOnce`, `APIKeysTable`, e o `data-ui-copy` no `ui.js`. ✅
4. `local-login`: a API por chave e a tela de administração. ✅
5. Referência de auth en e pt; `api/current.txt`, catálogo, cópias do kit. ✅
