package reactagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tian1363/scriptagent/internal/model"
)

const defaultMaxSteps = 6

type Tool struct {
	Name        string
	Description string
	InputSchema string
	Handler     func(context.Context, json.RawMessage) (string, error)
}

type Step struct {
	Index       int             `json:"index"`
	Kind        string          `json:"kind"`
	Reason      string          `json:"reason,omitempty"`
	Tool        string          `json:"tool,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	Observation string          `json:"observation,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type Result struct {
	Answer string `json:"answer"`
	Steps  []Step `json:"steps,omitempty"`
}

type Runner struct {
	client   *model.DashScopeClient
	maxSteps int
}

type RunInput struct {
	Scope         string
	RefID         string
	Goal          string
	ContextPrompt string
	Tools         []Tool
}

type actionEnvelope struct {
	Type   string          `json:"type"`
	Reason string          `json:"reason"`
	Tool   string          `json:"tool"`
	Input  json.RawMessage `json:"input"`
	Answer string          `json:"answer"`
}

func New(client *model.DashScopeClient, maxSteps int) *Runner {
	if maxSteps <= 0 {
		maxSteps = defaultMaxSteps
	}
	return &Runner{client: client, maxSteps: maxSteps}
}

func (r *Runner) Run(ctx context.Context, input RunInput) (Result, error) {
	if r.client == nil {
		return Result{}, errors.New("react agent model is not configured")
	}
	tools := map[string]Tool{}
	for _, tool := range input.Tools {
		tool.Name = strings.TrimSpace(tool.Name)
		if tool.Name == "" || tool.Handler == nil {
			continue
		}
		tools[tool.Name] = tool
	}

	steps := []Step{}
	for index := 0; index < r.maxSteps; index++ {
		prompt := reactPrompt(input.Goal, input.ContextPrompt, input.Tools, steps)
		result, err := r.client.GenerateDetailed(ctx, model.CallContext{
			Scope: valueOr(input.Scope, "react"),
			RefID: input.RefID,
			Step:  fmt.Sprintf("react_step_%02d", index+1),
		}, []model.ContentItem{{Text: prompt}})
		if err != nil {
			return Result{Steps: steps}, err
		}
		action, err := parseAction(result.Text)
		if err != nil {
			steps = append(steps, Step{
				Index:  index + 1,
				Kind:   "final",
				Reason: "模型未返回工具动作 JSON，按普通回复处理。",
			})
			return Result{Answer: strings.TrimSpace(result.Text), Steps: steps}, nil
		}
		switch strings.ToLower(strings.TrimSpace(action.Type)) {
		case "final":
			steps = append(steps, Step{
				Index:  index + 1,
				Kind:   "final",
				Reason: strings.TrimSpace(action.Reason),
			})
			return Result{Answer: strings.TrimSpace(action.Answer), Steps: steps}, nil
		case "tool":
			tool, ok := tools[action.Tool]
			step := Step{
				Index:  index + 1,
				Kind:   "tool",
				Reason: strings.TrimSpace(action.Reason),
				Tool:   strings.TrimSpace(action.Tool),
				Input:  normalizeRawJSON(action.Input),
			}
			if !ok {
				step.Error = "unknown tool"
				step.Observation = "工具不存在，请选择可用工具。"
				steps = append(steps, step)
				continue
			}
			observation, err := tool.Handler(ctx, action.Input)
			if err != nil {
				step.Error = err.Error()
				step.Observation = truncateRunes(err.Error(), 1600)
			} else {
				step.Observation = truncateRunes(observation, 5000)
			}
			steps = append(steps, step)
		default:
			steps = append(steps, Step{
				Index:       index + 1,
				Kind:        "final",
				Reason:      "模型返回了未知动作类型，按最终回复处理。",
				Observation: result.Text,
			})
			return Result{Answer: strings.TrimSpace(result.Text), Steps: steps}, nil
		}
	}
	return Result{
		Answer: "我已经完成多轮工具检查，但还没有得到足够稳定的最终答案。请缩小问题范围，或指定要使用的产品/skill。",
		Steps:  steps,
	}, nil
}

func reactPrompt(goal, contextPrompt string, tools []Tool, steps []Step) string {
	lines := []string{
		"你是 ScriptAgent 的 ReAct 编排 Agent。",
		"你可以在回答前调用工具或内置 skill。你必须逐步决策，但不要输出隐藏推理链。",
		"每一轮只能输出一个严格 JSON 对象，不要输出 Markdown，不要包裹代码块。",
		"",
		"可输出两类 JSON：",
		`1. 调用工具：{"type":"tool","reason":"一句可见决策摘要","tool":"工具名","input":{...}}`,
		`2. 最终回答：{"type":"final","reason":"一句可见决策摘要","answer":"给用户的最终回答"}`,
		"",
		"规则：",
		"- reason 只能写面向用户可见的决策摘要，不要写逐字思维链。",
		"- 需要产品资料时优先调用产品工具，不要凭空编造。",
		"- 需要可复用工作流时调用 skill 工具。",
		"- 工具返回的信息不足时可以继续调用工具；足够回答时输出 final。",
		"",
		"可用工具：",
	}
	for _, tool := range tools {
		lines = append(lines,
			fmt.Sprintf("- %s：%s", tool.Name, tool.Description),
			fmt.Sprintf("  输入 JSON：%s", valueOr(tool.InputSchema, "{}")),
		)
	}
	if strings.TrimSpace(contextPrompt) != "" {
		lines = append(lines, "", "上下文：", contextPrompt)
	}
	if len(steps) > 0 {
		lines = append(lines, "", "已执行步骤与观察：")
		for _, step := range steps {
			if step.Kind == "tool" {
				lines = append(lines,
					fmt.Sprintf("Step %d 工具：%s", step.Index, step.Tool),
					fmt.Sprintf("决策摘要：%s", step.Reason),
					fmt.Sprintf("输入：%s", string(step.Input)),
					fmt.Sprintf("观察：%s", step.Observation),
				)
				if step.Error != "" {
					lines = append(lines, "错误："+step.Error)
				}
			}
		}
	}
	lines = append(lines, "", "用户目标：", goal, "", "请输出下一步 JSON。")
	return strings.Join(lines, "\n")
}

func parseAction(text string) (actionEnvelope, error) {
	var action actionEnvelope
	raw := []byte(extractJSONObject(text))
	if len(raw) == 0 {
		return action, errors.New("action JSON not found")
	}
	if err := json.Unmarshal(raw, &action); err != nil {
		return action, err
	}
	if strings.TrimSpace(action.Type) == "" {
		return action, errors.New("action type is required")
	}
	return action, nil
}

func extractJSONObject(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}

func normalizeRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(`{}`)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return normalized
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "\n...[truncated]"
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
