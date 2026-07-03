package doltlite

import (
	"context"
	"strings"
	"sync"
	"time"
)

type telemetryContextKey struct{}

// Telemetry captures per-request contention signals for plugin trace logs.
type Telemetry struct {
	mu              sync.Mutex
	retryCount      int
	retryErrorClass string
	retryError      string
	lockWaitCount   int
	lockWait        time.Duration
	maxLockWait     time.Duration
	phases          map[string]time.Duration
}

type TelemetrySnapshot struct {
	RetryCount      int
	RetryErrorClass string
	RetryError      string
	LockWaitCount   int
	LockWait        time.Duration
	MaxLockWait     time.Duration
	Phases          map[string]time.Duration
}

func NewTelemetry() *Telemetry {
	return &Telemetry{}
}

func ContextWithTelemetry(ctx context.Context, t *Telemetry) context.Context {
	if t == nil {
		return ctx
	}
	return context.WithValue(ctx, telemetryContextKey{}, t)
}

func telemetryFromContext(ctx context.Context) *Telemetry {
	if ctx == nil {
		return nil
	}
	t, _ := ctx.Value(telemetryContextKey{}).(*Telemetry)
	return t
}

func recordRetryTelemetry(ctx context.Context, err error) {
	t := telemetryFromContext(ctx)
	if t == nil || err == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.retryCount++
	t.retryErrorClass = retryConcurrencyErrorClass(err)
	t.retryError = sanitizeTraceText(err.Error(), 500)
}

func recordLockWaitTelemetry(ctx context.Context, waited time.Duration) {
	t := telemetryFromContext(ctx)
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lockWaitCount++
	t.lockWait += waited
	if waited > t.maxLockWait {
		t.maxLockWait = waited
	}
}

func recordPhaseTelemetry(ctx context.Context, name string, elapsed time.Duration) {
	t := telemetryFromContext(ctx)
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.phases == nil {
		t.phases = make(map[string]time.Duration)
	}
	t.phases[name] += elapsed
}

func (t *Telemetry) Snapshot() TelemetrySnapshot {
	if t == nil {
		return TelemetrySnapshot{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	snap := TelemetrySnapshot{
		RetryCount:      t.retryCount,
		RetryErrorClass: t.retryErrorClass,
		RetryError:      t.retryError,
		LockWaitCount:   t.lockWaitCount,
		LockWait:        t.lockWait,
		MaxLockWait:     t.maxLockWait,
	}
	if len(t.phases) > 0 {
		snap.Phases = make(map[string]time.Duration, len(t.phases))
		for k, v := range t.phases {
			snap.Phases[k] = v
		}
	}
	return snap
}

func retryConcurrencyErrorClass(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "sqlite_busy"):
		return "sqlite_busy"
	case strings.Contains(msg, "database is locked"):
		return "database_locked"
	case strings.Contains(msg, "another connection committed"):
		return "concurrent_commit"
	case strings.Contains(msg, "please retry your transaction"):
		return "retry_transaction"
	case strings.Contains(msg, "failed to prepare catalog"):
		return "prepare_catalog"
	default:
		return ""
	}
}

func sanitizeTraceText(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		default:
			return r
		}
	}, strings.TrimSpace(s))
	if limit > 0 && len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}
