package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

type Handler struct {
	cfg       Config
	store     *jobs.Store
	files     *storage.LocalStore
	runner    *jobs.Runner
	publisher Publisher
	chat      ChatResponder
}

type Publisher interface {
	Publish(job jobs.Job) (string, error)
}

type ChatResponder interface {
	Send(ctx context.Context, conversationID, content, productID string) (*jobs.ChatThread, error)
}

func NewHandler(cfg Config, store *jobs.Store, files *storage.LocalStore, runner *jobs.Runner, publisher Publisher, chat ChatResponder) *Handler {
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
	var input struct {
		Content   string `json:"content"`
		ProductID string `json:"product_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	thread, err := h.chat.Send(r.Context(), chi.URLParam(r, "id"), input.Content, input.ProductID)
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
	var input struct {
		Content   string `json:"content"`
		ProductID string `json:"product_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	thread, err := h.chat.Send(r.Context(), "", input.Content, input.ProductID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusCreated, thread)
}

func (h *Handler) listModelCalls(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	result, err := h.store.ListModelCalls(r.URL.Query().Get("ref_id"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
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
		Endpoint string `json:"endpoint"`
		Model    string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := h.store.SaveModelSettings(jobs.ModelSettings{
		APIKey:   input.APIKey,
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
