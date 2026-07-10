//go:build cgo

// doltlite-client executes arbitrary SQL against a DoltLite database.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type metadataFile struct {
	Backend      string `json:"backend"`
	Database     string `json:"database"`
	DoltDatabase string `json:"dolt_database"`
}

type client struct {
	db   *sql.DB
	path string
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "doltlite-client: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("doltlite-client", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", ".", "Gas City or rig scope root")
	city := fs.String("city", "", "deprecated alias for --scope")
	dbPath := fs.String("db", "", "direct DoltLite database path; overrides scope metadata")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		usage(fs)
		return nil
	}
	if *city != "" {
		*scope = *city
	}

	c, err := openClient(*scope, *dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = c.db.Close() }()

	switch rest[0] {
	case "info":
		return c.info(ctx)
	case "query":
		if len(rest) < 2 {
			return errors.New("query requires SQL")
		}
		return c.query(ctx, rest[1], rest[2:]...)
	case "exec":
		if len(rest) < 2 {
			return errors.New("exec requires SQL")
		}
		return c.exec(ctx, rest[1], rest[2:]...)
	default:
		usage(fs)
		return fmt.Errorf("unknown command %q", rest[0])
	}
}

func usage(fs *flag.FlagSet) {
	_, _ = fmt.Fprintln(fs.Output(), "usage: doltlite-client [--scope PATH | --db FILE] <info|query|exec> ...")
}

func openClient(scope, directDBPath string) (*client, error) {
	if strings.TrimSpace(directDBPath) != "" {
		return openDB(filepath.Clean(directDBPath))
	}
	meta, err := readMetadata(scope)
	if err != nil {
		return nil, err
	}
	if meta.Backend != "doltlite" {
		return nil, fmt.Errorf("%s is not a doltlite scope (backend=%q)", scope, meta.Backend)
	}
	dbName := strings.TrimSpace(meta.DoltDatabase)
	if dbName == "" || dbName == "doltlite" {
		dbName = strings.TrimSpace(meta.Database)
	}
	if dbName == "" || dbName == "doltlite" {
		dbName = "hq"
	}
	return openDB(filepath.Join(scope, ".beads", "doltlite", dbName+".db"))
}

func openDB(dbPath string) (*client, error) {
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	db, err := sql.Open("sqlite3", "file:"+absPath+"?_busy_timeout=10000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("opening %s: %w", absPath, err)
	}
	return &client{db: db, path: absPath}, nil
}

func readMetadata(scope string) (metadataFile, error) {
	var meta metadataFile
	data, err := os.ReadFile(filepath.Join(scope, ".beads", "metadata.json"))
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func (c *client) info(ctx context.Context) error {
	fmt.Printf("db=%s\n", c.path)
	for _, item := range []struct {
		name string
		sql  string
	}{
		{name: "doltlite_version", sql: "SELECT dolt_version()"},
		{name: "main_hash", sql: "SELECT dolt_hashof('main')"},
	} {
		var value string
		if err := c.db.QueryRowContext(ctx, item.sql).Scan(&value); err != nil {
			return fmt.Errorf("%s: %w", item.name, err)
		}
		fmt.Printf("%s=%s\n", item.name, value)
	}
	return c.query(ctx, "SELECT name, type FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name")
}

func (c *client) query(ctx context.Context, query string, args ...string) error {
	rows, err := c.db.QueryContext(ctx, query, toAny(args)...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(cols, "\t"))
	count := 0
	for rows.Next() {
		values := make([]sql.NullString, len(cols))
		scan := make([]any, len(cols))
		for i := range values {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return err
		}
		out := make([]string, len(cols))
		for i, value := range values {
			if value.Valid {
				out[i] = value.String
			} else {
				out[i] = "NULL"
			}
		}
		fmt.Println(strings.Join(out, "\t"))
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Printf("rows=%d\n", count)
	return nil
}

func (c *client) exec(ctx context.Context, query string, args ...string) error {
	res, err := c.db.ExecContext(ctx, query, toAny(args)...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	fmt.Printf("rows_affected=%d\n", n)
	return nil
}

func toAny(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}
