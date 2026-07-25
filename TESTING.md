# Testing NJUProbe

Routine tests never contact NJU or M-Lab. Real bandwidth tests are explicit
operator acceptance steps.

## 1. Complete offline gate

```sh
make test-offline
```

This runs:

```text
go test ./...
go vet ./...
scripts/test-campus-fixture.sh
scripts/test-run-fixture.sh
scripts/test-homebrew-template.sh
scripts/test-release-artifact.sh
```

The fixtures exercise the real `njuprobe` executable with mock helper processes
and verify:

- NJU IPv4 and explicit IPv6 selection;
- M-Lab JSON event parsing and final summary parsing;
- campus-to-M-Lab sequential ordering;
- success, partial failure, timeout, malformed output, and cancellation;
- stable exit codes and zero-versus-null semantics;
- `doctor`, saved history, `last`, and `history`;
- JSON output without ANSI escape sequences;
- Homebrew Formula template resources and unresolved placeholders.
- deterministic release archives and Formula output, including preservation of
  the previous valid artifacts when a staged release build fails or is
  interrupted during publication, rejection of concurrent publication,
  atomic publication of complete lock-owner metadata before the lock becomes
  visible, recovery after `SIGKILL` immediately following lock acquisition,
  compatibility with legacy directory locks, recovery of locks owned by exited
  publishers or a reused PID, and fail-closed handling of malformed identities.
- release validation snapshots staged and unstaged tracked candidate changes
  into a temporary commit before invoking `git archive`, so pre-commit checks
  exercise the candidate source tree rather than silently archiving prior
  `HEAD` content.
- deterministic source archives with safe paths, required files, and a matching
  Formula checksum; published archives and Formula files are normalized to
  mode `0644` independently of the invoking user's umask.

Focused provider tests:

```sh
make test-campus-provider
make test-mlab-provider
```

Race detector:

```sh
GOTOOLCHAIN=auto go test -race ./...
```

For the reproducible Linux amd64 KVM gate used by local development:

```sh
make ci
```

This target verifies the module cache, runs the complete offline gate and race
suite, and builds the CLI. On Linux amd64 it uses either an already installed
exact Go version or the repository-pinned, checksummed Go archive; automatic Go
toolchain downloads are disabled for the gate. It may install a C compiler when
one is missing, but it never runs a real NJU or M-Lab bandwidth measurement.

## 2. Build and diagnose the pinned helpers

NJUProbe uses separately installed helper executables:

```sh
make tools
make build
./bin/njuprobe doctor
```

Expected diagnostics:

```text
Campus   ready
M-Lab    ready
```

Inspect pinned versions:

```sh
.tools/bin/librespeed-cli --version | head -1
cat .tools/bin/ndt7-client.version
```

Expected values:

```text
librespeed-cli v1.0.13 (...)
v0.10.1
```

Helper discovery order is:

1. `../libexec/njuprobe` relative to the installed executable;
2. repository-local `.tools/bin`;
3. `PATH` as a documented developer fallback.

A broken higher-priority helper fails closed instead of silently selecting a
newer executable.

## 3. First-run consent

Before a combined or M-Lab-only test:

```sh
./bin/njuprobe consent status
./bin/njuprobe consent accept
./bin/njuprobe consent status
```

Acceptance requires an interactive terminal and the exact word `accept`.
Noninteractive or JSON execution without a current record fails before M-Lab is
contacted.

Revoke the local record with:

```sh
./bin/njuprobe consent revoke
```

## 4. Real operator acceptance

The following commands perform real uploads and downloads.

### NJU only

```sh
./bin/njuprobe campus --no-save --json
printf 'exit code: %s\n' "$?"
```

Explicit families:

```sh
./bin/njuprobe campus --ipv4 --no-save --json
./bin/njuprobe campus --ipv6 --no-save --json
```

The IPv6 command selects NJU server ID 2 and passes `--ipv6`. It must never
silently return an IPv4 measurement. A machine without functional IPv6 should
produce a failed IPv6 result with exit code `2`.

### M-Lab only

```sh
./bin/njuprobe mlab --no-save --json
printf 'exit code: %s\n' "$?"
```

Verify the result contains:

```text
provider: mlab
method: ndt7-single-stream
helperVersion: v0.10.1
serverFqdn: an M-Lab server selected through Locate
```

### Combined daily run

```sh
./bin/njuprobe
```

The interactive view should remain one fixed block. NJU completes first; only
then should M-Lab begin. On completion, the block is replaced by one durable
summary containing both providers.

For script-safe acceptance:

```sh
./bin/njuprobe run --no-save --json > /tmp/njuprobe.json
python3 -m json.tool /tmp/njuprobe.json
```

The file must contain exactly one JSON document and no ANSI bytes.

### Cancellation

Start a test and press Ctrl-C:

```sh
./bin/njuprobe
```

Then inspect the shell status:

```sh
echo $?
```

Expected exit code: `130`. The active helper must stop, any later provider must
be marked skipped, and the saved run status must be `cancelled`.

## 5. History and high-frequency commands

Run and save a labeled result:

```sh
./bin/njuprobe run \
  --label "daily" \
  --note "home Wi-Fi"
```

Read it back:

```sh
./bin/njuprobe last
./bin/njuprobe last --json
./bin/njuprobe history --limit 10
./bin/njuprobe show RUN_ID --json
```

Export all history:

```sh
./bin/njuprobe export --format jsonl --output /tmp/njuprobe.jsonl
./bin/njuprobe export --format csv --output /tmp/njuprobe.csv
```

macOS storage path:

```text
~/Library/Application Support/njuprobe/history/v1/
```

Permission checks:

```sh
stat -f '%Sp %N' "$HOME/Library/Application Support/njuprobe/history/v1"
stat -f '%Sp %N' "$HOME/Library/Application Support/njuprobe/history/v1/"*.json
```

Expected modes:

```text
directory: drwx------
files:     -rw-------
```

## 6. Homebrew gate on macOS

The Linux development gate validates Formula generation but cannot replace an
actual Homebrew test. On a supported macOS machine:

```sh
make test-offline
./scripts/test-homebrew-template.sh
```

After creating a stable release asset and rendering the Formula:

```sh
brew update
brew audit --strict --new --formula ./dist/Formula/njuprobe.rb
HOMEBREW_NO_INSTALL_FROM_API=1 \
  brew install --build-from-source ./dist/Formula/njuprobe.rb
brew test njuprobe
brew uninstall njuprobe
```

The Formula test is offline. It checks `version`, `doctor --json`, and empty
history only. It must never execute `njuprobe`, `run`, `campus`, or `mlab` as a
measurement command.

## Exit codes

```text
0    all requested measurements completed
2    attempted provider failed, timed out, or combined result is partial
1    helper, version, consent, configuration, storage, renderer, or internal error
130  cancelled by the operator
```
