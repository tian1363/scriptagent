package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// generateOpenAICompatible supports the OpenAI Chat Completions protocol,
// which is also offered by most gateway and self-hosted model vendors.
func (c *DashScopeClient) generateOpenAICompatible(ctx context.Context, callCtx CallContext, runtime RuntimeConfig, content []ContentItem) (Generation, error) {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		if item.Video != "" {
			return Generation{}, errors.New("当前通用兼容接口不支持本地视频理解；请使用 DashScope 视频模型，或只发送文本任务")
		}
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqBody := struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}{Model: runtime.Model, Messages: []message{{Role: "user", Content: strings.Join(parts, "\n")}}}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return Generation{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, runtime.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return Generation{}, err
	}
	req.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	res, err := c.httpClient.Do(req)
	if err != nil {
		return Generation{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 20*1024*1024))
	if err != nil {
		return Generation{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Generation{}, fmt.Errorf("OpenAI 兼容接口请求失败 (%d)：%s", res.StatusCode, string(body))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Generation{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return Generation{}, errors.New("OpenAI 兼容接口没有返回文本")
	}
	result := Generation{Text: parsed.Choices[0].Message.Content, Model: runtime.Model, RequestJSON: string(raw), ResponseJSON: string(body), PromptTokens: parsed.Usage.PromptTokens, OutputTokens: parsed.Usage.CompletionTokens, TotalTokens: parsed.Usage.TotalTokens, LatencyMS: elapsedMS(started)}
	c.record(ctx, callCtx, runtime.Model, raw, body, result, nil)
	return result, nil
}
