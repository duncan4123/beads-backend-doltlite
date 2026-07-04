package provider

import "testing"

func TestBackendCapabilitiesAdvertiseVersionControlAndRemotes(t *testing.T) {
	caps := BackendCapabilities()

	if !caps.Versioning {
		t.Fatal("Versioning = false, want true")
	}
	if !caps.Branching {
		t.Fatal("Branching = false, want true")
	}
	if !caps.DoltRemotes {
		t.Fatal("DoltRemotes = false, want true")
	}
}
