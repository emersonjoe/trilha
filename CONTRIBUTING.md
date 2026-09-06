# Contributing to Trilha

> 🇺🇸 English · [🇧🇷 Português](docs/pt-BR/CONTRIBUTING.md)

Thanks for your interest. This guide explains how the project works so your contribution
lands without back and forth.

## Before coding

- **Bug**: open an issue with the `app/` tree that reproduces it.
- **New convention or API change**: open a proposal issue. Behavior changes go through a
  spec in `specs/NNN-name/` ([spec-kit](https://github.com/github/spec-kit) flow:
  spec → plan → tasks → implement). The maintainer opens the spec with you from the issue.
  A small change — one package, no new `app/` convention, no public API break — uses the
  short form instead: a single `spec.md` from `.specify/templates/spec-curta-template.md`.
- **Question**: use the [Discussions](https://github.com/emersonjoe/trilha/discussions).

## Principles that do not change without an amendment

They live in [`.specify/memory/constitution.md`](.specify/memory/constitution.md) (in
Portuguese). The ones that affect contributions the most:

1. Runtime and CLI use **only the standard library**. A PR that adds a `require` to `go.mod`
   is closed, however good the library is.
2. Routes come from **file conventions** and become **generated code**, checked by the
   compiler. No `reflect` to discover handlers.
3. Every new convention needs **three things**: a table test in the scanner
   (`internal/scan`), a route in `examples/blog` and an integration test in
   `examples/blog/blog_test.go`.
4. Tests first in the core. `go vet ./...` and `go test ./...` green.

## Environment

```bash
git clone https://github.com/emersonjoe/trilha && cd trilha
make test          # gofmt + vet + tests (includes the CLI e2e)
make dev-example   # examples/blog with reload
make golden        # rewrites the generator goldens (check the diff!)
make race          # go test -race ./... (TestConcorrencia is what gives it concurrency)
make fuzz          # 20s on each fuzz target, same as CI; FUZZTIME=2m make fuzz for longer
make fuzz-long     # 5 minutes per target, before a release
```

A failure found by fuzzing lands in `testdata/fuzz/<Target>/`. Commit that file with the
fix: it is the regression that keeps the bug from coming back.

Go 1.22 or newer. No other tool.

## Style

- Code, identifiers, comments and error messages in **English**. Everything public (site,
  README, community files, CLI, scaffold) in **English by default, with a Brazilian
  Portuguese translation in the same commit** (`/pt` on the site, `README.pt-BR.md`,
  `docs/pt-BR/`, the table in `cmd/trilha/i18n.go`). Specs and the constitution are in
  Portuguese.
- `gofmt`. Doc comment on every exported symbol.
- Small commits, imperative message with a prefix (`feat:`, `fix:`, `docs:`, `chore:`), no
  co-authorship trailers.

## Pull request

Use the template. One PR solves one issue. If you find another problem on the way, open
another issue instead of growing the PR. Review within a week; past that, mention
`@emersonjoe` on the PR.

`main` is protected: the PR needs green CI checks, one approval and every conversation
resolved before it is merged (see [GOVERNANCE.md](GOVERNANCE.md)). Use `git rebase main`
instead of merge to keep the history linear.

## Documentation

The site in `site/` is a Trilha app; the content lives in
`site/internal/docs/content/<en|pt>/**.md` (both trees have the same pages, matched by
position). Run `cd site && go run ../cmd/trilha dev --addr :3010` to see your changes. Every
visible behavior change needs documentation in the same PR, in both languages.

## License

By contributing you agree that your contribution is licensed under the project's MIT
license.
