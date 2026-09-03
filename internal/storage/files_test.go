package storage

import (
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveUploadRejectsSpoofedMedia(t *testing.T) {
	source := filepath.Join(t.TempDir(), "spoof.png")
	if err := os.WriteFile(source, []byte("this is not a png"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, err = NewLocalStore(t.TempDir()).SaveUpload(file, &multipart.FileHeader{Filename: "spoof.png", Size: 17}, "asset")
	if err == nil {
		t.Fatal("spoofed image was accepted")
	}
}

func TestSaveUploadAcceptsPNGSignature(t *testing.T) {
	content := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	source := filepath.Join(t.TempDir(), "valid.png")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	path, err := NewLocalStore(t.TempDir()).SaveUpload(file, &multipart.FileHeader{Filename: "valid.png", Size: int64(len(content))}, "asset")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
