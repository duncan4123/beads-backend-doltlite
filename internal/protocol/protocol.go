package protocol

import "github.com/duncan4123/beads-backend-doltlite/internal/provider"

const Version = "beads.backend.v1alpha1"

type Hello struct {
	Protocol     string                `json:"protocol"`
	Backend      string                `json:"backend"`
	Capabilities provider.Capabilities `json:"capabilities"`
}
