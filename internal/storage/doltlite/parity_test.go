//go:build cgo

package doltlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/duncan4123/beads-backend-doltlite/internal/provider"
	"github.com/duncan4123/beads-backend-doltlite/internal/storage/doltlite"
	"github.com/duncan4123/beads-backend-doltlite/internal/types"
)

type timestampParityBackend struct {
	name          string
	alreadySeeded bool
	db            func() *sql.DB
	create        func(context.Context, *types.Issue) error
	search        func(context.Context, types.IssueFilter) ([]*types.Issue, error)
	ready         func(context.Context, types.WorkFilter) ([]*types.Issue, error)
	close         func() error
}

func TestDoltLiteTimestampOrderingParity(t *testing.T) {
	for _, open := range []func(*testing.T) timestampParityBackend{
		openDirectDoltLiteParityBackend,
		openProviderSessionParityBackend,
		openRemoteServerDoltLiteParityBackend,
	} {
		backend := open(t)
		t.Run(backend.name, func(t *testing.T) {
			t.Cleanup(func() {
				if err := backend.close(); err != nil {
					t.Fatalf("close backend: %v", err)
				}
			})
			runTimestampOrderingParity(t, backend)
		})
	}
}

func openRemoteServerDoltLiteParityBackend(t *testing.T) timestampParityBackend {
	t.Helper()
	ctx := context.Background()
	remoteDir := t.TempDir()
	serverURL, stopServer := startDoltLiteRemoteServer(t, remoteDir)

	source, err := doltlite.New(ctx, filepath.Join(t.TempDir(), ".beads"), "beads", "main")
	if err != nil {
		stopServer()
		skipIfUnlinkedDoltLite(t, err)
		t.Fatalf("open source DoltLite store: %v", err)
	}
	if err := source.SetConfig(ctx, "issue_prefix", "ts"); err != nil {
		_ = source.Close()
		stopServer()
		t.Fatalf("set source issue_prefix: %v", err)
	}
	if err := source.Commit(ctx, "bd init"); err != nil {
		_ = source.Close()
		stopServer()
		t.Fatalf("commit source init: %v", err)
	}
	sourceBackend := timestampParityBackend{
		name: "remote-source",
		db:   source.DB,
		create: func(ctx context.Context, issue *types.Issue) error {
			return source.CreateIssue(ctx, issue, "tester")
		},
		search: func(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
			return source.SearchIssues(ctx, "", filter)
		},
		ready: source.GetReadyWork,
		close: source.Close,
	}
	seedTimestampFixtures(t, sourceBackend)
	if err := source.Commit(ctx, "seed timestamp parity fixtures"); err != nil {
		_ = source.Close()
		stopServer()
		t.Fatalf("commit source fixtures: %v", err)
	}
	if err := source.AddRemote(ctx, "origin", serverURL+"/beads.db"); err != nil {
		_ = source.Close()
		stopServer()
		t.Fatalf("add source remote: %v", err)
	}
	if err := source.PushRemote(ctx, "origin", true); err != nil {
		_ = source.Close()
		stopServer()
		t.Fatalf("push source remote: %v", err)
	}
	if err := source.Close(); err != nil {
		stopServer()
		t.Fatalf("close source store: %v", err)
	}

	replica, err := doltlite.New(ctx, filepath.Join(t.TempDir(), ".beads"), "beads", "main")
	if err != nil {
		stopServer()
		skipIfUnlinkedDoltLite(t, err)
		t.Fatalf("open replica DoltLite store: %v", err)
	}
	if err := replica.AddRemote(ctx, "origin", serverURL+"/beads.db"); err != nil {
		_ = replica.Close()
		stopServer()
		t.Fatalf("add replica remote: %v", err)
	}
	if err := replica.Fetch(ctx, "origin"); err != nil {
		_ = replica.Close()
		stopServer()
		t.Fatalf("fetch replica remote: %v", err)
	}
	if _, err := replica.DB().ExecContext(ctx, "SELECT dolt_reset('--hard', 'origin/main')"); err != nil {
		_ = replica.Close()
		stopServer()
		t.Fatalf("reset replica to remote: %v", err)
	}

	return timestampParityBackend{
		name:          "doltlite-remotesrv-replica",
		alreadySeeded: true,
		db:            replica.DB,
		create: func(context.Context, *types.Issue) error {
			return errors.New("remote replica backend is already seeded")
		},
		search: func(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
			return replica.SearchIssues(ctx, "", filter)
		},
		ready: replica.GetReadyWork,
		close: func() error {
			return errors.Join(replica.Close(), stopServer())
		},
	}
}

func openDirectDoltLiteParityBackend(t *testing.T) timestampParityBackend {
	t.Helper()
	ctx := context.Background()
	store, err := doltlite.New(ctx, filepath.Join(t.TempDir(), ".beads"), "beads", "main")
	if err != nil {
		skipIfUnlinkedDoltLite(t, err)
		t.Fatalf("open direct DoltLite store: %v", err)
	}
	if err := store.SetConfig(ctx, "issue_prefix", "ts"); err != nil {
		t.Fatalf("set issue_prefix: %v", err)
	}
	if err := store.Commit(ctx, "bd init"); err != nil {
		t.Fatalf("commit init: %v", err)
	}
	return timestampParityBackend{
		name: "direct-doltlite",
		db:   store.DB,
		create: func(ctx context.Context, issue *types.Issue) error {
			return store.CreateIssue(ctx, issue, "tester")
		},
		search: func(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
			return store.SearchIssues(ctx, "", filter)
		},
		ready: store.GetReadyWork,
		close: store.Close,
	}
}

