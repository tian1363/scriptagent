package web

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestParseProductDocumentMarkdown(t *testing.T) {
	got, err := parseProductDocument("产品资料.md", []byte("# 玉米产品\n\n核心卖点"))
	if err != nil || !strings.Contains(got, "核心卖点") {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestParseProductDocumentDOCX(t *testing.T) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	entry, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="urn:test"><w:body><w:p><w:r><w:t>第一段产品资料</w:t></w:r></w:p><w:p><w:r><w:t>第二段卖点</w:t></w:r></w:p></w:body></w:document>`))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := parseProductDocument("产品资料.docx", data.Bytes())
	if err != nil || !strings.Contains(got, "第一段产品资料\n第二段卖点") {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestParseProductDocumentRejectsUnsupportedFile(t *testing.T) {
	if _, err := parseProductDocument("产品资料.xlsx", []byte("data")); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
