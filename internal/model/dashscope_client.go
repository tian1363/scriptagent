package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultDashScopeEndpoint = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"

type DashScopeClient struct {
	apiKey     string
	endpoint   string
	model      string
	httpClient *http.Client
}

type DashScopeConfig struct {
	APIKey   string
	Endpoint string
	Model    string
}

func NewDashScopeClient(cfg DashScopeConfig) *DashScopeClient {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultDashScopeEndpoint
	}
	modelName := cfg.Model
	if modelName == "" {
		modelName = "qwen3.6-plus"
	}
	return &DashScopeClient{
		apiKey:     cfg.APIKey,
		endpoint:   endpoint,
		model:      modelName,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (c *DashScopeClient) Generate(ctx context.Context, content []ContentItem) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("DASHSCOPE_API_KEY is not configured")
	}
	reqBody := requestBody{
		Model: c.model,
		Input: requestInput{
			Messages: []message{
				{
					Role:    "user",
					Content: content,
				},
			},
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 20*1024*1024))
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("dashscope request failed with status %d: %s", res.StatusCode, string(body))
	}

	var parsed responseBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Output.Choices) == 0 {
		return "", errors.New("dashscope response has no choices")
	}
	choice := parsed.Output.Choices[0]
	for _, item := range choice.Message.Content {
		if item.Text != "" {
			return item.Text, nil
		}
	}
	return "", errors.New("dashscope response has no text content")
}

type ContentItem struct {
	Text  string `json:"text,omitempty"`
	Video string `json:"video,omitempty"`
	FPS   int    `json:"fps,omitempty"`
}

type requestBody struct {
	Model string       `json:"model"`
	Input requestInput `json:"input"`
}

type requestInput struct {
	Messages []message `json:"messages"`
}

type message struct {
	Role    string        `json:"role"`
	Content []ContentItem `json:"content"`
}

type responseBody struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
}
