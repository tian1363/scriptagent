package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
)

type MockScriptAgent struct{}

func NewMockScriptAgent() *MockScriptAgent {
	return &MockScriptAgent{}
}

func (a *MockScriptAgent) Run(ctx context.Context, job jobs.Job) (jobs.ScriptResult, error) {
	product, err := os.ReadFile(job.ProductMDPath)
	if err != nil {
		return jobs.ScriptResult{}, err
	}
	productTitle := firstMarkdownTitle(string(product))
	if productTitle == "" {
		productTitle = strings.TrimSuffix(job.ProductMDName, filepath.Ext(job.ProductMDName))
	}

	replica := scriptPayload{
		Title:           productTitle + " 复刻脚本",
		ScriptType:      "replica",
		Industry:        job.Industry,
		DurationSeconds: 24,
		SourceSummary:   "本地开发 mock 输出，用于验证任务、历史记录和前端结果展示流程。",
		Storyboards: []storyboard{
			{
				SceneIndex:   1,
				TimeRange:    "00:00-00:03",
				Visual:       "参考视频首帧结构占位，突出核心产品或游戏主画面。",
				Action:       "主体快速进入画面，建立观众注意力。",
				Voiceover:    "先用一句强钩子提出问题或结果。",
				Subtitle:     "核心钩子字幕",
				ShotSize:     "近景",
				CameraIntent: "强调前三秒吸引力。",
				PropsScene:   "参考视频中的主要场景和道具。",
				Audio:        "节奏型 BGM，关键动作配提示音。",
				Purpose:      "hook",
			},
			{
				SceneIndex:   2,
				TimeRange:    "00:03-00:18",
				Visual:       "按参考视频节奏展示产品卖点、操作过程或玩法反馈。",
				Action:       "连续展示关键动作和效果变化。",
				Voiceover:    "说明产品解决的问题和核心卖点。",
				Subtitle:     "卖点字幕",
				ShotSize:     "中景/特写",
				CameraIntent: "让用户看清解决路径和结果。",
				PropsScene:   "产品、使用场景、对比元素。",
				Audio:        "动作点加强音效。",
				Purpose:      "selling_point",
			},
			{
				SceneIndex:   3,
				TimeRange:    "00:18-00:24",
				Visual:       "展示最终效果和行动提示。",
				Action:       "主体完成操作，画面聚焦结果。",
				Voiceover:    "用简短 CTA 引导下一步。",
				Subtitle:     "立即体验",
				ShotSize:     "特写",
				CameraIntent: "收束信息并推动行动。",
				PropsScene:   "产品结果画面或下载/购买提示。",
				Audio:        "BGM 收束，CTA 音效。",
				Purpose:      "cta",
			},
		},
		Metadata: metadata{
			KeptElements:    []string{"镜头节奏", "三段式结构", "钩子到 CTA 的叙事链路"},
			ChangedElements: []string{"具体台词", "产品信息", "卖点表达"},
		},
	}

	fissions := make([]scriptPayload, 0, job.FissionCount)
	for i := 1; i <= job.FissionCount; i++ {
		item := replica
		item.Title = fmt.Sprintf("%s 裂变脚本 %02d", productTitle, i)
		item.ScriptType = "fission"
		item.SourceSummary = "基于复刻结构生成的开发 mock 裂变脚本。"
		item.Metadata.ParentScriptID = "replica"
		item.Metadata.FissionDimension = fissionDimension(i)
		item.Metadata.ChangedElements = []string{fissionDimension(i), "钩子或卖点措辞"}
		fissions = append(fissions, item)
	}

	replicaJSON, err := marshalPretty(replica)
	if err != nil {
		return jobs.ScriptResult{}, err
	}
	fissionJSON, err := marshalPretty(fissions)
	if err != nil {
		return jobs.ScriptResult{}, err
	}

	analysis := fmt.Sprintf(`# A. 行业判断

当前任务设置行业为：%s。此处为开发阶段 mock 视频理解结果，后续会替换为 qwen3.6-plus 的真实视频理解输出。

## B. 命名统一表

| 类型 | 统一名称 | 首次出现时间 | 描述 |
|---|---|---|---|
| 产品 | %s | 00:00 | 从产品 Markdown 中提取的产品对象 |
| 场景 | 参考视频主场景 | 00:00 | 用户上传视频中的主要画面空间 |

## C. 详细分镜表

| 时间段 | 行业类型 | 产品卖点 | 画面描述（逐帧） | 动作描述 | 视频信息 | 人物角色 | 道具场景 | 旁白/对话 | 景别 | 镜头动机描述 | 叙事节奏描述 | 首帧画面描述 | 音效 | BGM |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 00:00-00:01 | %s | 可基于产品信息推断 | mock 分析占位，后续由模型逐帧描述画面布局、主体、动作、道具和互动关系。 | 主体进入注意力焦点。 | %s | - | 参考视频主场景 | - | 近景 | 强化首帧注意力。 | 钩子 | 首帧核心主体出现。 | - | - |

## D. 核心亮点总结

最适合复刻的是原片的镜头节奏、前三秒钩子和卖点展示路径。本结果生成于 %s，用于验证应用流程。
`,
		html.EscapeString(job.Industry),
		html.EscapeString(productTitle),
		html.EscapeString(job.Industry),
		html.EscapeString(job.VideoOriginalName),
		time.Now().Format(time.RFC3339),
	)

	return jobs.ScriptResult{
		AnalysisMarkdown:   analysis,
		ReplicaScriptJSON:  replicaJSON,
		FissionScriptsJSON: fissionJSON,
	}, nil
}

func firstMarkdownTitle(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func fissionDimension(index int) string {
	dimensions := []string{"钩子裂变", "卖点裂变", "场景裂变", "用户人群裂变", "节奏裂变", "CTA 裂变"}
	return dimensions[(index-1)%len(dimensions)]
}

func marshalPretty(value any) (string, error) {
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type scriptPayload struct {
	Title           string       `json:"title"`
	ScriptType      string       `json:"script_type"`
	Industry        string       `json:"industry"`
	DurationSeconds int          `json:"duration_seconds"`
	SourceSummary   string       `json:"source_summary"`
	Storyboards     []storyboard `json:"storyboards"`
	Metadata        metadata     `json:"metadata"`
}

type storyboard struct {
	SceneIndex   int    `json:"scene_index"`
	TimeRange    string `json:"time_range"`
	Visual       string `json:"visual"`
	Action       string `json:"action"`
	Voiceover    string `json:"voiceover"`
	Subtitle     string `json:"subtitle"`
	ShotSize     string `json:"shot_size"`
	CameraIntent string `json:"camera_intent"`
	PropsScene   string `json:"props_scene"`
	Audio        string `json:"audio"`
	Purpose      string `json:"purpose"`
}

type metadata struct {
	ParentScriptID   string   `json:"parent_script_id"`
	FissionDimension string   `json:"fission_dimension"`
	KeptElements     []string `json:"kept_elements"`
	ChangedElements  []string `json:"changed_elements"`
}
