package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/duncan4123/beads-backend-doltlite/internal/gcplugin"
)

func main() {
	opts, args, err := parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		usage()
		os.Exit(2)
	}
	if len(args) != 1 {
		usage()
		os.Exit(2)
	}
	scope, err := filepath.Abs(opts.scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve scope: %v\n", err)
		os.Exit(1)
	}
	opts.scope = scope

	meta, err := gcplugin.LoadMetadata(gcplugin.MetadataPathForScope(opts.scope))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load metadata: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "layout":
		layout, err := gcplugin.ResolveLayout(opts.scope, meta, opts.profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve layout: %v\n", err)
			os.Exit(1)
		}
		writeJSON(layout)
	case "health":
		health := gcplugin.CheckHealth(opts.scope, meta, opts.profile)
		writeJSON(health)
		if !health.OK {
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

type options struct {
	scope   string
	profile gcplugin.Profile
}

func parse(args []string) (options, []string, error) {
	opts := options{scope: "."}
	for len(args) > 0 {
		arg := args[0]
		switch {
		case arg == "--scope":
			if len(args) < 2 {
				return opts, args, fmt.Errorf("--scope requires a path")
			}
			opts.scope = args[1]
			args = args[2:]
		case strings.HasPrefix(arg, "--scope="):
			opts.scope = strings.TrimPrefix(arg, "--scope=")
			args = args[1:]
		case arg == "--profile":
			if len(args) < 2 {
				return opts, args, fmt.Errorf("--profile requires a value")
			}
			profile, err := gcplugin.ParseProfile(args[1])
			if err != nil {
				return opts, args, err
			}
			opts.profile = profile
			args = args[2:]
		case strings.HasPrefix(arg, "--profile="):
			profile, err := gcplugin.ParseProfile(strings.TrimPrefix(arg, "--profile="))
			if err != nil {
				return opts, args, err
			}
			opts.profile = profile
			args = args[1:]
		case strings.HasPrefix(arg, "-"):
			return opts, args, fmt.Errorf("unknown option %q", arg)
		default:
			return opts, args, nil
		}
	}
	return opts, args, nil
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "encode json: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gc-doltlite [--scope=<city-or-rig>] [--profile=<ledger|local-runtime|mirror>] <layout|health>")
}
