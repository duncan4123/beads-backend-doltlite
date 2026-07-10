package gcbackend

import (
	"encoding/json"

	"github.com/duncan4123/beads-backend-doltlite/internal/types"
)

const ProtocolVersion = "gascity.backend.v1alpha1"

type Capabilities struct {
	GetIssue         bool `json:"get_issue"`
	SearchIssues     bool `json:"search_issues"`
	ReadyWork        bool `json:"ready_work"`
	ListWisps        bool `json:"list_wisps"`
	CountIssues      bool `json:"count_issues"`
	Labels           bool `json:"labels"`
	Dependencies     bool `json:"dependencies"`
	StorageCreate    bool `json:"storage_create"`
	ConditionalClaim bool `json:"conditional_claim"`
	BatchDeps        bool `json:"batch_deps"`
	WriteOperations  bool `json:"write_operations"`
}

func DefaultCapabilities() Capabilities {
	return Capabilities{
		GetIssue:         true,
		SearchIssues:     true,
		ReadyWork:        true,
		ListWisps:        true,
		CountIssues:      true,
		Labels:           true,
		Dependencies:     true,
		StorageCreate:    true,
		ConditionalClaim: true,
		BatchDeps:        true,
		WriteOperations:  true,
	}
}

type Hello struct {
	Protocol     string       `json:"protocol"`
	Backend      string       `json:"backend"`
	Capabilities Capabilities `json:"capabilities"`
}

type Request struct {
	ID     string          `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	ID     string          `json:"id,omitempty"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type OpenParams struct {
	BeadsDir string `json:"beads_dir"`
	Database string `json:"database,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

type OpenResult struct {
	SessionID string `json:"session_id"`
}

type SessionParams struct {
	SessionID string `json:"session_id"`
}

type GetIssueParams struct {
	SessionID string `json:"session_id"`
	ID        string `json:"id"`
}

type CreateIssueParams struct {
	SessionID string       `json:"session_id"`
	Issue     *types.Issue `json:"issue"`
	Actor     string       `json:"actor,omitempty"`
	Commit    bool         `json:"commit,omitempty"`
	Message   string       `json:"message,omitempty"`
}

type UpdateIssueParams struct {
	SessionID    string                 `json:"session_id"`
	ID           string                 `json:"id"`
	Updates      map[string]interface{} `json:"updates"`
	AddLabels    []string               `json:"add_labels,omitempty"`
	RemoveLabels []string               `json:"remove_labels,omitempty"`
	ParentID     *string                `json:"parent_id,omitempty"`
	Actor        string                 `json:"actor,omitempty"`
	Commit       bool                   `json:"commit,omitempty"`
	Message      string                 `json:"message,omitempty"`
}

type IssueActionParams struct {
	SessionID string `json:"session_id"`
	ID        string `json:"id"`
	Reason    string `json:"reason,omitempty"`
	Actor     string `json:"actor,omitempty"`
	Session   string `json:"session,omitempty"`
}

type SearchIssuesParams struct {
	SessionID string            `json:"session_id"`
	Query     string            `json:"query,omitempty"`
	Filter    types.IssueFilter `json:"filter,omitempty"`
}

type ReadyWorkParams struct {
	SessionID string           `json:"session_id"`
	Filter    types.WorkFilter `json:"filter,omitempty"`
}

type ListWispsParams struct {
	SessionID string           `json:"session_id"`
	Filter    types.WispFilter `json:"filter,omitempty"`
}

type CountIssuesParams struct {
	SessionID string            `json:"session_id"`
	Query     string            `json:"query,omitempty"`
	Filter    types.IssueFilter `json:"filter,omitempty"`
}

type LabelParams struct {
	SessionID string `json:"session_id"`
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	Actor     string `json:"actor,omitempty"`
	Commit    bool   `json:"commit,omitempty"`
	Message   string `json:"message,omitempty"`
}

type DependencyParams struct {
	SessionID   string            `json:"session_id"`
	Dependency  *types.Dependency `json:"dependency,omitempty"`
	IssueID     string            `json:"issue_id,omitempty"`
	DependsOnID string            `json:"depends_on_id,omitempty"`
	Actor       string            `json:"actor,omitempty"`
}

type MetadataSlotParams struct {
	SessionID string `json:"session_id"`
	IssueID   string `json:"issue_id"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	Actor     string `json:"actor,omitempty"`
}
