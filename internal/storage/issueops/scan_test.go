package issueops

import (
	"testing"
	"time"
)

func TestParseTimeStringAcceptsDoltLiteLayouts(t *testing.T) {
	want := time.Date(2026, 6, 7, 12, 13, 14, 123456789, time.UTC)
	cases := []string{
		"2026-06-07T12:13:14.123456789Z",
		"2026-06-07 12:13:14.123456789+00:00",
		"2026-06-07 12:13:14.123456789 +0000 UTC",
		"2026-06-07 12:13:14.123456789",
	}
	for _, in := range cases {
		got := ParseTimeString(in)
		if !got.Equal(want) {
			t.Fatalf("ParseTimeString(%q) = %s, want %s", in, got.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
		}
	}
}

func TestParseTimeStringAcceptsDoltDatetimeSeconds(t *testing.T) {
	want := time.Date(2026, 6, 7, 12, 13, 14, 0, time.UTC)
	got := ParseTimeString("2026-06-07 12:13:14")
	if !got.Equal(want) {
		t.Fatalf("ParseTimeString DATETIME = %s, want %s", got.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
}
