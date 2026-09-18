package ui

import (
	"testing"

	"github.com/summonhim/gzgspg/internal/version"
)

func TestVersionLabelPrefixesVersion(t *testing.T) {
	got := versionLabel()
	if got == "" {
		t.Fatal("versionLabel must never be empty")
	}
	if want := "版本 " + version.String(); got != want {
		t.Fatalf("versionLabel() = %q, want %q", got, want)
	}
}
