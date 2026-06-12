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
		api.Get("/skills", h.listSkills)
		api.Get("/products", h.listProducts)
		api.Post("/products", h.createProduct)
		api.Get("/products/{id}/markdown", h.getProductMarkdown)
		api.Get("/products/{id}/creative-reports", h.listCreativeReports)
		api.Post("/products/{id}/creative-reports", h.createCreativeReport)
		api.Get("/jobs", h.listJobs)
		api.Post("/jobs", h.createJob)
		api.Get("/jobs/{id}", h.getJob)
		api.Get("/jobs/{id}/result", h.getJob)
		api.Post("/jobs/{id}/retry", h.retryJob)
		api.Post("/jobs/{id}/publish", h.publishJob)
		api.Post("/jobs/{id}/video-prompts", h.generateVideoPrompts)
		api.Get("/chats", h.listChats)
		api.Post("/chats", h.createChat)
		api.Post("/chats/messages", h.sendNewChatMessage)
		api.Get("/chats/{id}", h.getChat)
		api.Post("/chats/{id}/messages", h.sendChatMessage)
		api.Get("/model-calls", h.listModelCalls)
		api.Get("/settings/model", h.getModelSettings)
		api.Put("/settings/model", h.saveModelSettings)
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
