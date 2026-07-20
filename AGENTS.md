# Repository guidance

- Keep the canonical repository, Formula token, and executable name
  `njuprobe`; use `NJUProbe` only as the human-facing product name.
- The v0.1 product measures the NJU LibreSpeed service and M-Lab NDT7. Do not
  add Google Fiber, Google CDN, USTC, continuous monitoring, background jobs,
  or scheduled tests without a new owner decision.
- Keep NJUProbe independent of soundVPN, SFM, and NJUConnect. It may report the
  active interface and observed endpoint addresses, but must not inspect those
  products' private configuration or claim which selector handled traffic.
- Run the NJU and M-Lab tests sequentially. NJU is a three-stream LibreSpeed
  measurement; NDT7 is a single-stream bulk-transport measurement. Preserve
  this distinction in the UI, stored schema, and documentation.
- An attempted but unreachable measurement records zero speed plus a stable
  failure stage. A measurement intentionally skipped records `null`, never
  zero. Preserve partial and cancelled outcomes.
- Interactive output must redraw one fixed inline block and leave one final
  summary. Never print provider measurement events as an expanding log.
  Redirected output and `--json` must contain no ANSI escape sequences.
- Persist summary files indefinitely under the platform application-support
  directory. Use an atomic temporary-file rename, directory mode `0700`, and
  file mode `0600`. Do not persist per-second samples.
- Public IP addresses are not redacted from local summaries. Do not add an ASN,
  geolocation, analytics, or unrelated public-IP lookup service in v0.1.
- Before an M-Lab test, require explicit acceptance of the current M-Lab
  privacy policy. M-Lab test data and the ISP-provided IP address are public and
  retained indefinitely. Noninteractive execution without prior consent must
  fail closed.
- Keep LibreSpeed CLI and ndt7-client as pinned, separately installed helper
  executables. Do not copy their source into this repository or statically link
  LGPL LibreSpeed code into the MIT executable.
- CI uses fixtures and mock helpers only. A real bandwidth test is an explicit
  operator acceptance step and must never run in routine tests or Formula tests.
- This source repository is private. Do not add a public tap Formula that points
  at private source or claim public installability. Publication requires a
  separate owner decision and a reviewed distribution design.

