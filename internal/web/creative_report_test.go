package web

import "testing"

func TestParseMaterialLinks(t *testing.T) {
	links, err := parseMaterialLinks("https://example.com/a\nhttps://example.org/b, https://example.net/c")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(links))
	}
}

func TestParseMaterialLinksRejectsUnsafeScheme(t *testing.T) {
	if _, err := parseMaterialLinks("file:///etc/passwd"); err == nil {
		t.Fatal("expected unsafe URL scheme to be rejected")
	}
}
