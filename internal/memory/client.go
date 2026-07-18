package memory

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

const defaultPlatformBaseURL = "https://api.mem0.ai"

type Config struct {
	Provider string
	BaseURL  string
	APIKey   string
	AgentID  string
	TopK     int
	Timeout  time.Duration
}

type Client struct {
	provider   string
	baseURL    string
	apiKey     string
	agentID    string
	topK       int
	httpClient *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SearchResult struct {
	ID       string         `json:"id"`
	Memory   string         `json:"memory"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Status struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider"`
	BaseURL    string `json:"base_url"`
	AgentID    string `json:"agent_id"`
	TopK       int    `json:"top_k"`
}

func NewClient(cfg Config) *Client {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "platform"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" && provider == "platform" {
		baseURL = defaultPlatformBaseURL
	}
	agentID := strings.TrimSpace(cfg.AgentID)
	if agentID == "" {
		agentID = "scriptagent"
	}
	topK := cfg.TopK
	if topK <= 0 || topK > 20 {
		topK = 5
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		provider: provider,
		baseURL:  baseURL,
		apiKey:   strings.TrimSpace(cfg.APIKey),
		agentID:  agentID,
		topK:     topK,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Status() Status {
	return Status{
		Configured: c.Configured(),
		Provider:   c.provider,
		BaseURL:    c.baseURL,
		AgentID:    c.agentID,
		TopK:       c.topK,
	}
}

func (c *Client) Configured() bool {
	if c == nil || c.baseURL == "" {
		return false
	}
	if c.provider == "platform" {
		return c.apiKey != ""
	}
	return c.provider == "oss"
}

func (c *Client) Search(ctx context.Context, userID, query string) ([]SearchResult, error) {
	if !c.Configured() {
		return nil, nil
	}
	userID = strings.TrimSpace(userID)
	query = strings.TrimSpace(query)
	if userID == "" || query == "" {
		return nil, nil
	}

	path := "/v3/memories/search/"
	body := map[string]any{
		"query":   query,
		"filters": map[string]string{"user_id": userID},
		"top_k":   c.topK,
	}
	if c.provider == "oss" {
		path = "/search"
		body = map[string]any{
			"query":    query,
			"user_id":  userID,
			"agent_id": c.agentID,
			"limit":    c.topK,
		}
	}

	raw, err := c.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return parseSearchResults(raw)
}

func (c *Client) Add(ctx context.Context, userID, runID string, messages []Message, metadata map[string]any) error {
	if !c.Configured() {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" || len(messages) == 0 {
		return nil
	}

	path := "/v3/memories/add/"
	body := map[string]any{
		"messages": messages,
		"user_id":  userID,
		"agent_id": c.agentID,
		"metadata": metadata,
		"infer":    true,
	}
	if strings.TrimSpace(runID) != "" {
		body["run_id"] = strings.TrimSpace(runID)
	}
	if c.provider == "oss" {
		path = "/memories"
	}
	_, err := c.doJSON(ctx, http.MethodPost, path, body)
	return err
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	if _, err := url.ParseRequestURI(c.baseURL + path); err != nil {
		return nil, fmt.Errorf("invalid mem0 URL: %w", err)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		if c.provider == "platform" {
			req.Header.Set("Authorization", "Token "+c.apiKey)
		} else {
			req.Header.Set("X-API-Key", c.apiKey)
		}
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("mem0 request failed with status %d: %s", res.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}

func parseSearchResults(raw []byte) ([]SearchResult, error) {
	var envelope struct {
		Results  []SearchResult `json:"results"`
		Memories []SearchResult `json:"memories"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		results := envelope.Results
		if len(results) == 0 {
			results = envelope.Memories
		}
		return normalizeResults(results), nil
	}
	var results []SearchResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, errors.New("mem0 search response has an unsupported shape")
	}
	return normalizeResults(results), nil
}

func normalizeResults(results []SearchResult) []SearchResult {
	normalized := make([]SearchResult, 0, len(results))
	for _, result := range results {
		result.Memory = strings.TrimSpace(result.Memory)
		if result.Memory == "" {
			continue
		}
		normalized = append(normalized, result)
	}
	return normalized
}
