//go:build cgo

package doltlite

import (
	"github.com/duncan4123/beads-backend-doltlite/internal/storage/schema"
)

// LatestVersion delegates to the shared schema package.
func LatestVersion() int {
	return schema.LatestVersion()
}
