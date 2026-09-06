# Governança

> 🇧🇷 Português · [🇺🇸 English](../../GOVERNANCE.md)

## Papéis

- **Mantenedor**: revisa e integra PRs, publica versões, decide sobre propostas. Hoje:
  Emerson Oliveira dos Santos ([@emersonjoe](https://github.com/emersonjoe)).
- **Contribuidor**: qualquer pessoa com um PR aceito. Contribuidores recorrentes, com
  histórico de revisões de qualidade, podem ser convidados a mantenedores.

## Como decisões são tomadas

1. Propostas de convenção ou de API começam em uma issue de proposta e, se aceitas, viram
   uma spec em `specs/` (spec → plan → tasks → implement).
2. A [constituição](../../.specify/memory/constitution.md) prevalece. Mudar um princípio exige uma
   emenda explícita (nova versão do arquivo, com justificativa), nunca uma exceção pontual.
3. Discordâncias são resolvidas por consenso na issue; sem consenso, o mantenedor decide e
   registra o porquê.

## Versões

Versionamento semântico. Enquanto o projeto está em 0.x, mudanças incompatíveis podem ocorrer
em versões menores (0.2, 0.3), sempre listadas no `CHANGELOG.md` com o caminho de migração.
A 1.0 será publicada quando as convenções de arquivo e a API de `Ctx`/`h` ficarem estáveis
por pelo menos três versões menores sem mudança incompatível.

O que "quebra" quer dizer, quais pacotes a promessa cobre e como um símbolo é aposentado estão
no [`API.md`](API.md); a superfície exportada é versionada em
[`api/current.txt`](../../api/current.txt) e um teste falha quando ela muda sem ninguém dizer.

Toda versão é uma tag anotada `vX.Y.Z` com uma release no GitHub. **Toda spec fechada
(mergeada na `main`) gera uma versão**, para que `go get ...@latest` e a documentação do
site descrevam sempre a mesma API (issue #5).

## Proteção da `main`

A `main` tem um *ruleset* no GitHub, no padrão dos projetos de comunidade:

- proibido apagar a branch e fazer *force push*; histórico linear (fast-forward ou rebase);
- toda mudança entra por pull request com **1 aprovação**, revisão dos donos em `CODEOWNERS`,
  todas as conversas resolvidas e os checks de CI verdes (`test (1.22)`, `test (1.25)`, `vuln`);
- aprovações anteriores caem quando novos commits chegam no PR.

O mantenedor tem *bypass* para integrar as specs fechadas por fast-forward (o fluxo
spec-kit local) e para correções urgentes; qualquer uso do bypass fica visível no histórico
do ruleset. Os arquivos deste repositório não são a fonte da regra: ela vive em
*Settings → Rules*, e mudanças nela são anunciadas em uma Discussão.

## Métricas de uso

Coletamos o mínimo, sem cookies e sem dado pessoal, e nada disso vive no código:

- **Site** (emersonjoe.github.io/trilha): contagem de páginas com o
  [GoatCounter](https://www.goatcounter.com) (software livre, sem cookies), ligada só quando
  a variável de repositório `SITE_ANALYTICS` existe (`Settings → Secrets and variables →
  Actions → Variables`, valor `goatcounter:<código>`). O rodapé passa a mostrar a nota de
  privacidade e o link público dos números. Para desligar, apague a variável e republique.
- **Repositório**: visitas, clones, caminhos e referenciadores dos últimos 14 dias
  (`scripts/traffic.sh`, com o `gh` do mantenedor). O workflow `traffic` grava um snapshot
  diário em `traffic.jsonl` na branch `stats` quando existe o segredo `TRAFFIC_TOKEN` (token
  fino com `Administration: read` só neste repositório). Sem o segredo ele não faz nada.

Estrelas, forks e watchers são públicos na página do repositório.

## Comunicação

Issues e Discussões são os canais oficiais. Decisões tomadas em outro lugar são registradas
lá antes de valer.
