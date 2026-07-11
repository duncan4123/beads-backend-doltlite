package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/duncan4123/beads-backend-doltlite/internal/gcbackend"
	"github.com/duncan4123/beads-backend-doltlite/internal/provider"
	"github.com/duncan4123/beads-backend-doltlite/internal/storage"
	doltlitestorage "github.com/duncan4123/beads-backend-doltlite/internal/storage/doltlite"
)

func main() {
	opts, args, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		usage()
		os.Exit(2)
	}
	if len(args) != 1 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "capabilities":
		writeJSON(gcbackend.DefaultCapabilities())
	case "serve":
		if err := serve(context.Background(), os.Stdin, os.Stdout, opts); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gc-doltlite-fastpath [--trace=<path>|--trace-stderr] <capabilities|serve>")
}

type options struct {
	tracePath   string
	traceStderr bool
}

func parseOptions(args []string) (options, []string, error) {
	var opts options
	for len(args) > 0 {
		arg := args[0]
		switch {
		case arg == "--trace":
			if len(args) < 2 {
				return opts, args, fmt.Errorf("--trace requires a path")
			}
			opts.tracePath = args[1]
			args = args[2:]
		case strings.HasPrefix(arg, "--trace="):
			opts.tracePath = strings.TrimPrefix(arg, "--trace=")
			args = args[1:]
		case arg == "--trace-stderr":
			opts.traceStderr = true
			args = args[1:]
		case arg == "--no-trace":
			opts.tracePath = ""
			opts.traceStderr = false
			args = args[1:]
		case strings.HasPrefix(arg, "-"):
			return opts, args, fmt.Errorf("unknown option %q", arg)
		default:
			return opts, args, nil
		}
	}
	return opts, args, nil
}

func serve(ctx context.Context, in io.Reader, out io.Writer, opts options) error {
	manager := provider.NewManager()
	defer func() { _ = manager.CloseAll() }()

	tracer, err := newTracer(opts)
	if err != nil {
		return err
	}
	defer tracer.Close()

	enc := json.NewEncoder(out)
	hello := gcbackend.Hello{
		Protocol:     gcbackend.ProtocolVersion,
		Backend:      provider.Name,
		Capabilities: gcbackend.DefaultCapabilities(),
	}
	if err := enc.Encode(gcbackend.Response{OK: true, Result: mustJSON(hello)}); err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var req gcbackend.Request
		start := time.Now()
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			resp := errorResponse("", "bad_request", err)
			tracer.Log(traceEntryFromResponse(start, req, resp, doltlitestorage.TelemetrySnapshot{}))
			_ = enc.Encode(resp)
			continue
		}
		telemetry := doltlitestorage.NewTelemetry()
		reqCtx := doltlitestorage.ContextWithTelemetry(ctx, telemetry)
		resp := handle(reqCtx, manager, req)
		tracer.Log(traceEntryFromResponse(start, req, resp, telemetry.Snapshot()))
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

type tracer struct {
	out io.WriteCloser
	enc *json.Encoder
}

type traceEntry struct {
	Timestamp  string `json:"ts"`
	PID        int    `json:"pid"`
	Backend    string `json:"backend"`
	Protocol   string `json:"protocol"`
	RequestID  string `json:"request_id,omitempty"`
	Method     string `json:"method,omitempty"`
	Params     any    `json:"params,omitempty"`
	OK         bool   `json:"ok"`
	ErrorCode  string `json:"error_code,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`

	RetryCount      *int             `json:"retry_count,omitempty"`
	RetryErrorClass string           `json:"retry_error_class,omitempty"`
	RetryError      string           `json:"retry_error,omitempty"`
	LockWaitCount   *int             `json:"lock_wait_count,omitempty"`
	LockWaitMS      *int64           `json:"lock_wait_ms,omitempty"`
	MaxLockWaitMS   *int64           `json:"max_lock_wait_ms,omitempty"`
	PhaseMS         map[string]int64 `json:"phase_ms,omitempty"`
}

