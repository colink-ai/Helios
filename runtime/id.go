package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// NewID returns a compact process-local identifier with an optional prefix.
func NewID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		fallback := hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
		if prefix == "" {
			return fallback
		}
		return strings.TrimSuffix(prefix, "-") + "-" + fallback
	}
	id := hex.EncodeToString(b[:])
	if prefix == "" {
		return id
	}
	return strings.TrimSuffix(prefix, "-") + "-" + id
}
