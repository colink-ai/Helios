package runtime

import (
	"path/filepath"
	"testing"
)

func TestRuntimeProfileDomainPaths(t *testing.T) {
	profile := RuntimeProfile{RuntimeRoot: "data/runtime"}
	home, workdir := profile.DomainPaths("Ops Team!")
	if home != filepath.Join("data/runtime", "domains", "opsteam", "home") {
		t.Fatalf("home = %q", home)
	}
	if workdir != filepath.Join("data/runtime", "domains", "opsteam", "workdir") {
		t.Fatalf("workdir = %q", workdir)
	}
}

func TestRuntimeProfileCompatibilityFallback(t *testing.T) {
	profile := RuntimeProfile{RuntimeHome: "legacy/home", WorkDir: "legacy/work"}
	home, workdir := profile.DomainPaths("ignored")
	if home != "legacy/home" || workdir != "legacy/work" {
		t.Fatalf("fallback paths = %q %q", home, workdir)
	}
}

func TestNormalizeDomainID(t *testing.T) {
	cases := map[string]string{
		"":           DefaultDomainID,
		"  Ops_Team": "ops_team",
		"中文!":        DefaultDomainID,
		"Team-42":    "team-42",
	}
	for input, want := range cases {
		if got := NormalizeDomainID(input); got != want {
			t.Fatalf("NormalizeDomainID(%q) = %q, want %q", input, got, want)
		}
	}
}
