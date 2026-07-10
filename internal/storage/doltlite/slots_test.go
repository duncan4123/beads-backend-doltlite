//go:build cgo

package doltlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/duncan4123/beads-backend-doltlite/internal/types"
)

func TestSlotSetHandlesNullMetadata(t *testing.T) {
	ctx := context.Background()
	store, err := New(ctx, filepath.Join(t.TempDir(), ".beads"), "beads", "main")
	if err != nil {
		if strings.Contains(err.Error(), "no such function: dolt_commit") {
			t.Skipf("libdoltlite SQL functions are not linked: %v", err)
		}
		t.Fatalf("open DoltLite store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close DoltLite store: %v", err)
		}
	})
	if err := store.SetConfig(ctx, "issue_prefix", "bd"); err != nil {
		t.Fatalf("set issue_prefix: %v", err)
	}

	now := time.Now().UTC()
	issue := &types.Issue{
		ID:        "bd-null-slot",
		Title:     "null slot metadata",
		Status:    types.StatusOpen,
		Priority:  2,
		IssueType: types.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "test",
		Metadata:  json.RawMessage("null"),
	}
	if err := store.CreateIssue(ctx, issue, "test"); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	if err := store.SlotSet(ctx, issue.ID, "state", "active", "test"); err != nil {
		t.Fatalf("SlotSet: %v", err)
	}

	got, err := store.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal(got.Metadata, &metadata); err != nil {
		t.Fatalf("decode stored metadata: %v", err)
	}
	if got := metadata["state"]; got != "active" {
		t.Fatalf("state = %v, want active", got)
	}
}

func TestDecodeSlotMetadataNormalizesNull(t *testing.T) {
	metadata, err := decodeSlotMetadata(json.RawMessage("null"), "bd-null")
	if err != nil {
		t.Fatalf("decodeSlotMetadata: %v", err)
	}

	metadata["state"] = "active"
	if got := metadata["state"]; got != "active" {
		t.Fatalf("state = %v, want active", got)
	}
}

func TestDecodeSlotMetadataPreservesObject(t *testing.T) {
	metadata, err := decodeSlotMetadata(json.RawMessage(`{"existing":"value"}`), "bd-object")
	if err != nil {
		t.Fatalf("decodeSlotMetadata: %v", err)
	}
	if got := metadata["existing"]; got != "value" {
		t.Fatalf("existing = %v, want value", got)
	}
}

func TestDecodeSlotMetadataRejectsInvalidJSON(t *testing.T) {
	_, err := decodeSlotMetadata(json.RawMessage("{"), "bd-invalid")
	if err == nil {
		t.Fatal("decodeSlotMetadata error = nil, want invalid JSON error")
	}
	if !strings.Contains(err.Error(), "parsing metadata for bd-invalid") {
		t.Fatalf("decodeSlotMetadata error = %q, want issue context", err)
	}
}
