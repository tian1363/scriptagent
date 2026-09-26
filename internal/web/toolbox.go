package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	chatpkg "github.com/tian1363/scriptagent/internal/chat"
)

type toolboxDrafter interface {
	GenerateToolboxDraft(context.Context, chatpkg.ToolboxDraftInput) (*chatpkg.ToolboxDraft, error)
}

func (h *Handler) generateToolboxDraft(w http.ResponseWriter, r *http.Request) {
	var input chatpkg.ToolboxDraftInput
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("输入格式无效或内容过长"))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, errors.New("请只提交一份生成请求"))
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	generator, ok := h.chat.(toolboxDrafter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, errors.New("AI 服务尚未配置"))
		return
	}
	if !h.authLimit.allow("toolbox:"+userIDFromRequest(r), 10, time.Minute) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, errors.New("生成请求较多，请稍后再试"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	result, err := generator.GenerateToolboxDraft(ctx, input)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
