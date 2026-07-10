#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCK_FILE="$ROOT/build/doltlite.lock"

die() {
  echo "zig-build: $*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
usage: scripts/zig-build.sh ACTION [ARGUMENTS] [OPTIONS]

Actions:
  dependency                       Prepare the locked DoltLite release library.
  go-binary PACKAGE NAME           Build one linked Go plugin binary.
  test                             Run linked plugin regression tests.
  provenance                       Write output hashes and dependency provenance.

Options:
  --doltlite-lib DIR               Use an existing DoltLite library directory.
  --cache-root DIR                 Native dependency cache (default .zig-cache/native).
  --output-root DIR                Binary output root (default zig-out).
  --go PATH                        Go executable (default go).
EOF
}

[ -r "$LOCK_FILE" ] || die "missing lock file: $LOCK_FILE"
# shellcheck disable=SC1090
source "$LOCK_FILE"

ACTION="${1:-}"
[ -n "$ACTION" ] || { usage >&2; exit 2; }
shift

PACKAGE=""
BINARY_NAME=""
if [ "$ACTION" = "go-binary" ]; then
  [ "$#" -ge 2 ] || die "go-binary requires PACKAGE and NAME"
  PACKAGE="$1"
  BINARY_NAME="$2"
  shift 2
fi

DOLTLITE_LIB=""
CACHE_ROOT="$ROOT/.zig-cache/native"
OUTPUT_ROOT="$ROOT/zig-out"
GO_EXE="go"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --doltlite-lib) DOLTLITE_LIB="${2:-}"; shift 2 ;;
    --cache-root) CACHE_ROOT="${2:-}"; shift 2 ;;
    --output-root) OUTPUT_ROOT="${2:-}"; shift 2 ;;
    --go) GO_EXE="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

