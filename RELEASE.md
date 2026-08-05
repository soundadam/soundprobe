# Releasing soundprobe through Homebrew

The release path is intentionally staged:

1. create and push a stable source tag in the public upstream repository;
2. attach the deterministic source archive and checksum to that GitHub Release;
3. publish and maintain the Formula in `soundadam/homebrew-tap`;
4. consider `homebrew/core` only after macOS acceptance evidence and sufficient
   independent use.

The Formula homepage and release asset URL must resolve anonymously before the
Formula is published.

## 1. Release prerequisites

Use a supported macOS machine for final acceptance. Confirm:

```sh
make test-offline
GOTOOLCHAIN=auto go test -race ./...
make tools
make build
./bin/soundprobe doctor --json
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

For version `0.3.0`:

```sh
git switch main
git pull --ff-only
git tag -a v0.3.0 -m "soundprobe v0.3.0"
git push origin v0.3.0
```

A signed tag is preferred. The tag must point at the exact commit that passed
operator and automated acceptance.

## 3. Build the source release and Formula

```sh
make release VERSION=0.3.0
```

This produces ignored local artifacts:

```text
dist/soundprobe-0.3.0.tar.gz
dist/Formula/soundprobe.rb
```

The archive is generated deterministically from tag `v0.3.0`; its SHA-256 is
inserted into the rendered Formula. Re-run the command and verify the archive
hash is unchanged.

Create the immutable GitHub Release in `soundadam/soundprobe` from tag `v0.3.0`.

Upload:

```text
soundprobe-0.3.0.tar.gz
soundprobe-0.3.0.tar.gz.sha256
```

The checksum file contains the archive SHA-256 and filename. Confirm the public
release page and archive URL both return HTTP 200 without GitHub authentication.

Do not edit or replace the asset after publishing the Formula. A changed source
requires a new version or Formula revision and a new checksum.

## 4. Test the Formula locally

With Homebrew updated on macOS:

```sh
brew update
brew style ./dist/Formula/soundprobe.rb

test_tap=soundadam/soundprobe-rc-test
brew tap-new --no-git "$test_tap"
cp ./dist/Formula/soundprobe.rb "$(brew --repo "$test_tap")/Formula/soundprobe.rb"
brew audit --strict --new --formula "$test_tap/soundprobe"
HOMEBREW_NO_INSTALL_FROM_API=1 brew install --build-from-source "$test_tap/soundprobe"
brew test "$test_tap/soundprobe"
soundprobe doctor
brew uninstall soundprobe
brew untap "$test_tap"
```

Also test upgrades from the previous released Formula once a previous version
exists:

```sh
brew upgrade soundprobe
soundprobe version
soundprobe doctor
```

Formula tests are offline. They must not run a bandwidth measurement.

## 5. Publish the tap

Maintain a public repository named:

```text
soundadam/homebrew-tap
```

Copy the validated Formula to:

```text
Formula/soundprobe.rb
```

Commit and push it. Users then install with:

```sh
brew tap soundadam/tap
brew install soundprobe
```

The fully qualified equivalent is:

```sh
brew install soundadam/tap/soundprobe
```

Document uninstall and rollback:

```sh
brew uninstall soundprobe
brew extract --version=0.3.0 soundprobe soundadam/tap
```

## 6. Bottles

The source Formula is sufficient for the first public release. Bottles may be
added after source builds pass consistently on supported
Apple Silicon and Intel macOS runners. Bottle generation must come from trusted
CI, include Homebrew-generated checksums, and never embed user history or consent
files.

## 7. `homebrew/core` consideration

Do not advertise `brew install soundprobe` from core until Homebrew accepts the
Formula. A future submission should demonstrate:

- a public, stable, immutable release;
- an actively maintained upstream project;
- successful builds and tests across Homebrew's supported matrix;
- checksummed and locked source dependencies;
- meaningful offline Formula tests;
- no runtime downloads or self-updates;
- clear licensing for soundprobe and both installed helper executables.

Until then, the supported installation command is through
`soundadam/homebrew-tap`.
