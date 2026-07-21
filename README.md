# NJUProbe

NJUProbe is a private, terminal-first network measurement product for comparing
two deliberately different paths:

- the current connection to Nanjing University's campus speed-test service;
- the current public Internet connection measured with M-Lab NDT7.

It is independent of soundVPN, SFM, and NJUConnect. It observes the network path
selected by the operating system but does not read, modify, or claim knowledge
of those products' routing configuration.

## Repository status

The v0.1 implementation is in progress. The repository contains the Go CLI,
stable result schema, consent and history persistence, JSON/plain output paths,
export support, deterministic helper discovery, and the NJU LibreSpeed campus
provider. The M-Lab NDT7 provider and interactive renderer are not implemented
yet.

Routine development must follow [SPEC.md](SPEC.md) and [AGENTS.md](AGENTS.md).
No executable, helper binary, release artifact, or Homebrew Formula has been
published.

## Development

```text
make test-offline
make tools
make build
./bin/njuprobe version
```

`make test-offline` uses mock helpers and committed sanitized fixtures; it never
runs a real bandwidth test. `make tools` builds the pinned LibreSpeed helper as
a separate ignored executable under `.tools/bin`. Real campus acceptance is an
explicit operator action. See [TESTING.md](TESTING.md) for exact commands and
expected exit codes.

## Intended command surface

```text
njuprobe
njuprobe run [--label TEXT] [--note TEXT] [--no-save]
njuprobe campus [--ipv4|--ipv6] [--label TEXT] [--note TEXT] [--no-save]
njuprobe mlab [--label TEXT] [--note TEXT] [--no-save]

njuprobe history [--limit N]
njuprobe show RUN_ID [--json]
njuprobe export --format jsonl|csv --output PATH

njuprobe consent status
njuprobe consent accept
njuprobe consent revoke

njuprobe version
```

The `campus` command currently executes the NJU LibreSpeed measurement. Running
bare `njuprobe`, `run`, or `mlab` remains unavailable until the NDT7 provider is
implemented. The final interactive implementation will redraw a fixed inline
block instead of printing an expanding stream of status lines.

## Privacy

Local summaries are intentionally retained indefinitely and may contain the
client's public IP address. M-Lab separately collects and publicly publishes
the client's ISP-provided IP address and measurement results. The first M-Lab
run must therefore require explicit consent and record the accepted policy
version locally.

The source repository is private. A normal public Homebrew Formula cannot use
this private repository as an anonymously downloadable source. Local/private
installation remains allowed; any public distribution is a separate owner
decision.

## License

NJUProbe's own source is licensed under the MIT License. Planned helper programs
retain their upstream licenses and remain separate executables; see
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

