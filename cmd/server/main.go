package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/tian1363/scriptagent/internal/agent"
	"github.com/tian1363/scriptagent/internal/chat"
	"github.com/tian1363/scriptagent/internal/creatibi"
	"github.com/tian1363/scriptagent/internal/creative"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/telemetry"
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
	shutdownTelemetry, telemetryEnabled, err := telemetry.InitLangfuse(context.Background(), telemetry.Config{
		PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"), SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
		BaseURL: env("LANGFUSE_BASE_URL", "https://cloud.langfuse.com"), Environment: env("LANGFUSE_ENVIRONMENT", "development"),
		Release: os.Getenv("LANGFUSE_RELEASE"), CaptureContent: envBool("LANGFUSE_CAPTURE_CONTENT", false),
	})
	if err != nil {
		log.Printf("Langfuse tracing disabled: %v", err)
	} else if telemetryEnabled {
		log.Printf("Langfuse tracing enabled")
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdownTelemetry(ctx); err != nil {
				log.Printf("flush Langfuse traces: %v", err)
			}
		}()
	}

	store, err := jobs.OpenStore(filepath.Join(cfg.DataDir, "scriptagent.db"))
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	fileStore := storage.NewLocalStore(cfg.UploadDir)
	modelClient := buildModelClient(store)
	runner := jobs.NewRunner(store, buildAgent(modelClient))
	handler := webserver.NewHandler(cfg, store, fileStore, runner, buildPublisher(), chat.NewService(store, modelClient), creative.NewService(store, modelClient))
	runner.ResumeUnfinished()
	handler.ResumeVideos()

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

func buildAgent(client *model.DashScopeClient) jobs.Agent {
	mode := env("SCRIPT_AGENT_MODE", "auto")
	if mode == "mock" {
		log.Printf("ScriptAgent model mode: mock")
		return agent.NewMockScriptAgent()
	}
	log.Printf("ScriptAgent model mode: qwen")
	return agent.NewQwenScriptAgent(agent.QwenConfig{
		Client:          client,
		VideoFPS:        envInt("SCRIPT_AGENT_VIDEO_FPS", 2),
		MaxDataURIBytes: int64(envInt("SCRIPT_AGENT_MAX_DATA_URI_MB", 20)) * 1024 * 1024,
	})
}

func buildModelClient(store *jobs.Store) *model.DashScopeClient {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	return model.NewDashScopeClient(model.DashScopeConfig{
		APIKey:              apiKey,
		Endpoint:            env("DASHSCOPE_ENDPOINT", model.DefaultDashScopeEndpoint),
		Model:               env("SCRIPT_AGENT_MODEL", "qwen3.6-plus"),
		EmbeddingEndpoint:   env("DASHSCOPE_EMBEDDING_ENDPOINT", model.DefaultDashScopeEmbeddingEndpoint),
		EmbeddingModel:      env("SCRIPT_AGENT_EMBEDDING_MODEL", "text-embedding-v4"),
		EmbeddingDimensions: envInt("SCRIPT_AGENT_EMBEDDING_DIMENSIONS", 1024),
		Recorder:            store,
		Provider:            store,
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

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
