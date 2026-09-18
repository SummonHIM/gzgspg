package version

import "testing"

func TestStringCombinesValueAndBuildTime(t *testing.T) {
	origValue, origBuildTime := Value, BuildTime
	t.Cleanup(func() {
		Value, BuildTime = origValue, origBuildTime
	})

	Value = "1.3.0"
	BuildTime = "2026-09-18T12:00:00Z"

	if got, want := String(), "1.3.0 2026-09-18T12:00:00Z"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestDefaults(t *testing.T) {
	if Value != "dev" {
		t.Fatalf("Value default = %q, want %q", Value, "dev")
	}
	if BuildTime != "unknown" {
		t.Fatalf("BuildTime default = %q, want %q", BuildTime, "unknown")
	}
}
