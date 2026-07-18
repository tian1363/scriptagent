package creative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentItemsBuildsImageInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.png")
	if err := os.WriteFile(path, []byte("small-image"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := contentItems([]Material{{Name: "reference.png", Path: path, Kind: "png"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.HasPrefix(items[0].Image, "data:image/png;base64,") {
		t.Fatalf("unexpected content items: %+v", items)
	}
}

func TestContentItemsRejectsUnsupportedType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.txt")
	if err := os.WriteFile(path, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := contentItems([]Material{{Name: "reference.txt", Path: path}}); err == nil {
		t.Fatal("expected unsupported material error")
	}
}
