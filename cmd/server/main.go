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
	authsvc "github.com/tian1363/scriptagent/internal/auth"
	"github.com/tian1363/scriptagent/internal/chat"
	"github.com/tian1363/scriptagent/internal/creatibi"
	"github.com/tian1363/scriptagent/internal/creative"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/memory"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/secret"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/userctx"
	webserver "github.com/tian1363/scriptagent/internal/web"
	"github.com/tian1363/scriptagent/internal/websearch"
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
	secretBox := secret.NewBox(env("APP_ENCRYPTION_KEY", "scriptagent-dev-only-change-me"))
	modelClient := buildModelClient(store, secretBox)
	memoryClient := buildMemoryClient()
	searchClient := buildSearchClient()
	chatService := chat.NewService(store, modelClient, memoryClient)
	chatService.SetSearchClient(searchClient)
	runner := jobs.NewRunner(store, buildAgent(modelClient))
	handler := webserver.NewHandler(cfg, store, fileStore, runner, buildPublisher(), chatService, creative.NewService(store, modelClient), authsvc.NewService(store), secretBox, memoryClient, searchClient)
	runner.ResumeUnfinished()

	log.Printf("ScriptAgent server listening on http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler.Routes()); err != nil {
		log.Fatal(err)
	}
}

func buildMemoryClient() *memory.Client {
	client := memory.NewClient(memory.Config{
		Provider: env("MEM0_PROVIDER", "platform"),
		BaseURL:  os.Getenv("MEM0_BASE_URL"),
		APIKey:   os.Getenv("MEM0_API_KEY"),
		AgentID:  env("MEM0_AGENT_ID", "scriptagent"),
		TopK:     envInt("MEM0_TOP_K", 5),
		Timeout:  time.Duration(envInt("MEM0_TIMEOUT_SECONDS", 15)) * time.Second,
	})
	status := client.Status()
	log.Printf("ScriptAgent memory: provider=%s configured=%t", status.Provider, status.Configured)
	return client
}

func buildSearchClient() *websearch.Client {
	client := websearch.NewClient(websearch.Config{
		Provider:   env("SEARCH_PROVIDER", "tavily"),
		BaseURL:    os.Getenv("SEARCH_BASE_URL"),
		APIKey:     os.Getenv("SEARCH_API_KEY"),
		MaxResults: envInt("SEARCH_MAX_RESULTS", 5),
		Timeout:    time.Duration(envInt("SEARCH_TIMEOUT_SECONDS", 15)) * time.Second,
	})
	status := client.Status()
	log.Printf("ScriptAgent search: provider=%s configured=%t", status.Provider, status.Configured)
	return client
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

func buildModelClient(store *jobs.Store, box *secret.Box) *model.DashScopeClient {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	return model.NewDashScopeClient(model.DashScopeConfig{
		APIKey:              apiKey,
		Endpoint:            env("DASHSCOPE_ENDPOINT", model.DefaultDashScopeEndpoint),
		Model:               env("SCRIPT_AGENT_MODEL", "qwen3.6-plus"),
		EmbeddingEndpoint:   env("DASHSCOPE_EMBEDDING_ENDPOINT", model.DefaultDashScopeEmbeddingEndpoint),
		EmbeddingModel:      env("SCRIPT_AGENT_EMBEDDING_MODEL", "text-embedding-v4"),
		EmbeddingDimensions: envInt("SCRIPT_AGENT_EMBEDDING_DIMENSIONS", 1024),
		Recorder:            store,
		Provider: modelConfigProvider{
			store:       store,
			box:         box,
			envAPIKey:   apiKey,
			envEndpoint: env("DASHSCOPE_ENDPOINT", model.DefaultDashScopeEndpoint),
			envModel:    env("SCRIPT_AGENT_MODEL", "qwen3.6-plus"),
		},
	})
}

type modelConfigProvider struct {
	store       *jobs.Store
	box         *secret.Box
	envAPIKey   string
	envEndpoint string
	envModel    string
}

func (p modelConfigProvider) GetModelRuntimeConfig(ctx context.Context) (model.RuntimeConfig, error) {
	runtime := model.RuntimeConfig{
		APIKey:   p.envAPIKey,
		Endpoint: p.envEndpoint,
		Model:    p.envModel,
		Source:   "env",
	}
	userID := userctx.UserID(ctx)
	if userID == "" {
		return runtime, nil
	}
	settings, err := p.store.GetModelSettings(userID)
	if err != nil {
		return runtime, nil
	}
	apiKey, err := p.box.Decrypt(settings.APIKey)
	if err != nil {
		return runtime, err
	}
	if apiKey == "" {
		return runtime, nil
	}
	runtime.APIKey = apiKey
	runtime.Endpoint = envValue(settings.Endpoint, runtime.Endpoint)
	runtime.Model = envValue(settings.Model, runtime.Model)
	runtime.Source = "user"
	return runtime, nil
}

func envValue(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
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
