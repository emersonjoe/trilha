# Baseline de desenvolvimento seguro

> [English](../../SECURITY-BASELINE.md) · Português

Esta baseline vale para toda mudança no Trilha. Ela usa o NIST SSDF 1.1 para o processo de
desenvolvimento, o OWASP ASVS 5.0 nível 2 para controles web aplicáveis e o OWASP Top 10:2025
para descobrir ameaças. Ela não afirma que o Trilha ou uma aplicação feita com ele é certificada.

## Evidências obrigatórias

| Área da mudança | Controle e evidência obrigatórios | Referência |
|---|---|---|
| Design | Identificar ativos, atores, fronteiras de confiança, casos de abuso e classificação dos dados na spec | SSDF PO.1, PW.1; ASVS V15 |
| Autenticação e sessão | Falhas genéricas, limites de taxa, ciclo seguro de cookies/tokens e testes | ASVS V6, V7, V9, V10 |
| Autorização e tenant | Negar por padrão, conferir no servidor cada objeto/ação e testar negação entre tenants | ASVS V8; Top 10 A01 |
| Entradas, saídas e arquivos | Validação positiva, escape contextual, corpos limitados e caminhos seguros | ASVS V1, V2, V4, V5; Top 10 A05 |
| Segredos e criptografia | Sem segredos no repositório/build, menor privilégio, rotação e primitivas aprovadas | ASVS V11, V13; Top 10 A04 |
| Logs e erros | Eventos de segurança sem credenciais, tokens, corpos de requisição ou valores de dados pessoais | ASVS V14, V16; Top 10 A09 |
| Dependências e build | Revisar dependências, fixar actions e executar `govulncheck` com o toolchain de segurança corrigido | SSDF PS.1, PW.4; Top 10 A03 |
| Verificação e resposta | Executar `make test` e `make security`; triar em privado e rastrear correção | SSDF PW.7, PW.8, RV.1-RV.3 |

Todo pull request nomeia as linhas aplicáveis. “Não se aplica” exige justificativa. Exceção exige
dono, validade, controle compensatório e issue; exceção vencida bloqueia release.

## Fronteira do CI self-hosted

- `push` na `main` e `workflow_dispatch` podem usar `RUNNER_CI=eoslab`; `pull_request` sempre usa
  `ubuntu-latest`.
- Nunca use `pull_request_target` para executar código de contribuidor num runner persistente.
- A conta de serviço é exclusiva do repositório e não pertence aos grupos `sudo` ou `docker`.
- Checkouts são limpos, credenciais não persistem, tokens têm permissões mínimas e caches Go
  vivem em `RUNNER_TEMP` e são removidos ao final de cada job self-hosted.
- O build do Pages pode rodar em `eoslab`; o deploy privilegiado continua hospedado pelo GitHub,
  com permissões `pages: write` e `id-token: write` limitadas ao job.
- Segredos de produção não ficam no runner. Em comprometimento: pare o serviço, remova o runner
  no GitHub, apague o diretório, rotacione credenciais expostas e reprovisione.

## Referências

- [NIST SP 800-218, Secure Software Development Framework (SSDF) Version 1.1](https://csrc.nist.gov/pubs/sp/800/218/final).
- [OWASP Application Security Verification Standard 5.0.0](https://github.com/OWASP/ASVS/tree/v5.0.0_release/5.0).
- [OWASP Top 10:2025](https://owasp.org/Top10/2025/).
