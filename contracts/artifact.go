package contracts

import "time"

// ArtifactType classifies runtime-created outputs.
type ArtifactType string

const (
	ArtifactCode     ArtifactType = "code"
	ArtifactDocument ArtifactType = "document"
	ArtifactReview   ArtifactType = "review"
	ArtifactTest     ArtifactType = "test"
	ArtifactConfig   ArtifactType = "config"
	ArtifactData     ArtifactType = "data"
	ArtifactOther    ArtifactType = "other"
)

// Artifact is an application-visible runtime output.
type Artifact struct {
	ID        string         `json:"id,omitempty"`
	RunID     string         `json:"runId,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Type      ArtifactType   `json:"type"`
	Name      string         `json:"name"`
	Path      string         `json:"path,omitempty"`
	Content   string         `json:"content,omitempty"`
	MimeType  string         `json:"mimeType,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt,omitempty"`
}
