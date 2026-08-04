# Maintained LibreSpeed CLI component

This directory is a source-level derivative of LibreSpeed CLI v1.0.13 and is
licensed under the adjacent GNU LGPL v3 license.

## Provenance

- Upstream: <https://github.com/librespeed/speedtest-cli>
- Upstream tag: `v1.0.13`
- Upstream commit: `2f2408764d88e9601aa64a03b340f8e3151003e4`
- Upstream source archive SHA-256:
  `5ad938b61e3edc0ca95e2ccff0c06e97a69383f3cbb0243bd47b21b9865f9f55`
- Maintained component version: `v1.0.13-campus.1`

## Local changes

- `--progress-json` emits structured progress events on stderr while preserving
  the final JSON document on stdout.
- `--proxy socks5h://LOOPBACK:PORT` uses an explicit loopback SOCKS5 proxy,
  resolves the measurement host through that proxy, and supports cancellation.
- Ambient HTTP proxy variables are ignored for measurement traffic so reported
  route labels remain accurate.

Keep modifications in this directory self-contained and covered by LGPL. A
consumer may choose only the capabilities it needs; the helper binary and source
remain independently buildable from the proprietary soundconnect application.
