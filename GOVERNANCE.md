# Governança

## Papéis

- **Mantenedor**: revisa e integra PRs, publica versões, decide sobre propostas. Hoje:
  Emerson Oliveira dos Santos ([@emersonjoe](https://github.com/emersonjoe)).
- **Contribuidor**: qualquer pessoa com um PR aceito. Contribuidores recorrentes, com
  histórico de revisões de qualidade, podem ser convidados a mantenedores.

## Como decisões são tomadas

1. Propostas de convenção ou de API começam em uma issue de proposta e, se aceitas, viram
   uma spec em `specs/` (spec → plan → tasks → implement).
2. A [constituição](.specify/memory/constitution.md) prevalece. Mudar um princípio exige uma
   emenda explícita (nova versão do arquivo, com justificativa), nunca uma exceção pontual.
3. Discordâncias são resolvidas por consenso na issue; sem consenso, o mantenedor decide e
   registra o porquê.

## Versões

Versionamento semântico. Enquanto o projeto está em 0.x, mudanças incompatíveis podem ocorrer
em versões menores (0.2, 0.3), sempre listadas no `CHANGELOG.md` com o caminho de migração.
A 1.0 será publicada quando as convenções de arquivo e a API de `Ctx`/`h` ficarem estáveis
por pelo menos três versões menores sem mudança incompatível.

Toda versão é uma tag anotada `vX.Y.Z` com uma release no GitHub.

## Comunicação

Issues e Discussões são os canais oficiais. Decisões tomadas em outro lugar são registradas
lá antes de valer.
