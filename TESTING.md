# Testing NJUProbe

Routine tests never contact real NJU, M-Lab, or domestic bandwidth servers.
Real measurements are explicit operator acceptance steps.

## 1. Complete offline gate

```sh
make test-offline
GOTOOLCHAIN=auto go test -race ./...
```

The offline gate runs package tests, vet, executable-level mock-helper fixtures,
Homebrew template checks, and deterministic release artifact tests. It covers:

- station registry and IPv4/IPv6/dual expansion;
- explicit provider IDs such as `nju-campus-ipv4` and `nju-edge-ipv6`;
- selector recommendation, target toggling, family changes, and cancellation;
- sequential multi-target execution and skipped targets after cancellation;
- LibreSpeed arguments with telemetry disabled;
- M-Lab JSON events, transient live rates, and final summaries;
- success, partial, failure, timeout, malformed output, and cancellation;
- legacy schema-v1 history without an explicit `targets` field;
- normalized one-measurement-per-row CSV export;
- Bubble Tea inline rendering and cursor restoration;
- JSON and redirected output without ANSI or provider event leakage;
- atomic storage and `0700`/`0600` modes;
- deterministic source archive and Formula generation.

Focused tests:

```sh
GOTOOLCHAIN=auto go test ./internal/target ./internal/ui ./internal/provider/...
```

On Linux amd64, the reproducible KVM gate remains:

```sh
make ci
```

CI must not run `njuprobe stations` because that command intentionally performs
real lightweight reachability probes.

## 2. Build and helpers

```sh
make tools
make build
./bin/njuprobe doctor --json
```

Expected helper versions:

```sh
.tools/bin/librespeed-cli --version | head -1
cat .tools/bin/ndt7-client.version
```

```text
librespeed-cli v1.0.13-campus.1 (...)
v0.10.1
```

Helper discovery order:

1. installed `../libexec/njuprobe`;
2. repository-local `.tools/bin`;
3. explicit developer PATH fallback.

## 3. M-Lab consent

```sh
./bin/njuprobe consent status
./bin/njuprobe consent accept
./bin/njuprobe consent status
```

Acceptance requires a terminal and the exact word `accept`. A noninteractive
plan containing M-Lab must fail before contacting it when consent is absent. A
plan without M-Lab must not ask for M-Lab consent.

## 4. Station discovery and selector

Station probes are lightweight but real:

```sh
./bin/njuprobe stations
./bin/njuprobe stations --json
```

Verify that every registry entry has a family/status row and M-Lab is shown as
automatic rather than as a pinned server.

Run the selector:

```sh
./bin/njuprobe
```

Check:

- the selector clears before progress begins;
- Campus is recommended only when reachable for the chosen family;
- NJU Edge is displayed as disabled with a browser-verification explanation;
- when Campus is unavailable, M-Lab alone is recommended;
- `4`, `6`, and `d` switch family modes;
- IPv4-only domestic stations are disabled in IPv6 mode;
- Space toggles stations and Enter starts the exact visible order;
- q, Esc, and Ctrl-C cancel without creating a history entry.

## 5. Real NJU acceptance

These commands perform real uploads and downloads.

### Campus

```sh
./bin/njuprobe campus --ipv4 --no-save --json
./bin/njuprobe campus --ipv6 --no-save --json
```

Expected target IDs:

```text
nju-campus-ipv4
nju-campus-ipv6
```

The IPv6 result must never contain an IPv4 family or server.

### Public Edge limitation

```sh
./bin/njuprobe edge --no-save --json
./bin/njuprobe run --targets nju-edge --family dual --no-save --json
```

Both commands must exit `1` before starting LibreSpeed and report that NJU Edge
is unavailable in terminal mode because its official backend requires browser
verification. `njuprobe stations` must show both Edge families as `unsupported`.
Do not add automated challenge solving to the acceptance test.

## 6. Domestic station acceptance

Run individual stations before a complete batch:

```sh
./bin/njuprobe run --targets cernet --family ipv4 --no-save --json
./bin/njuprobe run --targets qlu --family ipv4 --no-save --json
./bin/njuprobe run --targets tongji --family ipv4 --no-save --json
```

Then validate sequential batch behavior:

```sh
./bin/njuprobe domestic --no-save --json
```

Expected target order:

```json
["cernet-ipv4", "qlu-ipv4", "tongji-ipv4"]
```

One failed station must not stop later stations. Inspect helper arguments during
offline tests to ensure `--telemetry-level disabled` is always present.

## 7. M-Lab and mixed plans

```sh
./bin/njuprobe mlab --no-save --json
./bin/njuprobe run --targets nju-campus,mlab --family ipv4 --no-save --json
./bin/njuprobe run --targets nju-campus,mlab --family dual --no-save --json
```

M-Lab uses automatic Locate selection. During a TTY run its download and upload
rates should update within the same fixed-height panel. NJU or domestic failures
must not prevent M-Lab from running when it is later in the selected plan.

## 8. Terminal rendering

For an interactive combined plan, verify:

- explicit labels such as `NJU Edge · IPv6`;
- one equal four-row panel per target;
- fixed panel height while live M-Lab events arrive;
- observed LibreSpeed rates with no fabricated samples or percentage;
- no alternate-screen enter/leave sequences;
- one durable final summary after the live block clears;
- cursor hidden during rendering and restored at completion.

For redirected output:

```sh
./bin/njuprobe run --targets nju-campus --family dual --no-save > /tmp/plain.txt
```

`/tmp/plain.txt` must have no ANSI bytes and no raw JSON.

For JSON:

```sh
./bin/njuprobe run --targets nju-campus --family dual --no-save --json > /tmp/run.json
python3 -m json.tool /tmp/run.json
```

The file must contain exactly one JSON document.

## 9. Cancellation

Start a multi-target plan and press Ctrl-C during an active target:

```sh
./bin/njuprobe run --targets nju-campus,mlab --family dual
```

Expected exit code: `130`. The active target is cancelled, every later target is
skipped, and no later helper starts.

## 10. History and export

Save a labeled multi-target run:

```sh
./bin/njuprobe run \
  --targets nju-campus,mlab \
  --family ipv4 \
  --label daily \
  --note "home Wi-Fi"
```

Read it back:

```sh
./bin/njuprobe last --json
./bin/njuprobe history --limit 10
./bin/njuprobe show RUN_ID --json
```

Verify `targets` matches measurement order. Existing 0.1 history files without
`targets` must still load.

Export:

```sh
./bin/njuprobe export --format jsonl --output /tmp/njuprobe.jsonl
./bin/njuprobe export --format csv --output /tmp/njuprobe.csv
```

JSONL contains one run per line. CSV contains one row per measurement; a run with
five targets produces five data rows.

Permission checks:

```sh
stat -f '%Sp %N' "$HOME/Library/Application Support/njuprobe/history/v1"
stat -f '%Sp %N' "$HOME/Library/Application Support/njuprobe/history/v1/"*.json
```

Expected:

```text
directory: drwx------
files:     -rw-------
```

## 11. Homebrew gate

On supported macOS arm64 versions:

```sh
brew style Formula/njuprobe.rb
brew audit --strict --new --formula soundadam/tap/njuprobe
HOMEBREW_NO_INSTALL_FROM_API=1 brew install --build-from-source soundadam/tap/njuprobe
brew test soundadam/tap/njuprobe
njuprobe version
njuprobe doctor --json
```

Formula tests remain offline and do not invoke the selector, station probes, or
real bandwidth measurements.
