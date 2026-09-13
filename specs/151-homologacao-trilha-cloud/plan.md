# Plano — Spec 151

**Branch**: `151-homologacao-trilha-cloud` | **Spec**: [spec.md](spec.md) | **Issue**: #244

## Summary

Ampliar o capítulo do Trilha Cloud sem criar uma nova seção de navegação, usando evidência do
fluxo homologado e uma extensão mínima do Markdown para imagens de bloco.

## Technical Context

Go 1.22+, stdlib. Arquivos: `site/internal/md/md.go`, seus testes, `site/public/site.css`,
`site/internal/docs/content/{en/learn,pt/aprender}/agentic-cloud.md`, assets em
`site/public/docs/agentic-cloud/` e metadados da release.

## Constitution Check

Na spec. Sem violação; sem *Complexity Tracking*.

## Decisões

- **Imagem como bloco, não inline**: a documentação usa screenshots entre parágrafos; restringir
  a sintaxe reduz ambiguidade no conversor deliberadamente pequeno.
- **Legenda no title do Markdown**: `![alt](src "legenda")` vira `figure`, `img` e `figcaption`;
  sem title, a figura continua válida e acessível pelo alt.
- **Base path aplicado a `src` absoluto**: o mesmo conteúdo funciona localmente e no GitHub
  Pages sob `/trilha`, coberto por teste e exportação real.
- **Screenshots reais**: Chrome DevTools operou o portal e a aplicação gerada; nenhuma imagem é
  mock ou ilustração.
- **Driver `echo` no aceite do protocolo**: verifica integração sem tornar a documentação
  dependente de credencial, custo ou disponibilidade de modelo.
