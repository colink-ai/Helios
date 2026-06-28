package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/colink-ai/helios/contracts"
)

func TestFileArtifactStoreSaveAndRead(t *testing.T) {
	store := NewFileArtifactStore(t.TempDir())
	saved, err := store.SaveArtifact(context.Background(), contracts.Artifact{
		SessionID: "session-1",
		Type:      contracts.ArtifactCode,
		Name:      "patch.diff",
		Content:   "diff --git",
	})
	if err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	if saved.ID == "" || saved.Path == "" || saved.Content != "" {
		t.Fatalf("unexpected saved artifact: %+v", saved)
	}
	data, err := store.ReadArtifact(context.Background(), saved)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(data) != "diff --git" {
		t.Fatalf("data = %q", data)
	}
}

func TestFileArtifactStoreSanitizesPathSegments(t *testing.T) {
	root := t.TempDir()
	store := NewFileArtifactStore(root)
	saved, err := store.SaveArtifact(context.Background(), contracts.Artifact{
		SessionID: "../escape",
		Type:      contracts.ArtifactDocument,
		Name:      "../secret.md",
		Content:   "safe",
	})
	if err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	if !strings.HasPrefix(saved.Path, root) || strings.Contains(saved.Path, "..") {
		t.Fatalf("unsafe path: %s", saved.Path)
	}
}

func TestFileArtifactStoreRejectsReadOutsideRoot(t *testing.T) {
	store := NewFileArtifactStore(t.TempDir())
	if _, err := store.ReadArtifact(context.Background(), contracts.Artifact{Path: "/tmp/outside"}); err == nil {
		t.Fatalf("outside read should fail")
	}
}
