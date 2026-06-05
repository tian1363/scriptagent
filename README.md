# ScriptAgent

ScriptAgent is a CreatiBI script-generation workspace. Users upload a reference video and a product Markdown file, then generate a replica script and multiple fission scripts.

## Current Status

This repository currently contains the first application scaffold:

- Go API server.
- SQLite-backed job history.
- Local upload storage.
- React + Vite frontend.
- CreatiBI Design System tokens and assets.
- Qwen mode when `DASHSCOPE_API_KEY` is configured, with mock mode fallback for local workflow verification.

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

Set your DashScope API key before starting the Go server:

```bash
export DASHSCOPE_API_KEY="sk-your-key"
export SCRIPT_AGENT_MODEL="qwen3.6-plus"
export SCRIPT_AGENT_VIDEO_FPS="2"
go run ./cmd/server
```

When `DASHSCOPE_API_KEY` is present, the server uses Qwen mode automatically. To force mock mode:

```bash
export SCRIPT_AGENT_MODE="mock"
```

The first Qwen implementation sends local videos as Base64 data URLs. Keep videos modest in size, or set a different limit:

```bash
export SCRIPT_AGENT_MAX_VIDEO_MB="80"
```

## Persistence

Runtime data is persisted locally:

- SQLite database: `data/scriptagent.db`
- Uploaded files: `uploads/`

Closing and reopening the program keeps previous jobs and uploaded files as long as these directories are not deleted.

## Documentation

- Requirements: `docs/requirements.md`
- Technical design: `docs/technical-design.md`
