package creative

import (
	"context"
	"strings"
	"testing"
)

func TestParsePageMetadataPrefersOpenGraph(t *testing.T) {
	document := `<html><head>
		<title>fallback title</title>
		<meta name="description" content="public summary">
		<meta property="og:title" content="note title">
		<meta property="og:image" content="https://sns-img.example/cover.jpg">
		<meta property="og:image" content="https://sns-img.example/cover.jpg">
	</head></html>`
	title, description, images := parsePageMetadata(document)
	if title != "note title" || description != "public summary" {
		t.Fatalf("unexpected metadata: %q %q", title, description)
	}
	if len(images) != 1 || images[0] != "https://sns-img.example/cover.jpg" {
		t.Fatalf("unexpected images: %#v", images)
	}
}

func TestParsePageContentsKeepsOnlyVisibleArticleText(t *testing.T) {
	document := `<html><head><meta property="og:video" content="https://example.com/video.mp4"></head><body><nav>navigation</nav><article><h1>开箱体验</h1><p>三步展示产品</p><script>ignore me</script></article></body></html>`
	_, _, _, video, pageText := parsePageContents(document)
	if video != "https://example.com/video.mp4" || !strings.Contains(pageText, "开箱体验") || !strings.Contains(pageText, "三步展示产品") || strings.Contains(pageText, "navigation") || strings.Contains(pageText, "ignore me") {
		t.Fatalf("unexpected content: video=%q text=%q", video, pageText)
	}
}

func TestValidateXiaohongshuURLRejectsOtherHostsBeforeFetch(t *testing.T) {
	err := validateXiaohongshuURL(context.Background(), "https://example.com/note/123")
	if err == nil || !strings.Contains(err.Error(), "仅支持") {
		t.Fatalf("expected unsupported host error, got %v", err)
	}
}

func TestValidateXiaohongshuURLRequiresHTTPS(t *testing.T) {
	err := validateXiaohongshuURL(context.Background(), "http://www.xiaohongshu.com/explore/123")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS error, got %v", err)
	}
}

func TestHasNoteMetadataRejectsGenericAppShell(t *testing.T) {
	snapshot := XiaohongshuSnapshot{
		Title:  "小红书 - 你的生活兴趣社区",
		Images: []string{"https://sns-img.example/generic.png"},
	}
	if hasNoteMetadata(snapshot) {
		t.Fatal("generic app shell must not be treated as parsed note content")
	}
}
