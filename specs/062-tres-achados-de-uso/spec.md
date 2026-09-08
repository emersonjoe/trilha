# Spec 062 — Três achados de uso

- **Issues**: #92, #95, #93 — as issues são a fonte do escopo; aponte para elas, não as
  reescreva aqui.
- **Branch**: `062-tres-achados-de-uso`
- **Versão**: 0.44.0

## Por quê

Os três saíram da mesma coisa: rodar o que a 0.41.0–0.43.0 entregou contra um app real, e não
contra o fixture. Cada um foi verificado antes de virar issue, e cada um quebra a promessa da
issue que o entregou.

**#92 é o mais grave.** A #62 prometeu que "a tabela de usuários existente continua válida sem
migração de senha", e o `CheckPBKDF2` lia só o formato do Django. A tabela que motivou a issue
não é Django: o `hashlib.pbkdf2_hmac` devolve bytes e formato nenhum, então quem grava escolhe
um, e o que se escolhe é hex. Três diferenças, e cada uma sozinha já derruba a leitura — o
prefixo, o salt em hex decodificado para bytes, o digest em hex. Login que recusa a senha certa
é a pior forma de falhar: parece problema de quem digitou.

**#95** é o `Optional[T]` do Pydantic. `anyOf [T, null]` não é união, é "T ou nulo", e num
documento FastAPI é a forma mais comum que existe — 39 % dos campos do documento medido. Sair
como `json.RawMessage` obriga a marshalar à mão exatamente onde o cliente existia para não dar
esse trabalho.

**#93** são três defeitos no relatório do `migrate next`, e o efeito de todos é o mesmo: o
agente começa pela tela errada.

## O que muda

**`auth.CheckPBKDF2` lê duas grafias**, decididas pelo prefixo — `pbkdf2_sha256$…$b64` como
hoje, e `pbkdf2$…$hex` com o salt virando bytes quando lê como hex. `HashPBKDF2` continua
escrevendo uma só: uma grafia na escrita, duas na leitura.

**O gerador de cliente reconhece `anyOf`/`oneOf` de dois membros com um `null`** e emite o outro
como ponteiro. União de verdade — dois tipos, ou três membros — continua `json.RawMessage`, e
continua no relatório.

**O `migrate next` lê o que a página importa.** Imports relativos e `@/`, transitivo, com teto
de três níveis e trinta arquivos; os sinais rodam sobre a união, e a razão impressa nomeia o
arquivo que produziu o sinal quando não foi a página. A coluna de tamanho passa a somar
(`286 + 267`), que é o custo real. Além disso: soltar arquivo é `upload`, não `pointer` — os
dois sinais juntos é o que os distingue —, e `apiGet<T>("/x")` voltou a ser uma chamada.

## Fora de escopo

- **#94** (o cenário da régua de agente) — é enhancement e mede, não conserta.
- **`pbkdf2_sha512`** — continua `false`. Uma família por vez, e a que existe nas tabelas
  medidas é SHA-256.
- **Traduzir TSX** — o corpo das telas continua sendo do agente; o relatório só passa a dizer
  a verdade sobre o que ele vai encontrar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `encoding/hex` e `os`; nada novo no `go.mod`. |
| IV — superfície pública | Nenhum símbolo novo; `CheckPBKDF2` passa a aceitar mais, nunca menos. |
| VI — teste primeiro | Vetor gerado pelo `hashlib` fixado no teste, `anyOf` posto no documento sintético, e um app Next de mentira com o sinal no componente importado. |
| VII — segurança por padrão | Comparação continua em tempo constante; hash ilegível continua `false`, nunca pânico. |

## Tarefas

- [x] T001 `auth/pbkdf2.go`: as duas grafias, com o vetor do `hashlib` no teste.
- [x] T002 `internal/client`: `nullableOf`, e o documento sintético ganha as três formas.
- [x] T003 `internal/migrate`: seguir imports, sinal composto do drop, regex com genérico.
- [x] T004 Relatório somando as linhas do que a página importa.
- [ ] T005 Documentação da referência de auth nas duas línguas.
- [ ] T006 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** Um hash gerado pelo `hashlib` no formato hex autentica; a senha errada não.
- **SC-002** `Optional[T]` vira `*T`; união de três membros continua `json.RawMessage`.
- **SC-003** Página fina que importa componente com `onPointer*` sai como C, e a razão nomeia
  o componente.
- **SC-004** Tela com `onDrop` + `dataTransfer.files` sai como B; `apiGet<T>(...)` aparece na
  coluna de chamadas.
