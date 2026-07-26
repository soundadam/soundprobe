# NJUProbe

NJUProbe is a macOS-first, terminal-first network measurement tool. It measures
explicit, independent targets sequentially so they do not compete for bandwidth:

- **NJU Campus** — the path to NJU's campus-internal LibreSpeed service;
- **NJU Edge** — the public path to NJU's internet-facing LibreSpeed service;
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
- otherwise select NJU Edge when it is reachable;
- always select M-Lab when consent is available;
- when neither NJU path is reachable, recommend M-Lab alone.

The selector displays station, address-family support, reachability, and probe
latency. IPv4-only stations are disabled in IPv6 mode. Selecting `dual` expands
a dual-stack station into two independent measurements, one for each family.

During execution every target has the same four-row panel: status, animated
activity, download/upload rates, and server or failure detail. LibreSpeed targets
use an honest indeterminate activity bar because their helper has no stable live
rate stream. M-Lab adds transient live rates from NDT7 events. The renderer stays
inline rather than using the alternate screen, then leaves one durable summary.

## Script and batch use

Noninteractive and JSON commands never open the selector:

```sh
njuprobe run --targets nju-campus,mlab --family ipv4 --no-save --json
njuprobe run --targets nju-edge --family dual --no-save --json
njuprobe domestic --targets cernet,qlu,tongji --family ipv4 --no-save --json
```

Supported station IDs:

```text
nju-campus   NJU campus-internal service, IPv4 and IPv6
nju-edge     NJU public edge service, IPv4 and IPv6
mlab         M-Lab NDT7, automatic address family and server
cernet       CERNET LibreSpeed, IPv4
qlu          Qilu University of Technology LibreSpeed, IPv4
tongji       Tongji University LibreSpeed, IPv4
```

Compatibility commands remain available:

```text
njuprobe campus                  NJU Campus IPv4
njuprobe campus --ipv6           NJU Campus IPv6 only
njuprobe edge                    NJU Edge IPv4
njuprobe edge --ipv6             NJU Edge IPv6 only
njuprobe mlab                    M-Lab only
njuprobe domestic                CERNET, QLU, then Tongji over IPv4
njuprobe stations                Probe and list station availability
```

Global `--json` emits exactly one JSON document. Redirected plain output never
contains ANSI progress sequences.

## Results and history

Each summary records the ordered `targets` list and one measurement object per
target. Provider IDs include station and family, such as
`nju-edge-ipv6` or `qlu-ipv4`. This makes IPv4 and IPv6 visible rather than
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
