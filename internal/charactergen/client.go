package charactergen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tian1363/scriptagent/internal/model"
)

type Controls struct {
	Description string `json:"description"`
	Age         int    `json:"age"`
	Hair        string `json:"hair"`
	Outfit      string `json:"outfit"`
	Expression  string `json:"expression"`
	Pose        string `json:"pose"`
	Framing     string `json:"framing"`
	Background  string `json:"background"`
	Lighting    string `json:"lighting"`
	Size        string `json:"size"`
	Seed        int64  `json:"seed"`
	Negative    string `json:"negative"`
}

func (c Controls) Validate() error {
	if c.Age < 18 || c.Age > 90 {
		return errors.New("角色年龄请设置为 18–90 岁")
	}
	if c.Seed < 0 || c.Seed > 2147483647 {
		return errors.New("种子需为 0–2147483647 的整数")
	}
	if c.Size != "1024*1024" && c.Size != "720*1280" && c.Size != "960*1280" {
		return errors.New("请选择支持的角色图尺寸")
	}
	for _, v := range []string{c.Description, c.Hair, c.Outfit, c.Expression, c.Pose, c.Framing, c.Background, c.Lighting, c.Negative} {
		if utf8.RuneCountInString(v) > 200 {
			return errors.New("每项角色描述最多 200 字")
		}
	}
	return nil
}
func (c Controls) Prompt(reference bool) string {
	identity := fmt.Sprintf("创建一位虚构成年广告出镜角色。外观描述：%s。年龄：%d 岁。发型：%s。", c.Description, c.Age, c.Hair)
	if reference {
		identity = "以输入图片中同一位成年角色为身份基准，保持五官比例、脸型、肤色、年龄和发型一致。仅根据下列造型要求编辑，不替换成另一个人。"
	}
	return fmt.Sprintf("%s 服装：%s。表情：%s。姿态：%s。构图：%s。背景：%s。光线：%s。单人真实摄影，面部清晰，皮肤纹理自然，无文字与水印。", identity, c.Outfit, c.Expression, c.Pose, c.Framing, c.Background, c.Lighting)
}

type Client struct{ HTTP *http.Client }

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 180 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !TrustedURL(req.URL.String()) {
			return errors.New("不支持的图片重定向地址")
		}
		return nil
	}}}
}
func TrustedURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && strings.HasSuffix(strings.ToLower(u.Hostname()), ".aliyuncs.com")
}
func (c *Client) Generate(ctx context.Context, cfg model.RuntimeConfig, controls Controls, reference []byte) ([]byte, error) {
	if err := controls.Validate(); err != nil {
		return nil, err
	}
	if cfg.APIKey == "" {
		return nil, errors.New("请先在设置中连接图片生成／图片编辑模型")
	}
	if cfg.Provider != "dashscope" && cfg.Provider != "" {
		return nil, errors.New("角色图当前接入 Qwen 图片接口，请在对应图片能力中选择 Qwen 模型")
	}
	isEdit := len(reference) > 0
	supported := strings.HasPrefix(cfg.Model, "qwen-image-2.0") || (isEdit && (strings.HasPrefix(cfg.Model, "qwen-image-edit-plus") || strings.HasPrefix(cfg.Model, "qwen-image-edit-max")))
	if !supported {
		return nil, errors.New("请为角色生成配置 qwen-image-2.0 系列；造型编辑也支持 qwen-image-edit-plus/max")
	}
	if !TrustedURL(cfg.Endpoint) || !strings.Contains(cfg.Endpoint, "multimodal-generation/generation") {
		return nil, errors.New("请检查图片模型接口地址，应使用所属地域的 Qwen multimodal-generation/generation 接口")
	}
	content := []map[string]string{}
	if isEdit {
		if len(reference) > 10<<20 {
			return nil, errors.New("参考图请控制在 10MB 内")
		}
		content = append(content, map[string]string{"image": "data:" + http.DetectContentType(reference) + ";base64," + base64.StdEncoding.EncodeToString(reference)})
	}
	content = append(content, map[string]string{"text": controls.Prompt(isEdit)})
	body, _ := json.Marshal(map[string]any{"model": cfg.Model, "input": map[string]any{"messages": []any{map[string]any{"role": "user", "content": content}}}, "parameters": map[string]any{"n": 1, "size": controls.Size, "seed": controls.Seed, "prompt_extend": false, "watermark": false, "negative_prompt": "多人，面部模糊，肢体畸形，过度磨皮，文字，水印。" + controls.Negative}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("角色图服务连接失败或超时，请稍后重试")
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 != 2 {
		var problem struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(data, &problem)
		switch problem.Code {
		case "InvalidApiKey":
			return nil, errors.New("图片模型 API Key 无效，请检查对应能力设置")
		case "Arrearage":
			return nil, errors.New("图片模型额度不足")
		default:
			return nil, fmt.Errorf("图片服务未完成生成（%d），请检查模型、地域与额度后重试", res.StatusCode)
		}
	}
	var out struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Image string `json:"image"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	if json.Unmarshal(data, &out) != nil {
		return nil, errors.New("图片服务返回格式无效")
	}
	for _, choice := range out.Output.Choices {
		for _, item := range choice.Message.Content {
			if item.Image == "" {
				continue
			}
			if !TrustedURL(item.Image) {
				return nil, errors.New("图片服务返回了不支持的下载地址")
			}
			download, _ := http.NewRequestWithContext(ctx, http.MethodGet, item.Image, nil)
			r, err := c.HTTP.Do(download)
			if err != nil {
				return nil, errors.New("生成完成，但图片下载失败")
			}
			defer r.Body.Close()
			if r.StatusCode != 200 {
				return nil, errors.New("生成完成，但图片下载失败")
			}
			image, err := io.ReadAll(io.LimitReader(r.Body, (10<<20)+1))
			if err != nil || len(image) > 10<<20 {
				return nil, errors.New("生成图片过大或读取失败")
			}
			return image, nil
		}
	}
	return nil, errors.New("图片服务没有返回角色图，请重试")
}
