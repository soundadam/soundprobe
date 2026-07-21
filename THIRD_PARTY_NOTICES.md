# Third-party components

No third-party source or binaries are committed to this repository. External
helpers are pinned, built separately, and invoked through process interfaces
while preserving their upstream notices and licenses.

## LibreSpeed CLI

- Project: <https://github.com/librespeed/speedtest-cli>
- Pinned version: `v1.0.13`
- Pinned commit: `2f2408764d88e9601aa64a03b340f8e3151003e4`
- License: GNU Lesser General Public License v3.0
- Integration: separately built helper executable under `libexec/njuprobe`

NJUProbe communicates with this helper through its process and JSON interface.
The helper is not statically linked into the MIT executable. Development builds
are installed under `.tools/bin` by `scripts/install-dev-tools.sh`; release
packaging will place the separate executable under `libexec/njuprobe`.

## M-Lab ndt7-client-go

- Project: <https://github.com/m-lab/ndt7-client-go>
- Planned version: `v0.10.1`
- License: Apache License 2.0
- Integration: separately built `ndt7-client` helper under `libexec/njuprobe`

NJUProbe will consume the helper's JSON event stream and identify the client as
`njuprobe`.

## Bubble Tea

- Project: <https://github.com/charmbracelet/bubbletea>
- Planned major version: `v2`
- License: MIT
- Integration: terminal UI library for fixed-block inline rendering

Before the first release, replace this planning notice with the exact resolved
versions, copyright notices, license texts required by each dependency, and a
reproducible dependency manifest.

