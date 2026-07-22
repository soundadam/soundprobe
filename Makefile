.PHONY: build tools test test-campus-provider test-mlab-provider test-campus-fixture test-run-fixture test-homebrew-template test-offline vet check release clean clean-tools

BINARY := bin/njuprobe
GO ?= go

build:
	GOTOOLCHAIN=auto $(GO) build -o $(BINARY) ./cmd/njuprobe

tools:
	./scripts/install-dev-tools.sh

test:
	GOTOOLCHAIN=auto $(GO) test ./...

test-campus-provider:
	GOTOOLCHAIN=auto $(GO) test -v ./internal/helper ./internal/provider ./internal/provider/campus

test-mlab-provider:
	GOTOOLCHAIN=auto $(GO) test -v ./internal/helper ./internal/provider ./internal/provider/mlab

test-campus-fixture:
	./scripts/test-campus-fixture.sh

test-run-fixture:
	./scripts/test-run-fixture.sh

test-homebrew-template:
	./scripts/test-homebrew-template.sh

test-offline: check test-campus-fixture test-run-fixture test-homebrew-template

vet:
	GOTOOLCHAIN=auto $(GO) vet ./...

check: test vet

release:
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make release VERSION=0.1.0" >&2; exit 1)
	./scripts/build-release.sh "$(VERSION)"

clean:
	rm -rf bin

clean-tools:
	rm -rf .tools