case "$CACHE_ROOT" in /*) ;; *) CACHE_ROOT="$ROOT/$CACHE_ROOT" ;; esac
case "$OUTPUT_ROOT" in /*) ;; *) OUTPUT_ROOT="$ROOT/$OUTPUT_ROOT" ;; esac
if [ -n "$DOLTLITE_LIB" ]; then
  case "$DOLTLITE_LIB" in /*) ;; *) DOLTLITE_LIB="$ROOT/$DOLTLITE_LIB" ;; esac
fi

ZIG_EXE="${ZIG_EXE:-zig}"
command -v "$ZIG_EXE" >/dev/null 2>&1 || die "Zig executable not found: $ZIG_EXE"
command -v "$GO_EXE" >/dev/null 2>&1 || die "Go executable not found: $GO_EXE"

actual_zig_version="$($ZIG_EXE version)"
[ "$actual_zig_version" = "$ZIG_VERSION" ] || die "Zig $ZIG_VERSION required, got $actual_zig_version"

host_release() {
  case "$(uname -s):$(uname -m)" in
    Linux:x86_64|Linux:amd64)
      platform="linux-x64"
      asset="$DOLTLITE_LINUX_X64_ASSET"
      asset_sha256="$DOLTLITE_LINUX_X64_SHA256"
      ;;
    *) die "unsupported Zig plugin host: $(uname -s)/$(uname -m)" ;;
  esac
  asset_url="https://github.com/dolthub/doltlite/releases/download/v$DOLTLITE_VERSION/$asset"
}

verify_library() {
  [ -r "$DOLTLITE_LIB/doltlite.h" ] || die "missing doltlite.h in $DOLTLITE_LIB"
  [ -r "$DOLTLITE_LIB/libdoltlite.a" ] || [ -r "$DOLTLITE_LIB/libdoltlite.so" ] || \
    die "missing libdoltlite in $DOLTLITE_LIB"
  DOLTLITE_LIB="$(cd "$DOLTLITE_LIB" && pwd -P)"
}

ensure_dependency() {
  if [ -n "$DOLTLITE_LIB" ]; then
    dependency_source="local"
    dependency_archive_sha256=""
    verify_library
    return
  fi

  host_release
  command -v curl >/dev/null 2>&1 || die "curl is required to download DoltLite"
  command -v unzip >/dev/null 2>&1 || die "unzip is required to extract DoltLite"
  command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required to verify DoltLite"

  archive="$CACHE_ROOT/downloads/$asset"
  DOLTLITE_LIB="$CACHE_ROOT/doltlite/$DOLTLITE_VERSION/$platform"
  mkdir -p "$(dirname "$archive")" "$(dirname "$DOLTLITE_LIB")"
  if [ ! -r "$archive" ]; then
    tmp="$archive.download.$$"
    trap 'rm -f "${tmp:-}"' EXIT INT TERM HUP
    curl -fL --retry 3 -o "$tmp" "$asset_url"
    mv -f "$tmp" "$archive"
    trap - EXIT INT TERM HUP
  fi

  actual_archive_sha="$(sha256sum "$archive" | awk '{print $1}')"
  [ "$actual_archive_sha" = "$asset_sha256" ] || \
    die "DoltLite archive checksum mismatch: got $actual_archive_sha, want $asset_sha256"

  if [ ! -r "$DOLTLITE_LIB/doltlite.h" ] || \
     { [ ! -r "$DOLTLITE_LIB/libdoltlite.a" ] && [ ! -r "$DOLTLITE_LIB/libdoltlite.so" ]; }; then
    extract_root="$CACHE_ROOT/extract/$asset.$$"
    rm -rf "$extract_root" "$DOLTLITE_LIB"
    mkdir -p "$extract_root"
    unzip -q -o "$archive" -d "$extract_root"
    extracted="$extract_root/doltlite-lib-$platform-$DOLTLITE_VERSION"
    [ -d "$extracted" ] || die "unexpected DoltLite archive layout"
    mv -f "$extracted" "$DOLTLITE_LIB"
    rm -rf "$extract_root"
  fi

  dependency_source="$asset_url"
  dependency_archive_sha256="$actual_archive_sha"
  verify_library
}

prepare_go_env() {
  export CGO_ENABLED=1
  export CC="$ZIG_EXE cc"
  export CGO_CFLAGS="-I$DOLTLITE_LIB"
  if [ -r "$DOLTLITE_LIB/libdoltlite.a" ]; then
    export CGO_LDFLAGS="$DOLTLITE_LIB/libdoltlite.a -lz -lpthread -lm"
  else
    export CGO_LDFLAGS="-L$DOLTLITE_LIB -Wl,-rpath,$DOLTLITE_LIB -ldoltlite -lz -lpthread -lm"
  fi
  export GOCACHE="${GOCACHE:-$CACHE_ROOT/go/build}"
  export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/go/mod}"
  export GOTMPDIR="${GOTMPDIR:-$CACHE_ROOT/go/tmp}"
  export TMPDIR="${TMPDIR:-$CACHE_ROOT/go/tmp}"
  mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$TMPDIR"
}

build_go_binary() {
  ensure_dependency
  prepare_go_env
  mkdir -p "$OUTPUT_ROOT/bin"
  echo "building $BINARY_NAME with zig cc against DoltLite v$DOLTLITE_VERSION"
  (
    cd "$ROOT"
    "$GO_EXE" build -tags "${GO_TAGS:-libsqlite3 gms_pure_go}" \
      -o "$OUTPUT_ROOT/bin/$BINARY_NAME" "$PACKAGE"
  )
  chmod 0755 "$OUTPUT_ROOT/bin/$BINARY_NAME"
  "$GO_EXE" version -m "$OUTPUT_ROOT/bin/$BINARY_NAME" >/dev/null
}

run_tests() {
  ensure_dependency
  prepare_go_env
  (
    cd "$ROOT"
    "$GO_EXE" test -tags "${GO_TAGS:-libsqlite3 gms_pure_go}" \
      ./internal/storage/doltlite \
      -run TestCreateIssuePersistsEmbeddedDependencies -count=1
    "$GO_EXE" test -tags "${GO_TAGS:-libsqlite3 gms_pure_go}" \
      ./internal/provider ./cmd/gc-doltlite-fastpath -count=1
  )

  "$OUTPUT_ROOT/bin/bd-backend-doltlite" capabilities >/dev/null
  "$OUTPUT_ROOT/bin/gc-doltlite-fastpath" capabilities >/dev/null
  set +e
  companion_usage="$("$OUTPUT_ROOT/bin/gc-doltlite" 2>&1)"
  companion_rc=$?
  set -e
  [ "$companion_rc" -eq 2 ] || die "gc-doltlite without a command returned $companion_rc, want 2"
  [[ "$companion_usage" == usage:* ]] || die "gc-doltlite did not emit usage"
}

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  printf '%s' "$value"
}

write_provenance() {
  ensure_dependency
  mkdir -p "$OUTPUT_ROOT"
  plugin_change_id=""
  if command -v jj >/dev/null 2>&1 && jj -R "$ROOT" root >/dev/null 2>&1; then
    plugin_vcs="jj"
    plugin_commit="$(jj -R "$ROOT" log -r @ --no-graph -T 'commit_id ++ "\n"')"
    plugin_change_id="$(jj -R "$ROOT" log -r @ --no-graph -T 'change_id ++ "\n"')"
    plugin_dirty=false
  else
    plugin_vcs="git"
    plugin_commit="$(git -C "$ROOT" rev-parse HEAD)"
    plugin_dirty=false
    if ! git -C "$ROOT" diff --quiet || ! git -C "$ROOT" diff --cached --quiet; then
      plugin_dirty=true
    fi
  fi
  library_file="$DOLTLITE_LIB/libdoltlite.a"
  [ -r "$library_file" ] || library_file="$DOLTLITE_LIB/libdoltlite.so"
  library_sha="$(sha256sum "$library_file" | awk '{print $1}')"
  {
    printf '{\n'
    printf '  "schema_version": "1",\n'
    printf '  "zig_version": "%s",\n' "$(json_escape "$actual_zig_version")"
    printf '  "go_version": "%s",\n' "$(json_escape "$($GO_EXE version)")"
    printf '  "plugin_vcs": "%s",\n' "$plugin_vcs"
    printf '  "plugin_commit": "%s",\n' "$plugin_commit"
    printf '  "plugin_change_id": "%s",\n' "$plugin_change_id"
    printf '  "plugin_dirty": %s,\n' "$plugin_dirty"
    printf '  "doltlite_version": "%s",\n' "$DOLTLITE_VERSION"
    printf '  "doltlite_source": "%s",\n' "$(json_escape "$dependency_source")"
    printf '  "doltlite_archive_sha256": "%s",\n' "$dependency_archive_sha256"
    printf '  "doltlite_library": "%s",\n' "$(json_escape "$library_file")"
    printf '  "libdoltlite_sha256": "%s",\n' "$library_sha"
    printf '  "binaries": {\n'
    first=true
    for name in bd-backend-doltlite gc-doltlite-fastpath gc-doltlite; do
      path="$OUTPUT_ROOT/bin/$name"
      [ -x "$path" ] || die "missing plugin binary: $path"
      sha="$(sha256sum "$path" | awk '{print $1}')"
      if [ "$first" = true ]; then first=false; else printf ',\n'; fi
      printf '    "%s": "%s"' "$name" "$sha"
    done
    printf '\n  }\n}\n'
  } > "$OUTPUT_ROOT/build-provenance.json"
  echo "wrote $OUTPUT_ROOT/build-provenance.json"
}

case "$ACTION" in
  dependency) ensure_dependency ;;
  go-binary) build_go_binary ;;
  test) run_tests ;;
  provenance) write_provenance ;;
  -h|--help|help) usage ;;
  *) die "unknown action: $ACTION" ;;
esac
