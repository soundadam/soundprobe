# Third-party component plan

The planning-only initial commit contains no third-party source or binaries.
The implementation is expected to use the following pinned components as
separate executables or libraries while preserving their notices and licenses.

## LibreSpeed CLI

- Project: <https://github.com/librespeed/speedtest-cli>
- Planned version: `v1.0.13`
- License: GNU Lesser General Public License v3.0
- Integration: separately built helper executable under `libexec/njuprobe`

NJUProbe must communicate with this helper through its process and JSON
interface. The helper must not be statically linked into the MIT executable.

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

