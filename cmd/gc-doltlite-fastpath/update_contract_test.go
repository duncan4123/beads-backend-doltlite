//go:build cgo && doltlite_integration

package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	backendplugin "github.com/duncan4123/beads-backend-doltlite/backend/plugin"
	"github.com/duncan4123/beads-backend-doltlite/internal/gcbackend"
	"github.com/duncan4123/beads-backend-doltlite/internal/provider"
)

func TestUpdateIssueAppliesGasCityLabelDeltas(t *testing.T) {
	ctx := context.Background()
	manager := provider.NewManager()
	session, err := manager.Init(ctx, filepath.Join(t.TempDir(), ".beads"), "audit", "main", "audit", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(session.ID) })

	created, err := session.CreateIssue(ctx, &backendplugin.Issue{ID: "audit-1", Title: "before"}, "test", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.AddLabel(ctx, created.ID, "keep", "test", false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := session.AddLabel(ctx, created.ID, "remove", "test", false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := session.CreateIssue(ctx, &backendplugin.Issue{ID: "audit-parent", Title: "parent"}, "test", false, ""); err != nil {
		t.Fatal(err)
	}
	parentID := "audit-parent"

	params, err := json.Marshal(gcbackend.UpdateIssueParams{
		SessionID:    session.ID,
		ID:           created.ID,
		Updates:      map[string]any{"title": "after"},
		AddLabels:    []string{"added", "keep"},
		RemoveLabels: []string{"remove"},
		ParentID:     &parentID,
		Actor:        "gc",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := handle(ctx, manager, gcbackend.Request{ID: "1", Method: "update_issue", Params: params})
	if !resp.OK {
		t.Fatalf("update response: %#v", resp)
	}

	got, err := session.GetIssue(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "after" {
		t.Fatalf("title = %q, want after", got.Title)
	}
	labels, err := session.GetLabels(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"keep": true, "added": true}
	if len(labels) != len(want) {
		t.Fatalf("labels = %v, want keep and added", labels)
	}
	for _, label := range labels {
		if !want[label] {
			t.Fatalf("unexpected label %q in %v", label, labels)
		}
	}
	deps, err := session.GetDependencyRecords(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].DependsOnID != parentID || deps[0].Type != "parent-child" {
		t.Fatalf("dependencies = %#v, want parent-child to %s", deps, parentID)
	}
}
