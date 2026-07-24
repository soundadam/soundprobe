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

expect_invalid_version() {
  command=$1
  version=$2
  if "$command" "$version" "$dummy_sha" "$workdir/invalid.rb" >/dev/null 2>&1; then
    echo "test-homebrew-template: accepted invalid version $version" >&2
    exit 1
  fi
}

for invalid_version in \
  1.2 \
  1.2.3.4 \
  1..3 \
  01.2.3 \
  1.02.3 \
  1.2.03 \
  1.2.3-rc1; do
  expect_invalid_version "$ROOT/scripts/render-homebrew-formula.sh" "$invalid_version"
done

for invalid_version in 1.2 1.2.3.4 1..3 01.2.3 1.2.3-rc1; do
  if "$ROOT/scripts/build-release.sh" "$invalid_version" >/dev/null 2>&1; then
    echo "test-homebrew-template: release builder accepted invalid version $invalid_version" >&2
    exit 1
  fi
done

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
