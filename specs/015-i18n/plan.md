# Implementation Plan: Inglês por padrão, português como tradução (i18n)

**Branch**: `015-i18n` | **Date**: 2026-09-05 | **Spec**: [spec.md](spec.md)

## Summary

Tornar o inglês o idioma padrão de tudo que é público (site, README, comunidade, CLI,
scaffold, erros de biblioteca) mantendo o português do Brasil como tradução publicada junto.
No site, isso vira duas locales paralelas (`en` na raiz, `pt` sob `/pt`) com redirecionamento
dos caminhos antigos, switcher por página e `hreflang`. Na CLI, uma tabela de textos escolhida
por variável de ambiente. Emenda da constituição para v1.2.0.

## Technical Context

**Language/Version**: Go 1.22+ (módulo), sem dependências externas.
**Testing**: `go test ./...` (site, CLI e2e, export), `make test`.
**Target Platform**: site estático no GitHub Pages (`--base /trilha`), CLI local.
**Constraints**: export estático não tem servidor (redirecionamentos viram stubs HTML);
CSP do site bloqueia handlers inline (textos do `tema.js` vêm do `lang` do documento).

## Constitution Check

| Princípio | Como a feature respeita |
|---|---|
| I. Convenção sobre configuração | rotas novas do site vêm de pastas (`learn/`, `reference/`, `pt/aprender/`...); os caminhos antigos são páginas que devolvem redirecionamento, não tabela manual |
| II. Só stdlib | nada novo; `os.Getenv` e `strings` para a locale da CLI |
| III. Geração explícita | `trilha_gen.go` do site regenerado e commitado; goldens intactos |
| IV. Contrato de handler | páginas de redirecionamento usam `trilha.RedirectCode(url, 301)` |
| V. Dev < 2 s, um binário | inalterado; export ganha stub de redirecionamento (feature do runtime, testada) |
| VI. Teste primeiro | testes do site (locales, redirects, hreflang, sincronia), do export (stub 3xx) e da CLI (locale) escritos antes das mudanças |
| VII. Segurança | stubs de redirecionamento só para destinos internos (mesma origem/base); `meta refresh` sem script |
| Estilo, idioma e interface | **emendado por esta spec** (v1.2.0): inglês por padrão + pt-BR como tradução no mesmo commit |

## Project Structure

```text
.specify/memory/constitution.md         v1.2.0 (emenda de idioma)
export.go                               stub HTML para 3xx internos no Export
site/internal/docs/docs.go              Locale, Section, Page{Locale,...}, Translation()
site/internal/docs/content/en/learn/*.md, en/reference/*.md   (novo)
site/internal/docs/content/pt/aprender/*.md, pt/referencia/*.md (movido)
site/internal/md/md.go                  Options.Locale (títulos de callout, aliases en)
site/internal/demos/{demos,kit}.go      demos por locale, rótulos do card
site/internal/ui/{ui,text}.go           T(c,key), header com switcher, hreflang, DocPage por locale
site/internal/home/home.go              home nas duas línguas
site/app/{page,layout,middleware,not_found,setup}.go
site/app/learn/{page.go,slug_/page.go}  site/app/reference/{page.go,slug_/page.go}
site/app/pt/page.go  site/app/pt/aprender/...  site/app/pt/referencia/...
site/app/aprender/..., site/app/referencia/...   (redirecionam 301)
site/public/tema.js                     textos por lang
site/site_test.go, export_test.go       testes
cmd/trilha/i18n.go                      lang(), t(key); todos os comandos usam t()
cmd/trilha/new.go                       --lang
internal/scaffold/{scaffold.go,templates/**}   Data.Lang, textos por idioma
internal/scan/scan.go, internal/gen, internal/scaffold/ui.go, *.go raiz   mensagens em inglês
README.md (en), README.pt-BR.md, CONTRIBUTING.md, GOVERNANCE.md, SECURITY.md, SUPPORT.md,
CODE_OF_CONDUCT.md (en), docs/pt-BR/*.md (pt), .github/** (en), CHANGELOG.md (en)
```

## Decisões

1. **Slugs por idioma, mapeados por posição.** `Sections` de cada locale listam os slugs na
   mesma ordem; a tradução de `learn[3]` é `aprender[3]`. Nenhuma tabela extra para manter, e
   o teste de sincronia garante a contagem.
2. **Prefixo `/pt`**, não `/pt-BR`: nome de pasta válido para pacote Go; `hreflang="pt-BR"`.
3. **Redirecionamento 301 + stub no export.** O runtime passa a aceitar 3xx em `Export` para
   destinos internos (começam com o base path ou `/`), gravando um `index.html` com
   `meta refresh`. Fica disponível a qualquer app Trilha que mude URLs.
4. **Demos duplicadas por idioma** (fonte + nó): o texto mostrado e o executado precisam
   bater; uma tabela de strings tornaria o código de exemplo ilegível.
5. **Locale da CLI por ambiente**, sem flag global: `TRILHA_LANG` > `LC_ALL` > `LC_MESSAGES`
   > `LANG`; `trilha new --lang` sobrescreve só o projeto gerado.
6. **Erros de biblioteca em inglês**, sem tradução: aparecem em logs, testes e `errors.Is`;
   o padrão do ecossistema Go é inglês.
7. **Exemplos permanecem em português** (assumido na spec); a documentação em inglês avisa.

## Complexity Tracking

Nenhuma violação de princípio. A duplicação de demos (decisão 4) é aceita e coberta pelo
teste de sincronia.
