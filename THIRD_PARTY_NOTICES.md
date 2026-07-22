# Third-party components

No helper source or binary is committed to this repository. Helper sources are
pinned by immutable version and SHA-256, built separately, and invoked through
process interfaces. Go library dependencies are locked by `go.mod` and `go.sum`.

## LibreSpeed CLI

- Project: <https://github.com/librespeed/speedtest-cli>
- Version: `v1.0.13`
- Commit: `2f2408764d88e9601aa64a03b340f8e3151003e4`
- Source archive SHA-256:
  `5ad938b61e3edc0ca95e2ccff0c06e97a69383f3cbb0243bd47b21b9865f9f55`
- License: GNU Lesser General Public License v3.0
- Installed executable: `libexec/njuprobe/librespeed-cli`

NJUProbe communicates with LibreSpeed through its process and final JSON
interface. The LGPL helper is not statically linked into the MIT executable.
The Homebrew Formula installs the upstream license alongside the package.

## M-Lab ndt7-client-go

- Project: <https://github.com/m-lab/ndt7-client-go>
- Version: `v0.10.1`
- Commit: `4a5f6325d1d586ab38afb84566a5781b5d6c3d9a`
- Source archive SHA-256:
  `31b40268bd7a9d31bdb5507b7ade2fad2efb8abb9e7339d2f59e9cdee5340bef`
- License: Apache License 2.0
- Installed executable: `libexec/njuprobe/ndt7-client`

NJUProbe consumes the helper's JSON event stream and identifies the client as
`njuprobe`. Because the upstream CLI has no version-reporting command, packaging
installs `ndt7-client.version` containing `v0.10.1`; runtime preflight verifies
that sidecar before a test. The Homebrew Formula installs the upstream license.

## Bubble Tea

- Project: <https://github.com/charmbracelet/bubbletea>
- Module: `charm.land/bubbletea/v2`
- Version: `v2.0.6`
- License: MIT
- Integration: linked Go library for fixed-block inline terminal rendering

Bubble Tea and its transitive Go dependencies are recorded in `go.mod` and
`go.sum`. The renderer is disabled for redirected output and global JSON mode.

## Release verification

Before each release:

1. run `go mod verify`;
2. rebuild both helper archives from their recorded SHA-256 sources;
3. run the complete offline fixture and race gates;
4. verify the Formula installs the upstream helper license files;
5. review dependency changes in `go.mod`, `go.sum`, and this notice.
