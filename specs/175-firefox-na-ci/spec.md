# Spec 175 — Firefox estável na CI

**Feature Branch**: `175-firefox-na-ci` | **Created**: 2026-10-06 | **Status**: Entregue (0.155.1)

- **Origem**: o job `ui (firefox)` da CI da 0.154.0 reprovou em três cenários (`TestUIServerDeclaresURL`,
  `TestUIClientNavRunsRegionScripts`, `TestUISwapNewestClickWins`) que passam no Chromium e no
  WebKit da mesma execução, no Firefox local (macOS) e no Firefox da CI da 0.153.0. Pela regra
  "teste instável não é ignorado", é a primeira tarefa depois da 174.
- **Versão**: 0.155.1 (correção no módulo de teste; nada muda no framework)

## O que se viu

Todas as falhas são o primeiro `Navigate` do cenário: o servidor serviu o HTML (`status=200` no
log), e o evento `load` não chegou ao Playwright em 30 s. As duas tentativas de cada cenário
falham em sequência e o cenário seguinte passa — o desenho de um processo de navegador preso por
um minuto ou dois, que a segunda tentativa herdava, porque desde a 170 ela ganhava só um
contexto novo, não um navegador novo como a 164 prometia.

## O que muda

1. **A segunda tentativa relança o navegador** daquele motor: volta o contrato da 164 ("repete o
   cenário uma vez com navegador novo"). A primeira continua com o processo compartilhado, que é
   o que mantém três motores em pouco mais de um minuto.
2. **`Navigate` espera o documento (`commit`) e depois `readyState === "complete"`**, e quando não
   chega lá o relatório diz o que ainda está carregando (os `script`, `link`, `img` e `iframe`
   sem entrada no Resource Timing). A próxima vez que um motor travar, o relatório traz a causa
   em vez de "nothing happened in 30s".

## Jornada e risco

A de quem mantém o framework: um job vermelho por instabilidade do executor ensina a ignorar o
vermelho, que é o oposto do padrão de qualidade da 170.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| VI — teste primeiro / sem silêncio | a repetição registra a primeira falha no log (`attempt 1 … failed`), não esconde; a causa raiz, se o relatório novo a mostrar, vira issue |

## Tarefas

- [x] T001 `runOnce(…, fresh)` e `driver.launch(name, fresh)`.
- [x] T002 `Navigate` por `commit` + `readyState`, com os recursos pendentes no `got`.
- [x] T003 CI no branch (`workflow_dispatch`) nos três motores; CHANGELOG, versão.

## Evidências

| Nível | Onde | Resultado |
|---|---|---|
| Navegador, três motores | local, `UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui` | 42 passaram, 0 falharam, 0 pularam; nenhuma segunda tentativa |
| CI inteira no branch | execução 37476947675 (`workflow_dispatch`) | todos os jobs verdes; `ui (firefox)` sem nenhuma linha `attempt 1 … failed` |
| Unidade + integração | `make test` | 1593 passaram, 0 falharam, 2 pularam |

**O que fica em aberto, com dono.** A causa raiz não foi reproduzida: a CI da 0.153.0 e a da
0.155.0 passaram no Firefox sem esta mudança, então a falha da 0.154.0 é intermitente do
executor. Se voltar, o relatório agora diz o `readyState` e o recurso que não chegou; e se ela só
passar na segunda tentativa, o log do job mostra a primeira. Dono: o mantenedor do `uitest`
(quem fechar a próxima spec que mexa no módulo confere os logs do job `ui (firefox)` das
últimas execuções).
