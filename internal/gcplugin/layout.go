package gcplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	DefaultProfile = ProfileLedger
	DefaultOpsName = "ops"
)

type Profile string

const (
	// ProfileLedger keeps all synchronized Beads and Gas City workflow state in
	// DoltLite, with only local caches and diagnostics placed in SQLite.
	ProfileLedger Profile = "ledger"

	// ProfileLocalRuntime moves local workflow/runtime tables to attached SQLite.
	// This is appropriate for a single-machine city, or for deployments that add
	// explicit mirror tables before syncing runtime state elsewhere.
	ProfileLocalRuntime Profile = "local-runtime"

	// ProfileMirror is local-runtime plus explicit DoltLite mirror tables for
	// summarized runtime state that needs to travel with remotes.
	ProfileMirror Profile = "mirror"
)

type AttachedDatabase struct {
	Alias string `json:"alias"`
	Path  string `json:"path"`
}

type Metadata struct {
	Backend              string             `json:"backend,omitempty"`
	DoltDatabase         string             `json:"dolt_database,omitempty"`
	BackendPluginCommand string             `json:"backend_plugin_command,omitempty"`
	BackendPluginArgs    []string           `json:"backend_plugin_args,omitempty"`
	AttachedDatabases    []AttachedDatabase `json:"attached_databases,omitempty"`
	GasCity              GasCityMetadata    `json:"gascity,omitempty"`
}

type GasCityMetadata struct {
	DoltLiteProfile string `json:"doltlite_profile,omitempty"`
}

type Layout struct {
	Profile           Profile           `json:"profile"`
	Backend           string            `json:"backend,omitempty"`
	DoltDatabase      string            `json:"dolt_database,omitempty"`
	PluginCommand     string            `json:"backend_plugin_command,omitempty"`
	PluginArgs        []string          `json:"backend_plugin_args,omitempty"`
	OpsAlias          string            `json:"ops_alias"`
	OpsPath           string            `json:"ops_path,omitempty"`
	DoltLiteTables    []string          `json:"doltlite_tables"`
	SQLiteTables      []string          `json:"sqlite_tables"`
	MirrorTables      []string          `json:"mirror_tables,omitempty"`
	ResolvedTableName map[string]string `json:"resolved_table_name"`
}

