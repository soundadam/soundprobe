# Testing NJUProbe

## 1. Fully offline verification

These commands do not contact NJU or M-Lab and do not consume bandwidth beyond
normal dependency access already required by the Go toolchain:

```sh
make check
make test-campus-provider
make test-campus-fixture
```

`make test-campus-fixture` builds the real `njuprobe` executable, copies it to a
temporary directory, places a mock `librespeed-cli` on `PATH`, and runs both:

```text
njuprobe campus --no-save --json
njuprobe campus --ipv6 --no-save --json
```

The mock helper returns committed sanitized fixtures. The script verifies a
successful result, the selected IP family, the pinned helper version, and the
absence of ANSI escapes in JSON output.

Run the complete offline gate with one command:

```sh
make test-offline
```

## 2. Install the pinned development helper

This downloads the exact LibreSpeed CLI source tag and commit, builds it as a
separate executable, and installs it under the ignored `.tools/bin` directory:

```sh
make tools
.tools/bin/librespeed-cli --version
```

The expected first line is:

```text
librespeed-cli v1.0.13 (...)
```

Helper discovery uses this order:

1. `../libexec/njuprobe` relative to the installed `njuprobe` executable;
2. repository-local `.tools/bin`;
3. `PATH` as an explicitly documented developer fallback.

A broken higher-priority helper fails closed instead of silently selecting a
newer executable from `PATH`.

## 3. Real NJU operator acceptance

The commands in this section perform a real upload and download test and consume
network bandwidth. Start with an unsaved JSON run:

```sh
make tools
make build
./bin/njuprobe campus --no-save --json
printf 'exit code: %s\n' "$?"
```

Expected exit codes:

- `0`: campus measurement completed;
- `2`: the provider was attempted but failed, timed out, or was unreachable;
- `1`: helper discovery, version, configuration, storage, or internal failure;
- `130`: cancelled with Ctrl-C.

Explicit IPv4 and IPv6 checks:

```sh
./bin/njuprobe campus --ipv4 --no-save --json
./bin/njuprobe campus --ipv6 --no-save --json
```

The IPv6 command selects NJU server ID 2 and passes `--ipv6`; it does not fall
back to server ID 1 or IPv4. An environment without functional IPv6 should
therefore return a failed IPv6 measurement rather than a successful IPv4 result.

To verify persistence and readback, omit `--no-save`:

```sh
./bin/njuprobe campus --label "operator-test" --note "real NJU acceptance"
./bin/njuprobe history --limit 5
./bin/njuprobe show RUN_ID --json
```

Replace `RUN_ID` with the identifier printed by the campus summary or history
command. Saved files should be under:

```text
~/Library/Application Support/njuprobe/history/v1/
```

Directories must have mode `0700`; summary files must have mode `0600`.

## 4. Current iteration boundary

The `campus` command is implemented. `mlab` and the combined bare/`run` command
remain unavailable until the NDT7 provider is implemented. Automated and
Formula tests must not run a real bandwidth measurement.