func openProviderSessionParityBackend(t *testing.T) timestampParityBackend {
	t.Helper()
	ctx := context.Background()
	manager := provider.NewManager()
	session, err := manager.Init(ctx, filepath.Join(t.TempDir(), ".beads"), "beads", "main", "ts", "tester")
	if err != nil {
		skipIfUnlinkedDoltLite(t, err)
		t.Fatalf("open provider session: %v", err)
	}
	return timestampParityBackend{
		name: "provider-session",
		db:   session.Store.DB,
		create: func(ctx context.Context, issue *types.Issue) error {
			_, err := session.CreateIssue(ctx, issue, "tester", false, "")
			return err
		},
		search: func(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
			return session.SearchIssues(ctx, "", filter)
		},
		ready: session.ReadyWork,
		close: manager.CloseAll,
	}
}

func skipIfUnlinkedDoltLite(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "no such function: dolt_commit") {
		t.Skipf("libdoltlite SQL functions are not linked into the sqlite driver: %v", err)
	}
}

func runTimestampOrderingParity(t *testing.T, backend timestampParityBackend) {
	t.Helper()
	if !backend.alreadySeeded {
		seedTimestampFixtures(t, backend)
	}
	assertTimestampOrderingParity(t, backend)
}

func seedTimestampFixtures(t *testing.T, backend timestampParityBackend) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	fixtures := []struct {
		id        string
		title     string
		createdAt time.Time
		raw       string
	}{
		{
			id:        "ts-old",
			title:     "oldest RFC3339 text sorts high lexically",
			createdAt: base,
			raw:       "2026-06-07T12:00:00Z",
		},
		{
			id:        "ts-mid",
			title:     "middle Go zone text",
			createdAt: base.Add(5 * time.Minute),
			raw:       "2026-06-07 12:05:00.000000000 +0000 UTC",
		},
		{
			id:        "ts-new",
			title:     "newest SQLite offset text",
			createdAt: base.Add(10 * time.Minute),
			raw:       "2026-06-07 12:10:00.000000000+00:00",
		},
	}
	for _, fixture := range fixtures {
		issue := &types.Issue{
			ID:        fixture.id,
			Title:     fixture.title,
			Status:    types.StatusOpen,
			IssueType: types.TypeTask,
			Priority:  1,
			CreatedAt: fixture.createdAt,
			UpdatedAt: fixture.createdAt,
			CreatedBy: "tester",
		}
		if err := backend.create(ctx, issue); err != nil {
			t.Fatalf("create %s: %v", fixture.id, err)
		}
		if _, err := backend.db().ExecContext(ctx, "UPDATE issues SET created_at = ?, updated_at = ? WHERE id = ?", fixture.raw, fixture.raw, fixture.id); err != nil {
			t.Fatalf("inject raw timestamp for %s: %v", fixture.id, err)
		}
	}
}

func assertTimestampOrderingParity(t *testing.T, backend timestampParityBackend) {
	t.Helper()
	assertIssueIDs(t, "search created limit", searchIDs(t, backend, types.IssueFilter{
		SortBy: "created",
		Limit:  2,
	}), []string{"ts-new", "ts-mid"})

	assertIssueIDs(t, "search priority created tie-breaker", searchIDs(t, backend, types.IssueFilter{
		SortBy: "priority",
		Limit:  2,
	}), []string{"ts-new", "ts-mid"})

	assertIssueIDs(t, "ready priority created tie-breaker", readyIDs(t, backend, types.WorkFilter{
		SortPolicy: types.SortPolicyPriority,
		Limit:      2,
	}), []string{"ts-new", "ts-mid"})
}

func searchIDs(t *testing.T, backend timestampParityBackend, filter types.IssueFilter) []string {
	t.Helper()
	issues, err := backend.search(context.Background(), filter)
	if err != nil {
		t.Fatalf("search %s: %v", backend.name, err)
	}
	return issueIDs(issues)
}

func readyIDs(t *testing.T, backend timestampParityBackend, filter types.WorkFilter) []string {
	t.Helper()
	issues, err := backend.ready(context.Background(), filter)
	if err != nil {
		t.Fatalf("ready %s: %v", backend.name, err)
	}
	return issueIDs(issues)
}

func issueIDs(issues []*types.Issue) []string {
	ids := make([]string, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}
	return ids
}

func assertIssueIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func startDoltLiteRemoteServer(t *testing.T, dir string) (string, func() error) {
	t.Helper()
	bin := os.Getenv("DOLTLITE_REMOTESRV")
	if bin == "" {
		candidates := []string{
			"/data/projects/doltlite-gascity/doltlite/doltlite-remotesrv",
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				bin = candidate
				break
			}
		}
	}
	if bin == "" {
		var err error
		bin, err = exec.LookPath("doltlite-remotesrv")
		if err != nil {
			t.Skip("doltlite-remotesrv not found; set DOLTLITE_REMOTESRV to run HTTP remote parity test")
		}
	}

	port := freeTCPPort(t)
	cmd := exec.Command(bin, "-p", fmt.Sprintf("%d", port), "--bind", "127.0.0.1", dir)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start doltlite-remotesrv: %v", err)
	}
	stop := func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil
	}
	waitForTCP(t, port, stop)
	return fmt.Sprintf("http://127.0.0.1:%d", port), stop
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate TCP port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForTCP(t *testing.T, port int, stop func() error) {
	t.Helper()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = stop()
	t.Fatalf("doltlite-remotesrv did not listen on %s", addr)
}
