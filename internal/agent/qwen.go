package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
)

type QwenScriptAgent struct {
	client          *model.DashScopeClient
	videoFPS        int
	maxDataURIBytes int64
}

type QwenConfig struct {
	Client          *model.DashScopeClient
	VideoFPS        int
	MaxDataURIBytes int64
}

const defaultMaxDataURIBytes int64 = 20 * 1024 * 1024

func NewQwenScriptAgent(cfg QwenConfig) *QwenScriptAgent {
	videoFPS := cfg.VideoFPS
	if videoFPS <= 0 {
		videoFPS = 2
	}
	maxDataURIBytes := cfg.MaxDataURIBytes
	if maxDataURIBytes <= 0 {
		maxDataURIBytes = defaultMaxDataURIBytes
	}
	return &QwenScriptAgent{
		client:          cfg.Client,
		videoFPS:        videoFPS,
		maxDataURIBytes: maxDataURIBytes,
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
	videoDataURL, err := videoDataURL(ctx, job.VideoPath, a.maxDataURIBytes, progress)
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

func videoDataURL(ctx context.Context, path string, maxDataURIBytes int64, progress jobs.Progress) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	mimeType := videoMimeType(path)
	if dataURLByteLen(info.Size(), mimeType) <= maxDataURIBytes {
		return readVideoDataURL(path, mimeType, maxDataURIBytes)
	}

	if progress != nil {
		progress(jobs.StatusAnalyzingVideo, fmt.Sprintf("参考视频 %.1fMB 超过 DashScope data-uri 上限，正在自动压缩为临时 MP4。", mb(info.Size())))
	}
	compressedPath, cleanup, err := compressVideoForDataURI(ctx, path, maxDataURIBytes)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return "", err
	}
	compressedInfo, err := os.Stat(compressedPath)
	if err != nil {
		return "", err
	}
	if progress != nil {
		progress(jobs.StatusAnalyzingVideo, fmt.Sprintf("视频压缩完成：%.1fMB -> %.1fMB。", mb(info.Size()), mb(compressedInfo.Size())))
	}
	return readVideoDataURL(compressedPath, "video/mp4", maxDataURIBytes)
}

func readVideoDataURL(path, mimeType string, maxDataURIBytes int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if dataURLByteLen(info.Size(), mimeType) > maxDataURIBytes {
		return "", fmt.Errorf("video data-uri would be %.1fMB, exceeds DashScope data-uri limit %.1fMB; configure OSS/public URL upload or lower the video size", mb(dataURLByteLen(info.Size(), mimeType)), mb(maxDataURIBytes))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func videoMimeType(path string) string {
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = "video/mp4"
	}
	return mimeType
}

func dataURLByteLen(rawBytes int64, mimeType string) int64 {
	return int64(len("data:"+mimeType+";base64,")) + int64(base64.StdEncoding.EncodedLen(int(rawBytes)))
}

func maxRawBytesForDataURI(maxDataURIBytes int64, mimeType string) int64 {
	prefixBytes := int64(len("data:" + mimeType + ";base64,"))
	if maxDataURIBytes <= prefixBytes {
		return 0
	}
	return ((maxDataURIBytes - prefixBytes) / 4) * 3
}

func compressVideoForDataURI(ctx context.Context, inputPath string, maxDataURIBytes int64) (string, func(), error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", nil, errors.New("video exceeds DashScope data-uri limit and ffmpeg is not installed; install ffmpeg or use a smaller video")
	}
	duration := probeVideoDuration(inputPath)
	if duration <= 0 {
		duration = 30
	}
	targetRawBytes := maxRawBytesForDataURI(maxDataURIBytes, "video/mp4") - 768*1024
	if targetRawBytes < 2*1024*1024 {
		return "", nil, errors.New("SCRIPT_AGENT_MAX_DATA_URI_MB is too small for video compression")
	}

	tempDir, err := os.MkdirTemp("", "scriptagent-video-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	profiles := []struct {
		width  int
		fps    int
		factor float64
	}{
		{width: 720, fps: 15, factor: 0.90},
		{width: 540, fps: 12, factor: 0.70},
		{width: 360, fps: 10, factor: 0.50},
	}

	var lastErr error
	var lastSize int64
	for i, profile := range profiles {
		outputPath := filepath.Join(tempDir, fmt.Sprintf("compressed-%d.mp4", i+1))
		totalKbps := int(float64(targetRawBytes*8) / duration.Seconds() / 1000 * profile.factor)
		if totalKbps < 260 {
			totalKbps = 260
		}
		audioKbps := 48
		videoKbps := totalKbps - audioKbps
		if videoKbps < 180 {
			videoKbps = 180
		}

		cmdCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
		args := []string{
			"-nostdin", "-y", "-hide_banner",
			"-i", inputPath,
			"-vf", fmt.Sprintf("scale=trunc(min(iw\\,%d)/2)*2:-2,fps=%d", profile.width, profile.fps),
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-b:v", fmt.Sprintf("%dk", videoKbps),
			"-maxrate", fmt.Sprintf("%dk", videoKbps),
			"-bufsize", fmt.Sprintf("%dk", videoKbps*2),
			"-c:a", "aac",
			"-b:a", fmt.Sprintf("%dk", audioKbps),
			"-movflags", "+faststart",
			"-pix_fmt", "yuv420p",
			outputPath,
		}
		output, err := exec.CommandContext(cmdCtx, ffmpegPath, args...).CombinedOutput()
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("ffmpeg compression failed: %w: %s", err, strings.TrimSpace(string(output)))
			continue
		}
		info, err := os.Stat(outputPath)
		if err != nil {
			lastErr = err
			continue
		}
		lastSize = info.Size()
		if dataURLByteLen(info.Size(), "video/mp4") <= maxDataURIBytes {
			return outputPath, cleanup, nil
		}
		lastErr = fmt.Errorf("compressed video data-uri is still too large: %.1fMB", mb(dataURLByteLen(info.Size(), "video/mp4")))
	}

	cleanup()
	if lastErr != nil {
		if lastSize > 0 {
			return "", nil, fmt.Errorf("%w; final compressed file size %.1fMB", lastErr, mb(lastSize))
		}
		return "", nil, lastErr
	}
	return "", nil, errors.New("video compression failed")
}

func probeVideoDuration(path string) time.Duration {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func mb(bytes int64) float64 {
	return float64(bytes) / 1024 / 1024
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
	selected := selectedFissionDirectionList(selectedDirections)
	if len(selected) > 0 {
		if len(selected) != len(scripts) {
			return fmt.Errorf("selected fission directions count %d does not match scripts count %d", len(selected), len(scripts))
		}
		known := allowedFissionDirections("")
		for i, script := range scripts {
			dimension := strings.TrimSpace(script.Metadata.FissionDimension)
			if dimension == "" {
				return fmt.Errorf("fission script %d has empty fission_dimension", i+1)
			}
			if !known[dimension] {
				return fmt.Errorf("fission script %d uses invalid or mixed fission_dimension: %s", i+1, dimension)
			}
			if dimension != selected[i] {
				return fmt.Errorf("fission script %d must use selected fission_dimension %s, got %s", i+1, selected[i], dimension)
			}
		}
		return nil
	}

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

func selectedFissionDirectionList(selectedDirections string) []string {
	values := strings.Split(strings.TrimSpace(selectedDirections), "\n")
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
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
