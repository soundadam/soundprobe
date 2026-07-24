#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
BINARY="$ROOT/bin/njuprobe"
CAMPUS_FIXTURE="$ROOT/internal/provider/campus/testdata/librespeed-success-ipv4.json"
MLAB_SUCCESS_FIXTURE="$ROOT/internal/provider/mlab/testdata/ndt7-success.jsonl"
MLAB_FAILURE_FIXTURE="$ROOT/internal/provider/mlab/testdata/ndt7-upload-failure.jsonl"

mkdir -p "$ROOT/bin"
(
  cd "$ROOT"
  GOTOOLCHAIN=auto go build -o "$BINARY" ./cmd/njuprobe
)

workdir=$(mktemp -d "${TMPDIR:-/tmp}/njuprobe-run-fixture.XXXXXX")
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT HUP INT TERM

helper_dir="$workdir/app/libexec/njuprobe"
mkdir -p "$workdir/app/bin" "$helper_dir" "$workdir/home" "$workdir/work"
cp "$BINARY" "$workdir/app/bin/njuprobe"

cat > "$helper_dir/librespeed-cli" <<EOF
#!/bin/sh
if [ "\${1:-}" = "--version" ]; then
  printf 'librespeed-cli v1.0.13 (built on fixture)\n'
  exit 0
fi
if [ -n "\${NJUPROBE_FIXTURE_DELAY:-}" ]; then
  sleep "\$NJUPROBE_FIXTURE_DELAY"
fi
cat "$CAMPUS_FIXTURE"
EOF
chmod 0755 "$helper_dir/librespeed-cli"

cat > "$helper_dir/ndt7-client" <<EOF
#!/bin/sh
if [ -n "\${NJUPROBE_FIXTURE_DELAY:-}" ]; then
  sleep "\$NJUPROBE_FIXTURE_DELAY"
fi
if [ "\${NJUPROBE_NDT7_FIXTURE:-success}" = "failure" ]; then
  cat "$MLAB_FAILURE_FIXTURE"
  exit 1
fi
cat "$MLAB_SUCCESS_FIXTURE"
EOF
chmod 0755 "$helper_dir/ndt7-client"
printf '%s\n' 'v0.10.1' > "$helper_dir/ndt7-client.version"
chmod 0600 "$helper_dir/ndt7-client.version"

consent_dir="$workdir/home/Library/Application Support/njuprobe"
mkdir -p "$consent_dir"
cat > "$consent_dir/consent.json" <<'EOF'
{
  "schemaVersion": 1,
  "provider": "mlab",
  "policyVersion": "v5-2026-05-03",
  "acceptedAt": "2026-07-22T00:00:00Z",
  "toolVersion": "fixture"
}
EOF
chmod 0600 "$consent_dir/consent.json"

run_command() {
  output=$1
  shift
  (
    cd "$workdir/work"
    HOME="$workdir/home" PATH="$PATH" \
      "$workdir/app/bin/njuprobe" "$@"
  ) > "$output"
}

run_command "$workdir/doctor.json" doctor --json
run_command "$workdir/success.json" run --no-save --json

set +e
(
  cd "$workdir/work"
  HOME="$workdir/home" PATH="$PATH" NJUPROBE_NDT7_FIXTURE=failure \
    "$workdir/app/bin/njuprobe" run --no-save --json
) > "$workdir/partial.json"
partial_exit=$?
set -e
if [ "$partial_exit" -ne 2 ]; then
  echo "run fixture: partial exit code was $partial_exit, expected 2" >&2
  exit 1
fi

run_command "$workdir/saved.json" run --json
run_command "$workdir/last.json" last --json
run_command "$workdir/history.json" history --limit 1 --json

python3 - "$workdir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for name in ("doctor", "success", "partial", "saved", "last", "history"):
    raw = (root / f"{name}.json").read_bytes()
    if b"\x1b[" in raw:
        raise SystemExit(f"run fixture: ANSI escape found in {name}.json")

doctor = json.loads((root / "doctor.json").read_text())
assert doctor["ready"] is True
assert doctor["combinedReady"] is True
assert doctor["providers"] == {"campus": "ready", "mlab": "ready"}
assert doctor["consentAccepted"] is True

success = json.loads((root / "success.json").read_text())
assert success["status"] == "success"
assert [m["provider"] for m in success["measurements"]] == ["campus", "mlab"]
assert success["measurements"][0]["method"] == "librespeed-three-stream"
assert success["measurements"][1]["method"] == "ndt7-single-stream"
assert success["measurements"][1]["downloadMbps"] == 80
assert success["measurements"][1]["uploadMbps"] == 40

partial = json.loads((root / "partial.json").read_text())
assert partial["status"] == "partial"
assert partial["measurements"][0]["status"] == "success"
assert partial["measurements"][1]["status"] == "failed"
assert partial["measurements"][1]["failure"]["stage"] == "upload"

saved = json.loads((root / "saved.json").read_text())
last = json.loads((root / "last.json").read_text())
history = json.loads((root / "history.json").read_text())
assert last["runId"] == saved["runId"]
assert history[0]["runId"] == saved["runId"]
PY

python3 - "$workdir" <<'PY'
import fcntl
import os
import pathlib
import pty
import select
import struct
import subprocess
import sys
import termios
import time

root = pathlib.Path(sys.argv[1])
env = os.environ.copy()
env.update({
    "HOME": str(root / "home"),
    "PATH": env["PATH"],
    "NJUPROBE_FIXTURE_DELAY": "0.15",
    "TERM": "xterm-256color",
})
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 120, 0, 0))
process = subprocess.Popen(
    [str(root / "app/bin/njuprobe"), "run", "--no-save"],
    stdin=slave,
    stdout=slave,
    stderr=slave,
    cwd=root / "work",
    env=env,
    close_fds=True,
)
os.close(slave)
output = bytearray()
deadline = time.monotonic() + 10
while time.monotonic() < deadline:
    ready, _, _ = select.select([master], [], [], 0.1)
    if ready:
        try:
            chunk = os.read(master, 65536)
        except OSError:
            chunk = b""
        if chunk:
            output.extend(chunk)
    if process.poll() is not None:
        while True:
            ready, _, _ = select.select([master], [], [], 0)
            if not ready:
                break
            try:
                chunk = os.read(master, 65536)
            except OSError:
                break
            if not chunk:
                break
            output.extend(chunk)
        break
else:
    process.kill()
    raise SystemExit("interactive fixture timed out")
os.close(master)
if process.wait() != 0:
    raise SystemExit(f"interactive fixture exit code {process.returncode}")
raw = bytes(output)
assert b"\x1b[?1049h" not in raw, "alternate screen was enabled"
assert raw.count(b"Run ") == 1, "final summary was not durable and unique"
assert b"PROVIDER" in raw and b"campus" in raw and b"mlab" in raw
assert b'"Key":"measurement"' not in raw, "provider events leaked"
assert b"Ctrl-C" in raw, "interactive fixed block was not rendered"
PY

printf '%s\n' 'Offline combined run fixture test passed.'
printf '%s\n' 'Success result:'
cat "$workdir/success.json"
printf '%s\n' 'Partial result:'
cat "$workdir/partial.json"
