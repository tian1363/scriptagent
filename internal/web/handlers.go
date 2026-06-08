package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
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
}

type Publisher interface {
	Publish(job jobs.Job) (string, error)
}

func NewHandler(cfg Config, store *jobs.Store, files *storage.LocalStore, runner *jobs.Runner, publisher Publisher) *Handler {
	if publisher == nil {
		publisher = disabledPublisher{}
	}
	return &Handler{
		cfg:       cfg,
		store:     store,
		files:     files,
		runner:    runner,
		publisher: publisher,
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
	mdPath, mdName, err := h.saveRequiredFile(r, "product_md", "markdown")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	fissionCount, err := parseFissionCount(r.FormValue("fission_count"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	industry := r.FormValue("industry")
	if industry == "" {
		industry = "auto"
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(mdName, filepath.Ext(mdName))
	}

	job, err := h.store.CreateJob(jobs.CreateJobInput{
		Title:             title,
		VideoPath:         videoPath,
		VideoOriginalName: videoName,
		ProductMDPath:     mdPath,
		ProductMDName:     mdName,
		Requirement:       r.FormValue("requirement"),
		Industry:          industry,
		FissionCount:      fissionCount,
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
