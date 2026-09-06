# Política de segurança

> 🇧🇷 Português · [🇺🇸 English](../../SECURITY.md)

## Versões com suporte

| Versão | Suporte |
|---|---|
| 0.x mais recente | correções de segurança |
| anteriores | não |

## Como relatar

**Não abra issue pública.** Use o
[relato privado de vulnerabilidade](https://github.com/emersonjoe/trilha/security/advisories/new)
do GitHub. Você receberá resposta em até 72 horas com a avaliação inicial e, se confirmada,
uma previsão de correção. Créditos vão para quem relatou, no advisory e no `CHANGELOG.md`,
salvo pedido em contrário.

## O que está no escopo

O runtime (`trilha`, `h`, `tmpl`), a CLI e o arquivo gerado. Exemplos e o site de
documentação são bem-vindos, mas com prioridade menor.

O [modelo de ameaças](SECURITY-MODEL.md) diz contra o quê essas garantias defendem, e o que
continua aberto.

## Garantias que o framework oferece por padrão

- Escape de HTML em texto e atributos (`h`) e escape contextual (`tmpl`).
- `Content-Security-Policy` com nonce por requisição; HSTS em HTTPS; `X-Frame-Options`,
  `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy`.
- Proteção CSRF por *double-submit cookie* em métodos de escrita de páginas.
- Cookies assinados (HMAC-SHA256, com expiração e rotação de chave).
- Limite de corpo de requisição (1 MiB por padrão), timeouts e limite de cabeçalhos.
- Limite de taxa por cliente (opcional) e IP do cliente só via proxies confiáveis.
- Arquivos estáticos restritos a `public/` (sem *path traversal*).
- Erros em produção sem stack nem caminhos de arquivo; logs sem corpo nem cookies; eventos
  de segurança (CSRF, 401/403, 413, 429, panic) registrados e expostos por hook.

O mapeamento desses controles para o NIST CSF 2.0 e o OWASP ASVS 4.0 está na documentação:
<https://emersonjoe.github.io/trilha/pt/aprender/seguranca>.

Um relato que mostre qualquer uma dessas garantias falhando é tratado como vulnerabilidade.
