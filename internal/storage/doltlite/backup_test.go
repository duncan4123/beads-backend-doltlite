//go:build cgo

package doltlite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupDirRemoteURLMapsDirectoryToDatabaseRemote(t *testing.T) {
	dir := t.TempDir()
	store := &DoltliteStore{database: "beads"}

	got, err := store.backupDirRemoteURL(dir, "backup destination")
	if err != nil {
		t.Fatalf("backupDirRemoteURL() error = %v", err)
	}

	wantSuffix := "/" + filepath.Base(dir) + "/beads.db"
	if !strings.HasPrefix(got, "file://") {
		t.Fatalf("backupDirRemoteURL() = %q, want file:// URL", got)
	}
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("backupDirRemoteURL() = %q, want suffix %q", got, wantSuffix)
	}
}

func TestBackupDirRemoteURLRejectsMissingDirectory(t *testing.T) {
	store := &DoltliteStore{database: "beads"}

	_, err := store.backupDirRemoteURL(filepath.Join(t.TempDir(), "missing"), "backup source")
	if err == nil {
		t.Fatal("backupDirRemoteURL() error = nil, want missing directory error")
	}
	if !strings.Contains(err.Error(), "backup source does not exist") {
		t.Fatalf("backupDirRemoteURL() error = %q, want backup source wording", err)
	}
}

func TestBackupDirRemoteURLRejectsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	store := &DoltliteStore{database: "beads"}

	_, err := store.backupDirRemoteURL(file, "backup destination")
	if err == nil {
		t.Fatal("backupDirRemoteURL() error = nil, want not directory error")
	}
	if !strings.Contains(err.Error(), "backup destination is not a directory") {
		t.Fatalf("backupDirRemoteURL() error = %q, want not directory wording", err)
	}
}
