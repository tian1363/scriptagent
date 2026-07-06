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

const (
	DefaultDashScopeEndpoint          = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	DefaultDashScopeEmbeddingEndpoint = "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding"
)

type DashScopeClient struct {
	apiKey              string
	endpoint            string
	model               string
	embeddingEndpoint   string
	embeddingModel      string
	embeddingDimensions int
	httpClient          *http.Client
	recorder            Recorder
	provider            ConfigProvider
}

type DashScopeConfig struct {
	APIKey              string
	Endpoint            string
	Model               string
	EmbeddingEndpoint   string
	EmbeddingModel      string
	EmbeddingDimensions int
	Recorder            Recorder
	Provider            ConfigProvider
}

type RuntimeConfig struct {
	APIKey   string
	Endpoint string
	Model    string
	Source   string
}

type ConfigProvider interface {
	GetModelRuntimeConfig(ctx context.Context) (RuntimeConfig, error)
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
	embeddingEndpoint := cfg.EmbeddingEndpoint
	if embeddingEndpoint == "" {
		embeddingEndpoint = DefaultDashScopeEmbeddingEndpoint
	}
	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-v4"
	}
	embeddingDimensions := cfg.EmbeddingDimensions
	if embeddingDimensions <= 0 {
		embeddingDimensions = 1024
	}
	return &DashScopeClient{
		apiKey:              cfg.APIKey,
		endpoint:            endpoint,
		model:               modelName,
		embeddingEndpoint:   embeddingEndpoint,
		embeddingModel:      embeddingModel,
		embeddingDimensions: embeddingDimensions,
		httpClient:          &http.Client{Timeout: 10 * time.Minute},
		recorder:            cfg.Recorder,
		provider:            cfg.Provider,
	}
}

