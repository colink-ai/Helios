package runtime

import (
	"path/filepath"
	"strings"
)

const DefaultDomainID = "default"

// RuntimeProfile describes host-provided runtime storage roots.
type RuntimeProfile struct {
	RuntimeRoot string `json:"runtimeRoot,omitempty"`
	RuntimeHome string `json:"runtimeHome,omitempty"`
	WorkDir     string `json:"workDir,omitempty"`
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
