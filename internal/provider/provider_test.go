package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackendCapabilitiesAdvertiseVersionControlAndRemotes(t *testing.T) {
	caps := BackendCapabilities()

	if !caps.Versioning {
		t.Fatal("Versioning = false, want true")
	}
	if !caps.Branching {
		t.Fatal("Branching = false, want true")
	}
	if !caps.DoltRemotes {
		t.Fatal("DoltRemotes = false, want true")
	}
}

func TestManagerOpenReusesCachedStore(t *testing.T) {
	ctx := context.Background()
	manager := NewManager()
	beadsDir := filepath.Join(t.TempDir(), ".beads")

	initSession, err := manager.Init(ctx, beadsDir, "beads", "main", "ts", "tester")
	skipIfDoltLiteUnavailable(t, err)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := manager.Close(initSession.ID); err != nil {
		t.Fatalf("close init session: %v", err)
	}

	first, err := manager.Open(ctx, beadsDir, "beads", "main")
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	second, err := manager.Open(ctx, beadsDir, "beads", "main")
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	if first.Store != second.Store {
		t.Fatal("same store key opened two DoltLite stores")
	}
	if first.storeRef == nil || second.storeRef == nil {
		t.Fatal("cached sessions missing store reference")
	}

	if err := manager.Close(second.ID); err != nil {
		t.Fatalf("close second session: %v", err)
	}
	if err := manager.Close(first.ID); err != nil {
		t.Fatalf("close first session: %v", err)
	}
	if err := manager.CloseAll(); err != nil {
		t.Fatalf("close all: %v", err)
	}
}

func TestManagerOpenRepairsMissingIssuePrefixFromConfigYAML(t *testing.T) {
	ctx := context.Background()
	manager := NewManager()
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte("issue-prefix: cfg\n"), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	initSession, err := manager.Init(ctx, beadsDir, "beads", "main", "old", "tester")
	skipIfDoltLiteUnavailable(t, err)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := initSession.DeleteConfig(ctx, "issue_prefix"); err != nil {
		t.Fatalf("delete issue_prefix: %v", err)
	}
	if err := initSession.Commit(ctx, "remove issue_prefix for repair test"); err != nil {
		t.Fatalf("commit delete: %v", err)
	}
	if err := manager.Close(initSession.ID); err != nil {
		t.Fatalf("close init session: %v", err)
	}

	repaired, err := manager.Open(ctx, beadsDir, "beads", "main")
	if err != nil {
		t.Fatalf("open repaired store: %v", err)
	}
	got, err := repaired.GetConfig(ctx, "issue_prefix")
	if err != nil {
		t.Fatalf("get repaired issue_prefix: %v", err)
	}
	if got != "cfg" {
		t.Fatalf("issue_prefix = %q, want cfg", got)
	}
	if err := manager.CloseAll(); err != nil {
		t.Fatalf("close all: %v", err)
	}
}

func TestManagerOpenRepairsMissingCustomTypesFromConfigYAML(t *testing.T) {
	ctx := context.Background()
	manager := NewManager()
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte("issue-prefix: cfg\ntypes.custom: spec,convergence,step\n"), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	initSession, err := manager.Init(ctx, beadsDir, "beads", "main", "cfg", "tester")
	skipIfDoltLiteUnavailable(t, err)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := initSession.DeleteConfig(ctx, "types.custom"); err != nil {
		t.Fatalf("delete types.custom: %v", err)
	}
	if err := initSession.Commit(ctx, "remove types.custom for repair test"); err != nil {
		t.Fatalf("commit delete: %v", err)
	}
	if err := manager.Close(initSession.ID); err != nil {
		t.Fatalf("close init session: %v", err)
	}

	repaired, err := manager.Open(ctx, beadsDir, "beads", "main")
	if err != nil {
		t.Fatalf("open repaired store: %v", err)
	}
	got, err := repaired.GetConfig(ctx, "types.custom")
	if err != nil {
		t.Fatalf("get repaired types.custom: %v", err)
	}
	if got != "spec,convergence,step" {
		t.Fatalf("types.custom = %q, want spec,convergence,step", got)
	}
	customTypes, err := repaired.GetCustomTypes(ctx)
	if err != nil {
		t.Fatalf("get custom types: %v", err)
	}
	for _, want := range []string{"spec", "convergence", "step"} {
		if !containsString(customTypes, want) {
			t.Fatalf("custom types = %v, missing %q", customTypes, want)
		}
	}
	if err := manager.CloseAll(); err != nil {
		t.Fatalf("close all: %v", err)
	}
}

func TestManagerOpenRepairsNestedCustomTypesFromConfigYAML(t *testing.T) {
	ctx := context.Background()
	manager := NewManager()
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads dir: %v", err)
	}
	configYAML := []byte(`issue-prefix: cfg
types:
  custom:
    - spec
    - convergence
    - step
`)
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), configYAML, 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	initSession, err := manager.Init(ctx, beadsDir, "beads", "main", "cfg", "tester")
	skipIfDoltLiteUnavailable(t, err)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := initSession.DeleteConfig(ctx, "types.custom"); err != nil {
		t.Fatalf("delete types.custom: %v", err)
	}
	if err := initSession.Commit(ctx, "remove types.custom for nested repair test"); err != nil {
		t.Fatalf("commit delete: %v", err)
	}
	if err := manager.Close(initSession.ID); err != nil {
		t.Fatalf("close init session: %v", err)
	}

	repaired, err := manager.Open(ctx, beadsDir, "beads", "main")
	if err != nil {
		t.Fatalf("open repaired store: %v", err)
	}
	got, err := repaired.GetConfig(ctx, "types.custom")
	if err != nil {
		t.Fatalf("get repaired types.custom: %v", err)
	}
	if got != "spec,convergence,step" {
		t.Fatalf("types.custom = %q, want spec,convergence,step", got)
	}
	customTypes, err := repaired.GetCustomTypes(ctx)
	if err != nil {
		t.Fatalf("get custom types: %v", err)
	}
	for _, want := range []string{"spec", "convergence", "step"} {
		if !containsString(customTypes, want) {
			t.Fatalf("custom types = %v, missing %q", customTypes, want)
		}
	}
	if err := manager.CloseAll(); err != nil {
		t.Fatalf("close all: %v", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func skipIfDoltLiteUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "requires CGO") || strings.Contains(msg, "no such function: dolt_") {
		t.Skipf("DoltLite unavailable: %v", err)
	}
}
