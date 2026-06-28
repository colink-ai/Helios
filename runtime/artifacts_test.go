package runtime

import (
	"context"
	"io"
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

func TestFileArtifactStoreSaveReaderCanceled(t *testing.T) {
	store := NewFileArtifactStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.SaveArtifactReader(ctx, contracts.Artifact{Name: "x.txt"}, strings.NewReader("x")); err == nil {
		t.Fatalf("canceled context should fail")
	}
}

func TestContextReaderStopsAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := contextReader{ctx: ctx, reader: strings.NewReader("hello")}
	buf := make([]byte, 2)
	if _, err := reader.Read(buf); err != nil {
		t.Fatalf("first read: %v", err)
	}
	cancel()
	if _, err := reader.Read(buf); err == nil || err == io.EOF {
		t.Fatalf("expected context error, got %v", err)
	}
}
