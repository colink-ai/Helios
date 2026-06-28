package contracts

import "testing"

func TestNewEventSetsSchemaVersion(t *testing.T) {
	event := NewEvent(EventChunk)
	if event.SchemaVersion != SemanticSchemaVersion {
		t.Fatalf("schema version = %q", event.SchemaVersion)
	}
}
