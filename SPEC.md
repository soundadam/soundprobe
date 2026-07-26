# NJUProbe v0.1 implementation specification

## 1. Product contract

NJUProbe answers two separate questions:

1. Can this Mac reach the NJU campus measurement service, and what capacity does
   that path expose?
2. What single-stream bulk-transport performance does the current public
   Internet path expose through M-Lab NDT7?

The results describe different protocols and must be presented side by side,
not ranked or collapsed into one synthetic score.

The product is a macOS-first Go CLI. It has no daemon, privileged helper,
account, cloud synchronization, automatic scheduler, or dependency on
soundVPN/SFM/NJUConnect.

## 2. Provider behavior

### 2.1 NJU campus provider

Keep the two NJU server definitions pinned with the release, validate that the
selected ID resolves to the expected NJU hostname, and provide the JSON to the
pinned LibreSpeed CLI helper through `--local-json -`. Then run the helper with
the equivalent of:

```text
--local-json -
--server 1
--duration 10
--concurrent 3
--no-icmp
--json
```

Do not pass `--share` or any telemetry option. The default test includes both
download and upload. Server ID 1 is the IPv4 default. `campus --ipv6` explicitly
selects server ID 2 and passes `--ipv6` for the measurement connections.
Supplying the pinned configuration is not a measurement fallback; the selected
IPv6 measurement must never silently fall back to IPv4. Routine tests remain
offline and do not fetch the server list from NJU.

Parse the final JSON for server/client metadata, ping, jitter, upload/download
bit rates, and transferred byte counts. LibreSpeed does not provide a stable
machine-readable live-rate stream, so the UI shows the provider, elapsed time,
and an indeterminate running state until the final result. It must not invent a
live throughput number.

### 2.2 M-Lab provider

Run the pinned `ndt7-client` helper with JSON events, TLS verification enabled,
client name `njuprobe`, and a 55-second whole-test timeout. Use M-Lab's Locate
service rather than fixing a server. Run both download and upload.

Consume `starting`, `connected`, `measurement`, `error`, and `complete` events.
Measurement events may drive an ephemeral live rate in the terminal. Persist
only the final summary, selected server FQDN, client public IP/connection
identity available from NDT7, bytes, elapsed time, and final rate.

NDT7 is a single-stream measurement. Store `method = ndt7-single-stream` and do
not present it as directly equivalent to the three-stream NJU result.

### 2.3 Run ordering and result semantics

Bare `njuprobe` is equivalent to `njuprobe run`:

1. capture one best-effort network-context snapshot;
2. run the NJU provider;
3. run the M-Lab provider;
4. atomically save one combined summary unless `--no-save` was supplied;
5. replace the live view with the final compact table.

Providers run sequentially so they never compete for bandwidth.

- Successful attempted phase: store its measured value.
- Attempted but unreachable phase: store `0` and an error stage/code.
- Intentionally disabled or not run phase: store `null`.
- One successful provider and one failed provider: run status `partial`.
- Ctrl-C: stop the active child, do not start the next provider, save status
  `cancelled`, and return exit code 130.

Speed is the average over the provider's effective measurement window. Live UI
rates are explicitly transient and are not the saved result.

## 3. Terminal and command interface

### 3.1 Commands

```text
njuprobe
njuprobe run [--label TEXT] [--note TEXT] [--no-save]
njuprobe campus [--ipv4|--ipv6] [--label TEXT] [--note TEXT] [--no-save]
njuprobe mlab [--label TEXT] [--note TEXT] [--no-save]
njuprobe history [--limit N]
njuprobe last [--json]
njuprobe show RUN_ID [--json]
njuprobe export --format jsonl|csv --output PATH
njuprobe consent status|accept|revoke
njuprobe doctor [--json]
njuprobe version
```

Support global `--json`. When stdout is not a TTY, automatically disable the
dynamic renderer. JSON mode emits exactly one final JSON document. Plain
non-TTY mode emits one final summary and errors, never provider event logs.

Exit codes are stable:

- `0`: all requested measurements completed;
- `2`: partial result or an unreachable requested provider;
- `1`: configuration, helper, consent, or internal failure;
- `130`: cancelled by the operator.

### 3.2 Interactive renderer

Use Bubble Tea v2 in inline mode, not alternate-screen mode. Redraw one fixed
block at no more than four frames per second. Campus and M-Lab are peer
providers: they use the same status, activity, rate, and detail rows; their only
ordering distinction is that Campus runs before M-Lab. The view shows:

- NJUProbe version and active interface/SSID;
- explicit sequential execution order;
- independent waiting/active/complete/failed/cancelled state for each provider;
- one animated activity bar for each provider, without a fabricated percentage;
- per-provider and total elapsed time;
- transient M-Lab download/upload rates when measurement events are available;
- completed Campus/M-Lab download and upload values;
- selected server or a bounded provider-specific error;
- the Ctrl-C hint.

