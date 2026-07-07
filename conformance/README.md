# Conformance

The DoltLite backend plugin is tested from Beads, not from a private in-process
fixture in this repository. The important test is the Beads end-to-end
conformance harness: it builds a real `bd` binary, initializes one workspace
with the embedded Dolt reference backend and another with this plugin, runs the
same CLI scenarios against both, and requires the normalized output to match.

Build the plugin binary first:

```bash
cd /data/projects/doltlite-gascity/rigs/beads-backend-doltlite-plugin
OUT=/tmp/bd-backend-doltlite-conformance ./scripts/build.sh
```

Then run the DoltLite plugin profile from the Beads checkout:

```bash
cd /data/projects/doltlite-gascity/beads-doltlite
BEADS_DOLTLITE_PLUGIN_COMMAND=/tmp/bd-backend-doltlite-conformance \
BEADS_DOLTLITE_PLUGIN_ARGS=serve \
BEADS_CONFORMANCE_PROFILES=doltlite-plugin \
CGO_ENABLED=1 \
go test -tags 'gms_pure_go e2e' ./test/conformance/ -timeout 90m -count=1
```

The full profile takes about 15 minutes on the current development machine. Use
a focused Go subtest run while iterating on a known failure, then run the full
profile before publishing:

```bash
BEADS_DOLTLITE_PLUGIN_COMMAND=/tmp/bd-backend-doltlite-conformance \
BEADS_DOLTLITE_PLUGIN_ARGS=serve \
BEADS_CONFORMANCE_PROFILES=doltlite-plugin \
CGO_ENABLED=1 \
go test -tags 'gms_pure_go e2e' ./test/conformance/ \
  -run 'TestConformanceE2E/promote|TestExportImportRoundTripE2E/doltlite-plugin' \
  -timeout 20m -count=1
```

This path exercises the public backend-plugin process contract. It catches
differences in CLI behavior, `bd backend install`, `.beads/metadata.json`,
transport/protocol handling, SQL dialect behavior, dependency persistence, and
export/import round-tripping.
