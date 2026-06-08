package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/tian1363/scriptagent/internal/agent"
	"github.com/tian1363/scriptagent/internal/creatibi"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
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
	runner := jobs.NewRunner(store, buildAgent())
	handler := webserver.NewHandler(cfg, store, fileStore, runner, buildPublisher())
	runner.ResumeUnfinished()

	log.Printf("ScriptAgent server listening on http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler.Routes()); err != nil {
		log.Fatal(err)
	}
}

func buildPublisher() webserver.Publisher {
	return creatibi.NewCLIPublisher(creatibi.Config{
		Bin:       env("CREATIBI_CLI_BIN", "cbi"),
		ProjectID: envInt("CREATIBI_PROJECT_ID", 0),
		Timeout:   time.Duration(envInt("CREATIBI_PUBLISH_TIMEOUT_SECONDS", 120)) * time.Second,
	})
}

func buildAgent() jobs.Agent {
	mode := env("SCRIPT_AGENT_MODE", "auto")
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if mode == "mock" || (mode == "auto" && apiKey == "") {
		log.Printf("ScriptAgent model mode: mock")
		return agent.NewMockScriptAgent()
	}

	client := model.NewDashScopeClient(model.DashScopeConfig{
		APIKey:   apiKey,
		Endpoint: env("DASHSCOPE_ENDPOINT", model.DefaultDashScopeEndpoint),
		Model:    env("SCRIPT_AGENT_MODEL", "qwen3.6-plus"),
	})
	log.Printf("ScriptAgent model mode: qwen")
	return agent.NewQwenScriptAgent(agent.QwenConfig{
		Client:       client,
		VideoFPS:     envInt("SCRIPT_AGENT_VIDEO_FPS", 2),
		MaxVideoSize: int64(envInt("SCRIPT_AGENT_MAX_VIDEO_MB", 80)) * 1024 * 1024,
	})
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
