#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${DIST:-$ROOT/dist}"
CACHE_ROOT="${CACHE_ROOT:-$ROOT/.cache/release}"
DOLTLITE_VERSION="${DOLTLITE_VERSION:-0.11.23}"
GO_TAGS="${GO_TAGS:-libsqlite3 gms_pure_go}"

die() {
  echo "$*" >&2
  exit 1
}

host_tuple() {
  case "$(uname -s):$(uname -m)" in
    Linux:x86_64|Linux:amd64) echo "linux_amd64" ;;
    *) die "unsupported release host: $(uname -s)/$(uname -m); currently only linux_amd64 is packaged" ;;
  esac
}

download_file() {
  local url="$1" dest="$2"
  mkdir -p "$(dirname "$dest")"
  python3 - "$url" "$dest" <<'PY'
import os
import sys
import tempfile
import urllib.request

url, dest = sys.argv[1], sys.argv[2]
directory = os.path.dirname(dest) or "."
fd, tmp = tempfile.mkstemp(prefix=".download-", dir=directory)
os.close(fd)
try:
    with urllib.request.urlopen(url, timeout=120) as response:
        with open(tmp, "wb") as out:
            while True:
                chunk = response.read(1024 * 1024)
                if not chunk:
                    break
                out.write(chunk)
    os.replace(tmp, dest)
except Exception:
    try:
        os.unlink(tmp)
    except OSError:
        pass
    raise
PY
}

extract_zip_strip_one() {
  local zip_path="$1" dest="$2" work
  work="$CACHE_ROOT/extract/doltlite-$DOLTLITE_VERSION"
  rm -rf "$work" "$dest"
  mkdir -p "$work" "$(dirname "$dest")"
  python3 - "$zip_path" "$work" "$dest" <<'PY'
import os
import shutil
import sys
import zipfile

zip_path, work, dest = sys.argv[1], sys.argv[2], sys.argv[3]
with zipfile.ZipFile(zip_path) as archive:
    archive.extractall(work)
entries = [os.path.join(work, name) for name in os.listdir(work)]
src = entries[0] if len(entries) == 1 and os.path.isdir(entries[0]) else work
shutil.copytree(src, dest)
PY
}

ensure_doltlite_lib() {
  if [ -n "${DOLTLITE_LIB:-}" ]; then
    [ -r "$DOLTLITE_LIB/doltlite.h" ] || die "DOLTLITE_LIB missing doltlite.h: $DOLTLITE_LIB"
    [ -r "$DOLTLITE_LIB/libdoltlite.a" ] || [ -r "$DOLTLITE_LIB/libdoltlite.so" ] || die "DOLTLITE_LIB missing libdoltlite: $DOLTLITE_LIB"
    echo "$DOLTLITE_LIB"
    return 0
  fi

  local dest zip url
  dest="$CACHE_ROOT/doltlite/$DOLTLITE_VERSION/linux-x64"
  if [ -r "$dest/doltlite.h" ] && { [ -r "$dest/libdoltlite.a" ] || [ -r "$dest/libdoltlite.so" ]; }; then
    echo "$dest"
    return 0
  fi

  zip="$CACHE_ROOT/downloads/doltlite-lib-linux-x64-$DOLTLITE_VERSION.zip"
  url="https://github.com/dolthub/doltlite/releases/download/v$DOLTLITE_VERSION/$(basename "$zip")"
  echo "downloading DoltLite library: $url" >&2
  download_file "$url" "$zip"
  extract_zip_strip_one "$zip" "$dest"
  [ -r "$dest/doltlite.h" ] || die "downloaded DoltLite library missing doltlite.h: $dest"
  echo "$dest"
}

build_binary() {
  local package="$1" asset="$2" doltlite_lib="$3"
  local out="$DIST/$asset"
  mkdir -p "$DIST"
  export CGO_ENABLED=1
  export CGO_CFLAGS="${CGO_CFLAGS:-"-I${doltlite_lib}"}"
  if [ -r "$doltlite_lib/libdoltlite.a" ]; then
    export CGO_LDFLAGS="${CGO_LDFLAGS:-"-L${doltlite_lib} ${doltlite_lib}/libdoltlite.a -lz -lpthread -lm"}"
  else
    export CGO_LDFLAGS="${CGO_LDFLAGS:-"-L${doltlite_lib} -Wl,-rpath,${doltlite_lib} -ldoltlite -lz -lpthread -lm"}"
  fi
  export GOCACHE="${GOCACHE:-$ROOT/.cache/go/build}"
  export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go/mod}"
  export GOTMPDIR="${GOTMPDIR:-$ROOT/.cache/go/tmp}"
  case "${TMPDIR:-}" in
    ""|/tmp|/tmp/*) export TMPDIR="$ROOT/.cache/go/tmp" ;;
  esac
  mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$TMPDIR"
  echo "building $asset from $package"
  (cd "$ROOT" && go build -tags "$GO_TAGS" -o "$out" "$package")
  chmod 0755 "$out"
  go version -m "$out" >/dev/null
}

main() {
  local platform doltlite_lib
  platform="$(host_tuple)"
  doltlite_lib="$(ensure_doltlite_lib)"
  rm -rf "$DIST"
  mkdir -p "$DIST"

  build_binary ./cmd/bd-backend-doltlite "bd-backend-doltlite_${platform}" "$doltlite_lib"
  build_binary ./cmd/gc-doltlite-fastpath "gc-doltlite-fastpath_${platform}" "$doltlite_lib"
  build_binary ./cmd/gc-doltlite "gc-doltlite_${platform}" "$doltlite_lib"

  (cd "$DIST" && sha256sum * > checksums.txt)
  ls -l "$DIST"
}

main "$@"
