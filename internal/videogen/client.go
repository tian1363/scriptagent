package videogen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/model"
)

type ConfigProvider interface {
	GetModelRuntimeConfig(context.Context, string) (model.RuntimeConfig, error)
}

type Client struct {
	provider ConfigProvider
	http     *http.Client
}

type Request struct {
	Model, Prompt, NegativePrompt, Resolution, Ratio string
	References                                       []Reference
	Duration                                         int
	SoundEnabled                                     bool
}

type Reference struct {
	URL, Kind, Path, Name string
}

func New(provider ConfigProvider) *Client {
	return &Client{provider: provider, http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) Submit(ctx context.Context, input Request) (string, error) {
	cfg, err := c.provider.GetModelRuntimeConfig(ctx, "video_generation")
	if err != nil {
		return "", errors.New("请先在设置中配置视频生成模型")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return "", errors.New("视频生成 API Key 未配置")
	}
	if !strings.Contains(cfg.Endpoint, "video-generation/video-synthesis") {
		cfg.Endpoint = "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis"
	}
	modelName := input.Model
	if modelName == "" {
		modelName = cfg.Model
	}
	media := make([]map[string]string, 0, len(input.References))
	for _, reference := range input.References {
		if reference.Path != "" {
			reference.URL, err = c.uploadTemporary(ctx, cfg, modelName, reference.Path, reference.Name)
			if err != nil {
				return "", fmt.Errorf("上传参考视频失败：%w", err)
			}
		}
		if reference.URL != "" {
			media = append(media, map[string]string{"type": reference.Kind, "url": reference.URL})
		}
	}
	in := map[string]any{"prompt": input.Prompt}
	if input.NegativePrompt != "" {
		in["negative_prompt"] = input.NegativePrompt
	}
	if len(media) > 0 {
		in["media"] = media
	}
	params := map[string]any{"resolution": input.Resolution, "ratio": input.Ratio, "duration": input.Duration, "audio": input.SoundEnabled, "prompt_extend": true, "watermark": false}
	body, _ := json.Marshal(map[string]any{"model": modelName, "input": in, "parameters": params})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")
	if len(media) > 0 {
		req.Header.Set("X-DashScope-OssResourceResolve", "enable")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("视频任务提交失败：%s", providerMessage(payload, res.Status))
	}
	var out struct {
		Output struct {
			TaskID string `json:"task_id"`
		} `json:"output"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(payload, &out); err != nil || out.Output.TaskID == "" {
		return "", errors.New("视频服务未返回任务编号")
	}
	return out.Output.TaskID, nil
}

func (c *Client) uploadTemporary(ctx context.Context, cfg model.RuntimeConfig, modelName, path, name string) (string, error) {
	policyURL := "https://dashscope.aliyuncs.com/api/v1/uploads?action=getPolicy&model=" + url.QueryEscape(modelName)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, policyURL, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode/100 != 2 {
		return "", errors.New(providerMessage(data, res.Status))
	}
	var policy struct {
		Data struct {
			Policy          string `json:"policy"`
			Signature       string `json:"signature"`
			UploadDir       string `json:"upload_dir"`
			UploadHost      string `json:"upload_host"`
			AccessKey       string `json:"oss_access_key_id"`
			ACL             string `json:"x_oss_object_acl"`
			ForbidOverwrite string `json:"x_oss_forbid_overwrite"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &policy); err != nil || policy.Data.UploadHost == "" {
		return "", errors.New("未取得临时上传凭证")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if name == "" {
		name = filepath.Base(path)
	}
	key := strings.TrimSuffix(policy.Data.UploadDir, "/") + "/" + filepath.Base(name)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fields := [][2]string{{"OSSAccessKeyId", policy.Data.AccessKey}, {"Signature", policy.Data.Signature}, {"policy", policy.Data.Policy}, {"x-oss-object-acl", policy.Data.ACL}, {"x-oss-forbid-overwrite", policy.Data.ForbidOverwrite}, {"key", key}, {"success_action_status", "200"}}
	for _, field := range fields {
		_ = w.WriteField(field[0], field[1])
	}
	part, err := w.CreateFormFile("file", filepath.Base(name))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	_ = w.Close()
	uploadReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, policy.Data.UploadHost, &body)
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())
	uploadRes, err := c.http.Do(uploadReq)
	if err != nil {
		return "", err
	}
	defer uploadRes.Body.Close()
	if uploadRes.StatusCode/100 != 2 {
		payload, _ := io.ReadAll(io.LimitReader(uploadRes.Body, 1<<20))
		return "", fmt.Errorf("临时存储拒绝上传：%s", providerMessage(payload, uploadRes.Status))
	}
	return "oss://" + key, nil
}

func (c *Client) Poll(ctx context.Context, taskID string) (status, videoURL, message string, err error) {
	cfg, cfgErr := c.provider.GetModelRuntimeConfig(ctx, "video_generation")
	if cfgErr != nil {
		return "", "", "", cfgErr
	}
	endpoint, parseErr := url.Parse(cfg.Endpoint)
	if parseErr != nil {
		return "", "", "", parseErr
	}
	endpoint.Path = "/api/v1/tasks/" + url.PathEscape(taskID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	res, doErr := c.http.Do(req)
	if doErr != nil {
		return "", "", "", doErr
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode/100 != 2 {
		return "", "", "", fmt.Errorf("查询视频进度失败：%s", providerMessage(payload, res.Status))
	}
	var out struct {
		Output struct {
			TaskStatus string `json:"task_status"`
			VideoURL   string `json:"video_url"`
			Results    []struct {
				URL string `json:"url"`
			} `json:"results"`
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"output"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return "", "", "", err
	}
	videoURL = out.Output.VideoURL
	if videoURL == "" && len(out.Output.Results) > 0 {
		videoURL = out.Output.Results[0].URL
	}
	message = out.Output.Message
	if message == "" {
		message = out.Message
	}
	return strings.ToUpper(out.Output.TaskStatus), videoURL, message, nil
}

func (c *Client) Download(ctx context.Context, videoURL string) (io.ReadCloser, error) {
	u, err := url.Parse(videoURL)
	if err != nil || u.Scheme != "https" {
		return nil, errors.New("视频服务返回了无效下载地址")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 != 2 {
		res.Body.Close()
		return nil, fmt.Errorf("下载成片失败：%s", res.Status)
	}
	return res.Body, nil
}

func providerMessage(payload []byte, fallback string) string {
	var x struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if json.Unmarshal(payload, &x) == nil && x.Message != "" {
		return x.Message
	}
	return fallback
}
