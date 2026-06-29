package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// BinaryArtifactStore persists artifacts from bytes or streams.
type BinaryArtifactStore interface {
	SaveArtifactBytes(ctx context.Context, artifact contracts.Artifact, data []byte) (contracts.Artifact, error)
	SaveArtifactReader(ctx context.Context, artifact contracts.Artifact, reader io.Reader) (contracts.Artifact, error)
}

// FileArtifactStore stores artifacts below one root directory.
type FileArtifactStore struct {
	root string
}

func NewFileArtifactStore(root string) *FileArtifactStore {
	return &FileArtifactStore{root: root}
}

func (s *FileArtifactStore) SaveArtifact(ctx context.Context, artifact contracts.Artifact) (contracts.Artifact, error) {
	return s.SaveArtifactBytes(ctx, artifact, []byte(artifact.Content))
}

func (s *FileArtifactStore) SaveArtifactBytes(ctx context.Context, artifact contracts.Artifact, data []byte) (contracts.Artifact, error) {
	return s.SaveArtifactReader(ctx, artifact, bytesReader(data))
}

func (s *FileArtifactStore) SaveArtifactReader(ctx context.Context, artifact contracts.Artifact, reader io.Reader) (contracts.Artifact, error) {
	if s.root == "" {
		return contracts.Artifact{}, fmt.Errorf("artifact root is required")
	}
	if err := ctx.Err(); err != nil {
		return contracts.Artifact{}, err
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
	cleanPath, err := containedArtifactPath(s.root, path)
	if err != nil {
		return contracts.Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return contracts.Artifact{}, err
	}
	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return contracts.Artifact{}, err
	}
	if _, err := io.Copy(file, contextReader{ctx: ctx, reader: reader}); err != nil {
		_ = file.Close()
		return contracts.Artifact{}, err
	}
	if err := file.Close(); err != nil {
		return contracts.Artifact{}, err
	}
	artifact.Path = cleanPath
	artifact.Content = ""
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now().UTC()
	}
	return artifact, nil
}

func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if err != nil {
		return n, err
	}
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, nil
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
	cleanPath, err := containedArtifactPath(s.root, path)
	if err != nil {
		return nil, err
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
	artifactType := cleanPathSegment(string(artifact.Type))
	if artifactType == "" {
		artifactType = string(contracts.ArtifactOther)
	}
	return filepath.Join(sessionID, artifactType, name), nil
}

func containedArtifactPath(root string, path string) (string, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("artifact path escapes root: %s", path)
	}
	return cleanPath, nil
}

func cleanPathSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	value = filepath.Base(value)
	value = strings.Trim(value, ".")
	return value
}
