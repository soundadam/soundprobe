# Releasing NJUProbe through Homebrew

The release path is intentionally staged:

1. publish a stable NJUProbe source release;
2. publish and maintain the Formula in `soundadam/homebrew-tap`;
3. consider `homebrew/core` only after the project has a public stable release,
   macOS acceptance evidence, and sufficient independent use.

The repository and release assets must be publicly downloadable before a public
Formula is advertised.

## 1. Release prerequisites

Use a supported macOS machine for final acceptance. Confirm:

```sh
make test-offline
GOTOOLCHAIN=auto go test -race ./...
make tools
make build
./bin/njuprobe doctor --json
```

Perform the real operator tests from [TESTING.md](TESTING.md):

- NJU IPv4 success;
- NJU explicit IPv6 success or expected no-IPv6 failure;
- M-Lab success after consent;
- combined sequential success;
- partial provider failure behavior;
- Ctrl-C cancellation and history readback.

Review:

```sh
git diff --check
go mod verify
git status --short
```

Do not release from a dirty worktree.

## 2. Create the stable tag

For version `0.1.0`:

```sh
git switch main
git pull --ff-only
git tag -s v0.1.0 -m "NJUProbe v0.1.0"
git push origin v0.1.0
```

A signed tag is preferred. The tag must point at the exact commit that passed
operator and automated acceptance.

## 3. Build the source release and Formula

```sh
make release VERSION=0.1.0
```

This produces ignored local artifacts:

```text
dist/njuprobe-0.1.0.tar.gz
dist/Formula/njuprobe.rb
```

The archive is generated deterministically from tag `v0.1.0`; its SHA-256 is
inserted into the rendered Formula. Re-run the command and verify the archive
hash is unchanged.

Create a GitHub Release for `v0.1.0` and upload exactly:

```text
njuprobe-0.1.0.tar.gz
```

Do not edit or replace the asset after publishing the Formula. A changed source
requires a new version or Formula revision and a new checksum.

## 4. Test the Formula locally

With Homebrew updated on macOS:

```sh
brew update
brew audit --strict --new --formula ./dist/Formula/njuprobe.rb
HOMEBREW_NO_INSTALL_FROM_API=1 \
  brew install --build-from-source ./dist/Formula/njuprobe.rb
brew test njuprobe
njuprobe doctor
brew uninstall njuprobe
```

Also test upgrades from the previous released Formula once a previous version
exists:

```sh
brew upgrade njuprobe
njuprobe version
njuprobe doctor
```

Formula tests are offline. They must not run a bandwidth measurement.

## 5. Publish the tap

Maintain a public repository named:

```text
soundadam/homebrew-tap
```

Copy the validated Formula to:

```text
Formula/njuprobe.rb
```

Commit and push it. Users then install with:

```sh
brew tap soundadam/tap
brew install njuprobe
```

The fully qualified equivalent is:

```sh
brew install soundadam/tap/njuprobe
```

Document uninstall and rollback:

```sh
brew uninstall njuprobe
brew extract --version=0.1.0 njuprobe soundadam/tap
```

## 6. Bottles

The source Formula is sufficient for the first private acceptance and public tap
release. Bottles may be added after source builds pass consistently on supported
Apple Silicon and Intel macOS runners. Bottle generation must come from trusted
CI, include Homebrew-generated checksums, and never embed user history or consent
files.

## 7. `homebrew/core` consideration

Do not advertise `brew install njuprobe` from core until Homebrew accepts the
Formula. A future submission should demonstrate:

- a public, stable, immutable release;
- an actively maintained upstream project;
- successful builds and tests across Homebrew's supported matrix;
- checksummed and locked source dependencies;
- meaningful offline Formula tests;
- no runtime downloads or self-updates;
- clear licensing for NJUProbe and both installed helper executables.

Until then, the supported installation command is through
`soundadam/homebrew-tap`.
