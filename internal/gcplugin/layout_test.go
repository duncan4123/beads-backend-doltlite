package gcplugin

import (
	"path/filepath"
	"testing"
)

func TestResolveLayoutLedgerKeepsWorkflowStateInDoltlite(t *testing.T) {
	layout, err := ResolveLayout("/city", Metadata{
		Backend:              "doltlite",
		DoltDatabase:         "gascity",
		BackendPluginCommand: "/bin/bd-backend-doltlite",
		BackendPluginArgs:    []string{"serve"},
		AttachedDatabases: []AttachedDatabase{{
			Alias: "ops",
			Path:  ".gc/ops.sqlite",
		}},
	}, ProfileLedger)
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if got := layout.ResolvedTableName["wisps"]; got != "wisps" {
		t.Fatalf("wisps resolved table = %q, want wisps", got)
	}
	if got := layout.ResolvedTableName["repo_mtimes"]; got != "ops.repo_mtimes" {
		t.Fatalf("repo_mtimes resolved table = %q, want ops.repo_mtimes", got)
	}
	if got := layout.OpsPath; got != filepath.Clean("/city/.gc/ops.sqlite") {
		t.Fatalf("OpsPath = %q", got)
	}
}

func TestResolveLayoutLocalRuntimeMovesWorkflowStateToSQLite(t *testing.T) {
	layout, err := ResolveLayout("/city", Metadata{}, ProfileLocalRuntime)
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	for _, table := range []string{"wisps", "wisp_labels", "wisp_dependencies", "wisp_events", "wisp_comments"} {
		if got := layout.ResolvedTableName[table]; got != "ops."+table {
			t.Fatalf("%s resolved table = %q, want ops.%s", table, got, table)
		}
	}
	if got := layout.ResolvedTableName["issues"]; got != "issues" {
		t.Fatalf("issues resolved table = %q, want issues", got)
	}
}

func TestResolveLayoutMirrorAddsMirrorTablesToDoltlite(t *testing.T) {
	layout, err := ResolveLayout("/city", Metadata{}, ProfileMirror)
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	for _, table := range mirrorTables {
		if got := layout.ResolvedTableName[table]; got != table {
			t.Fatalf("%s resolved table = %q, want %s", table, got, table)
		}
	}
	if len(layout.MirrorTables) != len(mirrorTables) {
		t.Fatalf("MirrorTables len = %d, want %d", len(layout.MirrorTables), len(mirrorTables))
	}
}

func TestHealthValidatesPluginMetadata(t *testing.T) {
	h := CheckHealth("/city", Metadata{
		Backend:              "doltlite",
		BackendPluginCommand: "/bin/bd-backend-doltlite",
		BackendPluginArgs:    []string{"--trace", "/tmp/plugin.jsonl", "serve"},
	}, "")
	if !h.OK {
		t.Fatalf("Health OK = false, errors = %v", h.Errors)
	}

	h = CheckHealth("/city", Metadata{Backend: "doltlite"}, "")
	if h.OK {
		t.Fatalf("Health OK = true, want false")
	}
	if len(h.Errors) != 2 {
		t.Fatalf("Health errors = %v, want missing command and serve arg", h.Errors)
	}
}

func TestMetadataPathForScope(t *testing.T) {
	if got := MetadataPathForScope("/city"); got != "/city/.beads/metadata.json" {
		t.Fatalf("MetadataPathForScope city = %q", got)
	}
	if got := MetadataPathForScope("/city/.beads/metadata.json"); got != "/city/.beads/metadata.json" {
		t.Fatalf("MetadataPathForScope metadata = %q", got)
	}
}
