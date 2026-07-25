#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
workdir=$(mktemp -d "${TMPDIR:-/tmp}/njuprobe-release.XXXXXX")
repo="$workdir/repo"
candidate_probe=
concurrent_pid=
reused_pid=
cleanup() {
  if [ -n "$concurrent_pid" ]; then
    kill "$concurrent_pid" 2>/dev/null || true
    wait "$concurrent_pid" 2>/dev/null || true
  fi
  if [ -n "$reused_pid" ]; then
    kill "$reused_pid" 2>/dev/null || true
    wait "$reused_pid" 2>/dev/null || true
  fi
  if [ -n "$candidate_probe" ] && [ -e "$candidate_probe/.git" ]; then
    git -C "$ROOT" worktree remove --force "$candidate_probe" 2>/dev/null || true
  fi
  rm -rf "$repo/dist" 2>/dev/null || true
  if [ -e "$repo/.git" ]; then
    git -C "$ROOT" worktree remove "$repo" 2>/dev/null || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT HUP INT TERM

create_candidate_ref() {
  source_repo=$1
  candidate_ref=$(git -C "$source_repo" stash create "njuprobe release candidate") || return 1
  if [ -n "$candidate_ref" ]; then
    printf '%s\n' "$candidate_ref"
  else
    git -C "$source_repo" rev-parse --verify HEAD
  fi
}

archive="$repo/dist/njuprobe-0.1.0.tar.gz"
formula="$repo/dist/Formula/njuprobe.rb"

assert_no_publication_residue() {
  if find "$repo/dist" \
    \( -name '*.tmp.*' -o -name '*.backup.*' -o -name '.njuprobe-release.lock' \
      -o -name '.njuprobe-release.lock.stale.*' \
      -o -name '.njuprobe-release.lock.owner.*' \) \
    -print | grep -q .; then
    echo "release artifact test: build left publication staging files" >&2
    exit 1
  fi
}

candidate_ref=$(create_candidate_ref "$ROOT")
git -C "$ROOT" worktree add --detach --quiet "$repo" "$candidate_ref"

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

assert_publication_modes() {
  python3 - "$archive" "$formula" <<'PY'
import pathlib
import stat
import sys

for raw_path in sys.argv[1:]:
    path = pathlib.Path(raw_path)
    mode = stat.S_IMODE(path.stat().st_mode)
    if mode != 0o644:
        raise SystemExit(f"release artifact test: {path} mode is {mode:o}, expected 644")
PY
}

build_once "$workdir/archive-first.tar.gz" "$workdir/formula-first.rb" 022
assert_publication_modes
build_once "$workdir/archive-second.tar.gz" "$workdir/formula-second.rb" 077
assert_publication_modes

cmp "$workdir/archive-first.tar.gz" "$workdir/archive-second.tar.gz"
cmp "$workdir/formula-first.rb" "$workdir/formula-second.rb"

cp "$archive" "$workdir/archive-before-failure.tar.gz"
cp "$formula" "$workdir/formula-before-failure.rb"
cp "$repo/packaging/homebrew/njuprobe.rb.tmpl" "$workdir/formula-template-original.rb"
printf '\n# @UNRESOLVED_RELEASE_TOKEN@\n' >> "$repo/packaging/homebrew/njuprobe.rb.tmpl"
invalid_template_succeeded=0
if (
  cd "$repo"
  ./scripts/build-release.sh 0.1.0 HEAD >/dev/null 2>&1
); then
  invalid_template_succeeded=1
fi
cp "$workdir/formula-template-original.rb" "$repo/packaging/homebrew/njuprobe.rb.tmpl"
if [ "$invalid_template_succeeded" -eq 1 ]; then
  echo "release artifact test: invalid Formula template unexpectedly succeeded" >&2
  exit 1
fi
cmp "$archive" "$workdir/archive-before-failure.tar.gz"
cmp "$formula" "$workdir/formula-before-failure.rb"
assert_no_publication_residue

fake_bin="$workdir/fake-bin"
signal_marker="$workdir/term-injected"
mkdir "$fake_bin"
cat > "$fake_bin/mv" <<'SH'
#!/bin/sh
set -eu

"${NJU_REAL_MV:?}" "$@"
destination=
for argument in "$@"; do
  destination=$argument
done
case "$destination" in
  */dist/Formula/njuprobe.rb)
    if [ ! -e "${NJU_SIGNAL_MARKER:?}" ]; then
      : > "$NJU_SIGNAL_MARKER"
      kill -TERM "$PPID"
    fi
    ;;
