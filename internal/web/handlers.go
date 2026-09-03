package web

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/auth"
	chatpkg "github.com/tian1363/scriptagent/internal/chat"
	"github.com/tian1363/scriptagent/internal/creative"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/userctx"
	"github.com/tian1363/scriptagent/internal/videogen"
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
	auth      *auth.Service
	video     *videogen.Client
}

type Publisher interface {
	Publish(job jobs.Job) (string, error)
}

type ChatResponder interface {
	Send(ctx context.Context, conversationID, content, productID string) (*jobs.ChatThread, error)
	SendWithAttachments(ctx context.Context, conversationID, content, productID string, attachments []chatpkg.AttachmentInput) (*jobs.ChatThread, error)
	GenerateSkillDraft(ctx context.Context, requirement string) (*jobs.CreateCustomSkillInput, error)
	Progress(conversationID string) []jobs.AgentStep
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
		auth:      auth.NewService(store),
		video:     videogen.New(store),
	}
}

func (h *Handler) listVideos(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListUserVideoGenerations(userIDFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) listProactiveSuggestions(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.RefreshProactiveSuggestions(userIDFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) updateProactiveSuggestionStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := h.store.UpdateProactiveSuggestionStatus(userIDFromRequest(r), chi.URLParam(r, "id"), input.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) getVideo(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetUserVideoGeneration(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, errors.New("视频任务不存在"))
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createVideo(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProductID      string   `json:"product_id"`
		SpaceID        string   `json:"space_id"`
		ConversationID string   `json:"conversation_id"`
		SourceAssetID  string   `json:"source_asset_id"`
		SourceAssetIDs []string `json:"source_asset_ids"`
		Mode           string   `json:"mode"`
		Prompt         string   `json:"prompt"`
		NegativePrompt string   `json:"negative_prompt"`
		Model          string   `json:"model"`
		Resolution     string   `json:"resolution"`
		Ratio          string   `json:"ratio"`
		Duration       int      `json:"duration"`
		SoundEnabled   *bool    `json:"sound_enabled"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 128*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Prompt == "" {
		writeError(w, http.StatusBadRequest, errors.New("请描述要生成的 UGC 视频"))
		return
	}
	if input.Duration == 0 {
		input.Duration = 5
	}
	if input.Duration < 2 || input.Duration > 30 {
		writeError(w, http.StatusBadRequest, errors.New("视频时长需为 2 到 30 秒"))
		return
	}
	if input.Mode == "" {
		input.Mode = "text"
	}
	if input.Mode != "text" && input.Mode != "image" && input.Mode != "video" {
		writeError(w, http.StatusBadRequest, errors.New("参考类型仅支持无参考、图片或视频"))
		return
	}
	if input.Resolution == "" {
		input.Resolution = "720P"
	}
	if input.Ratio == "" {
		input.Ratio = "9:16"
	}
	if len(input.SourceAssetIDs) == 0 && input.SourceAssetID != "" {
		input.SourceAssetIDs = []string{input.SourceAssetID}
	}
	if input.Mode != "text" && len(input.SourceAssetIDs) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("请选择参考图片或视频"))
		return
	}
	if input.Model == "" {
		input.Model = "wan3.0-video-prime"
	}
	if input.Model != "wan3.0-video-prime" && input.Model != "wan3.0-video" {
		writeError(w, http.StatusBadRequest, errors.New("当前仅支持 Wan 3.0 视频模型"))
		return
	}
	if input.Resolution != "480P" && input.Resolution != "720P" && input.Resolution != "1080P" {
		writeError(w, http.StatusBadRequest, errors.New("分辨率仅支持 480P、720P 或 1080P"))
		return
	}
	if input.Ratio != "adaptive" && input.Ratio != "16:9" && input.Ratio != "9:16" && input.Ratio != "1:1" && input.Ratio != "4:3" && input.Ratio != "3:4" {
		writeError(w, http.StatusBadRequest, errors.New("不支持该视频画幅"))
		return
	}
	userID := userIDFromRequest(r)
	if input.ProductID != "" {
		if _, err := h.store.GetUserProduct(userID, input.ProductID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("产品资料不存在"))
			return
		}
	}
	if input.SpaceID != "" {
		if _, err := h.store.GetUserSpace(userID, input.SpaceID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("创意空间不存在"))
			return
		}
	}
	if input.ConversationID != "" {
		if _, err := h.store.GetUserChatThread(userID, input.ConversationID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("当前对话不存在"))
			return
		}
	}
	seenAssetIDs := map[string]bool{}
	imageCount, videoCount := 0, 0
	for _, assetID := range input.SourceAssetIDs {
		if seenAssetIDs[assetID] {
			writeError(w, http.StatusBadRequest, errors.New("参考素材不能重复选择"))
			return
		}
		seenAssetIDs[assetID] = true
		asset, err := h.store.GetProductAsset(assetID)
		if err != nil || asset.ProductID != input.ProductID || !h.store.OwnsResource(userID, "product", asset.ProductID) || asset.Kind != input.Mode {
			writeError(w, http.StatusBadRequest, errors.New("参考素材类型与所选模式不一致"))
			return
		}
		if asset.Kind == "image" && asset.SizeBytes > 20*1024*1024 {
			writeError(w, http.StatusBadRequest, errors.New("参考图片不能超过 20MB"))
			return
		}
		if asset.Kind == "video" && asset.SizeBytes > 100*1024*1024 {
			writeError(w, http.StatusBadRequest, errors.New("参考视频不能超过 100MB"))
			return
		}
		if asset.Kind == "video" && filepath.Ext(strings.ToLower(asset.OriginalName)) != ".mp4" && filepath.Ext(strings.ToLower(asset.OriginalName)) != ".mov" {
			writeError(w, http.StatusBadRequest, errors.New("参考视频仅支持 MP4 或 MOV"))
			return
		}
		if asset.Kind == "image" {
			imageCount++
		} else {
			videoCount++
		}
	}
	if imageCount > 10 {
		writeError(w, http.StatusBadRequest, errors.New("参考图片最多选择 10 张"))
		return
	}
	if videoCount > 5 {
		writeError(w, http.StatusBadRequest, errors.New("参考视频最多选择 5 段，且总时长不能超过 15 秒"))
		return
	}
	soundEnabled := true
	if input.SoundEnabled != nil {
		soundEnabled = *input.SoundEnabled
	}
	legacyAssetID := ""
	if len(input.SourceAssetIDs) > 0 {
		legacyAssetID = input.SourceAssetIDs[0]
	}
	item, err := h.store.CreateVideoGeneration(jobs.CreateVideoGenerationInput{UserID: userID, ProductID: input.ProductID, SpaceID: input.SpaceID, ConversationID: input.ConversationID, SourceAssetID: legacyAssetID, SourceAssetIDs: input.SourceAssetIDs, Mode: input.Mode, Prompt: input.Prompt, NegativePrompt: strings.TrimSpace(input.NegativePrompt), Model: input.Model, Resolution: input.Resolution, Ratio: input.Ratio, Duration: input.Duration, SoundEnabled: soundEnabled, EstimatedCostCNY: estimateVideoCost(input.Model, input.Resolution, input.Duration)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if input.ConversationID != "" {
		_, _ = h.store.AddChatMessage(input.ConversationID, "user", "生成视频")
	}
	user, _ := userctx.FromContext(r.Context())
	go h.runVideo(user, item)
	writeJSON(w, http.StatusAccepted, item)
}

func estimateVideoCost(modelName, resolution string, duration int) float64 {
	prices := map[string]map[string]float64{
		"wan3.0-video-prime": {"480P": 0.45, "720P": 0.9, "1080P": 1.8},
		"wan3.0-video":       {"480P": 0.3, "720P": 0.6, "1080P": 1.2},
	}
	return prices[modelName][resolution] * float64(duration)
}

func (h *Handler) retryVideo(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetUserVideoGeneration(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, errors.New("视频任务不存在"))
		return
	}
	if item.Status == "submitted" || item.Status == "running" || item.Status == "pending" {
		writeError(w, http.StatusConflict, errors.New("视频仍在生成"))
		return
	}
	_ = h.store.UpdateVideoGeneration(item.ID, "pending", "", "", "", "")
	item.ProviderTaskID = ""
	user, _ := userctx.FromContext(r.Context())
	go h.runVideo(user, item)
	item.Status, item.ErrorMessage = "pending", ""
	writeJSON(w, http.StatusAccepted, item)
}

func (h *Handler) getVideoFile(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetUserVideoGeneration(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil || item.LocalPath == "" {
		writeError(w, http.StatusNotFound, errors.New("成片尚未生成"))
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="ugc-%s.mp4"`, item.ID))
	http.ServeFile(w, r, item.LocalPath)
}

func (h *Handler) runVideo(user userctx.User, item *jobs.VideoGeneration) {
	ctx, cancel := context.WithTimeout(userctx.WithUser(context.Background(), user), 12*time.Minute)
	defer cancel()
	references := make([]videogen.Reference, 0, len(item.SourceAssetIDs))
	assetIDs := item.SourceAssetIDs
	if len(assetIDs) == 0 && item.SourceAssetID != "" {
		assetIDs = []string{item.SourceAssetID}
	}
	for _, assetID := range assetIDs {
		asset, err := h.store.GetProductAsset(assetID)
		if err != nil {
			_ = h.store.UpdateVideoGeneration(item.ID, "failed", "", "", "", "读取参考素材失败")
			return
		}
		if asset.Kind == "image" {
			data, err := os.ReadFile(asset.Path)
			if err != nil {
				_ = h.store.UpdateVideoGeneration(item.ID, "failed", "", "", "", "读取参考素材失败")
				return
			}
			references = append(references, videogen.Reference{URL: "data:" + asset.MimeType + ";base64," + base64.StdEncoding.EncodeToString(data), Kind: "reference_image", Name: asset.OriginalName})
		} else {
			references = append(references, videogen.Reference{Path: asset.Path, Name: asset.OriginalName, Kind: "reference_video"})
		}
	}
	taskID := item.ProviderTaskID
	if taskID == "" {
		var err error
		taskID, err = h.video.Submit(ctx, videogen.Request{Model: item.Model, Prompt: item.Prompt, NegativePrompt: item.NegativePrompt, References: references, Resolution: item.Resolution, Ratio: item.Ratio, Duration: item.Duration, SoundEnabled: item.SoundEnabled})
		if err != nil {
			_ = h.store.UpdateVideoGeneration(item.ID, "failed", "", "", "", err.Error())
			return
		}
		_ = h.store.UpdateVideoGeneration(item.ID, "submitted", taskID, "", "", "")
	}
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = h.store.UpdateVideoGeneration(item.ID, "failed", taskID, "", "", "生成超时，请重试")
			return
		case <-ticker.C:
			status, videoURL, message, err := h.video.Poll(ctx, taskID)
			if err != nil {
				continue
			}
			switch status {
			case "SUCCEEDED":
				body, err := h.video.Download(ctx, videoURL)
				if err != nil {
					_ = h.store.UpdateVideoGeneration(item.ID, "failed", taskID, videoURL, "", err.Error())
					return
				}
				path, saveErr := h.files.SaveGeneratedVideo(item.ID, body)
				body.Close()
				if saveErr != nil {
					_ = h.store.UpdateVideoGeneration(item.ID, "failed", taskID, videoURL, "", "保存成片失败")
					return
				}
				_ = h.store.UpdateVideoGeneration(item.ID, "completed", taskID, videoURL, path, "")
				return
			case "FAILED", "CANCELED", "UNKNOWN":
				if message == "" {
					message = "视频生成失败，请重试"
				}
				_ = h.store.UpdateVideoGeneration(item.ID, "failed", taskID, "", "", message)
				return
			default:
				_ = h.store.UpdateVideoGeneration(item.ID, "running", taskID, "", "", "")
			}
		}
	}
}

