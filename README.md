# NJUProbe

NJUProbe is a macOS-first, terminal-first network measurement tool that compares
two deliberately different paths:

- the current path to Nanjing University's LibreSpeed service;
- the current public Internet path measured with M-Lab NDT7.

The providers run sequentially and remain visibly distinct: NJU uses a
three-stream LibreSpeed measurement, while M-Lab NDT7 is a single-stream bulk
transport measurement. NJUProbe does not combine them into a synthetic score.

## Current status

The v0.1 implementation includes:

- NJU IPv4 and explicit IPv6 measurements;
- M-Lab NDT7 download and upload measurements;
- fixed-block Bubble Tea progress for interactive terminals;
- clean plain and JSON output when redirected;
- atomic local history, `last`, `show`, and export commands;
- explicit M-Lab privacy consent;
- pinned, separately installed helper executables;
- offline fixtures for all routine tests;
- source-release and Homebrew Formula scaffolding.

No public release or Homebrew tap has been published yet. See
[RELEASE.md](RELEASE.md) for the release gates.

## Daily use

Install the development helpers and build the CLI:

```sh
make tools
make build
./bin/njuprobe doctor
```

Accept the M-Lab policy once before the first combined or M-Lab-only run:

```sh
./bin/njuprobe consent accept
```

Common commands:

```text
njuprobe                         Run NJU, then M-Lab, and save the result
njuprobe campus                  Run only NJU IPv4
njuprobe campus --ipv6           Run only NJU IPv6, without IPv4 fallback
njuprobe mlab                    Run only M-Lab NDT7
njuprobe last                    Show the newest saved result
njuprobe history --limit 10      List recent runs
njuprobe show RUN_ID --json      Read one complete stored result
njuprobe doctor --json           Check helpers, consent, and storage
```

Use `--no-save` for disposable checks and global `--json` for scripts:

```sh
njuprobe run --no-save --json
njuprobe campus --json
```

Interactive output redraws one fixed block at no more than four frames per
second. Redirected and JSON output never contains ANSI progress sequences.

## Development

Building NJUProbe and the pinned ndt7-client v0.10.1 helper requires Go 1.25.8 or newer. Go's automatic
toolchain selection is supported.

```sh
make test-offline
make tools
make build
./bin/njuprobe doctor --json
```

`make test-offline` uses mock helpers and sanitized fixtures. It never performs
a real bandwidth test. Real operator acceptance is explicit and documented in
[TESTING.md](TESTING.md).

## Privacy and storage

Local summaries are retained indefinitely and may contain the client's public
IP address. Files are written atomically with directory mode `0700` and file
mode `0600` under:

```text
~/Library/Application Support/njuprobe/history/v1/
```

M-Lab separately collects and publicly publishes measurement data, including
the ISP-provided IP address, under its [current privacy policy](https://www.measurementlab.net/privacy/). NJUProbe therefore fails
closed until the user explicitly accepts that policy. NJUProbe has no analytics,
cloud synchronization, daemon, scheduler, ASN enrichment, or geolocation lookup.

## Independence

NJUProbe is independent of soundVPN, SFM, and NJUConnect. It observes the path
selected by the operating system but does not inspect or modify those products'
private configuration.

## License

NJUProbe's source is MIT licensed. LibreSpeed CLI and ndt7-client remain
separate helper executables under their upstream licenses. See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
