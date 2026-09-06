# Spec 049 — Ligar os agentes num projeto que já existe

- **Pedido**: do usuário, nesta sessão — "como usar a implementação com a flag agent em um
  projeto da Trilha já existente, de uma versão anterior?".
- **Branch**: `049-agentes-em-projeto-existente`
- **Versão**: documentação; entra na próxima release.

## Por quê

O suporte a agentes chegou em três releases seguidas — `AGENTS.md` e `llms.txt` na 0.36.0,
`trilha ctx` e `trilha check` na 0.37.0 — e a documentação conta as duas coisas do ponto de
vista de quem **cria** o projeto agora: `trilha new --agents`. Quem já tem um app rodando na
0.33.0 lê `--agents`, procura a flag no comando que ele usa todo dia e não acha: `--agents` é
do `new`, e o que ele precisa é `trilha agents`.

Falta também o detalhe que só aparece na atualização: o `AGENTS.md` **descreve os comandos
da CLI que o gravou**. Uma cópia escrita pela 0.36.0 manda o agente rodar `make test` e
nunca menciona `trilha check` — quer dizer, depois de atualizar a CLI é preciso rodar
`trilha agents` de novo, e a cópia intocada é regravada em silêncio. Sem essa frase, o
projeto atualiza a CLI e continua com o mapa velho na mão do agente.

## O que muda

Só documentação, em três lugares e nas duas locales:

1. **Receita de migração**, seção "Entre versões menores": uma subseção com a sequência
   completa (`go get -u`, `go install`, `gen`, `agents`, `check`), o que commitar, a linha
   única de CI e o que fazer quando o `AGENTS.md` foi editado à mão.
2. **Aprender → IA e agentes**: a frase que separa `trilha new --agents` (criação) de
   `trilha agents` (projeto que já existe) aponta para a receita.
3. **Referência da CLI**, `trilha agents`: rodar de novo depois de atualizar a CLI.

## Fora de escopo

- **Comando de upgrade** (`trilha upgrade`) — a sequência tem cinco linhas e cada uma é um
  comando padrão do Go ou da CLI; automatizar isso esconde o que aconteceu.
- **Mudar o `AGENTS.md`** — ele já se atualiza sozinho pelo carimbo; o que faltava era dizer
  quando rodar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| Público em inglês com pt-BR no mesmo commit | as três mudanças vão nas duas locales |
| A documentação é do repositório | nada de arquivo novo fora de `site/internal/docs/content` |

## Tarefas

- [x] T001 Receita `cookbook/migration.md` e `receitas/migracao.md`
- [x] T002 `learn/ai-and-agents.md` e `aprender/ia-e-agentes.md` apontando para ela
- [x] T003 `reference/cli.md` e `referencia/cli.md` no `trilha agents`
- [x] T004 `CHANGELOG.md` em `Unreleased`; `make test` verde

## Aceitação

- **SC-001** As duas locales dizem, no mesmo lugar, que a flag é do `new` e que o comando de
  quem já tem projeto é `trilha agents`.
- **SC-002** A sequência documentada roda de ponta a ponta num projeto vindo de versão
  anterior, terminando em `trilha check` verde.
- **SC-003** Está escrito que o `AGENTS.md` precisa ser regravado depois de atualizar a CLI,
  e o que acontece quando ele foi editado à mão.
