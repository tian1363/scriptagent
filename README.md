# ScriptAgent

ScriptAgent is a CreatiBI script-generation workspace. Users upload a reference video and a product Markdown file, then generate a replica script and multiple fission scripts.

## Current Status

This repository currently contains the first application scaffold:

- Go API server.
- SQLite-backed job history.
- Local upload storage.
- React + Vite frontend.
- CreatiBI Design System tokens and assets.
- Qwen mode with model settings configurable from the UI or environment variables.
- Product library for reusable product Markdown files.

## Run Locally

Install frontend dependencies:

```bash
cd web/app
npm install
```

Run the frontend dev server:

```bash
npm run dev
```

Run the Go API server from the repository root:

```bash
go run ./cmd/server
```

The frontend dev server runs at `http://127.0.0.1:5173`. The API server runs at `http://localhost:8080`.

## Configure Qwen

You can configure Qwen from the app UI at `配置 -> 模型配置`. For server-level defaults, set your DashScope API key before starting the Go server:

```bash
export DASHSCOPE_API_KEY="sk-your-key"
export SCRIPT_AGENT_MODEL="qwen3.6-plus"
export SCRIPT_AGENT_VIDEO_FPS="2"
export SCRIPT_AGENT_EMBEDDING_MODEL="text-embedding-v4"
export SCRIPT_AGENT_EMBEDDING_DIMENSIONS="1024"
go run ./cmd/server
```

User-saved model settings take precedence over environment variables. To force mock mode:

```bash
export SCRIPT_AGENT_MODE="mock"
```

The Qwen implementation sends local videos as Base64 data URLs. If a video would exceed the DashScope data-uri limit, ScriptAgent automatically creates a temporary compressed MP4 with `ffmpeg` and sends that instead. You can tune the data-uri ceiling:

```bash
export SCRIPT_AGENT_MAX_DATA_URI_MB="20"
```

Product Markdown retrieval uses embeddings for long documents. ScriptAgent stores product chunks in SQLite and uses DashScope embeddings for semantic Top-K retrieval; if embedding fails, it falls back to local keyword section matching.

## Configure Mem0 Memory

Mem0 is optional. When configured, general chat retrieves relevant cross-conversation memories before each ReAct run and asynchronously stores the successful user/assistant turn afterward. Memories are isolated with ScriptAgent's internal user ID.

For SaaS deployments, the operator configures one server-side Mem0 key for the whole deployment. End users do not provide Mem0 credentials; every search and write is scoped by the authenticated ScriptAgent `user_id`.

Mem0 Platform:

```bash
export MEM0_PROVIDER="platform"
export MEM0_API_KEY="m0-your-key"
export MEM0_AGENT_ID="scriptagent"
export MEM0_TOP_K="5"
```

Self-hosted Mem0 OSS:

```bash
export MEM0_PROVIDER="oss"
export MEM0_BASE_URL="http://127.0.0.1:8888"
export MEM0_API_KEY="optional-self-hosted-key"
```

If Mem0 is unavailable or not configured, chat continues with the existing local recent-message and conversation-summary strategy. Platform mode sends selected chat turns to Mem0's hosted service; use OSS mode when memory data must stay in your own deployment.

## Configure CreatiBI Publishing

The publish action uses the local `cbi` CLI. Make sure you are logged in:

```bash
cbi auth whoami
```

Optionally pin the target project. If omitted, ScriptAgent uses the first project returned by `cbi project list`.

```bash
export CREATIBI_PROJECT_ID="9944"
export CREATIBI_CLI_BIN="cbi"
```

## Persistence

Runtime data is persisted locally:

- SQLite database: `data/scriptagent.db`
- Uploaded files: `uploads/`

Closing and reopening the program keeps previous jobs and uploaded files as long as these directories are not deleted.

## Documentation

- Requirements: `docs/requirements.md`
- Technical design: `docs/technical-design.md`
