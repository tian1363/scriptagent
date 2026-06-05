package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
)

type QwenScriptAgent struct {
	client       *model.DashScopeClient
	videoFPS     int
	maxVideoSize int64
}

type QwenConfig struct {
	Client       *model.DashScopeClient
	VideoFPS     int
	MaxVideoSize int64
}

func NewQwenScriptAgent(cfg QwenConfig) *QwenScriptAgent {
	videoFPS := cfg.VideoFPS
	if videoFPS <= 0 {
		videoFPS = 2
	}
	maxVideoSize := cfg.MaxVideoSize
	if maxVideoSize <= 0 {
		maxVideoSize = 80 * 1024 * 1024
	}
	return &QwenScriptAgent{
		client:       cfg.Client,
		videoFPS:     videoFPS,
		maxVideoSize: maxVideoSize,
	}
}

func (a *QwenScriptAgent) Run(ctx context.Context, job jobs.Job) (jobs.ScriptResult, error) {
	if a.client == nil {
		return jobs.ScriptResult{}, errors.New("qwen client is not configured")
	}
	product, err := os.ReadFile(job.ProductMDPath)
	if err != nil {
		return jobs.ScriptResult{}, err
	}
	videoDataURL, err := videoDataURL(job.VideoPath, a.maxVideoSize)
	if err != nil {
		return jobs.ScriptResult{}, err
	}

	analysis, err := a.client.Generate(ctx, []model.ContentItem{
		{Video: videoDataURL, FPS: a.videoFPS},
		{Text: videoAnalysisPrompt(job, string(product))},
	})
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("video analysis failed: %w", err)
	}

	scriptRaw, err := a.client.Generate(ctx, []model.ContentItem{
		{Text: scriptGenerationPrompt(job, string(product), analysis)},
	})
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("script generation failed: %w", err)
	}

	parsed, err := parseScriptBundle(scriptRaw)
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("script response parse failed: %w", err)
	}

	replicaJSON, err := marshalPretty(parsed.ReplicaScript)
	if err != nil {
		return jobs.ScriptResult{}, err
	}
	fissionJSON, err := marshalPretty(parsed.FissionScripts)
	if err != nil {
		return jobs.ScriptResult{}, err
	}

	return jobs.ScriptResult{
		AnalysisMarkdown:   analysis,
		ReplicaScriptJSON:  replicaJSON,
		FissionScriptsJSON: fissionJSON,
	}, nil
}

func videoDataURL(path string, maxSize int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > maxSize {
		return "", fmt.Errorf("video file is %.1fMB, exceeds qwen base64 limit %.1fMB; use a smaller video or configure temporary URL upload later", float64(info.Size())/1024/1024, float64(maxSize)/1024/1024)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = "video/mp4"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func parseScriptBundle(raw string) (*scriptBundle, error) {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```json")
		clean = strings.TrimPrefix(clean, "```")
		clean = strings.TrimSuffix(clean, "```")
		clean = strings.TrimSpace(clean)
	}
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start >= 0 && end > start {
		clean = clean[start : end+1]
	}
	var parsed scriptBundle
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil, err
	}
	if len(parsed.ReplicaScript.Storyboards) == 0 {
		return nil, errors.New("replica_script.storyboards is empty")
	}
	if len(parsed.FissionScripts) == 0 {
		return nil, errors.New("fission_scripts is empty")
	}
	return &parsed, nil
}

type scriptBundle struct {
	ReplicaScript  scriptPayload   `json:"replica_script"`
	FissionScripts []scriptPayload `json:"fission_scripts"`
}
