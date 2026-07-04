#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ -z "${DOLTLITE_ROOT:-}" ]; then
  for candidate in \
    "/data/projects/doltlite-gascity/doltlite" \
    "$ROOT/../../doltlite" \
    "$ROOT/../../../doltlite"; do
    if [ -x "$candidate/doltlite-remotesrv" ] && { [ -f "$candidate/libdoltlite.so" ] || [ -f "$candidate/libdoltlite.a" ]; }; then
      DOLTLITE_ROOT="$(cd "$candidate" && pwd)"
      break
    fi
  done
fi

if [ -z "${DOLTLITE_ROOT:-}" ]; then
  echo "DOLTLITE_ROOT is not set and no local doltlite checkout was found." >&2
  echo "Set DOLTLITE_ROOT=/path/to/doltlite containing doltlite-remotesrv and libdoltlite." >&2
  exit 1
fi

DOLTLITE_REMOTESRV="${DOLTLITE_REMOTESRV:-$DOLTLITE_ROOT/doltlite-remotesrv}"
if [ ! -x "$DOLTLITE_REMOTESRV" ]; then
  echo "DOLTLITE_REMOTESRV is not executable: $DOLTLITE_REMOTESRV" >&2
  exit 1
fi

if [ ! -f "$DOLTLITE_ROOT/libdoltlite.so" ] && [ ! -f "$DOLTLITE_ROOT/libdoltlite.a" ]; then
  echo "No libdoltlite library found in DOLTLITE_ROOT: $DOLTLITE_ROOT" >&2
  exit 1
fi

LIB_DIR="${LIB_DIR:-$ROOT/.cache/doltlite-lib}"
mkdir -p "$LIB_DIR"
if [ -f "$DOLTLITE_ROOT/libdoltlite.so" ]; then
  ln -sfn "$DOLTLITE_ROOT/libdoltlite.so" "$LIB_DIR/libdoltlite.so"
  ln -sfn "$DOLTLITE_ROOT/libdoltlite.so" "$LIB_DIR/libdoltlite.so.0"
fi
if [ -f "$DOLTLITE_ROOT/libdoltlite.a" ]; then
  ln -sfn "$DOLTLITE_ROOT/libdoltlite.a" "$LIB_DIR/libdoltlite.a"
fi

export DOLTLITE_ROOT
export DOLTLITE_REMOTESRV
export CGO_ENABLED="${CGO_ENABLED:-1}"
export CGO_CFLAGS="${CGO_CFLAGS:-"-I${DOLTLITE_ROOT}"}"
export CGO_LDFLAGS="${CGO_LDFLAGS:-"-L${LIB_DIR} -Wl,-rpath,${LIB_DIR} -ldoltlite -lz -lpthread"}"
export LD_LIBRARY_PATH="${LIB_DIR}${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

cd "$ROOT"

if [ "$#" -eq 0 ]; then
  set -- ./internal/storage/doltlite -run 'TestDoltLiteTimestampOrderingParity' -count=1 -v
fi

go test -tags "${GO_TAGS:-libsqlite3 gms_pure_go}" "$@"
