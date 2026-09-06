package web

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (h *Handler) listInvites(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListInvites()
	if err != nil {
		writeError(w, 500, &publicError{"读取邀请码失败"})
		return
	}
	writeJSON(w, 200, items)
}
func (h *Handler) createInvite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Label string `json:"label"`
		Days  int    `json:"days"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, 400, &publicError{"请填写有效参数"})
		return
	}
	code, err := h.store.GenerateInvite(input.Label, input.Days)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, map[string]string{"code": code})
}
func (h *Handler) revokeInvite(w http.ResponseWriter, r *http.Request) {
	if err := h.store.RevokeInvite(chi.URLParam(r, "id")); err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
