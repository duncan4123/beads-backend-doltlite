//go:build cgo

package doltlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/duncan4123/beads-backend-doltlite/internal/storage/doltlite"
	"github.com/duncan4123/beads-backend-doltlite/internal/types"
)

func TestCreateIssuePersistsEmbeddedDependencies(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ephemeral bool
		noHistory bool
	}{
		{name: "durable issue"},
		{name: "no-history wisp", noHistory: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store, err := doltlite.New(ctx, filepath.Join(t.TempDir(), ".beads"), "beads", "main")
			if err != nil {
				skipIfUnlinkedDoltLite(t, err)
				t.Fatalf("open DoltLite store: %v", err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Fatalf("close DoltLite store: %v", err)
				}
			})
			if err := store.SetConfig(ctx, "issue_prefix", "dep"); err != nil {
				t.Fatalf("set issue prefix: %v", err)
			}
			if err := store.Commit(ctx, "bd init"); err != nil {
				t.Fatalf("commit init: %v", err)
			}

			blocker := &types.Issue{
				ID:        "dep-blocker",
				Title:     "Blocker",
				Status:    types.StatusOpen,
				IssueType: types.TypeTask,
				Ephemeral: tc.ephemeral,
				NoHistory: tc.noHistory,
			}
			if err := store.CreateIssue(ctx, blocker, "tester"); err != nil {
				t.Fatalf("create blocker: %v", err)
			}

			dependent := &types.Issue{
				ID:        "dep-dependent",
				Title:     "Dependent",
				Status:    types.StatusOpen,
				IssueType: types.TypeTask,
				Ephemeral: tc.ephemeral,
				NoHistory: tc.noHistory,
				Dependencies: []*types.Dependency{{
					IssueID:     "dep-dependent",
					DependsOnID: "dep-blocker",
					Type:        types.DepBlocks,
				}},
			}
			if err := store.CreateIssue(ctx, dependent, "tester"); err != nil {
				t.Fatalf("create dependent: %v", err)
			}

			deps, err := store.GetDependenciesWithMetadata(ctx, dependent.ID)
			if err != nil {
				t.Fatalf("get dependencies: %v", err)
			}
			if len(deps) != 1 {
				t.Fatalf("dependency count = %d, want 1", len(deps))
			}
			if deps[0].ID != blocker.ID || deps[0].DependencyType != types.DepBlocks {
				t.Fatalf("dependency = %s/%s, want %s/%s", deps[0].ID, deps[0].DependencyType, blocker.ID, types.DepBlocks)
			}
		})
	}
}
