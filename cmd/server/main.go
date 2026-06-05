package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tian1363/scriptagent/internal/agent"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	webserver "github.com/tian1363/scriptagent/internal/web"
)

func main() {
	cfg := webserver.Config{
		Port:      env("APP_PORT", "8080"),
		DataDir:   env("DATA_DIR", "./data"),
		UploadDir: env("UPLOAD_DIR", "./uploads"),
		StaticDir: env("STATIC_DIR", "./web/app/dist"),
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}

	store, err := jobs.OpenStore(filepath.Join(cfg.DataDir, "scriptagent.db"))
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	fileStore := storage.NewLocalStore(cfg.UploadDir)
	runner := jobs.NewRunner(store, agent.NewMockScriptAgent())
	handler := webserver.NewHandler(cfg, store, fileStore, runner)
	runner.ResumeUnfinished()

	log.Printf("ScriptAgent server listening on http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler.Routes()); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
