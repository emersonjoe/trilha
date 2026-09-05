# Feature Specification: Estatísticas de acesso (site e repositório)

**Feature Branch**: `010-estatisticas` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "adicionar estatísticas de acesso a https://emersonjoe.github.io/trilha/ e ao
repositório, se possível, da melhor forma e mais segura"

## Princípios de escolha

1. **Sem cookies e sem dado pessoal**: contagem por página, referenciador, país e tipo de
   dispositivo bastam para um projeto open source; nada de identificar visitante.
2. **Nenhum segredo no repositório**: token e código de conta ficam em *Secrets*/*Variables*
   do GitHub; o site exportado não muda quando a variável não existe.
3. **Sem dependência no runtime**: o script de analytics é uma linha no `<head>` do site,
   só quando habilitado; o framework não ganha nada disso.
4. **Dados do repositório ficam com o dono**: o GitHub só guarda 14 dias de tráfego; um
   snapshot diário preserva o histórico em uma branch `stats`, legível por qualquer um.

## User Scenarios & Testing

### US1 - Visitas ao site sem cookies (P1)
Com a variável de repositório `SITE_ANALYTICS=goatcounter:<codigo>`, o site publicado inclui
o script do GoatCounter (software livre, sem cookies, contagem por página) e uma nota de
privacidade no rodapé. Sem a variável, nada é incluído.
**Acceptance**: teste do site: com `SITE_ANALYTICS` o HTML tem o `<script data-goatcounter>`
e a nota; sem, não tem nenhum dos dois; o CSP de dev permite a origem do script.

### US2 - Tráfego do repositório preservado (P1)
Um workflow diário lê visitas, clones, caminhos e referenciadores (API de tráfego, que
exige um token com leitura de *Administration*) e acrescenta uma linha JSON por dia em
`traffic.jsonl` na branch `stats`. Sem o segredo `TRAFFIC_TOKEN` o workflow termina sem
fazer nada (e diz por quê). `scripts/traffic.sh` mostra os mesmos números localmente com o
`gh` já autenticado, hoje, sem configurar nada.
**Acceptance**: o script roda e imprime a tabela; o workflow tem `permissions` mínimas
(`contents: write` só para a branch `stats`) e não expõe o token em logs.

### US3 - Documentação para o mantenedor (P2)
GOVERNANCE.md ganha a seção "Métricas de uso": o que é coletado, onde fica, como ligar
(criar conta no GoatCounter → variável; criar token fino → segredo) e como desligar.

## Fora do escopo
Painel próprio, Google Analytics (cookies, dados fora do controle), analytics no framework.
