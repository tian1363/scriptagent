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
		api.Get("/auth/status", h.authStatus)
		api.Post("/auth/register", h.register)
		api.Post("/auth/login", h.login)
		api.Post("/auth/logout", h.logout)
		api.With(h.requireAuth).Get("/auth/me", h.me)
		api.Group(func(private chi.Router) {
			private.Use(h.requireAuth)
			private.Get("/skills", h.listSkills)
			private.Post("/skills/draft", h.generateSkillDraft)
			private.Post("/skills", h.createSkill)
			private.Put("/skills/{id}", h.updateSkill)
			private.Get("/products", h.listProducts)
			private.Post("/products", h.createProduct)
			private.Post("/products/parse", h.parseProductDocument)
			private.Put("/products/{id}", h.updateProduct)
			private.Get("/products/{id}/markdown", h.getProductMarkdown)
			private.Get("/products/{id}/assets", h.listProductAssets)
			private.Post("/products/{id}/assets", h.uploadProductAsset)
			private.Get("/assets/{id}/file", h.getProductAssetFile)
			private.Delete("/assets/{id}", h.deleteProductAsset)
			private.Get("/products/{id}/creative-reports", h.listCreativeReports)
			private.Post("/products/{id}/creative-reports", h.createCreativeReport)
			private.Get("/spaces", h.listSpaces)
			private.Post("/spaces", h.createSpace)
			private.Put("/spaces/{id}", h.updateSpace)
			private.Delete("/spaces/{id}", h.deleteSpace)
			private.Get("/spaces/{id}/observability", h.getSpaceObservability)
			private.Get("/intelligence", h.getIntelligenceDashboard)
			private.Post("/intelligence/demo", h.seedIntelligenceDemo)
			private.Post("/intelligence/signals/{id}/memory", h.promoteIntelligenceSignal)
			private.Put("/intelligence/memories/{id}", h.updateCreativeMemory)
			private.Delete("/intelligence/memories/{id}", h.deleteCreativeMemory)
			private.Post("/intelligence/competitors", h.createCompetitorMonitor)
			private.Post("/intelligence/competitors/{id}/scan", h.scanCompetitorMonitor)
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
			private.Get("/chats/{id}/progress", h.getChatProgress)
			private.Post("/chats/{id}/messages", h.sendChatMessage)
			private.Get("/model-calls", h.listModelCalls)
			private.Get("/videos", h.listVideos)
			private.Get("/suggestions", h.listProactiveSuggestions)
			private.Post("/suggestions/{id}/status", h.updateProactiveSuggestionStatus)
			private.Post("/videos", h.createVideo)
			private.Get("/videos/{id}", h.getVideo)
			private.Get("/videos/{id}/file", h.getVideoFile)
			private.Post("/videos/{id}/retry", h.retryVideo)
			private.Get("/settings/model", h.getModelSettings)
			private.Put("/settings/model", h.saveModelSettings)
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
