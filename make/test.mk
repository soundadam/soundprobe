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
