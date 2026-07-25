.PHONY: build tools verify-mod test test-race test-campus-provider test-mlab-provider test-campus-fixture test-run-fixture test-homebrew-template test-release-artifact test-offline vet check ci release clean clean-tools

BINARY := bin/njuprobe
GO ?= go
GOTOOLCHAIN ?= auto

build:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO) build -o $(BINARY) ./cmd/njuprobe

verify-mod:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO) mod verify

tools:
	./scripts/install-dev-tools.sh

test:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO) test ./...

test-race:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO) test -race ./...

test-campus-provider:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO) test -v ./internal/helper ./internal/provider ./internal/provider/campus

test-mlab-provider:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO) test -v ./internal/helper ./internal/provider ./internal/provider/mlab

test-campus-fixture:
	./scripts/test-campus-fixture.sh

test-run-fixture:
	./scripts/test-run-fixture.sh

test-homebrew-template:
	./scripts/test-homebrew-template.sh

test-release-artifact:
	./scripts/test-release-artifact.sh

test-offline: check test-campus-fixture test-run-fixture test-homebrew-template test-release-artifact

vet:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO) vet ./...

check: test vet

ci:
	./scripts/run-local-ci.sh

release:
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make release VERSION=0.1.0" >&2; exit 1)
	./scripts/build-release.sh "$(VERSION)"

clean:
	rm -rf bin

clean-tools:
	rm -rf .tools
