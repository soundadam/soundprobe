#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: $0 VERSION [GIT_REF]" >&2
  exit 1
fi

version=${1#v}
ref=${2:-"v$version"}
archive="$ROOT/dist/njuprobe-$version.tar.gz"

case "$version" in
  *[!0-9.]*|.*|*.)
    echo "build-release: VERSION must be numeric semver" >&2
    exit 1
    ;;
esac
case "$version" in
  *.*.*) ;;
  *)
    echo "build-release: VERSION must contain major.minor.patch" >&2
    exit 1
    ;;
esac

ensure_directory() {
  directory=$1
  if [ -d "$directory" ]; then
    return
  fi
  if ! mkdir -p "$directory" 2>/dev/null && [ ! -d "$directory" ]; then
    echo "build-release: cannot create directory $directory" >&2
    exit 1
  fi
}

commit=$(git -C "$ROOT" rev-parse --verify "$ref^{commit}")
ensure_directory "$ROOT/dist"

git -C "$ROOT" archive \
  --format=tar \
  --prefix="njuprobe-$version/" \
  "$commit" | gzip -n > "$archive"

if command -v sha256sum >/dev/null 2>&1; then
  source_sha256=$(sha256sum "$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  source_sha256=$(shasum -a 256 "$archive" | awk '{print $1}')
else
  echo "build-release: sha256sum or shasum is required" >&2
  exit 1
fi

"$ROOT/scripts/render-homebrew-formula.sh" "$version" "$source_sha256"

printf 'Release archive: %s\n' "$archive"
printf 'Git commit:      %s\n' "$commit"
printf 'SHA-256:         %s\n' "$source_sha256"
