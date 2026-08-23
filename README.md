# ScriptAgent

ScriptAgent 是一个可执行创作任务的工作台。用户把产品资料、长期目标和参考素材放进来，Agent 会生成并持续推进脚本任务。

## Current Status

This repository currently contains the first application scaffold:

- Go API server.
- SQLite-backed job history.
- Local upload storage.
- React + Vite frontend.
- Agent 首页、全局历史、创作空间与可编辑的产品资料库。
- Qwen mode with model settings configurable from the UI or environment variables.
- 产品资料可通过上传 Markdown、粘贴文字或直接编辑建立；创作空间会保留目标、产品和任务关系。

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

## 工作方式

- **开始创作**：描述目标、选择资料，进入脚本执行流程。
- **历史**：统一查看过去的任务和对话，从原位置继续。
- **创作空间**：为长期项目保存目标、要求、产品资料和执行记录。
- **产品资料**：资料会在任务中复用，可随时修改；右侧检查提示还缺哪些关键信息。

## Persistence

Runtime data is persisted locally:

- SQLite database: `data/scriptagent.db`
- Uploaded files: `uploads/`

Closing and reopening the program keeps previous jobs and uploaded files as long as these directories are not deleted.

## Documentation

- Requirements: `docs/requirements.md`
- Technical design: `docs/technical-design.md`
