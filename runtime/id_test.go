package runtime

import (
	"strings"
	"testing"
)

func TestNewIDPrefixHandling(t *testing.T) {
	plain := NewID("")
	if plain == "" || strings.Contains(plain, "-") {
		t.Fatalf("plain id = %q", plain)
	}
	withPrefix := NewID("run-")
	if !strings.HasPrefix(withPrefix, "run-") || strings.Contains(withPrefix, "run--") {
		t.Fatalf("prefixed id = %q", withPrefix)
	}
}