esac
SH
chmod +x "$fake_bin/mv"

interrupted_build_succeeded=0
if (
  cd "$repo"
  NJU_REAL_MV=$(command -v mv) \
    NJU_SIGNAL_MARKER="$signal_marker" \
    PATH="$fake_bin:$PATH" \
    ./scripts/build-release.sh 0.1.0 HEAD >/dev/null 2>&1
); then
  interrupted_build_succeeded=1
fi
if [ "$interrupted_build_succeeded" -eq 1 ]; then
  echo "release artifact test: interrupted publication unexpectedly succeeded" >&2
  exit 1
fi
cmp "$archive" "$workdir/archive-before-failure.tar.gz"
cmp "$formula" "$workdir/formula-before-failure.rb"
assert_no_publication_residue

kill_fake_bin="$workdir/kill-fake-bin"
kill_marker="$workdir/lock-link-kill-injected"
mkdir "$kill_fake_bin"
cat > "$kill_fake_bin/ln" <<'SH'
#!/bin/sh
set -eu

"${NJU_REAL_LN:?}" "$@"
destination=
for argument in "$@"; do
  destination=$argument
done
case "$destination" in
  */dist/.njuprobe-release.lock)
    if [ ! -e "${NJU_KILL_MARKER:?}" ]; then
      : > "$NJU_KILL_MARKER"
      kill -KILL "$PPID"
    fi
    ;;
esac
SH
chmod +x "$kill_fake_bin/ln"

atomic_lock_build_succeeded=0
if (
  cd "$repo"
  NJU_REAL_LN=$(command -v ln) \
    NJU_KILL_MARKER="$kill_marker" \
    PATH="$kill_fake_bin:$PATH" \
    ./scripts/build-release.sh 0.1.0 HEAD >/dev/null 2>&1
); then
  atomic_lock_build_succeeded=1
fi
if [ "$atomic_lock_build_succeeded" -eq 1 ] || [ ! -e "$kill_marker" ]; then
  echo "release artifact test: lock-link SIGKILL fixture did not interrupt publication" >&2
  exit 1
fi
if [ ! -f "$repo/dist/.njuprobe-release.lock" ] || \
  [ -L "$repo/dist/.njuprobe-release.lock" ]; then
  echo "release artifact test: interrupted atomic lock was not a regular file" >&2
  exit 1
fi
if ! (
  cd "$repo"
  ./scripts/build-release.sh 0.1.0 HEAD >"$workdir/atomic-lock-recovery.log" 2>&1
); then
  echo "release artifact test: complete atomic lock was not recovered after SIGKILL" >&2
  cat "$workdir/atomic-lock-recovery.log" >&2
  exit 1
fi
if ! grep -q 'recovered stale release publication lock from PID' "$workdir/atomic-lock-recovery.log"; then
  echo "release artifact test: atomic lock recovery was not reported" >&2
  cat "$workdir/atomic-lock-recovery.log" >&2
  exit 1
fi
cmp "$archive" "$workdir/archive-before-failure.tar.gz"
cmp "$formula" "$workdir/formula-before-failure.rb"
assert_no_publication_residue