// ResumeVideos continues polling provider tasks after a server restart.
func (h *Handler) ResumeVideos() {
	users, err := h.store.ListUsers()
	if err != nil {
		return
	}
	for _, user := range users {
		items, err := h.store.ListUserVideoGenerations(user.ID)
		if err != nil {
			continue
		}
		for index := range items {
			item := items[index]
			if item.ProviderTaskID != "" && (item.Status == "submitted" || item.Status == "running") {
				go h.runVideo(userctx.User{ID: user.ID, Email: user.Email, Name: user.Name}, &item)
			}
		}
	}
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) authStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"registration_available": true})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input struct{ Email, Password, Name string }
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, session, err := h.auth.Register(input.Email, input.Password, input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	h.setSessionCookie(w, r, session.Token, session.ExpiresAt)
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct{ Email, Password string }
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, session, err := h.auth.Login(input.Email, input.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	h.setSessionCookie(w, r, session.Token, session.ExpiresAt)
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.CookieName()); err == nil {
		_ = h.auth.Logout(cookie.Value)
	}
	h.setSessionCookie(w, r, "", time.Unix(0, 0))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := userctx.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("请先登录"))
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.CookieName())
		if err != nil {
			writeError(w, http.StatusUnauthorized, errors.New("请先登录"))
			return
		}
		user, err := h.auth.Authenticate(cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		ctx := userctx.WithUser(r.Context(), userctx.User{ID: user.ID, Email: user.Email, Name: user.Name})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userIDFromRequest(r *http.Request) string { return userctx.UserID(r.Context()) }

func (h *Handler) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: auth.CookieName(), Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: func() int {
		if token == "" {
			return -1
		}
		return int(time.Until(expires).Seconds())
	}()})
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.ListUserJobs(userIDFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listSkills(w http.ResponseWriter, r *http.Request) {
	result := make([]any, 0)
	for _, skill := range chatpkg.BuiltInSkills() {
		result = append(result, skill)
	}
	custom, err := h.store.ListUserCustomSkills(userIDFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, skill := range custom {
		result = append(result, skill)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) generateSkillDraft(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Requirement string `json:"requirement"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(input.Requirement) == "" {
		writeError(w, http.StatusBadRequest, errors.New("请描述你希望技能完成什么"))
		return
	}
	draft, err := h.chat.GenerateSkillDraft(r.Context(), input.Requirement)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

var customSkillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (h *Handler) createSkill(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name             string `json:"name"`
		Title            string `json:"title"`
		Description      string `json:"description"`
		Category         string `json:"category"`
		InvocationPrompt string `json:"invocation_prompt"`
		Content          string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 128*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Name = strings.ToLower(strings.TrimSpace(input.Name))
	input.Title, input.Description = strings.TrimSpace(input.Title), strings.TrimSpace(input.Description)
	input.Category, input.InvocationPrompt, input.Content = strings.TrimSpace(input.Category), strings.TrimSpace(input.InvocationPrompt), strings.TrimSpace(input.Content)
	if !customSkillNamePattern.MatchString(input.Name) || len(input.Name) > 64 {
		writeError(w, http.StatusBadRequest, errors.New("skill name must use lowercase letters, digits, and hyphens, up to 64 characters"))
		return
	}
	if input.Title == "" || input.Description == "" || input.Content == "" {
		writeError(w, http.StatusBadRequest, errors.New("title, description, and content are required"))
		return
	}
	if input.Category == "" {
		input.Category = "自定义"
	}
	if input.InvocationPrompt == "" {
		input.InvocationPrompt = fmt.Sprintf("调用 %s skill，%s", input.Name, input.Description)
	}
	if !strings.HasPrefix(input.Content, "# ") {
		input.Content = "# " + input.Title + "\n\n" + input.Content
	}
	skill, err := h.store.CreateCustomSkill(jobs.CreateCustomSkillInput{
		Name: input.Name, Title: input.Title, Description: input.Description, Category: input.Category,
		InvocationPrompt: input.InvocationPrompt, Content: input.Content,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, errors.New("skill name already exists"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.store.ClaimResource(userIDFromRequest(r), "skill", skill.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, skill)
}

func (h *Handler) updateSkill(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromRequest(r)
	var input struct {
		Name             string `json:"name"`
		Title            string `json:"title"`
		Description      string `json:"description"`
		Category         string `json:"category"`
		InvocationPrompt string `json:"invocation_prompt"`
		Content          string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 128*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Name = strings.ToLower(strings.TrimSpace(input.Name))
	input.Title, input.Description = strings.TrimSpace(input.Title), strings.TrimSpace(input.Description)
	input.Category, input.InvocationPrompt, input.Content = strings.TrimSpace(input.Category), strings.TrimSpace(input.InvocationPrompt), strings.TrimSpace(input.Content)
	if !customSkillNamePattern.MatchString(input.Name) || len(input.Name) > 64 {
		writeError(w, http.StatusBadRequest, errors.New("skill name must use lowercase letters, digits, and hyphens, up to 64 characters"))
		return
	}
	if input.Title == "" || input.Description == "" || input.Content == "" {
		writeError(w, http.StatusBadRequest, errors.New("title, description, and content are required"))
		return
	}
	if input.Category == "" {
		input.Category = "自定义"
	}
	if input.InvocationPrompt == "" {
		input.InvocationPrompt = fmt.Sprintf("调用 %s skill，%s", input.Name, input.Description)
	}
	if !strings.HasPrefix(input.Content, "# ") {
		input.Content = "# " + input.Title + "\n\n" + input.Content
	}
	if !h.store.OwnsResource(userID, "skill", chi.URLParam(r, "id")) {
		writeError(w, http.StatusNotFound, sql.ErrNoRows)
		return
	}
	skill, err := h.store.UpdateCustomSkill(chi.URLParam(r, "id"), jobs.CreateCustomSkillInput{
		Name: input.Name, Title: input.Title, Description: input.Description, Category: input.Category,
		InvocationPrompt: input.InvocationPrompt, Content: input.Content,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, errors.New("skill name already exists"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.store.GetUserJob(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.ListUserProducts(userIDFromRequest(r))
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
	if err := h.store.ClaimResource(userIDFromRequest(r), "product", product.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

func (h *Handler) parseProductDocument(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(storage.MaxDocumentBytes)+1024*1024)
	if err := r.ParseMultipartForm(storage.MaxDocumentBytes); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("文件不能超过 20MB"))
		return
	}
	file, header, err := r.FormFile("document")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("请选择 MD、PDF、DOC 或 DOCX 文件"))
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, int64(storage.MaxDocumentBytes)+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(content) > storage.MaxDocumentBytes {
		writeError(w, http.StatusBadRequest, errors.New("文件不能超过 20MB"))
		return
	}
	text, err := parseProductDocument(header.Filename, content)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	title := strings.TrimSpace(strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename)))
	writeJSON(w, http.StatusOK, map[string]string{"title": title, "content": text, "filename": header.Filename})
}

func (h *Handler) getProductMarkdown(w http.ResponseWriter, r *http.Request) {
	product, err := h.store.GetUserProduct(userIDFromRequest(r), chi.URLParam(r, "id"))
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
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), productID); err != nil {
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
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), productID); err != nil {
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
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), asset.ProductID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if asset.MimeType != "" {
		w.Header().Set("Content-Type", asset.MimeType)
	}
	w.Header().Set("Content-Disposition", "inline; filename="+strconv.Quote(asset.OriginalName))
	http.ServeFile(w, r, asset.Path)
}

func (h *Handler) deleteProductAsset(w http.ResponseWriter, r *http.Request) {
	asset, err := h.store.GetProductAsset(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, errors.New("素材不存在"))
		return
	}
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), asset.ProductID); err != nil {
		writeError(w, http.StatusNotFound, errors.New("素材不存在"))
		return
	}
	if err := h.store.DeleteProductAsset(asset.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := os.Remove(asset.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, errors.New("素材记录已删除，但本地文件清理失败"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	product, err := h.store.GetUserProduct(userIDFromRequest(r), chi.URLParam(r, "id"))
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

func (h *Handler) listSpaces(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.ListUserSpaces(userIDFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getIntelligenceDashboard(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.IntelligenceDashboard(userIDFromRequest(r), strings.TrimSpace(r.URL.Query().Get("space_id")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) seedIntelligenceDemo(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SpaceID string `json:"space_id"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&input)
	if err := h.store.SeedIntelligenceDemo(userIDFromRequest(r), strings.TrimSpace(input.SpaceID)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.store.IntelligenceDashboard(userIDFromRequest(r), strings.TrimSpace(input.SpaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) promoteIntelligenceSignal(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SpaceID string `json:"space_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	memory, err := h.store.PromoteSignalToMemory(userIDFromRequest(r), chi.URLParam(r, "id"), strings.TrimSpace(input.SpaceID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, memory)
}

func (h *Handler) updateCreativeMemory(w http.ResponseWriter, r *http.Request) {
	var input jobs.UpdateCreativeMemoryInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 32*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	memory, err := h.store.UpdateCreativeMemory(userIDFromRequest(r), chi.URLParam(r, "id"), input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("创意记忆不存在"))
			return
		}
		writeError(w, http.StatusBadRequest, errors.New("请填写标题和结论，并将置信度设置为 0% 到 100%"))
		return
	}
	writeJSON(w, http.StatusOK, memory)
}

func (h *Handler) deleteCreativeMemory(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteCreativeMemory(userIDFromRequest(r), chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("创意记忆不存在"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createCompetitorMonitor(w http.ResponseWriter, r *http.Request) {
	var input jobs.CreateCompetitorMonitorInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 32*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	monitor, err := h.store.CreateCompetitorMonitor(userIDFromRequest(r), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, monitor)
}

func (h *Handler) scanCompetitorMonitor(w http.ResponseWriter, r *http.Request) {
	signal, err := h.store.ScanCompetitorMonitorDemo(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, signal)
}
func (h *Handler) createSpace(w http.ResponseWriter, r *http.Request) {
	var input jobs.CreateSpaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(input.ProductID) != "" {
		if _, err := h.store.GetUserProduct(userIDFromRequest(r), input.ProductID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("产品资料不可用"))
			return
		}
	}
	space, err := h.store.CreateSpace(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.store.ClaimResource(userIDFromRequest(r), "space", space.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, space)
}

func (h *Handler) updateSpace(w http.ResponseWriter, r *http.Request) {
	if !h.store.OwnsResource(userIDFromRequest(r), "space", chi.URLParam(r, "id")) {
		writeError(w, http.StatusNotFound, sql.ErrNoRows)
		return
	}
	var input jobs.UpdateSpaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(input.ProductID) != "" {
		if _, err := h.store.GetUserProduct(userIDFromRequest(r), input.ProductID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("产品资料不可用"))
			return
		}
	}
	space, err := h.store.UpdateSpace(chi.URLParam(r, "id"), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, space)
}

func (h *Handler) deleteSpace(w http.ResponseWriter, r *http.Request) {
	if !h.store.OwnsResource(userIDFromRequest(r), "space", chi.URLParam(r, "id")) {
		writeError(w, http.StatusNotFound, sql.ErrNoRows)
		return
	}
	if err := h.store.DeleteSpace(chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *Handler) listCreativeReports(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), productID); err != nil {
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
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusNotFound, err)
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
	contextSnapshot, err := h.buildJobContextSnapshot(userIDFromRequest(r), product, strings.TrimSpace(r.FormValue("space_id")))
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
	if err := h.store.ClaimResource(userIDFromRequest(r), "job", job.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	h.runner.Enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, map[string]string{
		"job_id": job.ID,
		"status": job.Status,
	})
}

func (h *Handler) buildJobContextSnapshot(userID string, product *jobs.Product, spaceID string) (string, error) {
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
		space, err := h.store.GetUserSpace(userID, spaceID)
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
		return h.store.GetUserProduct(userIDFromRequest(r), productID)
	}
	mdPath, mdName, err := h.saveRequiredFile(r, "product_md", "markdown")
	if err != nil {
		return nil, errors.New("select a product or upload product Markdown")
	}
	product, err := h.store.CreateProduct(jobs.CreateProductInput{
		Title:  r.FormValue("product_title"),
		MDPath: mdPath,
		MDName: mdName,
	})
	if err != nil {
		return nil, err
	}
	if err := h.store.ClaimResource(userIDFromRequest(r), "product", product.ID); err != nil {
		return nil, err
	}
	return product, nil
}

func (h *Handler) publishJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.store.GetUserJob(userIDFromRequest(r), chi.URLParam(r, "id"))
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
	job, err := h.store.GetUserJob(userIDFromRequest(r), chi.URLParam(r, "id"))
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
	job, err := h.store.GetUserJob(userIDFromRequest(r), chi.URLParam(r, "id"))
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

func (h *Handler) listChats(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.ListUserChats(userIDFromRequest(r))
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
	if err := h.store.ClaimResource(userIDFromRequest(r), "chat", conversation.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, conversation)
}

func (h *Handler) getChat(w http.ResponseWriter, r *http.Request) {
	thread, err := h.store.GetUserChatThread(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

func (h *Handler) getChatProgress(w http.ResponseWriter, r *http.Request) {
	conversationID := chi.URLParam(r, "id")
	if _, err := h.store.GetUserChatThread(userIDFromRequest(r), conversationID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if h.chat == nil {
		writeJSON(w, http.StatusOK, map[string]any{"steps": []jobs.AgentStep{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"steps": h.chat.Progress(conversationID)})
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
	conversationID := chi.URLParam(r, "id")
	threadBefore, err := h.store.GetUserChatThread(userIDFromRequest(r), conversationID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if threadBefore.Conversation.SpaceID != "" {
		space, spaceErr := h.store.GetUserSpace(userIDFromRequest(r), threadBefore.Conversation.SpaceID)
		if spaceErr != nil {
			writeError(w, http.StatusBadRequest, spaceErr)
			return
		}
		input.ProductID = space.ProductID
	}
	thread, err := h.chat.SendWithAttachments(r.Context(), conversationID, input.Content, input.ProductID, attachments)
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
	conversationID := ""
	if input.SpaceID != "" {
		space, spaceErr := h.store.GetUserSpace(userIDFromRequest(r), input.SpaceID)
		if spaceErr != nil {
			writeError(w, http.StatusBadRequest, spaceErr)
			return
		}
		input.ProductID = space.ProductID
		title := input.Content
		if strings.TrimSpace(title) == "" {
			title = "素材分析"
		}
		conversation, createErr := h.store.CreateChatConversationWithContext(title, space.ID, space.ProductID)
		if createErr != nil {
			writeError(w, http.StatusInternalServerError, createErr)
			return
		}
		if err := h.store.ClaimResource(userIDFromRequest(r), "chat", conversation.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		conversationID = conversation.ID
	}
	thread, err := h.chat.SendWithAttachments(r.Context(), conversationID, input.Content, input.ProductID, attachments)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusCreated, thread)
}

type chatMessageInput struct {
	Content   string
	ProductID string
	SpaceID   string
}

func (h *Handler) parseChatMessageInput(r *http.Request) (chatMessageInput, []chatpkg.AttachmentInput, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(storage.MaxChatAttachmentBytes + 1024*1024); err != nil {
			return chatMessageInput{}, nil, err
		}
		input := chatMessageInput{Content: r.FormValue("content"), ProductID: r.FormValue("product_id"), SpaceID: r.FormValue("space_id")}
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
		SpaceID   string `json:"space_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return chatMessageInput{}, nil, err
	}
	return chatMessageInput{Content: input.Content, ProductID: input.ProductID, SpaceID: input.SpaceID}, nil, nil
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
	result, err := h.store.ListUserModelCalls(userIDFromRequest(r), r.URL.Query().Get("ref_id"), r.URL.Query().Get("space_id"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getSpaceObservability(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.GetUserSpace(userIDFromRequest(r), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
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

func (h *Handler) getModelSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.publicModelSettings(userIDFromRequest(r)))
}

func (h *Handler) saveModelSettings(w http.ResponseWriter, r *http.Request) {
	type profileInput struct {
		Capability string `json:"capability"`
		Mode       string `json:"mode"`
		APIKey     string `json:"api_key"`
		Provider   string `json:"provider"`
		Endpoint   string `json:"endpoint"`
		Model      string `json:"model"`
	}
	var input struct {
		APIKey   string         `json:"api_key"`
		Provider string         `json:"provider"`
		Endpoint string         `json:"endpoint"`
		Model    string         `json:"model"`
		Profiles []profileInput `json:"profiles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(input.Profiles) == 0 {
		input.Profiles = []profileInput{{Capability: "text", Mode: "byok",
			APIKey:   input.APIKey,
			Provider: input.Provider,
			Endpoint: input.Endpoint,
			Model:    input.Model}}
	}
	for _, profile := range input.Profiles {
		if !validModelCapability(profile.Capability) {
			writeError(w, http.StatusBadRequest, errors.New("unsupported model capability"))
			return
		}
		if profile.Mode != "managed" && profile.Mode != "byok" {
			writeError(w, http.StatusBadRequest, errors.New("model mode must be managed or byok"))
			return
		}
		if _, err := h.store.SaveUserModelSettings(userIDFromRequest(r), jobs.ModelSettings{Capability: profile.Capability, Mode: profile.Mode, APIKey: profile.APIKey,
			Provider: profile.Provider, Endpoint: profile.Endpoint, Model: profile.Model}); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, h.publicModelSettings(userIDFromRequest(r)))
}

func validModelCapability(value string) bool {
	switch value {
	case "text", "multimodal", "image_generation", "image_edit", "video_generation", "embedding":
		return true
	}
	return false
}

func (h *Handler) publicModelSettings(userID string) jobs.PublicModelConfiguration {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	profiles := []jobs.PublicModelSettings{}
	stored, _ := h.store.ListUserModelSettings(userID)
	for _, settings := range stored {
		configured := settings.Mode == "managed" && strings.TrimSpace(apiKey) != "" || settings.Mode == "byok" && strings.TrimSpace(settings.APIKey) != ""
		profiles = append(profiles, jobs.PublicModelSettings{Capability: settings.Capability, Mode: settings.Mode, Configured: configured,
			Source: valueOr(settings.Mode, "byok"), APIKeyMask: maskAPIKey(settings.APIKey), Provider: settings.Provider,
			Endpoint: settings.Endpoint, Model: settings.Model, UpdatedAt: settings.UpdatedAt})
	}
	if len(profiles) == 0 {
		profiles = append(profiles, jobs.PublicModelSettings{Capability: "text", Mode: "managed", Configured: strings.TrimSpace(apiKey) != "", Source: "managed",
			APIKeyMask: maskAPIKey(apiKey), Provider: "dashscope", Endpoint: valueOr(os.Getenv("DASHSCOPE_ENDPOINT"), "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"), Model: valueOr(os.Getenv("SCRIPT_AGENT_MODEL"), "qwen3.6-plus")})
	}
	configured := false
	for _, profile := range profiles {
		if profile.Configured {
			configured = true
			break
		}
	}
	return jobs.PublicModelConfiguration{Configured: configured, Source: "capability_profiles", Profiles: profiles}
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
