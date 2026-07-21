.PHONY: build tools test test-campus-provider test-campus-fixture test-offline vet check clean clean-tools

BINARY := bin/njuprobe

build:
	go build -o $(BINARY) ./cmd/njuprobe

tools:
	./scripts/install-dev-tools.sh

test:
	go test ./...

test-campus-provider:
	go test -v ./internal/helper ./internal/provider ./internal/provider/campus

test-campus-fixture:
	./scripts/test-campus-fixture.sh

test-offline: check test-campus-fixture

vet:
	go vet ./...

check: test vet

clean:
	rm -rf bin

clean-tools:
	rm -rf .tools
