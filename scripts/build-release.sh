#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: $0 VERSION [GIT_REF]" >&2
  exit 1
fi

version=${1#v}
ref=${2:-"v$version"}
archive="$ROOT/dist/njuprobe-$version.tar.gz"
formula="$ROOT/dist/Formula/njuprobe.rb"

validate_version() {
  candidate=$1
  major=${candidate%%.*}
  remainder=${candidate#*.}
  if [ "$remainder" = "$candidate" ]; then
    return 1
  fi
  minor=${remainder%%.*}
  patch=${remainder#*.}
  if [ "$patch" = "$remainder" ]; then
    return 1
  fi
  case "$patch" in
    *.*) return 1 ;;
  esac

  for component in "$major" "$minor" "$patch"; do
    case "$component" in
      ""|*[!0-9]*) return 1 ;;
      0) ;;
      0*) return 1 ;;
      *) ;;
    esac
  done
}

if ! validate_version "$version"; then
  echo "build-release: VERSION must be numeric major.minor.patch without leading zeros" >&2
  exit 1
fi

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
ensure_directory "$ROOT/dist/Formula"

temporary_tar="$ROOT/dist/.njuprobe-$version.tar.$$"
temporary_archive="$archive.tmp.$$"
temporary_formula="$ROOT/dist/Formula/.njuprobe.rb.$$"
backup_archive="$archive.backup.$$"
backup_formula="$formula.backup.$$"
cleanup() {
  rm -f \
    "$temporary_tar" \
    "$temporary_archive" \
    "$temporary_formula"
}
trap cleanup EXIT HUP INT TERM

git -C "$ROOT" archive \
  --format=tar \
  --prefix="njuprobe-$version/" \
  --output="$temporary_tar" \
  "$commit"
gzip -n -c "$temporary_tar" > "$temporary_archive"
rm -f "$temporary_tar"

if command -v sha256sum >/dev/null 2>&1; then
  source_sha256=$(sha256sum "$temporary_archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  source_sha256=$(shasum -a 256 "$temporary_archive" | awk '{print $1}')
else
  echo "build-release: sha256sum or shasum is required" >&2
  exit 1
fi

"$ROOT/scripts/render-homebrew-formula.sh" "$version" "$source_sha256" "$temporary_formula"

had_archive=0
had_formula=0
if [ -e "$archive" ] || [ -L "$archive" ]; then
  mv "$archive" "$backup_archive"
  had_archive=1
fi
if [ -e "$formula" ] || [ -L "$formula" ]; then
  if ! mv "$formula" "$backup_formula"; then
    if [ "$had_archive" -eq 1 ]; then
      mv "$backup_archive" "$archive"
    fi
    echo "build-release: cannot stage the existing Formula for replacement" >&2
    exit 1
  fi
  had_formula=1
fi

if ! mv "$temporary_archive" "$archive"; then
  if [ "$had_archive" -eq 1 ]; then
    mv "$backup_archive" "$archive"
  fi
  if [ "$had_formula" -eq 1 ]; then
    mv "$backup_formula" "$formula"
  fi
  echo "build-release: cannot publish the release archive" >&2
  exit 1
fi
temporary_archive=

if ! mv "$temporary_formula" "$formula"; then
  rm -f "$archive"
  if [ "$had_archive" -eq 1 ]; then
    mv "$backup_archive" "$archive"
  fi
  if [ "$had_formula" -eq 1 ]; then
    mv "$backup_formula" "$formula"
  fi
  echo "build-release: cannot publish the Homebrew Formula" >&2
  exit 1
fi
temporary_formula=
rm -f "$backup_archive" "$backup_formula"

printf 'Release archive: %s\n' "$archive"
printf 'Git commit:      %s\n' "$commit"
printf 'SHA-256:         %s\n' "$source_sha256"
