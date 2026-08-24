ci:
	./scripts/run-local-ci.sh

release:
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make release VERSION=0.1.0" >&2; exit 1)
	./scripts/build-release.sh "$(VERSION)"

# GoReleaser-based flow (see RELEASE.md). Requires a local goreleaser install.

# Full local dry run: builds every platform and archive into dist/ without
# publishing anything.
snapshot:
	goreleaser release --snapshot --clean

# Validate .goreleaser.yaml.
release-check:
	goreleaser check
