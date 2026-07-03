#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_BIN="${PLUGIN_BIN:-$ROOT/bin/bd-backend-doltlite}"
BD_BIN="${BD_BIN:-}"
DOLTLITE_LIB="${DOLTLITE_LIB:-/data/projects/doltlite-gascity/doltlite-work/build}"

if [ ! -x "$PLUGIN_BIN" ]; then
  OUT="$PLUGIN_BIN" DOLTLITE_LIB="$DOLTLITE_LIB" "$ROOT/scripts/build.sh" >/dev/null
fi

if [ -z "$BD_BIN" ]; then
  for candidate in \
    /data/projects/doltlite-gascity/workspaces/beads-plugin-architecture/bin/bd \
    /data/projects/doltlite-gascity/beads-doltlite/bin/bd \
    "$(command -v bd 2>/dev/null || true)"; do
    if [ -n "$candidate" ] && [ -x "$candidate" ]; then
      BD_BIN="$candidate"
      break
    fi
  done
fi

if [ ! -x "$BD_BIN" ]; then
  echo "BD_BIN must point at a bd binary built from feat/backend-plugin-architecture" >&2
  exit 1
fi

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

mkdir -p "$tmp/.beads"
chmod 700 "$tmp/.beads"
cat >"$tmp/.beads/metadata.json" <<JSON
{
  "database": "beads.db",
  "backend": "doltlite",
  "dolt_database": "beads",
  "backend_plugin_command": "$PLUGIN_BIN"
}
JSON

printf '%s\n' "{\"id\":\"init\",\"method\":\"init\",\"params\":{\"beads_dir\":\"$tmp/.beads\",\"database\":\"beads\",\"prefix\":\"bp\",\"actor\":\"smoke\"}}" \
  | "$PLUGIN_BIN" serve >/dev/null

(
  cd "$tmp"
  "$BD_BIN" config get issue_prefix | grep -q "bp"
  "$BD_BIN" create "Plugin adapter smoke" --id bp-1 >/dev/null
  "$BD_BIN" show bp-1 --json | grep -q "Plugin adapter smoke"
  "$BD_BIN" update bp-1 --add-label plugin-smoke >/dev/null
  "$BD_BIN" ready --json | grep -q "bp-1"
)

echo "ok: bd used backend_plugin_command=$PLUGIN_BIN"
