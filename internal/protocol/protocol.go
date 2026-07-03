package protocol

import (
	"encoding/json"

	"github.com/steveyegge/beads/internal/types"
	"github.com/steveyegge/beads/plugins/backend/doltlite/internal/provider"
)

const Version = "beads.backend.v1alpha1"

type Hello struct {
	Protocol     string                `json:"protocol"`
	Backend      string                `json:"backend"`
	Capabilities provider.Capabilities `json:"capabilities"`
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

type InitParams struct {
	BeadsDir string `json:"beads_dir"`
	Database string `json:"database,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Actor    string `json:"actor,omitempty"`
}

type ConfigParams struct {
	SessionID string `json:"session_id"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
}

type CreateIssueParams struct {
	SessionID string       `json:"session_id"`
	Issue     *types.Issue `json:"issue"`
	Actor     string       `json:"actor,omitempty"`
	Commit    bool         `json:"commit,omitempty"`
	Message   string       `json:"message,omitempty"`
}

type IssueIDParams struct {
	SessionID string `json:"session_id"`
	ID        string `json:"id"`
}

type SearchIssuesParams struct {
	SessionID string            `json:"session_id"`
	Query     string            `json:"query,omitempty"`
	Filter    types.IssueFilter `json:"filter,omitempty"`
}

type UpdateIssueParams struct {
	SessionID string                 `json:"session_id"`
	ID        string                 `json:"id"`
	Updates   map[string]interface{} `json:"updates"`
	Actor     string                 `json:"actor,omitempty"`
	Commit    bool                   `json:"commit,omitempty"`
	Message   string                 `json:"message,omitempty"`
}

type AddLabelParams struct {
	SessionID string `json:"session_id"`
	ID        string `json:"id"`
	Label     string `json:"label"`
	Actor     string `json:"actor,omitempty"`
	Commit    bool   `json:"commit,omitempty"`
	Message   string `json:"message,omitempty"`
}

type CommitParams struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}
