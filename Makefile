.PHONY: test vet fmt example dev-example golden reload

test: vet
	go test ./...

vet:
	test -z "$$(gofmt -l *.go h internal cmd examples tmpl)"
	go vet ./...

fmt:
	gofmt -w *.go h internal cmd examples tmpl

golden:
	go test ./internal/gen/ -update

example:
	cd examples/blog && go run ../../cmd/trilha gen && go run .

dev-example:
	cd examples/blog && go run ../../cmd/trilha dev

reload:
	./scripts/measure-reload.sh