Campus uses an indeterminate activity bar because LibreSpeed does not expose a
stable machine-readable live-rate stream. M-Lab uses the same panel and activity
bar, with transient live rates added as NDT7 measurement events arrive. Either
provider may fail independently; a Campus failure must not visually subordinate
M-Lab, and an M-Lab failure must use the same failure treatment as Campus.

On success, failure, or cancellation, restore the cursor and replace the block
with one durable final table. Tests must verify that raw JSON, ANSI fragments,
and unbounded status lines do not leak.

## 4. Consent and privacy

Before the first M-Lab test, explain that M-Lab collects the ISP-provided public
IP and measurement results and publishes/retains experiment data indefinitely.
Require explicit interactive acceptance. Persist:

```json
{
  "schemaVersion": 1,
  "provider": "mlab",
  "policyVersion": "v5-2026-05-03",
  "acceptedAt": "RFC3339 timestamp",
  "toolVersion": "njuprobe version"
}
```

`consent revoke` removes only this local acceptance record. If no prior consent
exists in a noninteractive invocation, fail before contacting M-Lab. The NJU-
only command never requires M-Lab consent.

NJUProbe has no own analytics or remote result service. Local public IP values
are not redacted. Do not add IP geolocation, ASN enrichment, or another public-
IP lookup provider in v0.1.

## 5. Permanent summary storage

Use these macOS paths:

```text
~/Library/Application Support/njuprobe/history/v1/<run-id>.json
~/Library/Application Support/njuprobe/consent.json
```

Create directories as `0700` and files as `0600`. Write to a same-directory
temporary file, fsync as appropriate, then atomically rename. Never prune
history automatically. Do not implement a bulk-clear command in v0.1. Export
is read-only.

Each run file contains:

- schema version, UUID run ID, tool version;
- start/end timestamps, command, final status, label, and note;
- macOS version and architecture;
- active interface, interface kind, SSID/BSSID when permitted;
- local IPv4/IPv6 addresses, default gateway, and DNS servers;
- one measurement object per requested provider;
- provider/method, server name/FQDN/address, client public IP;
- ping, jitter, download/upload Mbps, byte counts, test duration, concurrency;
- provider status and an optional sanitized failure object.

The failure object contains a stable stage (`helper`, `dns`, `connect`,
`download`, `upload`, `timeout`, or `cancelled`), stable code, and sanitized
message. It must not contain environment dumps or provider raw events.

SSID/BSSID and other best-effort fields are `null` when macOS permissions or
hardware do not expose them; their absence must not fail a bandwidth test. A
provider-reported client public IP is also `null` when that provider omits it;
a missing client IP must not invalidate otherwise complete measurement data.

## 6. Helper discovery and packaging

At runtime resolve helpers in this order:

1. paths relative to the installed `njuprobe` executable under
   `../libexec/njuprobe`;
2. repository-local `.tools/bin` for development;
3. PATH fallback for an explicitly documented developer workflow.

Production installation must not silently pick arbitrary newer helpers from
PATH when the pinned libexec helper exists. Include helper versions in every
saved run.

Provide a reproducible development command that downloads/builds the pinned
helper source into the ignored `.tools/bin` directory and verifies expected
versions. Do not commit helper binaries or source copies.

The owner has approved public distribution through Homebrew. Releases use an
immutable, checksummed source asset and a Formula that builds NJUProbe plus the
two pinned helper resources from source. Publication starts in the maintained
`soundadam/homebrew-tap`; a later `homebrew/core` submission is conditional on a
public stable release, supported-macOS acceptance, Formula audit results, and
Homebrew review.

Formula tests are offline and may invoke only version, diagnostics, and
read-only history commands. They must not contact NJU or M-Lab.

## 7. Verification gates

Automated tests use mock helpers and committed sanitized fixtures only:

- parse LibreSpeed final JSON and NDT7 JSON event streams;
- cover unreachable, timeout, partial, malformed output, helper crash, and
  cancellation;
- verify zero versus null semantics and stable exit codes;
- verify fixed-block UI state transitions and cursor restoration;
- verify redirected/JSON output contains no ANSI;
- verify atomic storage, modes `0700`/`0600`, permanent history, and export;
- verify consent accept/status/revoke and noninteractive fail-closed behavior;
- verify helper precedence and version capture.

CI must never execute a real bandwidth test. Operator acceptance after a local
build covers NJU success, NJU unreachable with M-Lab continuing, M-Lab failure,
Ctrl-C, history readback, and local Homebrew installation. Formula tests invoke
only `njuprobe version` and other offline/read-only commands.

## 8. Release boundary

Do not tag or claim v0.1.0 readiness until all automated gates, real operator
acceptance, dependency notices, and local Homebrew installation tests pass on a
supported macOS machine.

A v0.1.0 release consists of:

1. a public immutable source release asset;
2. recorded SHA-256 checksums for NJUProbe and both helper sources;
3. a rendered `njuprobe` Formula in `soundadam/homebrew-tap`;
4. successful `brew audit`, source installation, `brew test`, and upgrade checks;
5. documented install, privacy, and rollback instructions.

Inclusion in `homebrew/core` is not part of the v0.1.0 acceptance claim and must
not be implied before Homebrew review.