func newTracer(opts options) (*tracer, error) {
	if env := os.Getenv("GC_DOLTLITE_FASTPATH_TRACE"); env != "" {
		switch strings.ToLower(env) {
		case "0", "false", "off", "none":
			opts.tracePath = ""
			opts.traceStderr = false
		case "stderr":
			opts.tracePath = ""
			opts.traceStderr = true
		default:
			opts.tracePath = env
			opts.traceStderr = false
		}
	}
	if opts.traceStderr {
		return &tracer{out: nopWriteCloser{os.Stderr}, enc: json.NewEncoder(os.Stderr)}, nil
	}
	if opts.tracePath == "" {
		return &tracer{}, nil
	}
	if err := os.MkdirAll(filepath.Dir(opts.tracePath), 0o755); err != nil {
		return nil, fmt.Errorf("create trace dir: %w", err)
	}
	f, err := os.OpenFile(opts.tracePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}
	return &tracer{out: f, enc: json.NewEncoder(f)}, nil
}

func (t *tracer) Log(entry traceEntry) {
	if t == nil || t.enc == nil {
		return
	}
	_ = t.enc.Encode(entry)
}

func (t *tracer) Close() {
	if t == nil || t.out == nil {
		return
	}
	_ = t.out.Close()
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

func traceEntryFromResponse(start time.Time, req gcbackend.Request, resp gcbackend.Response, telemetry doltlitestorage.TelemetrySnapshot) traceEntry {
	var code, message string
	if resp.Error != nil {
		code = resp.Error.Code
		message = sanitizeTraceText(resp.Error.Message, 500)
	}
	entry := traceEntry{
		Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
		PID:             os.Getpid(),
		Backend:         provider.Name,
		Protocol:        gcbackend.ProtocolVersion,
		RequestID:       req.ID,
		Method:          req.Method,
		Params:          traceParams(req),
		OK:              resp.OK,
		ErrorCode:       code,
		Error:           message,
		DurationMS:      time.Since(start).Milliseconds(),
		RetryErrorClass: telemetry.RetryErrorClass,
		RetryError:      telemetry.RetryError,
	}
	if telemetry.RetryCount > 0 {
		entry.RetryCount = intPtr(telemetry.RetryCount)
	}
	if telemetry.LockWaitCount > 0 {
		entry.LockWaitCount = intPtr(telemetry.LockWaitCount)
		entry.LockWaitMS = int64Ptr(telemetry.LockWait.Milliseconds())
		entry.MaxLockWaitMS = int64Ptr(telemetry.MaxLockWait.Milliseconds())
	}
	if len(telemetry.Phases) > 0 {
		entry.PhaseMS = make(map[string]int64, len(telemetry.Phases))
		for name, elapsed := range telemetry.Phases {
			entry.PhaseMS[name] = elapsed.Milliseconds()
		}
	}
	return entry
}

func traceParams(req gcbackend.Request) any {
	if len(req.Params) == 0 {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(req.Params, &raw); err != nil {
		return map[string]any{"decode_error": err.Error()}
	}
	delete(raw, "session_id")
	for _, key := range []string{"issue", "updates", "metadata", "value"} {
		if _, ok := raw[key]; ok {
			raw[key] = "<redacted>"
		}
	}
	for key, value := range raw {
		if s, ok := value.(string); ok {
			raw[key] = sanitizeTraceText(s, 300)
		}
	}
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func intPtr(v int) *int { return &v }

func int64Ptr(v int64) *int64 { return &v }

func sanitizeTraceText(s string, limit int) string {
	s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(strings.TrimSpace(s))
	if limit > 0 && len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}

func handle(ctx context.Context, manager *provider.Manager, req gcbackend.Request) gcbackend.Response {
	switch req.Method {
	case "ping":
		return okResponse(req.ID, map[string]bool{"ok": true})
	case "open":
		var p gcbackend.OpenParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Open(ctx, p.BeadsDir, p.Database, p.Branch)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, gcbackend.OpenResult{SessionID: session.ID})
	case "close":
		var p gcbackend.SessionParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		if err := manager.Close(p.SessionID); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"closed": true})
	case "get_issue":
		var p gcbackend.GetIssueParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		issue, err := session.GetIssue(ctx, p.ID)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, issue)
	case "create_issue":
		var p gcbackend.CreateIssueParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		issue, err := session.CreateIssue(ctx, p.Issue, p.Actor, p.Commit, p.Message)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, issue)
	case "update_issue":
		var p gcbackend.UpdateIssueParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		issue, err := session.ApplyIssueUpdate(ctx, p.ID, p.Updates, p.AddLabels, p.RemoveLabels, p.ParentID, p.Actor, p.Commit, p.Message)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, issue)
	case "close_issue":
		var p gcbackend.IssueActionParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.CloseIssue(ctx, p.ID, p.Reason, p.Actor, p.Session); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"closed": true})
	case "reopen_issue":
		var p gcbackend.IssueActionParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.ReopenIssue(ctx, p.ID, p.Reason, p.Actor); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"reopened": true})
	case "delete_issue":
		var p gcbackend.IssueActionParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.DeleteIssue(ctx, p.ID); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"deleted": true})
	case "search_issues":
		var p gcbackend.SearchIssuesParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		issues, err := session.SearchIssues(ctx, p.Query, p.Filter)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, issues)
	case "ready_work":
		var p gcbackend.ReadyWorkParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		issues, err := session.ReadyWork(ctx, p.Filter)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, issues)
	case "list_wisps":
		var p gcbackend.ListWispsParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		issues, err := session.ListWisps(ctx, p.Filter)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, issues)
	case "count_issues":
		var p gcbackend.CountIssuesParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		count, err := session.CountIssues(ctx, p.Query, p.Filter)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]int64{"count": count})
	case "add_label":
		var p gcbackend.LabelParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		labels, err := session.AddLabel(ctx, p.ID, p.Label, p.Actor, p.Commit, p.Message)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, labels)
	case "remove_label":
		var p gcbackend.LabelParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.RemoveLabel(ctx, p.ID, p.Label, p.Actor); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"removed": true})
	case "get_labels":
		var p gcbackend.LabelParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		labels, err := session.GetLabels(ctx, p.ID)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, labels)
	case "add_dependency":
		var p gcbackend.DependencyParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.AddDependency(ctx, p.Dependency, p.Actor); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"added": true})
	case "remove_dependency":
		var p gcbackend.DependencyParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.RemoveDependency(ctx, p.IssueID, p.DependsOnID, p.Actor); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"removed": true})
	case "get_dependencies":
		var p gcbackend.DependencyParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		deps, err := session.GetDependencyRecords(ctx, p.IssueID)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, deps)
	case "get_dependents":
		var p gcbackend.DependencyParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		all, err := session.GetAllDependencyRecords(ctx)
		if err != nil {
			return storageError(req.ID, err)
		}
		deps := make([]any, 0)
		for _, records := range all {
			for _, dep := range records {
				if dep != nil && dep.DependsOnID == p.IssueID {
					deps = append(deps, dep)
				}
			}
		}
		return okResponse(req.ID, deps)
	case "slot_set":
		var p gcbackend.MetadataSlotParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.SlotSet(ctx, p.IssueID, p.Key, p.Value, p.Actor); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"set": true})
	case "slot_get":
		var p gcbackend.MetadataSlotParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		value, err := session.SlotGet(ctx, p.IssueID, p.Key)
		if err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]string{"value": value})
	case "slot_clear":
		var p gcbackend.MetadataSlotParams
		if err := decode(req.Params, &p); err != nil {
			return errorResponse(req.ID, "bad_request", err)
		}
		session, err := manager.Get(p.SessionID)
		if err != nil {
			return storageError(req.ID, err)
		}
		if err := session.SlotClear(ctx, p.IssueID, p.Key, p.Actor); err != nil {
			return storageError(req.ID, err)
		}
		return okResponse(req.ID, map[string]bool{"cleared": true})
	default:
		return errorResponse(req.ID, "unknown_method", fmt.Errorf("unknown method %q", req.Method))
	}
}

func decode(data json.RawMessage, out any) error {
	if len(data) == 0 {
		data = []byte("{}")
	}
	return json.Unmarshal(data, out)
}

func okResponse(id string, value any) gcbackend.Response {
	return gcbackend.Response{ID: id, OK: true, Result: mustJSON(value)}
}

func errorResponse(id, code string, err error) gcbackend.Response {
	return gcbackend.Response{
		ID: id,
		OK: false,
		Error: &gcbackend.Error{
			Code:    code,
			Message: err.Error(),
		},
	}
}

func storageError(id string, err error) gcbackend.Response {
	code := "storage_error"
	if errors.Is(err, storage.ErrNotFound) {
		code = "not_found"
	}
	return errorResponse(id, code, err)
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func writeJSON(value any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		fmt.Fprintf(os.Stderr, "encode json: %v\n", err)
		os.Exit(1)
	}
}
