package gcbackend

import "testing"

func TestDefaultCapabilitiesConformance(t *testing.T) {
	caps := DefaultCapabilities()
	if !caps.GetIssue {
		t.Fatal("GetIssue capability is required")
	}
	if !caps.SearchIssues {
		t.Fatal("SearchIssues capability is required")
	}
	if !caps.ReadyWork {
		t.Fatal("ReadyWork capability is required")
	}
	if !caps.ListWisps {
		t.Fatal("ListWisps capability is required")
	}
	if !caps.CountIssues {
		t.Fatal("CountIssues capability is required")
	}
	if !caps.StorageCreate {
		t.Fatal("StorageCreate capability is required")
	}
	if !caps.ConditionalClaim {
		t.Fatal("ConditionalClaim capability is required")
	}
	if !caps.BatchDeps {
		t.Fatal("BatchDeps capability is required")
	}
	if !caps.WriteOperations {
		t.Fatal("WriteOperations should be true so gc does not need direct linked writes")
	}
}

func TestHelloUsesBackendProtocol(t *testing.T) {
	hello := Hello{
		Protocol:     ProtocolVersion,
		Backend:      "doltlite",
		Capabilities: DefaultCapabilities(),
	}
	if hello.Protocol != "gascity.backend.v1alpha1" {
		t.Fatalf("Protocol = %q", hello.Protocol)
	}
	if hello.Backend != "doltlite" {
		t.Fatalf("Backend = %q", hello.Backend)
	}
	if !hello.Capabilities.ReadyWork {
		t.Fatal("hello should advertise ready_work")
	}
}
