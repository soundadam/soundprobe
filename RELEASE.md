# Releasing NJUProbe through Homebrew

The release path is intentionally staged:

1. create a stable source tag in the private upstream repository;
2. publish the immutable source archive in the public `soundadam/homebrew-dist`;
3. publish and maintain the Formula in `soundadam/homebrew-tap`;
4. consider `homebrew/core` only after the project has a public stable release,
   macOS acceptance evidence, and sufficient independent use.

The upstream repository may remain private. The Formula homepage and source URL
must resolve anonymously through `soundadam/homebrew-dist` before a public
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
inserted into the rendered Formula. The Formula points to the public
`homebrew-dist` release tag `njuprobe-v0.1.0`, not to the private upstream
repository. Re-run the command and verify the archive hash is unchanged.

Create an immutable GitHub Release in `soundadam/homebrew-dist` with tag:

```text
njuprobe-v0.1.0
```

Upload:

```text
njuprobe-0.1.0.tar.gz
njuprobe-0.1.0.tar.gz.sha256
provenance.json
```

The checksum file contains the archive SHA-256 and filename. Provenance schema
version 1 records the upstream source repository, source tag, source commit,
distribution repository, distribution tag, archive size, and archive checksum.
Confirm the public release page and archive URL both return HTTP 200 without
GitHub authentication.

Do not edit or replace the asset after publishing the Formula. A changed source
requires a new version or Formula revision and a new checksum.

## 4. Test the Formula locally

With Homebrew updated on macOS:

```sh
brew update
brew style ./dist/Formula/njuprobe.rb

test_tap=soundadam/njuprobe-rc-test
brew tap-new --no-git "$test_tap"
cp ./dist/Formula/njuprobe.rb "$(brew --repo "$test_tap")/Formula/njuprobe.rb"
brew audit --strict --new --formula "$test_tap/njuprobe"
HOMEBREW_NO_INSTALL_FROM_API=1 brew install --build-from-source "$test_tap/njuprobe"
brew test "$test_tap/njuprobe"
njuprobe doctor
brew uninstall njuprobe
brew untap "$test_tap"
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
