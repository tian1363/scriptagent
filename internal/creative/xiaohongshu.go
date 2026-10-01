package creative

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const maxXiaohongshuPageBytes = 2 << 20

type XiaohongshuSnapshot struct {
	Status       string   `json:"status"`
	RequestedURL string   `json:"requested_url"`
	ResolvedURL  string   `json:"resolved_url,omitempty"`
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	Images       []string `json:"images,omitempty"`
	VideoURL     string   `json:"video_url,omitempty"`
	PageText     string   `json:"page_text,omitempty"`
	FetchedAt    string   `json:"fetched_at,omitempty"`
	Warning      string   `json:"warning,omitempty"`
}

func fetchXiaohongshuSnapshot(ctx context.Context, rawURL string) XiaohongshuSnapshot {
	snapshot := XiaohongshuSnapshot{Status: "not_provided", RequestedURL: strings.TrimSpace(rawURL)}
	if snapshot.RequestedURL == "" {
		return snapshot
	}
	if err := validateXiaohongshuURL(ctx, snapshot.RequestedURL); err != nil {
		snapshot.Status, snapshot.Warning = "invalid", err.Error()
		return snapshot
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{Proxy: nil, DialContext: safeDialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("小红书链接重定向次数过多")
			}
			return validateXiaohongshuURL(req.Context(), req.URL.String())
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, snapshot.RequestedURL, nil)
	if err != nil {
		snapshot.Status, snapshot.Warning = "invalid", "小红书链接格式无效"
		return snapshot
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		snapshot.Status, snapshot.Warning = "unavailable", "暂时无法读取该小红书页面，请补充素材备注或上传参考素材"
		return snapshot
	}
	defer resp.Body.Close()
	snapshot.ResolvedURL = resp.Request.URL.String()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snapshot.Status = "unavailable"
		snapshot.Warning = fmt.Sprintf("小红书页面返回 HTTP %d，请检查链接或登录权限", resp.StatusCode)
		return snapshot
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		snapshot.Status, snapshot.Warning = "unavailable", "该链接没有返回可解析的网页内容"
		return snapshot
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxXiaohongshuPageBytes+1))
	if err != nil {
		snapshot.Status, snapshot.Warning = "unavailable", "读取小红书页面失败"
		return snapshot
	}
	if len(body) > maxXiaohongshuPageBytes {
		snapshot.Status, snapshot.Warning = "unavailable", "小红书页面超过解析大小限制"
		return snapshot
	}
	snapshot.Title, snapshot.Description, snapshot.Images, snapshot.VideoURL, snapshot.PageText = parsePageContents(string(body))
	snapshot.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	if !hasNoteMetadata(snapshot) {
		snapshot.Status = "limited"
		snapshot.Warning = "链接可访问，但公开页面没有提供可解析的笔记信息；报告仅使用链接和用户备注"
		return snapshot
	}
	snapshot.Status = "parsed"
	snapshot.Warning = "仅解析公开页面提供的文字和媒体地址；媒体画面、评论及互动表现未自动获取"
	return snapshot
}

func hasNoteMetadata(snapshot XiaohongshuSnapshot) bool {
	if strings.TrimSpace(snapshot.PageText) != "" {
		return true
	}
	title := strings.TrimSpace(snapshot.Title)
	description := strings.TrimSpace(snapshot.Description)
	if description != "" {
		return true
	}
	return title != "" && !strings.Contains(title, "你的生活兴趣社区") && !strings.EqualFold(title, "小红书")
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("无法解析小红书链接域名")
	}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return nil, errors.New("小红书链接解析到了不安全的网络地址")
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	var dialErr error
	for _, address := range addresses {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = err
	}
	return nil, dialErr
}

func validateXiaohongshuURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return errors.New("请输入有效的 HTTPS 小红书链接")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host != "xiaohongshu.com" && !strings.HasSuffix(host, ".xiaohongshu.com") && host != "xhslink.com" && !strings.HasSuffix(host, ".xhslink.com") {
		return errors.New("目前仅支持 xiaohongshu.com 和 xhslink.com 链接")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return errors.New("无法解析小红书链接域名")
	}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return errors.New("小红书链接解析到了不安全的网络地址")
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !ip.IsMulticast()
}

func parsePageMetadata(document string) (string, string, []string) {
	title, description, images, _, _ := parsePageContents(document)
	return title, description, images
}

func parsePageContents(document string) (string, string, []string, string, string) {
	node, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return "", "", nil, "", ""
	}
	var title, description, videoURL string
	images := []string{}
	textParts := []string{}
	seenImages := map[string]bool{}
	var walk func(*html.Node, bool)
	walk = func(current *html.Node, inContent bool) {
		if current.Type == html.ElementNode && (current.Data == "article" || current.Data == "main") {
			inContent = true
		}
		if current.Type == html.TextNode && inContent {
			value := strings.TrimSpace(current.Data)
			if value != "" && len(strings.Join(textParts, "")) < 8000 {
				textParts = append(textParts, value)
			}
		}
		if current.Type == html.ElementNode && (current.Data == "script" || current.Data == "style" || current.Data == "noscript") {
			return
		}
		if current.Type == html.ElementNode && current.Data == "title" && title == "" && current.FirstChild != nil {
			title = strings.TrimSpace(current.FirstChild.Data)
		}
		if current.Type == html.ElementNode && current.Data == "meta" {
			attrs := map[string]string{}
			for _, attr := range current.Attr {
				attrs[strings.ToLower(attr.Key)] = strings.TrimSpace(attr.Val)
			}
			key := strings.ToLower(valueOr(attrs["property"], attrs["name"]))
			content := attrs["content"]
			switch key {
			case "og:title", "twitter:title":
				if content != "" {
					title = content
				}
			case "og:description", "description", "twitter:description":
				if description == "" && content != "" {
					description = content
				}
			case "og:image", "twitter:image":
				if content != "" && !seenImages[content] && len(images) < 6 {
					seenImages[content] = true
					images = append(images, content)
				}
			case "og:video", "og:video:url", "twitter:player:stream":
				if videoURL == "" {
					videoURL = content
				}
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inContent)
		}
	}
	walk(node, false)
	return strings.TrimSpace(title), strings.TrimSpace(description), images, videoURL, truncateRunes(strings.Join(textParts, "\n"), 8000)
}
