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

func TestValidatePositiveInt(t *testing.T) {
	for _, in := range []string{"1", "5", "999"} {
		if err := validatePositiveInt(in); err != nil {
			t.Errorf("validatePositiveInt(%q) = %v, want nil", in, err)
		}
	}
	for _, in := range []string{"", "0", "-1", "abc", "1.5"} {
		if err := validatePositiveInt(in); err == nil {
			t.Errorf("validatePositiveInt(%q) = nil, want error", in)
		}
	}
}

func TestValidateNonNegativeInt(t *testing.T) {
	for _, in := range []string{"0", "3", "999"} {
		if err := validateNonNegativeInt(in); err != nil {
			t.Errorf("validateNonNegativeInt(%q) = %v, want nil", in, err)
		}
	}
	for _, in := range []string{"", "-1", "abc", "1.5"} {
		if err := validateNonNegativeInt(in); err == nil {
			t.Errorf("validateNonNegativeInt(%q) = nil, want error", in)
		}
	}
}
