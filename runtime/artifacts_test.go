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

func TestFileArtifactStoreSaveBytes(t *testing.T) {
	store := NewFileArtifactStore(t.TempDir())
	saved, err := store.SaveArtifactBytes(context.Background(), contracts.Artifact{
		SessionID: "session-1",
		Type:      contracts.ArtifactData,
		Name:      "blob.bin",
		MimeType:  "application/octet-stream",
	}, []byte{0, 1, 2, 255})
	if err != nil {
		t.Fatalf("save bytes: %v", err)
	}
	data, err := store.ReadArtifact(context.Background(), saved)
	if err != nil {
		t.Fatalf("read bytes: %v", err)
	}
	if len(data) != 4 || data[3] != 255 {
		t.Fatalf("unexpected data: %v", data)
	}
}

func TestFileArtifactStoreSaveReader(t *testing.T) {
	store := NewFileArtifactStore(t.TempDir())
	saved, err := store.SaveArtifactReader(context.Background(), contracts.Artifact{
		SessionID: "session-1",
		Type:      contracts.ArtifactDocument,
		Name:      "doc.txt",
	}, strings.NewReader("streamed"))
	if err != nil {
		t.Fatalf("save reader: %v", err)
	}
	data, err := store.ReadArtifact(context.Background(), saved)
	if err != nil {
		t.Fatalf("read reader: %v", err)
	}
	if string(data) != "streamed" {
		t.Fatalf("unexpected data: %q", data)
	}
}
