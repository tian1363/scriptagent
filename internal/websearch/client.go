package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTavilyBaseURL = "https://api.tavily.com"

type Config struct {
	Provider   string
	BaseURL    string
	APIKey     string
	MaxResults int
	Timeout    time.Duration
}

type Client struct {
	provider   string
	baseURL    string
	apiKey     string
	maxResults int
	httpClient *http.Client
}

type Query struct {
	Text       string
	Topic      string
	TimeRange  string
	MaxResults int
}

type Result struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Snippet       string  `json:"snippet"`
	PublishedDate string  `json:"published_at,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

type Response struct {
	Provider string   `json:"provider"`
	Query    string   `json:"query"`
	Results  []Result `json:"results"`
}

type Status struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider"`
	BaseURL    string `json:"base_url"`
	MaxResults int    `json:"max_results"`
}

func NewClient(cfg Config) *Client {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "tavily"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" && provider == "tavily" {
		baseURL = defaultTavilyBaseURL
	}
	maxResults := cfg.MaxResults
	if maxResults <= 0 || maxResults > 10 {
		maxResults = 5
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		provider:   provider,
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(cfg.APIKey),
		maxResults: maxResults,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Configured() bool {
	if c == nil || c.baseURL == "" {
		return false
	}
	if c.provider == "tavily" {
		return c.apiKey != ""
	}
	return c.provider == "searxng"
}

func (c *Client) Status() Status {
	return Status{Configured: c.Configured(), Provider: c.provider, BaseURL: c.baseURL, MaxResults: c.maxResults}
}

func (c *Client) Search(ctx context.Context, query Query) (*Response, error) {
	if !c.Configured() {
		return nil, errors.New("web search is not configured")
	}
	query.Text = strings.TrimSpace(query.Text)
	if query.Text == "" {
		return nil, errors.New("search query is required")
	}
	if len([]rune(query.Text)) > 500 {
		return nil, errors.New("search query exceeds 500 characters")
	}
	if query.MaxResults <= 0 || query.MaxResults > c.maxResults {
		query.MaxResults = c.maxResults
	}
	switch c.provider {
	case "tavily":
		return c.searchTavily(ctx, query)
	case "searxng":
		return c.searchSearXNG(ctx, query)
	default:
		return nil, fmt.Errorf("unsupported search provider %q", c.provider)
	}
}

func (c *Client) searchTavily(ctx context.Context, query Query) (*Response, error) {
	body := map[string]any{
		"query":               query.Text,
		"topic":               normalizeTopic(query.Topic),
		"search_depth":        "basic",
		"max_results":         query.MaxResults,
		"include_answer":      false,
		"include_raw_content": false,
		"include_images":      false,
	}
	if value := normalizeTimeRange(query.TimeRange); value != "" {
		body["time_range"] = value
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	responseBody, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Results []struct {
			Title         string  `json:"title"`
			URL           string  `json:"url"`
			Content       string  `json:"content"`
			PublishedDate string  `json:"published_date"`
			Score         float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(payload.Results))
	for _, item := range payload.Results {
		if result, ok := normalizeResult(item.Title, item.URL, item.Content, item.PublishedDate, item.Score); ok {
			results = append(results, result)
		}
	}
	return &Response{Provider: c.provider, Query: query.Text, Results: results}, nil
}

func (c *Client) searchSearXNG(ctx context.Context, query Query) (*Response, error) {
	endpoint, err := url.Parse(c.baseURL + "/search")
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, errors.New("invalid SearXNG base URL")
	}
	values := endpoint.Query()
	values.Set("q", query.Text)
	values.Set("format", "json")
	values.Set("safesearch", "1")
	values.Set("language", "all")
	if value := normalizeSearXNGTimeRange(query.TimeRange); value != "" {
		values.Set("time_range", value)
	}
	endpoint.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	responseBody, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Results []struct {
			Title     string `json:"title"`
			URL       string `json:"url"`
			Content   string `json:"content"`
			Published string `json:"publishedDate"`
		} `json:"results"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, err
	}
	results := make([]Result, 0, min(query.MaxResults, len(payload.Results)))
	for _, item := range payload.Results {
		if len(results) >= query.MaxResults {
			break
		}
		if result, ok := normalizeResult(item.Title, item.URL, item.Content, item.Published, 0); ok {
			results = append(results, result)
		}
	}
	return &Response{Provider: c.provider, Query: query.Text, Results: results}, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("search request failed with status %d: %s", res.StatusCode, truncate(strings.TrimSpace(string(body)), 500))
	}
	return body, nil
}

func normalizeResult(title, rawURL, snippet, published string, score float64) (Result, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return Result{}, false
	}
	title = truncate(strings.TrimSpace(title), 240)
	snippet = truncate(strings.TrimSpace(snippet), 1200)
	if title == "" || snippet == "" {
		return Result{}, false
	}
	return Result{Title: title, URL: parsed.String(), Snippet: snippet, PublishedDate: truncate(strings.TrimSpace(published), 64), Score: score}, true
}

func normalizeTopic(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "news", "finance":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "general"
	}
}

func normalizeTimeRange(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "day", "week", "month", "year":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeSearXNGTimeRange(value string) string {
	switch normalizeTimeRange(value) {
	case "day", "month", "year":
		return normalizeTimeRange(value)
	case "week":
		return "day"
	default:
		return ""
	}
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
