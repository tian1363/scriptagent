package creative

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
)

const maxProductMarkdownForReport = 12000

type DataEyeConfig struct {
	SourceType   string `json:"source_type"`
	DataEyeURL   string `json:"dataeye_url"`
	DataEyeID    string `json:"dataeye_id"`
	ProductName  string `json:"product_name"`
	DateRange    string `json:"date_range"`
	Media        string `json:"media"`
	Country      string `json:"country"`
	SortMetric   string `json:"sort_metric"`
	SampleCount  int    `json:"sample_count"`
	Requirement  string `json:"requirement"`
	MaterialNote string `json:"material_note"`
}

type Service struct {
	store  *jobs.Store
	client *model.DashScopeClient
}

func NewService(store *jobs.Store, client *model.DashScopeClient) *Service {
	return &Service{store: store, client: client}
}

func (s *Service) GenerateReport(ctx context.Context, userID, productID string, config DataEyeConfig) (*jobs.CreativeReport, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("creative report service is not configured")
	}
	if s.client == nil {
		return nil, errors.New("model client is not configured")
	}
	product, err := s.store.GetProduct(strings.TrimSpace(userID), strings.TrimSpace(productID))
	if err != nil {
		return nil, err
	}
	markdownBytes, err := os.ReadFile(product.MDPath)
	if err != nil {
		return nil, fmt.Errorf("read product Markdown: %w", err)
	}
	config = normalizeConfig(config, product.Title)
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	result, err := s.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "creative_report",
		RefID: product.ID,
		Step:  "generate_strategy_report",
	}, []model.ContentItem{{Text: reportPrompt(*product, string(markdownBytes), string(configJSON))}})
	if err != nil {
		return nil, err
	}
	reportMarkdown := strings.TrimSpace(result.Text)
	if reportMarkdown == "" {
		return nil, errors.New("creative report is empty")
	}
	return s.store.CreateCreativeReport(jobs.CreateCreativeReportInput{
		UserID:           userID,
		ProductID:        product.ID,
		ProductTitle:     product.Title,
		SourceConfigJSON: string(configJSON),
		ReportMarkdown:   reportMarkdown,
		ReportSummary:    summarizeReport(reportMarkdown),
	})
}

func normalizeConfig(config DataEyeConfig, productTitle string) DataEyeConfig {
	config.SourceType = valueOr(config.SourceType, "dataeye")
	config.ProductName = valueOr(config.ProductName, productTitle)
	config.DateRange = valueOr(config.DateRange, "近 30 天")
	config.SortMetric = valueOr(config.SortMetric, "热度/曝光/播放优先")
	if config.SampleCount <= 0 {
		config.SampleCount = 50
	}
	return config
}

func reportPrompt(product jobs.Product, markdown, configJSON string) string {
	return strings.Join([]string{
		"你是 ScriptAgent 的创意策略分析 Agent，服务短视频运营和广告素材团队。",
		"任务：基于产品 Markdown 和 DataEye 来源配置，生成一份可转入裂变脚本任务的创意策略报告。",
		"",
		"重要边界：",
		"- 当前输入只有产品资料和 DataEye 来源配置，不代表已经成功拉取 DataEye 素材。",
		"- 如果没有提供真实素材元数据、指标表或视频拆解结果，必须把数据状态写为“待拉取素材/待补充指标”。",
		"- 不得编造曝光、播放、热度、点赞、评论、下载、投放时间、国家、媒体等指标。",
		"- 可以基于产品资料提出创意假设、素材筛选口径、创意方向和裂变任务 brief，但必须明确这是“策略预案”。",
		"",
		"输出结构必须包含：",
		"1. 数据口径与拉取配置：列出近 30 天筛选、排序指标、样本数、媒体/国家限制和缺失数据。",
		"2. 产品核心卖点提炼：只基于产品 Markdown。",
		"3. 爆款素材分析计划：说明 DataEye 素材拉取后要观察哪些字段和维度。",
		"4. 创意策略方向：给 5-8 个方向，每个方向包含适用素材假设、主钩子、卖点呈现、建议裂变元素、脚本 brief、验收指标。",
		"5. 转裂变脚本任务摘要：给一段 300 字以内的补充要求，可直接放入 ScriptAgent 脚本任务。",
		"6. 风险与待补数据：列出不能判断和需要用户补充的内容。",
		"",
		"产品信息：",
		"产品名称：" + product.Title,
		"Markdown 文件：" + product.MDName,
		"",
		"DataEye 来源配置：",
		configJSON,
		"",
		"产品 Markdown：",
		truncateRunes(markdown, maxProductMarkdownForReport),
	}, "\n")
}

func summarizeReport(markdown string) string {
	lines := strings.Split(markdown, "\n")
	capturing := false
	parts := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if capturing && len(parts) > 0 {
				break
			}
			continue
		}
		if strings.Contains(trimmed, "转裂变脚本任务摘要") {
			capturing = true
			continue
		}
		if capturing {
			if strings.HasPrefix(trimmed, "#") && len(parts) > 0 {
				break
			}
			parts = append(parts, strings.TrimLeft(trimmed, "-* "))
		}
	}
	if len(parts) == 0 {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			parts = append(parts, strings.TrimLeft(trimmed, "-* "))
			if len(strings.Join(parts, "\n")) > 280 {
				break
			}
		}
	}
	return truncateRunes(strings.Join(parts, "\n"), 320)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "\n\n[内容过长，已截断]"
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
