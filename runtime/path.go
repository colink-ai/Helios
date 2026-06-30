package runtime

import (
	"path/filepath"
	"strings"
)

const DefaultDomainID = "default"

// RuntimeConfigMode controls whether Helios should provide an isolated agent
// configuration home or leave the underlying CLI to use its user-level config.
type RuntimeConfigMode string

const (
	// RuntimeConfigIsolated tells adapters to use the provided RuntimeHome when
	// configuring agent-local files such as HERMES_HOME or OPENCODE_CONFIG_DIR.
	RuntimeConfigIsolated RuntimeConfigMode = "isolated"

	// RuntimeConfigUser tells adapters not to override the agent's config home.
	// The child CLI can then use its normal user-level config, such as ~/.hermes
	// or the CLI's own login state.
	RuntimeConfigUser RuntimeConfigMode = "user"
)

// RuntimeProfile describes host-provided runtime storage roots.
type RuntimeProfile struct {
	ConfigMode  RuntimeConfigMode `json:"configMode,omitempty"`
	RuntimeRoot string            `json:"runtimeRoot,omitempty"`
	RuntimeHome string            `json:"runtimeHome,omitempty"`
	WorkDir     string            `json:"workDir,omitempty"`
}

// ResolvedRuntimePaths is the concrete storage decision for one runtime scope.
type ResolvedRuntimePaths struct {
	ConfigMode     RuntimeConfigMode `json:"configMode"`
	RuntimeHome    string            `json:"runtimeHome,omitempty"`
	WorkDir        string            `json:"workDir,omitempty"`
	UsesUserConfig bool              `json:"usesUserConfig,omitempty"`
}

// DomainPaths returns isolated runtime paths for a domain. If RuntimeRoot is
// empty, the explicit RuntimeHome and WorkDir fields are returned unchanged.
func (p RuntimeProfile) DomainPaths(domainID string) (runtimeHome string, workDir string) {
	domainID = NormalizeDomainID(domainID)
	if p.RuntimeRoot == "" {
		return p.RuntimeHome, p.WorkDir
	}
	return filepath.Join(p.RuntimeRoot, "domains", domainID, "home"),
		filepath.Join(p.RuntimeRoot, "domains", domainID, "workdir")
}

// Resolve returns the runtime path policy for a domain. In user config mode,
// RuntimeHome is intentionally empty so adapters can preserve the agent CLI's
// normal user-level configuration directory. WorkDir can still be isolated.
func (p RuntimeProfile) Resolve(domainID string) ResolvedRuntimePaths {
	mode := p.ConfigMode
	if mode == "" {
		if p.RuntimeRoot == "" && p.RuntimeHome == "" && p.WorkDir == "" {
			mode = RuntimeConfigUser
		} else {
			mode = RuntimeConfigIsolated
		}
	}
	if mode == RuntimeConfigUser {
		workDir := p.WorkDir
		if p.RuntimeRoot != "" {
			workDir = filepath.Join(p.RuntimeRoot, "domains", NormalizeDomainID(domainID), "workdir")
		}
		return ResolvedRuntimePaths{ConfigMode: RuntimeConfigUser, WorkDir: workDir, UsesUserConfig: true}
	}
	runtimeHome, workDir := p.DomainPaths(domainID)
	return ResolvedRuntimePaths{ConfigMode: RuntimeConfigIsolated, RuntimeHome: runtimeHome, WorkDir: workDir}
}

// EffectiveRuntimeConfigMode returns the request-level mode, then the agent
// default, then infers user config only when no config path is provided.
func EffectiveRuntimeConfigMode(req SessionRequest) RuntimeConfigMode {
	if req.RuntimeConfigMode != "" {
		return req.RuntimeConfigMode
	}
	if req.Agent.RuntimeConfigMode != "" {
		return req.Agent.RuntimeConfigMode
	}
	if EffectiveConfigDir(req) == "" && EffectiveRuntimeHome(req) == "" && EffectiveWorkDir(req) == "" {
		return RuntimeConfigUser
	}
	return RuntimeConfigIsolated
}

// EffectiveRuntimeHome resolves the runtime home with request-level precedence.
func EffectiveRuntimeHome(req SessionRequest) string {
	if req.RuntimeHome != "" {
		return req.RuntimeHome
	}
	return req.Agent.RuntimeHome
}

// EffectiveConfigDir resolves a host-provided agent config directory with
// request-level precedence. Unlike RuntimeHome, ConfigDir means the host has
// already prepared the exact directory the adapter should give to the CLI.
func EffectiveConfigDir(req SessionRequest) string {
	if req.ConfigDir != "" {
		return req.ConfigDir
	}
	return req.Agent.ConfigDir
}

// EffectiveWorkDir resolves the working directory with request-level precedence.
func EffectiveWorkDir(req SessionRequest) string {
	if req.WorkDir != "" {
		return req.WorkDir
	}
	return req.Agent.WorkDir
}

// NormalizeDomainID produces a filesystem-safe domain identifier.
func NormalizeDomainID(domainID string) string {
	domainID = strings.TrimSpace(strings.ToLower(domainID))
	if domainID == "" {
		return DefaultDomainID
	}
	var b strings.Builder
	for _, r := range domainID {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return DefaultDomainID
	}
	return b.String()
}
