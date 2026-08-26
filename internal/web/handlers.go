package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	chatpkg "github.com/tian1363/scriptagent/internal/chat"
	"github.com/tian1363/scriptagent/internal/creative"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/videoprompt"
)

type Handler struct {
	cfg       Config
	store     *jobs.Store
	files     *storage.LocalStore
	runner    *jobs.Runner
	publisher Publisher
	chat      ChatResponder
	creative  *creative.Service
}

type Publisher interface {
	Publish(job jobs.Job) (string, error)
}

type ChatResponder interface {
	Send(ctx context.Context, conversationID, content, productID string) (*jobs.ChatThread, error)
	SendWithAttachments(ctx context.Context, conversationID, content, productID string, attachments []chatpkg.AttachmentInput) (*jobs.ChatThread, error)
}

func NewHandler(cfg Config, store *jobs.Store, files *storage.LocalStore, runner *jobs.Runner, publisher Publisher, chat ChatResponder, creativeReports *creative.Service) *Handler {
	if publisher == nil {
		publisher = disabledPublisher{}
	}
	return &Handler{
		cfg:       cfg,
		store:     store,
		files:     files,
		runner:    runner,
		publisher: publisher,
		chat:      chat,
		creative:  creativeReports,
	}
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listJobs(w http.ResponseWriter, _ *http.Request) {
	result, err := h.store.ListJobs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listSkills(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, chatpkg.BuiltInSkills())
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.store.GetJob(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) listProducts(w http.ResponseWriter, _ *http.Request) {
	result, err := h.store.ListProducts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(storage.MaxMarkdownBytes); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	mdPath, mdName, err := h.saveRequiredFile(r, "product_md", "markdown")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	product, err := h.store.CreateProduct(jobs.CreateProductInput{
		Title:  r.FormValue("title"),
		MDPath: mdPath,
		MDName: mdName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

func (h *Handler) getProductMarkdown(w http.ResponseWriter, r *http.Request) {
	product, err := h.store.GetProduct(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	file, err := os.Open(product.MDPath)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, int64(storage.MaxMarkdownBytes)+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(content) > storage.MaxMarkdownBytes {
		writeError(w, http.StatusBadRequest, errors.New("product Markdown is too large to preview"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":      product.ID,
		"title":   product.Title,
		"md_name": product.MDName,
		"content": string(content),
	})
}

func (h *Handler) listProductAssets(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	if _, err := h.store.GetProduct(productID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	assets, err := h.store.ListProductAssets(productID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

func (h *Handler) uploadProductAsset(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	if _, err := h.store.GetProduct(productID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err := r.ParseMultipartForm(storage.MaxAssetBytes); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("asset")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("请选择图片或视频"))
		return
	}
	defer file.Close()
	path, err := h.files.SaveUpload(file, header, "asset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	kind := "image"
	if ext == ".mp4" || ext == ".mov" || ext == ".webm" {
		kind = "video"
	}
	asset, err := h.store.CreateProductAsset(jobs.ProductAsset{ProductID: productID, Kind: kind, Path: path, OriginalName: header.Filename, MimeType: mime.TypeByExtension(ext), SizeBytes: header.Size})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (h *Handler) getProductAssetFile(w http.ResponseWriter, r *http.Request) {
	asset, err := h.store.GetProductAsset(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if asset.MimeType != "" {
		w.Header().Set("Content-Type", asset.MimeType)
	}
	w.Header().Set("Content-Disposition", "inline; filename="+strconv.Quote(asset.OriginalName))
	http.ServeFile(w, r, asset.Path)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	product, err := h.store.GetProduct(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, int64(storage.MaxMarkdownBytes)+4096)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Title, input.Content = strings.TrimSpace(input.Title), strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" {
		writeError(w, http.StatusBadRequest, errors.New("产品名称和资料内容不能为空"))
		return
	}
	if len([]byte(input.Content)) > storage.MaxMarkdownBytes {
		writeError(w, http.StatusBadRequest, errors.New("资料内容过大"))
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(product.MDPath), ".product-update-*.md")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.WriteString(input.Content + "\n"); err != nil {
		temp.Close()
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err = temp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err = os.Rename(name, product.MDPath); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	updated, err := h.store.UpdateProduct(product.ID, jobs.UpdateProductInput{Title: input.Title})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) listSpaces(w http.ResponseWriter, _ *http.Request) {
	result, err := h.store.ListSpaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) createSpace(w http.ResponseWriter, r *http.Request) {
	var input jobs.CreateSpaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	space, err := h.store.CreateSpace(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, space)
}

func (h *Handler) listCreativeReports(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	if _, err := h.store.GetProduct(productID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	reports, err := h.store.ListCreativeReports(productID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func (h *Handler) createCreativeReport(w http.ResponseWriter, r *http.Request) {
	if h.creative == nil {
		writeError(w, http.StatusBadRequest, errors.New("creative report service is not configured"))
		return
	}
	var input creative.DataEyeConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	report, err := h.creative.GenerateReport(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusCreated, report)
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(storage.MaxVideoBytes + storage.MaxMarkdownBytes); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	videoPath, videoName, err := h.saveRequiredFile(r, "video", "video")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	product, err := h.resolveJobProduct(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	fissionCount, err := parseFissionCount(r.FormValue("fission_count"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fissionDirections := parseFissionDirections(r)
	if err := validateFissionDirectionCount(fissionDirections, fissionCount); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	industry := r.FormValue("industry")
	if industry == "" {
		industry = "auto"
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = product.Title
	}
	contextSnapshot, err := h.buildJobContextSnapshot(product, strings.TrimSpace(r.FormValue("space_id")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	job, err := h.store.CreateJob(jobs.CreateJobInput{
		Title:             title,
		VideoPath:         videoPath,
		VideoOriginalName: videoName,
		ProductMDPath:     product.MDPath,
		ProductMDName:     product.MDName,
		Requirement:       r.FormValue("requirement"),
		Industry:          industry,
		FissionCount:      fissionCount,
		FissionDirections: fissionDirections,
		SpaceID:           strings.TrimSpace(r.FormValue("space_id")),
		ParentJobID:       strings.TrimSpace(r.FormValue("parent_job_id")),
		ContextSnapshot:   contextSnapshot,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	h.runner.Enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, map[string]string{
		"job_id": job.ID,
		"status": job.Status,
	})
}

func (h *Handler) buildJobContextSnapshot(product *jobs.Product, spaceID string) (string, error) {
	markdown, err := os.ReadFile(product.MDPath)
	if err != nil {
		return "", err
	}
	assets, err := h.store.ListProductAssets(product.ID)
	if err != nil {
		return "", err
	}
	lines := []string{"# 任务上下文快照", "", "## 产品资料（创建任务时版本）", string(markdown)}
	if spaceID != "" {
		space, err := h.store.GetSpace(spaceID)
		if err != nil {
			return "", err
		}
		lines = append(lines, "", "## 创作空间", "名称："+space.Title, "目标："+space.Summary, "长期要求："+space.AgentBrief)
	}
	if len(assets) > 0 {
		lines = append(lines, "", "## 可用图片/视频素材")
		for _, asset := range assets {
			lines = append(lines, "- "+asset.Kind+"："+asset.OriginalName)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (h *Handler) resolveJobProduct(r *http.Request) (*jobs.Product, error) {
	productID := strings.TrimSpace(r.FormValue("product_id"))
	if productID != "" {
		return h.store.GetProduct(productID)
	}
	mdPath, mdName, err := h.saveRequiredFile(r, "product_md", "markdown")
	if err != nil {
		return nil, errors.New("select a product or upload product Markdown")
	}
	return h.store.CreateProduct(jobs.CreateProductInput{
		Title:  r.FormValue("product_title"),
		MDPath: mdPath,
		MDName: mdName,
	})
}

func (h *Handler) publishJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.store.GetJob(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if job.Status != jobs.StatusCompleted && job.Status != jobs.StatusPublished {
		writeError(w, http.StatusBadRequest, errors.New("job is not ready to publish"))
		return
	}
	if err := h.store.UpdateStatus(job.ID, jobs.StatusPublishing, ""); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.store.AppendLog(job.ID, "开始发布至 CreatiBI。")
	result, err := h.publisher.Publish(*job)
	if err != nil {
		_ = h.store.AppendLog(job.ID, "发布至 CreatiBI 失败："+err.Error())
		_ = h.store.SavePublishResult(job.ID, jobs.StatusCompleted, "", err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = h.store.AppendLog(job.ID, "发布至 CreatiBI 成功。")
	if err := h.store.SavePublishResult(job.ID, jobs.StatusPublished, result, ""); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": jobs.StatusPublished})
}

func (h *Handler) retryJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.store.GetJob(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if isRunningStatus(job.Status) {
		writeError(w, http.StatusBadRequest, errors.New("job is already running"))
		return
	}
	if err := h.store.AppendLog(job.ID, "用户触发重试，准备重新生成。"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.store.ResetForRetry(job.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	h.runner.Enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, map[string]string{
		"job_id": job.ID,
		"status": jobs.StatusPending,
	})
}

func (h *Handler) generateVideoPrompts(w http.ResponseWriter, r *http.Request) {
	job, err := h.store.GetJob(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var input struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if input.Source == "" {
		input.Source = r.URL.Query().Get("source")
	}
	content, err := videoprompt.GenerateFromJob(*job, input.Source)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"job_id":  job.ID,
		"source":  valueOr(input.Source, "all"),
		"content": content,
	})
}

func (h *Handler) listChats(w http.ResponseWriter, _ *http.Request) {
	result, err := h.store.ListChatConversations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createChat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	conversation, err := h.store.CreateChatConversation(input.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, conversation)
}

func (h *Handler) getChat(w http.ResponseWriter, r *http.Request) {
	thread, err := h.store.GetChatThread(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

func (h *Handler) sendChatMessage(w http.ResponseWriter, r *http.Request) {
	if h.chat == nil {
		writeError(w, http.StatusBadRequest, errors.New("chat is not configured"))
		return
	}
	input, attachments, err := h.parseChatMessageInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	thread, err := h.chat.SendWithAttachments(r.Context(), chi.URLParam(r, "id"), input.Content, input.ProductID, attachments)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

func (h *Handler) sendNewChatMessage(w http.ResponseWriter, r *http.Request) {
	if h.chat == nil {
		writeError(w, http.StatusBadRequest, errors.New("chat is not configured"))
		return
	}
	input, attachments, err := h.parseChatMessageInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	thread, err := h.chat.SendWithAttachments(r.Context(), "", input.Content, input.ProductID, attachments)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusCreated, thread)
}

type chatMessageInput struct {
	Content   string
	ProductID string
}

func (h *Handler) parseChatMessageInput(r *http.Request) (chatMessageInput, []chatpkg.AttachmentInput, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(storage.MaxChatAttachmentBytes + 1024*1024); err != nil {
			return chatMessageInput{}, nil, err
		}
		input := chatMessageInput{Content: r.FormValue("content"), ProductID: r.FormValue("product_id")}
		file, header, err := r.FormFile("attachment")
		if errors.Is(err, http.ErrMissingFile) {
			return input, nil, nil
		}
		if err != nil {
			return chatMessageInput{}, nil, err
		}
		defer file.Close()
		path, err := h.files.SaveUpload(file, header, "chat")
		if err != nil {
			return chatMessageInput{}, nil, err
		}
		return input, []chatpkg.AttachmentInput{{Path: path, Name: header.Filename, Kind: chatAttachmentKind(header.Filename), Size: header.Size}}, nil
	}
	var input struct {
		Content   string `json:"content"`
		ProductID string `json:"product_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return chatMessageInput{}, nil, err
	}
	return chatMessageInput{Content: input.Content, ProductID: input.ProductID}, nil, nil
}

func chatAttachmentKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return "图片"
	case ".mp4", ".mov", ".webm":
		return "视频"
	default:
		return "素材"
	}
}

func (h *Handler) listModelCalls(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	result, err := h.store.ListModelCalls(r.URL.Query().Get("ref_id"), r.URL.Query().Get("space_id"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getSpaceObservability(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	result, err := h.store.GetSpaceObservability(chi.URLParam(r, "id"), limit)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getModelSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.publicModelSettings())
}

func (h *Handler) saveModelSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		APIKey   string `json:"api_key"`
		Provider string `json:"provider"`
		Endpoint string `json:"endpoint"`
		Model    string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := h.store.SaveModelSettings(jobs.ModelSettings{
		APIKey:   input.APIKey,
		Provider: input.Provider,
		Endpoint: input.Endpoint,
		Model:    input.Model,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, h.publicModelSettings())
}

func (h *Handler) publicModelSettings() jobs.PublicModelSettings {
	if settings, err := h.store.GetModelSettings(); err == nil && strings.TrimSpace(settings.APIKey) != "" {
		return jobs.PublicModelSettings{
			Configured: true,
			Source:     "user",
			APIKeyMask: maskAPIKey(settings.APIKey),
			Provider:   settings.Provider,
			Endpoint:   settings.Endpoint,
			Model:      settings.Model,
			UpdatedAt:  settings.UpdatedAt,
		}
	}
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	return jobs.PublicModelSettings{
		Configured: strings.TrimSpace(apiKey) != "",
		Source:     "env",
		APIKeyMask: maskAPIKey(apiKey),
		Provider:   "dashscope",
		Endpoint:   valueOr(os.Getenv("DASHSCOPE_ENDPOINT"), "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"),
		Model:      valueOr(os.Getenv("SCRIPT_AGENT_MODEL"), "qwen3.6-plus"),
	}
}

func maskAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func (h *Handler) saveRequiredFile(r *http.Request, field, kind string) (string, string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	path, err := h.files.SaveUpload(file, header, kind)
	if err != nil {
		return "", "", err
	}
	return path, header.Filename, nil
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func isRunningStatus(status string) bool {
	switch status {
	case jobs.StatusPending,
		jobs.StatusRunning,
		jobs.StatusAnalyzingVideo,
		jobs.StatusExtractingStructure,
		jobs.StatusGeneratingReplica,
		jobs.StatusGeneratingFission,
		jobs.StatusValidating,
		jobs.StatusPublishing:
		return true
	default:
		return false
	}
}

func parseFissionCount(raw string) (int, error) {
	if raw == "" {
		return 5, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < 1 || value > 20 {
		return 0, errors.New("fission_count must be between 1 and 20")
	}
	return value, nil
}

func parseFissionDirections(r *http.Request) string {
	values := r.MultipartForm.Value["fission_directions"]
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return strings.Join(result, "\n")
}

func validateFissionDirectionCount(raw string, expected int) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	actual := len(strings.Split(strings.TrimSpace(raw), "\n"))
	if actual != expected {
		return fmt.Errorf("fission_directions must contain exactly %d items, got %d", expected, actual)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

type disabledPublisher struct{}

func (disabledPublisher) Publish(job jobs.Job) (string, error) {
	return "", errors.New("CreatiBI publisher is not configured")
}
