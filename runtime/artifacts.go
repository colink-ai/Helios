package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/colink-ai/helios/contracts"
)

// ArtifactStore persists runtime artifacts without imposing an application
// database schema.
type ArtifactStore interface {
	SaveArtifact(ctx context.Context, artifact contracts.Artifact) (contracts.Artifact, error)
	ReadArtifact(ctx context.Context, artifact contracts.Artifact) ([]byte, error)
}

// FileArtifactStore stores artifacts below one root directory.
type FileArtifactStore struct {
	root string
}

func NewFileArtifactStore(root string) *FileArtifactStore {
	return &FileArtifactStore{root: root}
}

func (s *FileArtifactStore) SaveArtifact(_ context.Context, artifact contracts.Artifact) (contracts.Artifact, error) {
	if s.root == "" {
		return contracts.Artifact{}, fmt.Errorf("artifact root is required")
	}
	if artifact.ID == "" {
		artifact.ID = NewID("artifact")
	}
	if artifact.Type == "" {
		artifact.Type = contracts.ArtifactOther
	}
	if artifact.Name == "" {
		artifact.Name = artifact.ID
	}
	rel, err := safeArtifactPath(artifact)
	if err != nil {
		return contracts.Artifact{}, err
	}
	path := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return contracts.Artifact{}, err
	}
	if err := os.WriteFile(path, []byte(artifact.Content), 0o644); err != nil {
		return contracts.Artifact{}, err
	}
	artifact.Path = path
	artifact.Content = ""
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now().UTC()
	}
	return artifact, nil
}

func (s *FileArtifactStore) ReadArtifact(_ context.Context, artifact contracts.Artifact) ([]byte, error) {
	if s.root == "" {
		return nil, fmt.Errorf("artifact root is required")
	}
	path := artifact.Path
	if path == "" {
		rel, err := safeArtifactPath(artifact)
		if err != nil {
			return nil, err
		}
		path = filepath.Join(s.root, rel)
	}
	cleanRoot, err := filepath.Abs(s.root)
	if err != nil {
		return nil, err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return nil, fmt.Errorf("artifact path escapes root: %s", artifact.Path)
	}
	return os.ReadFile(cleanPath)
}

func safeArtifactPath(artifact contracts.Artifact) (string, error) {
	sessionID := cleanPathSegment(artifact.SessionID)
	if sessionID == "" {
		sessionID = "sessions"
	}
	name := cleanPathSegment(artifact.Name)
	if name == "" {
		name = cleanPathSegment(artifact.ID)
	}
	if name == "" {
		return "", fmt.Errorf("artifact name or id is required")
	}
	return filepath.Join(sessionID, string(artifact.Type), name), nil
}

func cleanPathSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	value = filepath.Base(value)
	value = strings.Trim(value, ".")
	return value
}
