# Política de segurança

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

## Garantias que o framework oferece por padrão

- Escape de HTML em texto e atributos (`h`) e escape contextual (`tmpl`).
- Proteção CSRF por *double-submit cookie* em métodos de escrita de páginas.
- Limite de corpo de requisição (1 MiB por padrão).
- Cabeçalhos `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`.
- Arquivos estáticos restritos a `public/` (sem *path traversal*).
- Erros em produção sem stack nem caminhos de arquivo; logs sem corpo nem cookies.

Um relato que mostre qualquer uma dessas garantias falhando é tratado como vulnerabilidade.
