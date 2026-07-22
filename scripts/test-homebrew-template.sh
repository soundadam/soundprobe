#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
workdir=$(mktemp -d "${TMPDIR:-/tmp}/njuprobe-formula.XXXXXX")
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT HUP INT TERM

output="$workdir/njuprobe.rb"
dummy_sha="0000000000000000000000000000000000000000000000000000000000000000"
"$ROOT/scripts/render-homebrew-formula.sh" 0.1.0 "$dummy_sha" "$output" >/dev/null

if command -v ruby >/dev/null 2>&1; then
  ruby -c "$output" >/dev/null
else
  printf '%s\n' 'Ruby not installed; syntax check deferred to the macOS Homebrew gate.'
fi

grep -q 'class Njuprobe < Formula' "$output"
grep -q 'depends_on "go" => :build' "$output"
grep -q 'resource "librespeed-cli"' "$output"
grep -q 'resource "ndt7-client"' "$output"
grep -q 'assert_match '\''"ready":true' "$output"
if grep -q '@[A-Z_][A-Z_]*@' "$output"; then
  echo "test-homebrew-template: unresolved placeholder" >&2
  exit 1
fi

printf '%s\n' 'Homebrew Formula template test passed.'
