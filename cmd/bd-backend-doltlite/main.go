package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/duncan4123/beads-backend-doltlite/internal/protocol"
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
		payload = protocol.Hello{
			Protocol:     protocol.Version,
			Backend:      provider.Name,
			Capabilities: provider.BackendCapabilities(),
		}
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
