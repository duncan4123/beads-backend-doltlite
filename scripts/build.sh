#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${OUT:-$ROOT/bin/bd-backend-doltlite}"
DOLTLITE_LIB="${DOLTLITE_LIB:-/data/projects/doltlite-gascity/doltlite-work/build}"

if [ ! -d "$DOLTLITE_LIB" ]; then
  echo "DOLTLITE_LIB does not exist: $DOLTLITE_LIB" >&2
  echo "Set DOLTLITE_LIB=/path/to/doltlite/build" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUT")"

export CGO_ENABLED="${CGO_ENABLED:-1}"
export CGO_LDFLAGS="${CGO_LDFLAGS:-"-L${DOLTLITE_LIB} -Wl,-rpath,${DOLTLITE_LIB} -ldoltlite"}"

cd "$ROOT"
go build -tags "${GO_TAGS:-libsqlite3 gms_pure_go}" -o "$OUT" ./cmd/bd-backend-doltlite

echo "$OUT"
