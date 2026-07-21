#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TOOLS_DIR="$ROOT/.tools/bin"
LIBRESPEED_VERSION="v1.0.13"
LIBRESPEED_COMMIT="2f2408764d88e9601aa64a03b340f8e3151003e4"
LIBRESPEED_REPOSITORY="https://github.com/librespeed/speedtest-cli.git"
DESTINATION="$TOOLS_DIR/librespeed-cli"

command -v git >/dev/null 2>&1 || {
  echo "install-dev-tools: git is required" >&2
  exit 1
}
command -v go >/dev/null 2>&1 || {
  echo "install-dev-tools: Go is required" >&2
  exit 1
}

mkdir -p "$TOOLS_DIR"
workdir=$(mktemp -d "${TMPDIR:-/tmp}/njuprobe-tools.XXXXXX")
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT HUP INT TERM

source_dir="$workdir/speedtest-cli"
git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$LIBRESPEED_VERSION" "$LIBRESPEED_REPOSITORY" "$source_dir"
actual_commit=$(git -C "$source_dir" rev-parse HEAD)
if [ "$actual_commit" != "$LIBRESPEED_COMMIT" ]; then
  echo "install-dev-tools: LibreSpeed commit mismatch: got $actual_commit" >&2
  exit 1
fi

built_binary="$workdir/librespeed-cli"
build_date="1970-01-01T00:00:00Z"
ldflags="-w -s -X github.com/librespeed/speedtest-cli/defs.ProgName=librespeed-cli -X github.com/librespeed/speedtest-cli/defs.ProgVersion=$LIBRESPEED_VERSION -X github.com/librespeed/speedtest-cli/defs.BuildDate=$build_date"
(
  cd "$source_dir"
  go build -trimpath -ldflags "$ldflags" -o "$built_binary" ./
)

temporary="$TOOLS_DIR/.librespeed-cli.$$"
install -m 0755 "$built_binary" "$temporary"
mv -f "$temporary" "$DESTINATION"

first_line=$($DESTINATION --version | sed -n '1p')
case "$first_line" in
  "librespeed-cli $LIBRESPEED_VERSION "*) ;;
  *)
    echo "install-dev-tools: unexpected LibreSpeed version output: $first_line" >&2
    exit 1
    ;;
esac

printf 'Installed %s at %s\n' "$first_line" "$DESTINATION"
