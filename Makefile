.PHONY: test vet fmt example dev-example golden reload race fuzz fuzz-long bench bench-results release

test: vet
	go test ./...

vet:
	test -z "$$(gofmt -l *.go h internal cmd examples tmpl)"
	go vet ./...

fmt:
	gofmt -w *.go h internal cmd examples tmpl

golden:
	go test ./internal/gen/ -update

# O detector de corrida sobre a suíte inteira; TestConcorrencia é quem lhe dá
# concorrência de verdade.
race:
	go test -race ./...

# Uma rodada curta em cada alvo, igual à do CI.
fuzz:
	./scripts/fuzz.sh $(if $(FUZZTIME),$(FUZZTIME))

# Rodada longa, para antes de uma release ou depois de mexer no parser.
fuzz-long:
	FUZZTIME=5m ./scripts/fuzz.sh

example:
	cd examples/blog && go run ../../cmd/trilha gen && go run .

dev-example:
	cd examples/blog && go run ../../cmd/trilha dev

reload:
	./scripts/measure-reload.sh

bench:
	cd bench && go test -run XXX -bench . -benchmem

# Regrava bench/RESULTS.md com a máquina atual.
bench-results:
	cd bench && sh results.sh > RESULTS.md && cat RESULTS.md

# Fecha a spec do branch atual: testa, funde na main, marca a versão, publica a
# release e fecha as issues. Veja scripts/release.sh.
# Uso: make release VERSION=0.11.0 ISSUES="20 21" [DRY_RUN=1]
release:
	@test -n "$(VERSION)" || { echo 'uso: make release VERSION=X.Y.Z [ISSUES="20 21"] [DRY_RUN=1]'; exit 2; }
	./scripts/release.sh $(VERSION) $(if $(ISSUES),--issues "$(ISSUES)") $(if $(DRY_RUN),--dry-run)
