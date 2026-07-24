#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
workdir=$(mktemp -d "${TMPDIR:-/tmp}/njuprobe-release.XXXXXX")
repo="$workdir/repo"
cleanup() {
  rm -rf "$repo/dist" 2>/dev/null || true
  if [ -e "$repo/.git" ]; then
    git -C "$ROOT" worktree remove "$repo" 2>/dev/null || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT HUP INT TERM

archive="$repo/dist/njuprobe-0.1.0.tar.gz"
formula="$repo/dist/Formula/njuprobe.rb"

git -C "$ROOT" worktree add --detach --quiet "$repo" HEAD

build_once() {
  output_archive=$1
  output_formula=$2
  requested_umask=$3

  (
    cd "$repo"
    rm -rf dist
    umask "$requested_umask"
    ./scripts/build-release.sh 0.1.0 HEAD >/dev/null
  )
  cp "$archive" "$output_archive"
  cp "$formula" "$output_formula"
}

build_once "$workdir/archive-first.tar.gz" "$workdir/formula-first.rb" 022
build_once "$workdir/archive-second.tar.gz" "$workdir/formula-second.rb" 077

cmp "$workdir/archive-first.tar.gz" "$workdir/archive-second.tar.gz"
cmp "$workdir/formula-first.rb" "$workdir/formula-second.rb"

python3 - "$workdir/archive-first.tar.gz" "$workdir/formula-first.rb" <<'PY'
import hashlib
import pathlib
import sys
import tarfile

archive_path = pathlib.Path(sys.argv[1])
formula_path = pathlib.Path(sys.argv[2])
prefix = "njuprobe-0.1.0"

with tarfile.open(archive_path, "r:gz") as archive:
    members = archive.getmembers()
    names = [member.name for member in members]

unsafe = [
    name
    for name in names
    if name.startswith("/") or ".." in pathlib.PurePosixPath(name).parts
]
if unsafe:
    raise SystemExit(f"release artifact contains unsafe paths: {unsafe[:5]}")

top_levels = {name.split("/", 1)[0] for name in names if name}
if top_levels != {prefix}:
    raise SystemExit(f"release artifact has unexpected top-level paths: {top_levels}")

required = {
    f"{prefix}/LICENSE",
    f"{prefix}/README.md",
    f"{prefix}/go.mod",
    f"{prefix}/go.sum",
    f"{prefix}/cmd/njuprobe/main.go",
    f"{prefix}/packaging/homebrew/njuprobe.rb.tmpl",
}
missing = required.difference(names)
if missing:
    raise SystemExit(f"release artifact is missing required files: {sorted(missing)}")

forbidden_parts = {".git", ".tools", "bin", "dist"}
forbidden = [
    name
    for name in names
    if forbidden_parts.intersection(pathlib.PurePosixPath(name).parts[1:])
]
if forbidden:
    raise SystemExit(f"release artifact contains generated files: {forbidden[:5]}")

source_sha256 = hashlib.sha256(archive_path.read_bytes()).hexdigest()
formula = formula_path.read_text(encoding="utf-8")
if f'sha256 "{source_sha256}"' not in formula:
    raise SystemExit("rendered Formula does not contain the source archive SHA-256")
if "@VERSION@" in formula or "@SOURCE_SHA256@" in formula:
    raise SystemExit("rendered Formula contains unresolved placeholders")

print(
    f"Release artifact test passed: {len(members)} entries, "
    "deterministic archive and Formula."
)
PY
