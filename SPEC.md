# SoundProbe v0.2 implementation specification

## 1. Product contract

SoundProbe measures explicitly selected network targets. A target represents one
measurement purpose and, where relevant, one address family. Results from
separate targets must never be silently substituted, ranked, or collapsed into a
synthetic score.

The maintained targets are:

| Station ID | Measurement purpose | Families | Method |
| --- | --- | --- | --- |
| `nju-campus` | current path to NJU's campus-internal service | IPv4, IPv6 | LibreSpeed, three concurrent streams |
| `nju-edge` | public path to NJU's internet-facing edge | IPv4, IPv6 | displayed but terminal execution disabled by browser verification |
| `mlab` | general Internet bulk-transport performance | automatic | M-Lab NDT7, single stream |
| `cernet` | CERNET public station | IPv4 | LibreSpeed, three concurrent streams |
| `qlu` | Qilu University of Technology station | IPv4 | LibreSpeed, three concurrent streams |
| `tongji` | Tongji University station | IPv4 | LibreSpeed, three concurrent streams |

NJU Campus and NJU Edge answer different questions. NJU Edge remains visible in
the product model, but its official backend currently redirects terminal clients
to a browser-verification challenge. SoundProbe must not bypass that protection or
publish a target that returns null. Edge selection therefore fails before
measurement with a bounded explanation. IPv4 and IPv6 are independent
measurements for supported stations; a dual plan expands them into ordered
targets.

The product is a macOS-first Go CLI. It has no daemon, privileged helper,
account, cloud synchronization, automatic scheduler, or dependency on
soundVPN/SFM/NJUConnect.

## 2. Target identities

Stable JSON provider IDs encode station and family:

```text
nju-campus-ipv4
nju-campus-ipv6
nju-edge-ipv4
nju-edge-ipv6
mlab
cernet-ipv4
qlu-ipv4
tongji-ipv4
```

The legacy provider ID `campus` remains valid only so schema-v1 history written
by SoundProbe 0.1 can still be read. New measurements use explicit IDs.

Every new run summary includes an ordered `targets` array. The number and order
of measurement objects must match the requested targets. Duplicate targets are
rejected or deduplicated before execution, not executed twice accidentally.

## 3. LibreSpeed target behavior

Use the maintained LibreSpeed CLI source in `components/librespeed-cli`, based
on pinned upstream v1.0.13. Keep every supported server definition in the
release and pass one selected definition through `--local-json -`. Do not fetch
an uncontrolled remote server directory during routine execution.

Run the helper with the equivalent of:

```text
--local-json -
--server 1
--duration 10
--concurrent 3
--no-icmp
--telemetry-level disabled
--json
--ipv4 | --ipv6
```

Do not pass `--share`. Validate the pinned URL scheme and expected hostname
before execution. Parse exactly one final JSON result and preserve server/client
metadata, ping, jitter, upload/download rates, byte counts, duration,
concurrency, and helper version.

The maintained helper exposes a machine-readable live-rate stream through
`--progress-json`. Interactive output may display those observed rates but must
not fabricate samples or percentages.

### 3.1 NJU Campus

Pinned endpoints:

```text
IPv4  http://speed.nju.edu.cn
IPv6  http://speed6.nju.edu.cn
```

The service describes the path to NJU's campus-internal measurement servers.
`campus` defaults to IPv4. `campus --ipv6` runs IPv6 only, with no IPv4
fallback.

### 3.2 NJU Edge

Published endpoints:

```text
IPv4  http://test.nju.edu.cn
IPv6  http://test6.nju.edu.cn
```

The service represents the path to NJU's public internet-facing edge, but its
measurement backend is protected by an Anubis browser challenge. Standard
LibreSpeed CLI receives a redirect and returns no measurement. SoundProbe displays
NJU Edge as `terminal unsupported`; `edge` and `--targets nju-edge` fail before
starting a helper. Do not automate or bypass the browser challenge. Enable this
target only after NJU publishes a terminal-compatible endpoint or explicit
integration contract.

### 3.3 Domestic stations

Pinned IPv4 endpoints:

```text
CERNET  http://speedtest.sec.edu.cn
QLU     https://speed.qlu.edu.cn
Tongji  https://dev.tongji.edu.cn/speedtest
```

These are optional independent targets. One station failure must not prevent
later selected stations from running. The default `domestic` command runs
CERNET, QLU, then Tongji sequentially.

## 4. M-Lab behavior

Run pinned `ndt7-client` v0.10.1 with JSON events, TLS verification, client name
`soundprobe`, both download and upload, and a 55-second whole-test timeout. Use
M-Lab Locate rather than pinning a server.

Consume `starting`, `connected`, `measurement`, `error`, and `complete` events.
Measurement events may drive transient download/upload rates in the interactive
view. Persist only the durable final summary, selected server, client identity,
bytes, duration, and final rates.

M-Lab is a peer target, not a fallback or reference score. Its single-stream
NDT7 result is not directly equivalent to three-stream LibreSpeed results.

## 5. Planning and ordering

All selected targets run sequentially so they never compete for bandwidth. The
ordered plan is visible before and during execution.

### 5.1 Interactive selector

Bare `soundprobe` in an interactive terminal performs bounded, lightweight
reachability probes and opens a Bubble Tea inline selector. Probes may check DNS,
connection establishment, TLS, and a small backend response; they must not run a
bandwidth test.

Controls:

```text
↑/↓ or j/k   move
Space        toggle station
4            IPv4
6            IPv6
d            dual stack
a            restore recommendation
Enter        execute
q / Esc      cancel
```