func (c *DashScopeClient) Generate(ctx context.Context, content []ContentItem) (string, error) {
	result, err := c.GenerateDetailed(ctx, CallContext{}, content)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (c *DashScopeClient) GenerateDetailed(ctx context.Context, callCtx CallContext, content []ContentItem) (Generation, error) {
	runtime := c.runtimeConfig(ctx)
	if runtime.APIKey == "" {
		return Generation{}, errors.New("DASHSCOPE_API_KEY is not configured")
	}
	reqBody := requestBody{
		Model: runtime.Model,
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
		return Generation{}, err
	}
	logRaw, err := json.Marshal(sanitizedRequestBody(reqBody))
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
		c.record(ctx, callCtx, runtime.Model, logRaw, nil, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return Generation{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 20*1024*1024))
	if err != nil {
		c.record(ctx, callCtx, runtime.Model, logRaw, nil, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return Generation{}, err
	}
	logBody := sanitizedResponseJSON(body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		err := fmt.Errorf("dashscope request failed with status %d: %s", res.StatusCode, string(body))
		c.record(ctx, callCtx, runtime.Model, logRaw, logBody, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return Generation{}, err
	}

	var parsed responseBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		c.record(ctx, callCtx, runtime.Model, logRaw, logBody, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return Generation{}, err
	}
	if len(parsed.Output.Choices) == 0 {
		err := errors.New("dashscope response has no choices")
		c.record(ctx, callCtx, runtime.Model, logRaw, logBody, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return Generation{}, err
	}
	choice := parsed.Output.Choices[0]
	for _, item := range choice.Message.Content {
		if item.Text != "" {
			result := Generation{
				Text:         item.Text,
				Model:        runtime.Model,
				RequestJSON:  string(logRaw),
				ResponseJSON: string(logBody),
				PromptTokens: parsed.Usage.promptTokens(),
				OutputTokens: parsed.Usage.outputTokens(),
				TotalTokens:  parsed.Usage.totalTokens(),
				LatencyMS:    elapsedMS(started),
			}
			c.record(ctx, callCtx, runtime.Model, logRaw, logBody, result, nil)
			return result, nil
		}
	}
	err = errors.New("dashscope response has no text content")
	c.record(ctx, callCtx, runtime.Model, logRaw, logBody, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
	return Generation{}, err
}

func (c *DashScopeClient) runtimeConfig(ctx context.Context) RuntimeConfig {
	runtime := RuntimeConfig{
		APIKey:   c.apiKey,
		Endpoint: c.endpoint,
		Model:    c.model,
		Source:   "env",
	}
	if c.provider != nil {
		if provided, err := c.provider.GetModelRuntimeConfig(ctx); err == nil && provided.APIKey != "" {
			runtime.APIKey = provided.APIKey
			runtime.Endpoint = valueOr(provided.Endpoint, runtime.Endpoint)
			runtime.Model = valueOr(provided.Model, runtime.Model)
			runtime.Source = valueOr(provided.Source, "user")
		}
	}
	return runtime
}

func (c *DashScopeClient) EmbedDetailed(ctx context.Context, callCtx CallContext, texts []string, textType string) (EmbeddingGeneration, error) {
	runtime := c.runtimeConfig(ctx)
	if runtime.APIKey == "" {
		return EmbeddingGeneration{}, errors.New("DASHSCOPE_API_KEY is not configured")
	}
	if len(texts) == 0 {
		return EmbeddingGeneration{}, errors.New("embedding texts are required")
	}
	if len(texts) > 10 {
		return EmbeddingGeneration{}, errors.New("embedding texts cannot exceed 10 per request")
	}
	if textType == "" {
		textType = "document"
	}
	reqBody := embeddingRequestBody{
		Model: c.embeddingModel,
		Input: embeddingRequestInput{Texts: texts},
		Parameters: embeddingParameters{
			TextType:   textType,
			Dimension:  c.embeddingDimensions,
			OutputType: "dense",
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return EmbeddingGeneration{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.embeddingEndpoint, bytes.NewReader(raw))
	if err != nil {
		return EmbeddingGeneration{}, err
	}
	req.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	req.Header.Set("Content-Type", "application/json")

	started := time.Now()
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.recordEmbedding(ctx, callCtx, reqBody, EmbeddingGeneration{Model: c.embeddingModel, LatencyMS: elapsedMS(started)}, err)
		return EmbeddingGeneration{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 20*1024*1024))
	if err != nil {
		c.recordEmbedding(ctx, callCtx, reqBody, EmbeddingGeneration{Model: c.embeddingModel, LatencyMS: elapsedMS(started)}, err)
		return EmbeddingGeneration{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		err := fmt.Errorf("dashscope embedding request failed with status %d: %s", res.StatusCode, string(body))
		c.recordEmbedding(ctx, callCtx, reqBody, EmbeddingGeneration{Model: c.embeddingModel, LatencyMS: elapsedMS(started)}, err)
		return EmbeddingGeneration{}, err
	}
	var parsed embeddingResponseBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		c.recordEmbedding(ctx, callCtx, reqBody, EmbeddingGeneration{Model: c.embeddingModel, LatencyMS: elapsedMS(started)}, err)
		return EmbeddingGeneration{}, err
	}
	if len(parsed.Output.Embeddings) == 0 {
		err := errors.New("dashscope embedding response has no embeddings")
		c.recordEmbedding(ctx, callCtx, reqBody, EmbeddingGeneration{Model: c.embeddingModel, LatencyMS: elapsedMS(started)}, err)
		return EmbeddingGeneration{}, err
	}
	vectors := make([]EmbeddingResult, 0, len(parsed.Output.Embeddings))
	for _, item := range parsed.Output.Embeddings {
		vectors = append(vectors, EmbeddingResult{Index: item.TextIndex, Vector: item.Embedding})
	}
	result := EmbeddingGeneration{
		Model:       c.embeddingModel,
		Vectors:     vectors,
		TotalTokens: parsed.Usage.TotalTokens,
		LatencyMS:   elapsedMS(started),
	}
	c.recordEmbedding(ctx, callCtx, reqBody, result, nil)
	return result, nil
}

type ContentItem struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
	Video string `json:"video,omitempty"`
	FPS   int    `json:"fps,omitempty"`
}

type CallContext struct {
	Scope string
	RefID string
	Step  string
}

type Generation struct {
	Text         string `json:"text"`
	Model        string `json:"model"`
	RequestJSON  string `json:"request_json"`
	ResponseJSON string `json:"response_json"`
	PromptTokens int    `json:"prompt_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	LatencyMS    int64  `json:"latency_ms"`
}

type EmbeddingResult struct {
	Index  int
	Vector []float64
}

type EmbeddingGeneration struct {
	Model       string
	Vectors     []EmbeddingResult
	TotalTokens int
	LatencyMS   int64
}

type CallRecord struct {
	Scope        string
	RefID        string
	Step         string
	Model        string
	InputJSON    string
	OutputText   string
	ResponseJSON string
	PromptTokens int
	OutputTokens int
	TotalTokens  int
	LatencyMS    int64
	ErrorMessage string
}

type Recorder interface {
	RecordModelCall(ctx context.Context, record CallRecord) error
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
	Usage usage `json:"usage"`
}

type embeddingRequestBody struct {
	Model      string                `json:"model"`
	Input      embeddingRequestInput `json:"input"`
	Parameters embeddingParameters   `json:"parameters"`
}

type embeddingRequestInput struct {
	Texts []string `json:"texts"`
}

type embeddingParameters struct {
	TextType   string `json:"text_type"`
	Dimension  int    `json:"dimension"`
	OutputType string `json:"output_type"`
}

type embeddingResponseBody struct {
	Output struct {
		Embeddings []struct {
			Embedding []float64 `json:"embedding"`
			TextIndex int       `json:"text_index"`
		} `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

type usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (u usage) promptTokens() int {
	if u.InputTokens > 0 {
		return u.InputTokens
	}
	return u.PromptTokens
}

func (u usage) outputTokens() int {
	if u.OutputTokens > 0 {
		return u.OutputTokens
	}
	return u.CompletionTokens
}

func (u usage) totalTokens() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.promptTokens() + u.outputTokens()
}

func sanitizedRequestBody(body requestBody) requestBody {
	copied := body
	copied.Input.Messages = make([]message, 0, len(body.Input.Messages))
	for _, msg := range body.Input.Messages {
		next := msg
		next.Content = make([]ContentItem, 0, len(msg.Content))
		for _, item := range msg.Content {
			if item.Image != "" {
				item.Image = "[image omitted from log]"
			}
			if item.Video != "" {
				item.Video = "[video omitted from log]"
			}
			next.Content = append(next.Content, item)
		}
		copied.Input.Messages = append(copied.Input.Messages, next)
	}
	return copied
}

func sanitizedResponseJSON(raw []byte) []byte {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	redactReasoning(value)
	out, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return out
}

func redactReasoning(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "reasoning_content" {
				typed[key] = "[redacted]"
				continue
			}
			redactReasoning(child)
		}
	case []any:
		for _, child := range typed {
			redactReasoning(child)
		}
	}
}

func (c *DashScopeClient) record(ctx context.Context, callCtx CallContext, modelName string, req, res []byte, result Generation, err error) {
	if c.recorder == nil {
		return
	}
	record := CallRecord{
		Scope:        callCtx.Scope,
		RefID:        callCtx.RefID,
		Step:         callCtx.Step,
		Model:        modelName,
		InputJSON:    string(req),
		OutputText:   result.Text,
		ResponseJSON: string(res),
		PromptTokens: result.PromptTokens,
		OutputTokens: result.OutputTokens,
		TotalTokens:  result.TotalTokens,
		LatencyMS:    result.LatencyMS,
	}
	if err != nil {
		record.ErrorMessage = err.Error()
	}
	_ = c.recorder.RecordModelCall(ctx, record)
}

func (c *DashScopeClient) recordEmbedding(ctx context.Context, callCtx CallContext, req embeddingRequestBody, result EmbeddingGeneration, err error) {
	if c.recorder == nil {
		return
	}
	reqJSON, _ := json.Marshal(req)
	response := map[string]any{
		"embedding_count": len(result.Vectors),
		"dimensions":      c.embeddingDimensions,
		"total_tokens":    result.TotalTokens,
	}
	responseJSON, _ := json.Marshal(response)
	record := CallRecord{
		Scope:        callCtx.Scope,
		RefID:        callCtx.RefID,
		Step:         callCtx.Step,
		Model:        result.Model,
		InputJSON:    string(reqJSON),
		OutputText:   fmt.Sprintf("%d embedding vector(s)", len(result.Vectors)),
		ResponseJSON: string(responseJSON),
		PromptTokens: result.TotalTokens,
		TotalTokens:  result.TotalTokens,
		LatencyMS:    result.LatencyMS,
	}
	if err != nil {
		record.ErrorMessage = err.Error()
	}
	_ = c.recorder.RecordModelCall(ctx, record)
}

func elapsedMS(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}

func valueOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
