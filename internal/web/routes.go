package web

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Config struct {
	Port      string
	DataDir   string
	UploadDir string
	StaticDir string
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(api chi.Router) {
		api.Get("/health", h.health)
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", h.register)
			auth.Post("/login", h.login)
			auth.Post("/logout", h.logout)
			auth.With(h.requireAuth).Get("/me", h.me)
		})
		api.Group(func(private chi.Router) {
			private.Use(h.requireAuth)
			private.Group(func(admin chi.Router) {
				admin.Use(h.requireAdmin)
				admin.Get("/admin/users", h.listAdminUsers)
				admin.Patch("/admin/users/{id}/status", h.updateAdminUserStatus)
			})
			private.Get("/skills", h.listSkills)
			private.Get("/products", h.listProducts)
			private.Post("/products", h.createProduct)
			private.Get("/products/{id}/markdown", h.getProductMarkdown)
			private.Get("/products/{id}/creative-reports", h.listCreativeReports)
			private.Post("/products/{id}/creative-reports", h.createCreativeReport)
			private.Get("/jobs", h.listJobs)
			private.Post("/jobs", h.createJob)
			private.Get("/jobs/{id}", h.getJob)
			private.Get("/jobs/{id}/result", h.getJob)
			private.Post("/jobs/{id}/retry", h.retryJob)
			private.Post("/jobs/{id}/publish", h.publishJob)
			private.Post("/jobs/{id}/video-prompts", h.generateVideoPrompts)
			private.Get("/chats", h.listChats)
			private.Post("/chats", h.createChat)
			private.Post("/chats/messages", h.sendNewChatMessage)
			private.Get("/chats/{id}", h.getChat)
			private.Post("/chats/{id}/messages", h.sendChatMessage)
			private.Get("/model-calls", h.listModelCalls)
			private.Get("/settings/model", h.getModelSettings)
			private.Put("/settings/model", h.saveModelSettings)
			private.Get("/settings/memory", h.getMemorySettings)
			private.Get("/settings/search", h.getSearchSettings)
		})
	})

	if stat, err := os.Stat(h.cfg.StaticDir); err == nil && stat.IsDir() {
		r.Handle("/*", spaHandler(h.cfg.StaticDir))
	}
	return r
}

func spaHandler(staticDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(staticDir))
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	}
}