(
  sleep 1
) &
stale_lock_pid=$!
stale_lock_start=$(LC_ALL=C ps -p "$stale_lock_pid" -o lstart= | awk '{$1=$1; print}')
wait "$stale_lock_pid"
mkdir "$repo/dist/.njuprobe-release.lock"
printf '%s\n' "$stale_lock_pid" > "$repo/dist/.njuprobe-release.lock/pid"
printf '%s\n' "$stale_lock_start" > "$repo/dist/.njuprobe-release.lock/start"
if ! (
  cd "$repo"
  ./scripts/build-release.sh 0.1.0 HEAD >"$workdir/stale-lock.log" 2>&1
); then
  echo "release artifact test: stale release lock was not recovered" >&2
  cat "$workdir/stale-lock.log" >&2
  exit 1
fi
if ! grep -q "recovered stale release publication lock from PID $stale_lock_pid" "$workdir/stale-lock.log"; then
  echo "release artifact test: stale release lock recovery was not reported" >&2
  cat "$workdir/stale-lock.log" >&2
  exit 1
fi
cmp "$archive" "$workdir/archive-before-failure.tar.gz"
cmp "$formula" "$workdir/formula-before-failure.rb"
assert_no_publication_residue

sleep 30 &
reused_pid=$!
reused_start=$(LC_ALL=C ps -p "$reused_pid" -o lstart= | awk '{$1=$1; print}')
mkdir "$repo/dist/.njuprobe-release.lock"
printf '%s\n' "$reused_pid" > "$repo/dist/.njuprobe-release.lock/pid"
printf '%s\n' 'Thu Jan 1 00:00:00 1970' > "$repo/dist/.njuprobe-release.lock/start"
if ! (
  cd "$repo"
  ./scripts/build-release.sh 0.1.0 HEAD >"$workdir/reused-pid-lock.log" 2>&1
); then
  echo "release artifact test: reused PID release lock was not recovered" >&2
  cat "$workdir/reused-pid-lock.log" >&2
  exit 1
fi
if ! grep -q "recovered stale release publication lock from PID $reused_pid (process identity changed)" "$workdir/reused-pid-lock.log"; then
  echo "release artifact test: reused PID recovery was not reported" >&2
  cat "$workdir/reused-pid-lock.log" >&2
  exit 1
fi
if ! kill -0 "$reused_pid" 2>/dev/null; then
  echo "release artifact test: reused PID fixture was terminated" >&2
  exit 1
fi
if [ "$(LC_ALL=C ps -p "$reused_pid" -o lstart= | awk '{$1=$1; print}')" != "$reused_start" ]; then
  echo "release artifact test: reused PID fixture identity changed unexpectedly" >&2
  exit 1
fi
kill "$reused_pid"
wait "$reused_pid" 2>/dev/null || true
reused_pid=
cmp "$archive" "$workdir/archive-before-failure.tar.gz"
cmp "$formula" "$workdir/formula-before-failure.rb"
assert_no_publication_residue

mkdir "$repo/dist/.njuprobe-release.lock"
printf '%s\n' 'invalid-owner' > "$repo/dist/.njuprobe-release.lock/pid"
printf '%s\n' 'invalid-start' > "$repo/dist/.njuprobe-release.lock/start"
invalid_lock_succeeded=0
if (
  cd "$repo"
  ./scripts/build-release.sh 0.1.0 HEAD >"$workdir/invalid-lock.log" 2>&1
); then
  invalid_lock_succeeded=1
fi
if [ "$invalid_lock_succeeded" -eq 1 ]; then
  echo "release artifact test: invalid release lock unexpectedly succeeded" >&2
  exit 1
fi
if ! grep -q 'release publication lock has no valid owner process identity' "$workdir/invalid-lock.log"; then
  echo "release artifact test: invalid release lock did not fail closed" >&2
  cat "$workdir/invalid-lock.log" >&2
  exit 1
fi
if [ "$(cat "$repo/dist/.njuprobe-release.lock/pid")" != 'invalid-owner' ]; then
  echo "release artifact test: invalid release lock was modified" >&2
  exit 1
fi
rm -f "$repo/dist/.njuprobe-release.lock/pid"
rm -f "$repo/dist/.njuprobe-release.lock/start"
rmdir "$repo/dist/.njuprobe-release.lock"
assert_no_publication_residue

