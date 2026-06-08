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

func (a *QwenScriptAgent) Run(ctx context.Context, job jobs.Job, progress jobs.Progress) (jobs.ScriptResult, error) {
	if a.client == nil {
		return jobs.ScriptResult{}, errors.New("qwen client is not configured")
	}
	progress(jobs.StatusAnalyzingVideo, "开始读取产品 Markdown 和参考视频。")
	product, err := os.ReadFile(job.ProductMDPath)
	if err != nil {
		return jobs.ScriptResult{}, err
	}
	videoDataURL, err := videoDataURL(job.VideoPath, a.maxVideoSize)
	if err != nil {
		return jobs.ScriptResult{}, err
	}

	progress(jobs.StatusAnalyzingVideo, "开始调用 qwen3.6-plus 进行视频理解。")
	analysisResult, err := a.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "job",
		RefID: job.ID,
		Step:  "video_analysis",
	}, []model.ContentItem{
		{Video: videoDataURL, FPS: a.videoFPS},
		{Text: videoAnalysisPrompt(job, string(product))},
	})
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("video analysis failed: %w", err)
	}
	analysis := analysisResult.Text

	progress(jobs.StatusExtractingStructure, "视频理解完成，准备生成复刻结构。")
	progress(jobs.StatusGeneratingReplica, "开始调用 qwen3.6-plus 生成复刻脚本。")
	replicaResult, err := a.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "job",
		RefID: job.ID,
		Step:  "replica_script",
	}, []model.ContentItem{
		{Text: replicaScriptPrompt(job, string(product), analysis)},
	})
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("replica script generation failed: %w", err)
	}
	replicaRaw := replicaResult.Text

	progress(jobs.StatusValidating, "模型已返回复刻脚本，正在解析 JSON 并校验结构。")
	replica, err := parseReplicaScript(replicaRaw)
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("replica script response parse failed: %w", err)
	}
	replicaJSON, err := marshalPretty(replica)
	if err != nil {
		return jobs.ScriptResult{}, err
	}

	progress(jobs.StatusGeneratingFission, "开始基于复刻脚本生成裂变脚本。")
	fissionResult, err := a.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "job",
		RefID: job.ID,
		Step:  "fission_scripts",
	}, []model.ContentItem{
		{Text: fissionScriptPrompt(job, string(product), analysis, replicaJSON)},
	})
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("fission script generation failed: %w", err)
	}
	fissionRaw := fissionResult.Text

	progress(jobs.StatusValidating, "模型已返回裂变脚本，正在解析 JSON 并校验结构。")
	fissions, err := parseFissionScripts(fissionRaw)
	if err != nil {
		return jobs.ScriptResult{}, fmt.Errorf("fission script response parse failed: %w", err)
	}
	if len(fissions) != job.FissionCount {
		return jobs.ScriptResult{}, fmt.Errorf("fission scripts count mismatch: want %d, got %d", job.FissionCount, len(fissions))
	}
	if err := validateFissionDimensions(fissions, job.FissionDirections); err != nil {
		return jobs.ScriptResult{}, err
	}
	fissionJSON, err := marshalPretty(fissions)
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

func parseReplicaScript(raw string) (scriptPayload, error) {
	var wrapped struct {
		ReplicaScript scriptPayload `json:"replica_script"`
	}
	if err := parseJSON(raw, &wrapped); err == nil && len(wrapped.ReplicaScript.Storyboards) > 0 {
		return wrapped.ReplicaScript, nil
	}
	var script scriptPayload
	if err := parseJSON(raw, &script); err != nil {
		return scriptPayload{}, err
	}
	if len(script.Storyboards) == 0 {
		return scriptPayload{}, errors.New("replica script storyboards is empty")
	}
	return script, nil
}

func parseFissionScripts(raw string) ([]scriptPayload, error) {
	var wrapped struct {
		FissionScripts []scriptPayload `json:"fission_scripts"`
	}
	if err := parseJSON(raw, &wrapped); err == nil && len(wrapped.FissionScripts) > 0 {
		return wrapped.FissionScripts, nil
	}
	var scripts []scriptPayload
	if err := parseJSON(raw, &scripts); err != nil {
		return nil, err
	}
	if len(scripts) == 0 {
		return nil, errors.New("fission scripts is empty")
	}
	return scripts, nil
}

func parseJSON(raw string, target any) error {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```json")
		clean = strings.TrimPrefix(clean, "```")
		clean = strings.TrimSuffix(clean, "```")
		clean = strings.TrimSpace(clean)
	}
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if strings.HasPrefix(clean, "[") {
		start = strings.Index(clean, "[")
		end = strings.LastIndex(clean, "]")
	}
	if start >= 0 && end > start {
		clean = clean[start : end+1]
	}
	return json.Unmarshal([]byte(clean), target)
}

func validateFissionDimensions(scripts []scriptPayload, selectedDirections string) error {
	allowed := allowedFissionDirections(selectedDirections)
	for i, script := range scripts {
		dimension := strings.TrimSpace(script.Metadata.FissionDimension)
		if dimension == "" {
			return fmt.Errorf("fission script %d has empty fission_dimension", i+1)
		}
		if !allowed[dimension] {
			return fmt.Errorf("fission script %d uses invalid or mixed fission_dimension: %s", i+1, dimension)
		}
	}
	return nil
}

func allowedFissionDirections(selectedDirections string) map[string]bool {
	values := strings.Split(strings.TrimSpace(selectedDirections), "\n")
	if strings.TrimSpace(selectedDirections) == "" {
		values = allFissionDirections()
	}
	allowed := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			allowed[value] = true
		}
	}
	return allowed
}

func allFissionDirections() []string {
	return []string{
		"视听层-换BGM",
		"视听层-换音效",
		"视听层-换色调/滤镜",
		"视听层-换字幕&花字",
		"视听层-换画幅",
		"视听层-换配音(语速/声线)",
		"结构层-换开头钩子",
		"结构层-换CTA",
		"结构层-时长压缩/拉伸",
		"结构层-变速·节奏调整",
		"结构层-换首帧/封面",
		"结构层-同素材高光重剪",
		"元素层-换局部角色/群演",
		"元素层-换局部场景贴片",
		"元素层-换局部道具/UI",
		"元素层-字幕语言本地化",
	}
}
