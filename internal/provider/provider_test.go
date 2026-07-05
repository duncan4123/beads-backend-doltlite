package provider

import (
	"context"
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
