#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEMPLATE="$ROOT/packaging/homebrew/njuprobe.rb.tmpl"

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  echo "usage: $0 VERSION SOURCE_SHA256 [OUTPUT]" >&2
  exit 1
fi

version=${1#v}
source_sha256=$2
output=${3:-"$ROOT/dist/Formula/njuprobe.rb"}

case "$version" in
  *[!0-9.]*|.*|*.)
    echo "render-homebrew-formula: VERSION must be numeric semver" >&2
    exit 1
    ;;
esac
case "$version" in
  *.*.*) ;;
  *)
    echo "render-homebrew-formula: VERSION must contain major.minor.patch" >&2
    exit 1
    ;;
esac
case "$source_sha256" in
  *[!0-9a-f]*|"")
    echo "render-homebrew-formula: SOURCE_SHA256 must be lowercase hexadecimal" >&2
    exit 1
    ;;
esac
if [ "${#source_sha256}" -ne 64 ]; then
  echo "render-homebrew-formula: SOURCE_SHA256 must contain 64 characters" >&2
  exit 1
fi

output_directory=$(dirname -- "$output")
if [ ! -d "$output_directory" ]; then
  if ! mkdir -p "$output_directory" 2>/dev/null && [ ! -d "$output_directory" ]; then
    echo "render-homebrew-formula: cannot create directory $output_directory" >&2
    exit 1
  fi
fi
sed \
  -e "s/@VERSION@/$version/g" \
  -e "s/@SOURCE_SHA256@/$source_sha256/g" \
  "$TEMPLATE" > "$output"

if grep -q '@[A-Z_][A-Z_]*@' "$output"; then
  echo "render-homebrew-formula: unresolved template placeholder" >&2
  exit 1
fi

printf 'Rendered Homebrew Formula: %s\n' "$output"
