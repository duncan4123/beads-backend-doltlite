package provider

const Name = "doltlite"

type Capabilities struct {
	Embedded          bool `json:"embedded"`
	Transactions      bool `json:"transactions"`
	RawSQL            bool `json:"raw_sql"`
	Leases            bool `json:"leases"`
	Maintenance       bool `json:"maintenance"`
	Versioning        bool `json:"versioning"`
	Branching         bool `json:"branching"`
	DoltRemotes       bool `json:"dolt_remotes"`
	ConcurrentWriters bool `json:"concurrent_writers"`
}

type Diagnostic struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func BackendCapabilities() Capabilities {
	return Capabilities{
		Embedded:          true,
		Transactions:      true,
		RawSQL:            true,
		Leases:            true,
		Maintenance:       true,
		Versioning:        true,
		Branching:         false,
		DoltRemotes:       false,
		ConcurrentWriters: true,
	}
}

func Doctor() []Diagnostic {
	return []Diagnostic{
		{
			Level:   "info",
			Code:    "prototype",
			Message: "DoltLite backend plugin protocol skeleton is present; storage RPCs are not implemented yet.",
		},
	}
}
