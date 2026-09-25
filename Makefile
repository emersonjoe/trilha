.PHONY: test test-otel test-ui test-ui-a vet fmt security example dev-example golden api reload race fuzz fuzz-long bench bench-results bench-agent bench-agent-agents bench-agent-dry bench-agent-measure bench-agent-verify release

GOVULNCHECK_VERSION ?= v1.1.4
SECURITY_GO_VERSION ?= go1.25.13

test: vet
	go test ./...

vet:
	test -z "$$(gofmt -l *.go h internal cmd examples tmpl)"
	go vet ./...

# O módulo opcional do exportador OpenTelemetry: módulo próprio, com o SDK
# de fora, para que o núcleo continue só com a biblioteca padrão.
test-otel:
	test -z "$$(gofmt -l otel)"
	cd otel && go vet ./... && go test ./...

# Testes de UI, camada A (spec 164): o HTML servido, sem navegador — retrato
# normalizado, token CSRF, nonce, cookies, foco do 422. Já roda dentro do
# make test; este alvo é o recorte, para rodar só ele.
test-ui-a:
	go test . ./examples/blog/ ./internal/recipes/ ./internal/uidoc/ -run 'Snapshot|MatchGolden|Has|Focused|ParseElements|PaginasContraOGolden|PatternGoldens|Install'

# Camada B: o app de verdade no Chrome, pelo módulo uitest/ (go.mod próprio,
# com o chromedp). Sem Chrome os cenários pulam; UITEST_REQUIRED=1 reprova.
# Uma falha deixa uitest/report/<cenário>.txt.
test-ui:
	test -z "$$(gofmt -l uitest)"
	cd uitest && go vet ./... && go test -count=1 ./...

fmt:
	gofmt -w *.go h internal cmd examples tmpl otel uitest

# NIST SSDF/OWASP evidence. Go 1.22+ downloads the patched toolchain automatically.
security:
	test -z "$$(gofmt -l *.go h internal cmd examples tmpl)"
	GOTOOLCHAIN=$(SECURITY_GO_VERSION) go vet ./...
	GOTOOLCHAIN=$(SECURITY_GO_VERSION) go test -race ./...
	GOTOOLCHAIN=$(SECURITY_GO_VERSION) go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

golden:
	go test ./internal/gen/ ./internal/openapi/ ./internal/ctx/ ./internal/recipes/ ./ui/ ./internal/scaffold/ ./internal/uidoc/ ./internal/migrate/ ./internal/client/ ./internal/islands/ ./cmd/trilha/ ./examples/blog/ -update

# A superfície pública versionada: o diff de api/current.txt é a parte da
# revisão que diz o que quem usa o framework vai sentir.
api:
	go test . -run TestSuperficiePublica -update

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

# Régua da Fase 5: quanto um agente gasta para entregar uma feature. Exige
# `claude auth login`; um cenário por convenção medida x 3 execuções, custo real.
# Veja bench/agent/RESULTS.md.
bench-agent:
	cd bench/agent && go run . -runs 3 && cat RESULTS.md

# A mesma régua com o AGENTS.md da spec 044 no fixture: o "depois" da medição.
bench-agent-agents:
	cd bench/agent && go run . -runs 3 -agents && cat RESULTS.md

# Monta os cenários e prova que o teste escondido falha sem o agente; não gasta token.
bench-agent-dry:
	cd bench/agent && go run . -dry

# Fecha a spec do branch atual: testa, funde na main, marca a versão, publica a
# release e fecha as issues. Veja scripts/release.sh.
# Uso: make release VERSION=0.11.0 ISSUES="20 21" [DRY_RUN=1]
release:
	@test -n "$(VERSION)" || { echo 'uso: make release VERSION=X.Y.Z [ISSUES="20 21"] [DRY_RUN=1]'; exit 2; }
	./scripts/release.sh $(VERSION) $(if $(ISSUES),--issues "$(ISSUES)") $(if $(DRY_RUN),--dry-run)

# A régua v2 (spec 159): mede a série de economia — os dois lados de cada
# cenário com baseline — em results/results.json. Exige `claude auth login`;
# dezenas de execuções e custo real. O gate `bench-agent-verify` é quem decide
# se a série avança o marco (média ≥ MIN, piso 60% por cenário, regressão ≤ 5
# pontos); a primeira medição é do mantenedor.
bench-agent-measure:
	cd bench/agent && go run . -measure -runs 3

bench-agent-verify:
	cd bench/agent && go run . -verify -min $(if $(MIN),$(MIN),45)
