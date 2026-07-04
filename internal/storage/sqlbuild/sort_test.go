package sqlbuild

import (
	"strings"
	"testing"
)

func TestOrderByDialectNormalizesSQLiteCreatedAt(t *testing.T) {
	got := OrderByDialect("created", false, "i", CountsDialectSQLite)
	want := "ORDER BY replace(substr(i.created_at, 1, 19), 'T', ' ') DESC, i.id ASC"
	if got != want {
		t.Fatalf("OrderByDialect SQLite created = %q, want %q", got, want)
	}
}

func TestOrderByDialectKeepsDoltCreatedAtRaw(t *testing.T) {
	got := OrderByDialect("created", false, "i", CountsDialectDolt)
	if strings.Contains(got, "replace(substr(") {
		t.Fatalf("Dolt order should not use SQLite timestamp normalization: %q", got)
	}
	want := "ORDER BY i.created_at DESC, i.id ASC"
	if got != want {
		t.Fatalf("OrderByDialect Dolt created = %q, want %q", got, want)
	}
}

func TestOrderByDialectNormalizesPriorityCreatedTieBreaker(t *testing.T) {
	got := OrderByDialect("priority", false, "i", CountsDialectSQLite)
	want := "ORDER BY i.priority ASC, replace(substr(i.created_at, 1, 19), 'T', ' ') DESC, i.id ASC"
	if got != want {
		t.Fatalf("OrderByDialect SQLite priority = %q, want %q", got, want)
	}
}

func TestReadyWorkOrderDialectNormalizesSQLiteCreatedAt(t *testing.T) {
	got := BuildReadyWorkOrderDialect("priority", "created_at", "priority", CountsDialectSQLite)
	want := "ORDER BY priority ASC, replace(substr(created_at, 1, 19), 'T', ' ') DESC, id ASC"
	if got.SQL != want {
		t.Fatalf("BuildReadyWorkOrderDialect SQLite priority = %q, want %q", got.SQL, want)
	}
}
