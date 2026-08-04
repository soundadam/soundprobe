# NJUProbe

> Measure each network path for what it is. Do not hide different targets,
> address families, or failure modes behind one synthetic speed score.

## Design principles

NJUProbe separates **measurement intent** from the concrete server used to
perform the test:

```text
measurement purpose
        │
        ▼
ordered target plan
        │
        ├── NJU Campus · IPv4
        ├── NJU Campus · IPv6
        ├── M-Lab · automatic node
        └── Domestic station · IPv4
        │
        ▼
sequential measurements
        │
        ▼
one durable, machine-readable summary
```

The product follows several rules:

- **Purpose before server.** People select a measurement purpose; NJUProbe
  maps it to a pinned, reviewed target definition.
- **Explicit identity.** Station and address family remain visible in target
  IDs such as `nju-campus-ipv6` and `tongji-ipv4`.
- **Sequential execution.** Targets never compete with each other for the same
  bandwidth during one run.
- **Independent outcomes.** A failed target does not erase a successful one or
  prevent the next selected target from running.
- **Honest progress.** The UI shows an indeterminate activity bar when a helper
  cannot report real progress; it does not fabricate percentages.
- **No silent fallback.** IPv6 never becomes IPv4, one station never impersonates
  another, and browser protection is never bypassed.
- **Human and machine interfaces share one model.** The interactive selector
  and `--targets`/`--family` batch flags produce the same ordered plan.

## Terminal preview

The bare command first probes station availability and opens a multi-select
plan editor:

```text
$ njuprobe
NJUProbe 0.2.0 · select measurement targets
Address family  ipv4   [4] IPv4  [6] IPv6  [d] dual

› [ ] NJU Campus   NJU campus-internal path
      ipv4 unreachable 6ms
  [-] NJU Edge     public path to NJU's internet-facing edge
      terminal unsupported · browser verification blocks the official backend
  [x] M-Lab        general Internet NDT7 measurement
      automatic node
  [ ] CERNET       CERNET public LibreSpeed station
      ipv4 reachable 30ms
  [ ] QLU          Qilu University of Technology LibreSpeed station
      ipv4 reachable 55ms
  [ ] Tongji       Tongji University LibreSpeed station
      ipv4 reachable 8ms

↑/↓ move   Space toggle   a recommended   Enter start   q cancel
```

Selected targets then receive equal, fixed-height progress panels. The example
below shows one failed Campus target while M-Lab continues independently:

```text
NJUProbe 0.2.0
Network   utun6 · tunnel
Order     NJU Campus · IPv4 → M-Lab · sequential

NJU Campus · IPv4    × failed · 00:00
          Activity  [────────────────────────]
          Rate      ↓ 0.00 Mbps · ↑ 0.00 Mbps
          Detail    error: server did not produce a measurement

M-Lab                ◐ uploading · 00:21
          Activity  [░░░░░░░░░░█████░░░░░░░░]
          Rate      ↓ 33.67 Mbps · ↑ 4.75 Mbps
          Detail    server ndt-mlab3-hkg03.mlab-oti.measurement-lab.org

Elapsed   00:21
Ctrl-C    cancel
```

When the run ends, the dynamic block is replaced by one durable summary:

```text
NJUProbe 0.2.0 · partial · 29.7s
Run 7e98fa57-f4c8-4883-959f-4d5c5e58916d
Network utun6 · tunnel
TARGET             METHOD                   DOWNLOAD    UPLOAD     SERVER       STATUS
NJU Campus · IPv4  librespeed-three-stream  0.00 Mbps   0.00 Mbps  —            failed
M-Lab              ndt7-single-stream       33.67 Mbps  4.75 Mbps  ndt-mlab3…   success
NJU Campus · IPv4 error [connect/server_unreachable]: server did not produce a measurement
```

NJUProbe is a macOS-first, terminal-first network measurement tool. It measures
explicit, independent targets sequentially so they do not compete for bandwidth:

- **NJU Campus** — the path to NJU's campus-internal LibreSpeed service;
- **NJU Edge** — shown as a distinct public-path purpose, but currently disabled in terminal mode because the official backend requires browser verification;
- **M-Lab** — a general Internet NDT7 measurement with automatic server selection;
- **Domestic stations** — pinned CERNET, QLU, and Tongji LibreSpeed services.

These targets answer different questions. NJUProbe never substitutes one for
another, silently changes address family, or collapses their results into a
synthetic score.

