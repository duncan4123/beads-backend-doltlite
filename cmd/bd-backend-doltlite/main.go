package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	backendplugin "github.com/steveyegge/beads/backend/plugin"

	"github.com/duncan4123/beads-backend-doltlite/internal/provider"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var payload any
	switch os.Args[1] {
	case "capabilities":
		payload = provider.BackendCapabilities()
	case "doctor":
		payload = provider.Doctor()
	case "serve":
		if err := serve(context.Background(), os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}
		return
	default:
		usage()
		os.Exit(2)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "encode response: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: bd-backend-doltlite <capabilities|doctor|serve>")
}

func serve(ctx context.Context, in *os.File, out *os.File) error {
	manager := provider.NewManager()
	defer func() { _ = manager.CloseAll() }()

	enc := json.NewEncoder(out)
	hello := backendplugin.Hello{
		Protocol:     backendplugin.ProtocolVersion,
		Backend:      provider.Name,
		Capabilities: provider.BackendCapabilities(),
	}
	if err := enc.Encode(backendplugin.Response{OK: true, Result: mustJSON(hello)}); err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var req backendplugin.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = enc.Encode(errorResponse("", "bad_request", err))
			continue
		}
		resp := handle(ctx, manager, req)
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func handle(ctx context.Context, manager *provider.Manager, req backendplugin.Request) backendplugin.Response {
	switch req.Method {
	case "capabilities":
		return ok(req.ID, provider.BackendCapabilities())
	case "doctor":
		return ok(req.ID, provider.Doctor())
	case "init":
		var p backendplugin.InitParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Init(ctx, p.BeadsDir, p.Database, p.Branch, p.Prefix, p.Actor)
		if err != nil {
			return errorResponse(req.ID, "init_failed", err)
		}
		return ok(req.ID, backendplugin.OpenResult{SessionID: s.ID})
	case "open":
		var p backendplugin.OpenParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Open(ctx, p.BeadsDir, p.Database, p.Branch)
		if err != nil {
			return errorResponse(req.ID, "open_failed", err)
		}
		return ok(req.ID, backendplugin.OpenResult{SessionID: s.ID})
	case "close":
		var p backendplugin.SessionParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		if err := manager.Close(p.SessionID); err != nil {
			return errorResponse(req.ID, "close_failed", err)
		}
		return ok(req.ID, map[string]bool{"closed": true})
	case "set_config":
		var p backendplugin.ConfigParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		if err := s.SetConfig(ctx, p.Key, p.Value); err != nil {
			return errorResponse(req.ID, "set_config_failed", err)
		}
		return ok(req.ID, map[string]string{"key": p.Key})
	case "get_config":
		var p backendplugin.ConfigParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		value, err := s.GetConfig(ctx, p.Key)
		if err != nil {
			return errorResponse(req.ID, "get_config_failed", err)
		}
		return ok(req.ID, map[string]string{"key": p.Key, "value": value})
	case "create_issue":
		var p backendplugin.CreateIssueParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		issue, err := s.CreateIssue(ctx, p.Issue, p.Actor, p.Commit, p.Message)
		if err != nil {
			return errorResponse(req.ID, "create_issue_failed", err)
		}
		return ok(req.ID, issue)
	case "get_issue":
		var p backendplugin.IssueIDParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		issue, err := s.GetIssue(ctx, p.ID)
		if err != nil {
			return errorResponse(req.ID, "get_issue_failed", err)
		}
		return ok(req.ID, issue)
	case "search_issues":
		var p backendplugin.SearchIssuesParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		issues, err := s.SearchIssues(ctx, p.Query, p.Filter)
		if err != nil {
			return errorResponse(req.ID, "search_issues_failed", err)
		}
		return ok(req.ID, issues)
	case "update_issue":
		var p backendplugin.UpdateIssueParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		issue, err := s.UpdateIssue(ctx, p.ID, p.Updates, p.Actor, p.Commit, p.Message)
		if err != nil {
			return errorResponse(req.ID, "update_issue_failed", err)
		}
		return ok(req.ID, issue)
	case "add_label":
		var p backendplugin.AddLabelParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		labels, err := s.AddLabel(ctx, p.ID, p.Label, p.Actor, p.Commit, p.Message)
		if err != nil {
			return errorResponse(req.ID, "add_label_failed", err)
		}
		return ok(req.ID, labels)
	case "get_labels":
		var p backendplugin.IssueIDParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		labels, err := s.GetLabels(ctx, p.ID)
		if err != nil {
			return errorResponse(req.ID, "get_labels_failed", err)
		}
		return ok(req.ID, labels)
	case "ready_work":
		var p struct {
			SessionID string                   `json:"session_id"`
			Filter    backendplugin.WorkFilter `json:"filter,omitempty"`
		}
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		issues, err := s.ReadyWork(ctx, p.Filter)
		if err != nil {
			return errorResponse(req.ID, "ready_work_failed", err)
		}
		return ok(req.ID, issues)
	case "commit":
		var p backendplugin.CommitParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_params", err)
		}
		s, err := manager.Get(p.SessionID)
		if err != nil {
			return errorResponse(req.ID, "unknown_session", err)
		}
		if err := s.Commit(ctx, p.Message); err != nil {
			return errorResponse(req.ID, "commit_failed", err)
		}
		return ok(req.ID, map[string]bool{"committed": true})
	default:
		return errorResponse(req.ID, "unknown_method", fmt.Errorf("%s", req.Method))
	}
}

func decode(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	return json.Unmarshal(raw, out)
}

func ok(id string, payload any) backendplugin.Response {
	return backendplugin.Response{ID: id, OK: true, Result: mustJSON(payload)}
}

func errorResponse(id, code string, err error) backendplugin.Response {
	return backendplugin.Response{
		ID: id,
		OK: false,
		Error: &backendplugin.Error{
			Code:    code,
			Message: err.Error(),
		},
	}
}

func mustJSON(payload any) json.RawMessage {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return data
}
