package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	backenddoltlite "github.com/steveyegge/beads/backend/doltlite"
	backendplugin "github.com/steveyegge/beads/backend/plugin"
)

const Name = "doltlite"

type Capabilities = backendplugin.Capabilities

type Diagnostic = backendplugin.Diagnostic

func BackendCapabilities() Capabilities {
	return Capabilities{
		Embedded:          true,
		Transactions:      true,
		RawSQL:            true,
		Leases:            true,
		Maintenance:       true,
		Versioning:        true,
		Branching:         false,
		DoltRemotes:       false,
		ConcurrentWriters: true,
	}
}

func Doctor() []Diagnostic {
	return []Diagnostic{
		{
			Level:   "info",
			Code:    "protocol",
			Message: "DoltLite backend plugin process protocol is available.",
		},
	}
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

type Session struct {
	ID       string
	BeadsDir string
	Database string
	Branch   string
	Store    *backenddoltlite.Store
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func (m *Manager) Init(ctx context.Context, beadsDir, database, branch, prefix, actor string) (*Session, error) {
	if prefix = strings.TrimSpace(prefix); prefix == "" {
		prefix = "bd"
	}
	if actor = strings.TrimSpace(actor); actor == "" {
		actor = "bd-backend-doltlite"
	}
	s, err := m.Open(ctx, beadsDir, database, branch)
	if err != nil {
		return nil, err
	}
	if err := s.Store.SetConfig(ctx, "issue_prefix", prefix); err != nil {
		_ = m.Close(s.ID)
		return nil, fmt.Errorf("set issue_prefix: %w", err)
	}
	if err := s.Store.Commit(ctx, "bd init"); err != nil {
		_ = m.Close(s.ID)
		return nil, fmt.Errorf("commit init: %w", err)
	}
	return s, nil
}

func (m *Manager) Open(ctx context.Context, beadsDir, database, branch string) (*Session, error) {
	if strings.TrimSpace(beadsDir) == "" {
		return nil, errors.New("beads_dir is required")
	}
	if strings.TrimSpace(database) == "" {
		database = "beads"
	}
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	absBeadsDir, err := filepath.Abs(beadsDir)
	if err != nil {
		return nil, err
	}
	store, err := backenddoltlite.New(ctx, absBeadsDir, database, branch)
	if err != nil {
		return nil, err
	}
	s := &Session{
		ID:       newSessionID(),
		BeadsDir: absBeadsDir,
		Database: database,
		Branch:   branch,
		Store:    store,
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) Get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("unknown session: %s", id)
	}
	return s, nil
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown session: %s", id)
	}
	return s.Store.Close()
}

func (m *Manager) CloseAll() error {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for id, s := range m.sessions {
		delete(m.sessions, id)
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	var err error
	for _, s := range sessions {
		if closeErr := s.Store.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return err
}

func (s *Session) SetConfig(ctx context.Context, key, value string) error {
	return s.Store.SetConfig(ctx, key, value)
}

func (s *Session) GetConfig(ctx context.Context, key string) (string, error) {
	return s.Store.GetConfig(ctx, key)
}

func (s *Session) CreateIssue(ctx context.Context, issue *backendplugin.Issue, actor string, commit bool, message string) (*backendplugin.Issue, error) {
	if issue == nil {
		return nil, errors.New("issue is required")
	}
	withDefaults(issue)
	if actor == "" {
		actor = "bd-backend-doltlite"
	}
	if err := s.Store.CreateIssue(ctx, issue, actor); err != nil {
		return nil, err
	}
	if commit {
		if message == "" {
			message = "create issue " + issue.ID
		}
		if err := s.Store.Commit(ctx, message); err != nil {
			return nil, err
		}
	}
	return s.Store.GetIssue(ctx, issue.ID)
}

func (s *Session) GetIssue(ctx context.Context, id string) (*backendplugin.Issue, error) {
	return s.Store.GetIssue(ctx, id)
}

func (s *Session) SearchIssues(ctx context.Context, query string, filter backendplugin.IssueFilter) ([]*backendplugin.Issue, error) {
	return s.Store.SearchIssues(ctx, query, filter)
}

func (s *Session) UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string, commit bool, message string) (*backendplugin.Issue, error) {
	if actor == "" {
		actor = "bd-backend-doltlite"
	}
	if err := s.Store.UpdateIssue(ctx, id, updates, actor); err != nil {
		return nil, err
	}
	if commit {
		if message == "" {
			message = "update issue " + id
		}
		if err := s.Store.Commit(ctx, message); err != nil {
			return nil, err
		}
	}
	return s.Store.GetIssue(ctx, id)
}

func (s *Session) AddLabel(ctx context.Context, id, label, actor string, commit bool, message string) ([]string, error) {
	if actor == "" {
		actor = "bd-backend-doltlite"
	}
	if err := s.Store.AddLabel(ctx, id, label, actor); err != nil {
		return nil, err
	}
	if commit {
		if message == "" {
			message = "add label " + label + " to " + id
		}
		if err := s.Store.Commit(ctx, message); err != nil {
			return nil, err
		}
	}
	return s.Store.GetLabels(ctx, id)
}

func (s *Session) GetLabels(ctx context.Context, id string) ([]string, error) {
	return s.Store.GetLabels(ctx, id)
}

func (s *Session) ReadyWork(ctx context.Context, filter backendplugin.WorkFilter) ([]*backendplugin.Issue, error) {
	return s.Store.GetReadyWork(ctx, filter)
}

func (s *Session) Commit(ctx context.Context, message string) error {
	return s.Store.Commit(ctx, message)
}

func withDefaults(issue *backendplugin.Issue) {
	if issue.Status == "" {
		issue.Status = backendplugin.StatusOpen
	}
	if issue.IssueType == "" {
		issue.IssueType = backendplugin.TypeTask
	}
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "session"
	}
	return hex.EncodeToString(b[:])
}