## Install

```sh
brew tap soundadam/tap
brew install njuprobe
njuprobe doctor
```

M-Lab publishes measurement data, including the ISP-provided public IP address.
Accept its policy once before selecting M-Lab:

```sh
njuprobe consent accept
```

## Interactive use

Run the bare command in a terminal:

```sh
njuprobe
```

NJUProbe performs short reachability probes and opens a Bubble Tea selector:

```text
↑/↓ or j/k   move
Space        select or deselect a station
4            IPv4
6            IPv6
d            dual stack
a            restore the recommended plan
Enter        start sequential measurements
q / Esc      cancel
```

The recommendation is deliberately conservative:

- select NJU Campus when the requested family is reachable;
- always select M-Lab when consent is available;
- when Campus is unreachable, recommend M-Lab alone;
- show NJU Edge as disabled until NJU provides an official terminal-compatible backend.

The selector displays station, address-family support, reachability, and probe
latency. IPv4-only stations are disabled in IPv6 mode. Browser-protected
stations are displayed but disabled with an explanation. Selecting `dual`
expands a supported dual-stack station into two independent measurements.

During execution every target has the same four-row panel: status, animated
activity, download/upload rates, and server or failure detail. The maintained
LibreSpeed helper and M-Lab NDT7 events both provide transient live rates. The
renderer stays inline rather than using the alternate screen, then leaves one
durable summary.

## Script and batch use

Noninteractive and JSON commands never open the selector:

```sh
njuprobe run --targets nju-campus,mlab --family ipv4 --no-save --json
njuprobe domestic --targets cernet,qlu,tongji --family ipv4 --no-save --json
```

Supported station IDs:

```text
nju-campus   NJU campus-internal service, IPv4 and IPv6
nju-edge     NJU public edge purpose; terminal measurement currently unavailable
mlab         M-Lab NDT7, automatic address family and server
cernet       CERNET LibreSpeed, IPv4
qlu          Qilu University of Technology LibreSpeed, IPv4
tongji       Tongji University LibreSpeed, IPv4
```

Compatibility commands remain available:

```text
njuprobe campus                  NJU Campus IPv4
njuprobe campus --ipv6           NJU Campus IPv6 only
njuprobe edge                    Explain that NJU Edge is browser-protected and unavailable in terminal mode
njuprobe mlab                    M-Lab only
njuprobe domestic                CERNET, QLU, then Tongji over IPv4
njuprobe stations                Probe and list station availability
```

Global `--json` emits exactly one JSON document. Redirected plain output never
contains ANSI progress sequences.

## Results and history

Each summary records the ordered `targets` list and one measurement object per
target. Provider IDs include station and family, such as
`nju-campus-ipv6` or `qlu-ipv4`. This makes IPv4 and IPv6 visible rather than
hiding them behind a generic Campus label.

```sh
njuprobe last
njuprobe history --limit 10
njuprobe show RUN_ID --json
njuprobe export --format jsonl --output /tmp/njuprobe.jsonl
njuprobe export --format csv --output /tmp/njuprobe.csv
```

JSONL preserves one complete summary per line. CSV is normalized to one row per
measurement so multi-station and dual-stack runs do not lose data.

Local files are written atomically with directory mode `0700` and file mode
`0600` under:

```text
~/Library/Application Support/njuprobe/history/v1/
```

Existing schema-v1 history from NJUProbe 0.1 remains readable.

## Development

NJUProbe and the pinned helpers require Go 1.25.8 or newer.

```sh
make test-offline
make tools
make build
./bin/njuprobe doctor --json
```

Routine tests use mock helpers and sanitized fixtures; they do not run real
bandwidth measurements or probe real stations. Real operator acceptance is
listed in [TESTING.md](TESTING.md). Release and Homebrew gates are documented in
[RELEASE.md](RELEASE.md).

## Privacy and independence

NJUProbe has no analytics, cloud synchronization, daemon, scheduler, ASN
enrichment, or geolocation lookup. LibreSpeed telemetry and sharing are disabled.
M-Lab remains governed by its own privacy policy and requires explicit consent.

NJUProbe is independent of soundVPN, SFM, and NJUConnect. It observes the route
selected by macOS but does not inspect or modify those products' private
configuration.

## License

NJUProbe's source is MIT licensed. LibreSpeed CLI and ndt7-client remain separate
helper executables under their upstream licenses. See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
