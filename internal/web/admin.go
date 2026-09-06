package web

import (
	"github.com/tian1363/scriptagent/internal/userctx"
	"net/http"
)

func (h *Handler) isAdministrator(r *http.Request) bool {
	current, ok := userctx.FromContext(r.Context())
	return ok && h.cfg.AdminUserID != "" && current.ID == h.cfg.AdminUserID && current.Role == "admin"
}

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.isAdministrator(r) {
			writeError(w, http.StatusForbidden, &publicError{"无权访问运营后台"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) ownerSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": h.isAdministrator(r)})
}

func (h *Handler) ownerOverview(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.AdminOverview()
	if err != nil {
		writeError(w, http.StatusInternalServerError, &publicError{"运营数据暂时不可用"})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