type Health struct {
	OK       bool     `json:"ok"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Layout   Layout   `json:"layout"`
}

var durableTables = []string{
	"issues",
	"labels",
	"dependencies",
	"events",
	"comments",
	"config",
	"metadata",
	"routes",
	"counters",
	"custom_fields",
}

var workflowTables = []string{
	"wisps",
	"wisp_labels",
	"wisp_dependencies",
	"wisp_events",
	"wisp_comments",
}

var localTables = []string{
	"repo_mtimes",
	"local_metadata",
	"diagnostics",
	"metrics",
}

var mirrorTables = []string{
	"runtime_snapshots",
	"workflow_handoffs",
	"completed_run_summaries",
	"mirror_watermarks",
}

func LoadMetadata(path string) (Metadata, error) {
	data, err := os.ReadFile(path) // #nosec G304 - caller supplies workspace metadata path
	if err != nil {
		return Metadata{}, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func MetadataPathForScope(scope string) string {
	if strings.HasSuffix(scope, ".json") {
		return scope
	}
	return filepath.Join(scope, ".beads", "metadata.json")
}

func ResolveLayout(scope string, meta Metadata, override Profile) (Layout, error) {
	profile, err := resolveProfile(meta, override)
	if err != nil {
		return Layout{}, err
	}

	ops := resolveOps(scope, meta)
	layout := Layout{
		Profile:       profile,
		Backend:       meta.Backend,
		DoltDatabase:  meta.DoltDatabase,
		PluginCommand: meta.BackendPluginCommand,
		PluginArgs:    slices.Clone(meta.BackendPluginArgs),
		OpsAlias:      DefaultOpsName,
		OpsPath:       ops,
	}

	switch profile {
	case ProfileLedger:
		layout.DoltLiteTables = sortedJoin(durableTables, workflowTables)
		layout.SQLiteTables = slices.Clone(localTables)
	case ProfileLocalRuntime:
		layout.DoltLiteTables = slices.Clone(durableTables)
		layout.SQLiteTables = sortedJoin(workflowTables, localTables)
	case ProfileMirror:
		layout.DoltLiteTables = sortedJoin(durableTables, mirrorTables)
		layout.SQLiteTables = sortedJoin(workflowTables, localTables)
		layout.MirrorTables = slices.Clone(mirrorTables)
	default:
		return Layout{}, fmt.Errorf("unsupported profile %q", profile)
	}

	layout.ResolvedTableName = make(map[string]string, len(layout.DoltLiteTables)+len(layout.SQLiteTables))
	for _, table := range layout.DoltLiteTables {
		layout.ResolvedTableName[table] = table
	}
	for _, table := range layout.SQLiteTables {
		layout.ResolvedTableName[table] = layout.OpsAlias + "." + table
	}
	return layout, nil
}

func CheckHealth(scope string, meta Metadata, override Profile) Health {
	layout, err := ResolveLayout(scope, meta, override)
	if err != nil {
		return Health{OK: false, Errors: []string{err.Error()}}
	}
	h := Health{OK: true, Layout: layout}

	if strings.TrimSpace(meta.Backend) != "doltlite" {
		h.Warnings = append(h.Warnings, `metadata backend is not "doltlite"`)
	}
	if strings.TrimSpace(meta.BackendPluginCommand) == "" {
		h.Errors = append(h.Errors, "backend_plugin_command is missing")
	}
	if !hasServeArg(meta.BackendPluginArgs) {
		h.Errors = append(h.Errors, `backend_plugin_args does not include "serve"`)
	}
	if strings.TrimSpace(layout.OpsPath) == "" {
		h.Errors = append(h.Errors, "ops attached database path is missing")
	}

	if len(h.Errors) > 0 {
		h.OK = false
	}
	return h
}

func resolveProfile(meta Metadata, override Profile) (Profile, error) {
	if override != "" {
		return validateProfile(override)
	}
	raw := strings.TrimSpace(meta.GasCity.DoltLiteProfile)
	if raw == "" {
		raw = os.Getenv("GC_DOLTLITE_PROFILE")
	}
	if raw == "" {
		return DefaultProfile, nil
	}
	return validateProfile(Profile(raw))
}

func validateProfile(profile Profile) (Profile, error) {
	switch profile {
	case ProfileLedger, ProfileLocalRuntime, ProfileMirror:
		return profile, nil
	default:
		return "", fmt.Errorf("unknown Gas City DoltLite profile %q", profile)
	}
}

func resolveOps(scope string, meta Metadata) string {
	for _, db := range meta.AttachedDatabases {
		if db.Alias == DefaultOpsName {
			return resolvePath(scope, db.Path)
		}
	}
	return resolvePath(scope, filepath.Join(".gc", "ops.sqlite"))
}

func resolvePath(scope, p string) string {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	if strings.HasSuffix(scope, ".json") {
		scope = filepath.Dir(filepath.Dir(scope))
	}
	return filepath.Clean(filepath.Join(scope, p))
}

func hasServeArg(args []string) bool {
	return slices.Contains(args, "serve")
}

func sortedJoin(groups ...[]string) []string {
	var out []string
	for _, group := range groups {
		out = append(out, group...)
	}
	slices.Sort(out)
	return out
}

func ParseProfile(raw string) (Profile, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	return validateProfile(Profile(raw))
}

func ReadScopeLayout(scope string, override Profile) (Layout, error) {
	meta, err := LoadMetadata(MetadataPathForScope(scope))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Layout{}, fmt.Errorf("metadata not found at %s", MetadataPathForScope(scope))
		}
		return Layout{}, err
	}
	return ResolveLayout(scope, meta, override)
}
