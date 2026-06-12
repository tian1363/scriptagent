package videoprompt

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/tian1363/scriptagent/internal/jobs"
)

const defaultNegativePrompt = "避免画面文字乱码、Logo 变形、产品外观不一致、人物肢体畸形、镜头闪烁、过曝、低清晰度、无关品牌、水印、版权角色、夸大功效表现。"

type scriptDoc struct {
	Title       string
	Dimension   string
	Storyboards []map[string]any
}

func GenerateFromJob(job jobs.Job, source string) (string, error) {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "all"
	}
	docs := []scriptDoc{}
	if source == "all" || source == "replica" {
		replica, err := parseReplica(job.ReplicaScriptJSON)
		if err == nil && len(replica.Storyboards) > 0 {
			docs = append(docs, replica)
		}
	}
	if source == "all" || source == "fission" {
		fissionDocs, err := parseFission(job.FissionScriptsJSON)
		if err == nil {
			docs = append(docs, fissionDocs...)
		}
	}
	if len(docs) == 0 {
		return "", errors.New("no script storyboards available for video prompt generation")
	}
	return render(job, docs), nil
}

func parseReplica(raw string) (scriptDoc, error) {
	root, err := parseObject(raw)
	if err != nil {
		return scriptDoc{}, err
	}
	if nested, ok := objectValue(root["replica_script"]); ok {
		root = nested
	}
	return mapToScript(root, "复刻脚本"), nil
}

func parseFission(raw string) ([]scriptDoc, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty fission script json")
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	items := []any{}
	if root, ok := objectValue(value); ok {
		items, _ = arrayValue(root["fission_scripts"])
	} else if direct, ok := arrayValue(value); ok {
		items = direct
	}
	result := []scriptDoc{}
	for index, item := range items {
		obj, ok := objectValue(item)
		if !ok {
			continue
		}
		result = append(result, mapToScript(obj, fmt.Sprintf("裂变脚本 %02d", index+1)))
	}
	return result, nil
}

func parseObject(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty script json")
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	if obj, ok := objectValue(value); ok {
		return obj, nil
	}
	if items, ok := arrayValue(value); ok && len(items) > 0 {
		if obj, ok := objectValue(items[0]); ok {
			return obj, nil
		}
	}
	return nil, errors.New("script json must be object or array")
}

func mapToScript(obj map[string]any, fallbackTitle string) scriptDoc {
	metadata, _ := objectValue(obj["metadata"])
	title := firstNonEmpty(
		stringValue(obj["title"]),
		stringValue(metadata["title"]),
		fallbackTitle,
	)
	dimension := firstNonEmpty(
		stringValue(metadata["fission_dimension"]),
		stringValue(metadata["industry"]),
		stringValue(obj["fission_dimension"]),
	)
	storyboards := []map[string]any{}
	if items, ok := arrayValue(obj["storyboards"]); ok {
		for _, item := range items {
			if storyboard, ok := objectValue(item); ok {
				storyboards = append(storyboards, storyboard)
			}
		}
	}
	return scriptDoc{Title: title, Dimension: dimension, Storyboards: storyboards}
}

func render(job jobs.Job, docs []scriptDoc) string {
	lines := []string{
		"# Seedance 视频生成提示词",
		"",
		"任务：" + job.Title,
		"产品资料：" + job.ProductMDName,
		"建议规格：竖屏 9:16，短视频广告，画面主体稳定，镜头连续，产品外观全片保持一致。",
		"统一负向提示词：" + defaultNegativePrompt,
		"",
	}
	for scriptIndex, doc := range docs {
		lines = append(lines, fmt.Sprintf("## %02d. %s", scriptIndex+1, doc.Title))
		if strings.TrimSpace(doc.Dimension) != "" {
			lines = append(lines, "脚本标签："+doc.Dimension)
		}
		lines = append(lines,
			"全片连续性：保持同一产品外观、同一视觉风格和前后镜头空间关系；如果出现同一角色、道具或 UI，后续分镜必须沿用一致设定。",
			"",
		)
		for index, storyboard := range doc.Storyboards {
			lines = append(lines, renderStoryboard(index, storyboard)...)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderStoryboard(index int, storyboard map[string]any) []string {
	timeRange := field(storyboard, "time_range", "time", "duration", "timestamp")
	visual := field(storyboard, "visual", "visual_description", "description", "画面描述")
	action := field(storyboard, "action", "movement", "动作描述")
	shotSize := field(storyboard, "shot_size", "景别")
	camera := field(storyboard, "camera_intent", "camera", "镜头动机描述", "camera_movement")
	scene := field(storyboard, "props_scene", "scene", "setting", "道具场景")
	audio := field(storyboard, "audio", "sound", "音效")
	bgm := field(storyboard, "bgm", "BGM")
	voiceover := field(storyboard, "voiceover", "dialogue", "旁白对话", "copy")
	subtitle := field(storyboard, "subtitle", "caption", "字幕")
	purpose := field(storyboard, "purpose", "shot_purpose", "镜头功能")

	promptParts := []string{
		"竖屏 9:16 短视频广告分镜",
		joinLabel("时间", timeRange),
		joinLabel("景别", shotSize),
		joinLabel("画面", visual),
		joinLabel("动作", action),
		joinLabel("场景道具", scene),
		joinLabel("镜头", camera),
		joinLabel("叙事目的", purpose),
		"真实自然的商业广告质感，主体清晰，构图干净，光线稳定，运动连贯",
	}
	positive := compactJoin(promptParts, "；")
	sound := compactJoin([]string{
		joinLabel("旁白", voiceover),
		joinLabel("字幕", subtitle),
		joinLabel("音效", audio),
		joinLabel("BGM", bgm),
	}, "；")
	if sound == "" {
		sound = "无明确声音信息，保持与画面节奏匹配的自然环境声。"
	}

	return []string{
		fmt.Sprintf("### 分镜 %02d%s", index+1, suffixTime(timeRange)),
		"正向提示词：" + positive,
		"声音与字幕：" + sound,
		"负向提示词：" + defaultNegativePrompt,
		"",
	}
}

func field(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(obj[key]); value != "" && value != "-" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", typed), "0"), ".")
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case []any:
		parts := []string{}
		for _, item := range typed {
			if text := stringValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "、")
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := []string{}
		for _, key := range keys {
			if text := stringValue(typed[key]); text != "" {
				parts = append(parts, key+"="+text)
			}
		}
		return strings.Join(parts, "；")
	default:
		return ""
	}
}

func objectValue(value any) (map[string]any, bool) {
	obj, ok := value.(map[string]any)
	return obj, ok
}

func arrayValue(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func joinLabel(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return label + "：" + value
}

func compactJoin(values []string, sep string) string {
	parts := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, strings.TrimSpace(value))
		}
	}
	return strings.Join(parts, sep)
}

func suffixTime(timeRange string) string {
	if strings.TrimSpace(timeRange) == "" {
		return ""
	}
	return "（" + strings.TrimSpace(timeRange) + "）"
}
