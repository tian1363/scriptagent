package agent

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataURLByteLenIncludesBase64Expansion(t *testing.T) {
	rawSize := int64(3)
	want := int64(len("data:video/mp4;base64,")) + int64(len(base64.StdEncoding.EncodeToString([]byte{1, 2, 3})))
	if got := dataURLByteLen(rawSize, "video/mp4"); got != want {
		t.Fatalf("expected %d bytes, got %d", want, got)
	}
}

func TestMaxRawBytesForDataURIFitsLimit(t *testing.T) {
	limit := int64(20 * 1024 * 1024)
	rawSize := maxRawBytesForDataURI(limit, "video/mp4")
	if dataURLByteLen(rawSize, "video/mp4") > limit {
		t.Fatalf("expected raw size %d to fit data-uri limit %d", rawSize, limit)
	}
	if dataURLByteLen(rawSize+3, "video/mp4") <= limit {
		t.Fatalf("expected raw size %d to exceed data-uri limit %d", rawSize+3, limit)
	}
}

func TestVideoDataURLUsesOriginalWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.mp4")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}

	value, err := videoDataURL(context.Background(), path, 1024, nil)
	if err != nil {
		t.Fatalf("expected small video to pass: %v", err)
	}
	if want := "data:video/mp4;base64,AQID"; value != want {
		t.Fatalf("expected %q, got %q", want, value)
	}
}

func TestVideoDataURLCompressesWhenOverLimit(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.mp4")
	args := []string{
		"-nostdin", "-y", "-hide_banner",
		"-f", "lavfi",
		"-i", "testsrc2=duration=4:size=1280x720:rate=30",
		"-an",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-b:v", "16000k",
		sourcePath,
	}
	if output, err := exec.Command(ffmpegPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("create test video: %v: %s", err, strings.TrimSpace(string(output)))
	}

	limit := int64(6 * 1024 * 1024)
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if dataURLByteLen(info.Size(), "video/mp4") <= limit {
		t.Skip("generated video did not exceed test data-uri limit")
	}

	value, err := videoDataURL(context.Background(), sourcePath, limit, nil)
	if err != nil {
		t.Fatalf("expected oversized video to be compressed: %v", err)
	}
	if int64(len(value)) > limit {
		t.Fatalf("expected compressed data-uri to fit %d bytes, got %d", limit, len(value))
	}
	if !strings.HasPrefix(value, "data:video/mp4;base64,") {
		t.Fatalf("expected mp4 data-uri, got prefix %.24q", value)
	}
}
