# Implementation Plan: Testes UI a UI

**Branch**: `164-testes-ui-a-ui` | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)

## Summary

Camada A em `snapshot.go` (raiz, stdlib): retrato normalizado, asserções com conserto, golden.
Camada B no módulo `uitest/` (chromedp): `Run`, `Session`, relatório, fixture gerado e os onze
cenários. Makefile, CI, catálogo (`E_CSP_NONCE`) e regra do audit.

## Constitution Check

- [x] II — raiz só stdlib; `chromedp` só em `uitest/go.mod` (precedente `otel/`).
- [x] IV — adições públicas com doc comment e uso em `examples/`; nenhuma remoção.
- [x] VI — goldens estáveis (normalização testada), cenários nomeados, relatório testado.
- [x] VII — CSP, CSRF, cookies e segredos verificados por teste nas telas das receitas.

## Design

```
snapshot.go, snapshot_test.go        # PageSnapshot, CapturePage, asserções, seletor mínimo
testing.go                           # TestResponse guarda o app (para o snapshot)
examples/blog/snapshot_test.go       # golden da home, estável entre execuções
internal/recipes/{billing,notify,admin}.go  # asserções da camada A nos testes gerados
internal/uidoc/render_test.go        # goldens dos padrões pela normalização da camada A
cmd/trilha/audit.go                  # regra E_CSP_NONCE
internal/checkerr/checkerr.go        # E_CSP_NONCE
uitest/{go.mod,uitest.go,report.go,report_test.go,scenarios_test.go,fixture_test.go}
uitest/testdata/fixture/app/...      # ilha, navegação, tooltip, padrões com dados
Makefile (test-ui, test-ui-a), .github/workflows/ci.yml (job ui)
site: reference/testing (en/pt) com a camada A e a B
```