The selector shows station description, family support, reachability, and probe
latency. IPv4-only stations are disabled in IPv6 mode.

Recommendation rules:

1. Select M-Lab.
2. Select NJU Campus when at least one requested family is reachable.
3. If Campus is not reachable, recommend M-Lab alone.
4. Display NJU Edge as disabled with its browser-verification explanation.
5. Domestic stations are available but not preselected.

Recommendations set defaults only. They do not authorize silent fallback during
measurement.

### 5.2 Noninteractive plans

JSON, redirected, and explicitly scripted commands never open the selector.
They resolve a deterministic target list from command defaults and flags:

```text
soundprobe run --targets LIST --family ipv4|ipv6|dual
soundprobe domestic --targets LIST --family ipv4|dual
soundprobe campus [--ipv4|--ipv6]
soundprobe edge [--ipv4|--ipv6]
soundprobe mlab
```

`--targets` accepts comma-separated station IDs. Invalid station IDs and
unsupported family/station combinations fail before any provider is contacted.

## 6. Result semantics

For each requested target:

- success stores measured values;
- attempted provider failure stores zero or partial measured values and a stable
  failure object;
- cancellation stores null speeds and a cancelled failure;
- targets not started after cancellation are marked skipped with null speeds.

Run status:

- all requested measurements successful: `success`;
- at least one success and at least one failure: `partial`;
- no successful measurement: `failed`;
- operator cancellation: `cancelled`.

Stable exit codes:

```text
0    all requested measurements succeeded
1    invalid configuration, missing helper, consent, or internal failure
2    failed target or partial result
130  cancelled by the operator
```

## 7. Terminal interface

Use Bubble Tea v2 in inline mode, never alternate-screen mode. The selector must
clear before measurement progress begins. During execution redraw one fixed
block at no more than four frames per second.

Every target receives the same four-row panel:

1. explicit station/family label and phase;
2. animated activity bar;
3. download/upload rates;
4. selected server or bounded failure detail.

Targets have independent waiting, active, complete, failed, cancelled, and
skipped states. M-Lab may add transient live rates; LibreSpeed targets do not.
At completion restore the cursor and replace the block with one durable summary
table using human-readable target labels.

JSON output emits exactly one document. Redirected plain output emits only one
final summary and provider failures. Neither mode may contain ANSI sequences or
raw provider events.

## 8. Commands

```text
soundprobe
soundprobe run [--targets LIST] [--family ipv4|ipv6|dual] [--label TEXT] [--note TEXT] [--no-save]
soundprobe campus [--ipv4|--ipv6] [--label TEXT] [--note TEXT] [--no-save]
soundprobe edge [--ipv4|--ipv6]  # reports terminal unsupported
soundprobe domestic [--targets LIST] [--family ipv4|dual] [--label TEXT] [--note TEXT] [--no-save]
soundprobe mlab [--label TEXT] [--note TEXT] [--no-save]
soundprobe stations [--json]
soundprobe history [--limit N]
soundprobe last [--json]
soundprobe show RUN_ID [--json]
soundprobe export --format jsonl|csv --output PATH
soundprobe consent status|accept|revoke
soundprobe doctor [--json]
soundprobe version
```

## 9. Consent and privacy

Before a selected plan containing M-Lab starts, explain that M-Lab collects the
ISP-provided public IP address and measurement results and publishes/retains
experiment data indefinitely. Require exact interactive acceptance and store the
policy version and timestamp locally.

A plan without M-Lab never requires M-Lab consent. Noninteractive execution
without current consent fails before contacting M-Lab. LibreSpeed telemetry and
sharing are always disabled.

SoundProbe has no own analytics, remote result service, geolocation enrichment, or
ASN lookup.

## 10. Storage and export

Store summaries under:

```text
~/Library/Application Support/soundprobe/history/v1/<run-id>.json
```

Directories are `0700`; files are `0600`. Use same-directory temporary files,
fsync, and atomic rename. Never prune history automatically.

Schema version remains 1. Existing 0.1 history without a `targets` field must
remain readable. New summaries include the ordered target IDs.

JSONL export writes one complete summary per line. CSV export writes one row per
measurement, repeating run metadata. This normalized form preserves arbitrary
multi-station and dual-stack plans.

## 11. Helper discovery and packaging

Resolve helpers in this order:

1. installed `../libexec/soundprobe` relative to the executable;
2. repository-local `.tools/bin`;
3. documented developer PATH fallback.

Verify exact helper versions and include them in results. Production must not
silently select an arbitrary newer PATH helper when a pinned libexec helper
exists.

Homebrew Formula tests are offline and may invoke only version, diagnostics, and
read-only history commands. They must not probe real stations or run bandwidth
measurements.

## 12. Verification gates

Automated tests use mock helpers and local HTTP fixtures. They cover:

- explicit station/family expansion and ordering;
- selector recommendation, family switching, toggling, cancellation, and clear;
- NJU Campus identity, Edge unsupported handling, and no fallback;
- domestic station identity and telemetry-disabled helper arguments;
- M-Lab live event parsing and independent failure;
- multi-target success, partial, failure, cancellation, and skipped results;
- schema-v1 legacy history compatibility;
- normalized CSV and JSONL export;
- inline terminal rendering, cursor restoration, and no ANSI in redirected/JSON
  output;
- atomic storage and `0700`/`0600` modes;
- deterministic release artifacts and Homebrew template rendering.

Routine CI must never run a real bandwidth measurement or station probe.
Operator acceptance validates real NJU Campus, domestic stations, M-Lab
continuation, Edge unsupported reporting, selector interaction, Homebrew
installation, and upgrade on a supported macOS host.
