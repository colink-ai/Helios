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

func TestRuntimeProfileResolveIsolatedRoot(t *testing.T) {
	profile := RuntimeProfile{RuntimeRoot: "data/runtime"}
	resolved := profile.Resolve("Ops Team!")
	if resolved.ConfigMode != RuntimeConfigIsolated || resolved.UsesUserConfig {
		t.Fatalf("unexpected mode: %+v", resolved)
	}
	if resolved.RuntimeHome != filepath.Join("data/runtime", "domains", "opsteam", "home") {
		t.Fatalf("home = %q", resolved.RuntimeHome)
	}
	if resolved.WorkDir != filepath.Join("data/runtime", "domains", "opsteam", "workdir") {
		t.Fatalf("workdir = %q", resolved.WorkDir)
	}
}

func TestRuntimeProfileResolveUserConfigWithIsolatedWorkDir(t *testing.T) {
	profile := RuntimeProfile{ConfigMode: RuntimeConfigUser, RuntimeRoot: "data/runtime"}
	resolved := profile.Resolve("Ops Team!")
	if resolved.ConfigMode != RuntimeConfigUser || !resolved.UsesUserConfig {
		t.Fatalf("unexpected mode: %+v", resolved)
	}
	if resolved.RuntimeHome != "" {
		t.Fatalf("user config mode should not set runtime home: %q", resolved.RuntimeHome)
	}
	if resolved.WorkDir != filepath.Join("data/runtime", "domains", "opsteam", "workdir") {
		t.Fatalf("workdir = %q", resolved.WorkDir)
	}
}

func TestRuntimeProfileResolveImplicitUserConfig(t *testing.T) {
	resolved := RuntimeProfile{}.Resolve("team")
	if resolved.ConfigMode != RuntimeConfigUser || !resolved.UsesUserConfig || resolved.RuntimeHome != "" || resolved.WorkDir != "" {
		t.Fatalf("unexpected implicit user config resolution: %+v", resolved)
	}
}

func TestEffectiveRuntimeConfigMode(t *testing.T) {
	if got := EffectiveRuntimeConfigMode(SessionRequest{}); got != RuntimeConfigUser {
		t.Fatalf("empty request mode = %q", got)
	}
	if got := EffectiveRuntimeConfigMode(SessionRequest{WorkDir: "work"}); got != RuntimeConfigIsolated {
		t.Fatalf("workdir request mode = %q", got)
	}
	if got := EffectiveRuntimeConfigMode(SessionRequest{RuntimeHome: "home"}); got != RuntimeConfigIsolated {
		t.Fatalf("runtime home request mode = %q", got)
	}
	if got := EffectiveRuntimeConfigMode(SessionRequest{ConfigDir: "config"}); got != RuntimeConfigIsolated {
		t.Fatalf("config dir request mode = %q", got)
	}
	if got := EffectiveRuntimeConfigMode(SessionRequest{RuntimeConfigMode: RuntimeConfigUser, WorkDir: "work"}); got != RuntimeConfigUser {
		t.Fatalf("explicit user mode = %q", got)
	}
	if got := EffectiveRuntimeConfigMode(SessionRequest{Agent: AgentSpec{RuntimeConfigMode: RuntimeConfigUser, WorkDir: "work"}}); got != RuntimeConfigUser {
		t.Fatalf("agent user mode = %q", got)
	}
}

func TestEffectiveConfigDir(t *testing.T) {
	req := SessionRequest{ConfigDir: "request-config", Agent: AgentSpec{ConfigDir: "agent-config"}}
	if got := EffectiveConfigDir(req); got != "request-config" {
		t.Fatalf("request config dir = %q", got)
	}
	req.ConfigDir = ""
	if got := EffectiveConfigDir(req); got != "agent-config" {
		t.Fatalf("agent config dir = %q", got)
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