concurrent_fake_bin="$workdir/concurrent-fake-bin"
concurrent_ready="$workdir/concurrent-ready"
concurrent_continue="$workdir/concurrent-continue"
concurrent_log="$workdir/concurrent-build.log"
mkdir "$concurrent_fake_bin"
cat > "$concurrent_fake_bin/git" <<'SH'
#!/bin/sh
set -eu

is_archive=0
for argument in "$@"; do
  if [ "$argument" = "archive" ]; then
    is_archive=1
  fi
done
if [ "$is_archive" -eq 1 ]; then
  : > "${NJU_CONCURRENT_READY:?}"
  while [ ! -e "${NJU_CONCURRENT_CONTINUE:?}" ]; do
    sleep 0.05
  done
fi
exec "${NJU_REAL_GIT:?}" "$@"
SH
chmod +x "$concurrent_fake_bin/git"

(
  cd "$repo"
  NJU_REAL_GIT=$(command -v git) \
    NJU_CONCURRENT_READY="$concurrent_ready" \
    NJU_CONCURRENT_CONTINUE="$concurrent_continue" \
    PATH="$concurrent_fake_bin:$PATH" \
    ./scripts/build-release.sh 0.1.0 HEAD >"$concurrent_log" 2>&1
) &
concurrent_pid=$!

wait_count=0
while [ ! -e "$concurrent_ready" ]; do
  if ! kill -0 "$concurrent_pid" 2>/dev/null; then
    wait "$concurrent_pid" || true
    concurrent_pid=
    echo "release artifact test: concurrent fixture exited before acquiring the lock" >&2
    cat "$concurrent_log" >&2
    exit 1
  fi
  wait_count=$((wait_count + 1))
  if [ "$wait_count" -ge 200 ]; then
    echo "release artifact test: timed out waiting for the release lock fixture" >&2
    exit 1
  fi
  sleep 0.05
done

concurrent_second_succeeded=0
if (
  cd "$repo"
  ./scripts/build-release.sh 0.2.0 HEAD >"$workdir/concurrent-second.log" 2>&1
); then
  concurrent_second_succeeded=1
fi
if [ "$concurrent_second_succeeded" -eq 1 ]; then
  echo "release artifact test: concurrent release unexpectedly succeeded" >&2
  exit 1
fi
if ! grep -q 'another release publication is already in progress' "$workdir/concurrent-second.log"; then
  echo "release artifact test: concurrent release did not report lock contention" >&2
  cat "$workdir/concurrent-second.log" >&2
  exit 1
fi
if [ -e "$repo/dist/njuprobe-0.2.0.tar.gz" ]; then
  echo "release artifact test: rejected concurrent release published an archive" >&2
  exit 1
fi

: > "$concurrent_continue"
if ! wait "$concurrent_pid"; then
  concurrent_pid=
  echo "release artifact test: lock-holding release failed" >&2
  cat "$concurrent_log" >&2
  exit 1
fi
concurrent_pid=
cmp "$archive" "$workdir/archive-before-failure.tar.gz"
cmp "$formula" "$workdir/formula-before-failure.rb"
assert_no_publication_residue

candidate_probe="$workdir/candidate-probe"
printf '\nRELEASE_CANDIDATE_SNAPSHOT_PROBE\n' >> "$repo/README.md"
probe_ref=$(create_candidate_ref "$repo")
git -C "$ROOT" worktree add --detach --quiet "$candidate_probe" "$probe_ref"
(
  cd "$candidate_probe"
  ./scripts/build-release.sh 0.1.0 HEAD >/dev/null
)
if ! tar -xOzf "$candidate_probe/dist/njuprobe-0.1.0.tar.gz" \
  njuprobe-0.1.0/README.md | grep -q RELEASE_CANDIDATE_SNAPSHOT_PROBE; then
  echo "release artifact test: candidate tracked changes were not archived" >&2
  exit 1
fi
git -C "$ROOT" worktree remove --force "$candidate_probe"
candidate_probe=
git -C "$repo" checkout -- README.md

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
