# Langfuse 可观测性接入

> 最近维护：2026-08-28 · 状态：可选接入 · 界面地址默认为 <https://cloud.langfuse.com/>

本地开发者模式位于 ScriptAgent 前台左侧菜单；它读取 SQLite，是排障事实来源。Langfuse 用于跨运行趋势、Trace 和生成分析，项目详情地址取决于 Langfuse 账号与项目，仓库无法写死。

## Langfuse 是什么

Langfuse 是面向 LLM 和 Agent 应用的开源可观测平台。它可以把一次用户请求中的 Agent、模型生成、Embedding 和工具步骤组织为 Trace，并提供 Token、延迟、错误、模型版本、会话和运行维度的查询与分析。

ScriptAgent 仍以本地 SQLite 记录作为运行事实来源。Langfuse 是可选的外部分析层，不参与任务调度，也不影响生成结果。

## 接入方式

Go 没有 Langfuse 官方 SDK，因此项目使用 OpenTelemetry Go SDK，通过 OTLP/HTTP 将 Span 发送到：

```text
{LANGFUSE_BASE_URL}/api/public/otel/v1/traces
```

请求使用项目 Public Key 和 Secret Key 进行 Basic Auth，并携带 `x-langfuse-ingestion-version: 4`，接入 Langfuse v4 实时摄取链路。

## Trace 结构

脚本任务：

```text
script-generation-workflow (agent)
  ├── video_analysis (generation)
  ├── replica_script (generation)
  └── fission_scripts (generation)
```

通用对话：

```text
chat-agent-loop (agent)
  ├── chat_summary (generation，可选)
  ├── product_embed_index (embedding，可选)
  ├── product_embed_query (embedding，可选)
  └── react_step_01..04 (generation)
```

创意报告模型调用使用 `creative-strategy-report` Trace。

## 数据映射

每个相关 Span 会按 Langfuse v4 OpenTelemetry 属性规范记录：

- `langfuse.trace.name`
- `langfuse.session.id`
- `langfuse.trace.metadata.run_id`
- `langfuse.trace.metadata.space_id`
- `langfuse.trace.metadata.ref_id`
- `langfuse.observation.type`
- `langfuse.observation.model.name`
- `langfuse.observation.input`
- `langfuse.observation.output`
- `langfuse.observation.usage_details`
- `langfuse.environment`
- `langfuse.release`

模型错误会写入 OpenTelemetry Span Status，同时映射为 Langfuse `ERROR` observation。

## 配置

```bash
export LANGFUSE_PUBLIC_KEY="pk-lf-..."
export LANGFUSE_SECRET_KEY="sk-lf-..."
export LANGFUSE_BASE_URL="https://cloud.langfuse.com"
export LANGFUSE_ENVIRONMENT="development"
export LANGFUSE_RELEASE="local-dev"
```

常见区域地址：

- EU：`https://cloud.langfuse.com`
- US：`https://us.cloud.langfuse.com`
- Japan：`https://jp.cloud.langfuse.com`
- 自托管：实例根地址，例如 `http://localhost:3000`

如果两个 Key 都未配置，Langfuse 自动关闭。只配置其中一个 Key 会打印配置错误并关闭外部追踪，服务仍正常启动。

## 内容安全

默认不向 Langfuse 发送 Prompt 和模型输出正文，input/output 会显示为：

```text
[content capture disabled]
```

确认产品资料、用户对话和生成内容允许发送到所选 Langfuse 实例后，才能显式开启：

```bash
export LANGFUSE_CAPTURE_CONTENT="true"
```

视频 Base64 数据永远不会写入 Trace，即使开启内容采集也只记录 `[video data omitted]`。

API Key、模型供应商密钥和 Langfuse Secret Key 不会作为 Span Attribute 上报。

## 失败与关闭语义

- Langfuse 未配置时使用 OpenTelemetry No-op Tracer。
- Span 采用批量异步上报，不阻塞模型请求。
- 上报失败不改变 Job 或 Chat 的状态。
- 服务正常关闭时最多等待 5 秒 Flush 队列。
- 本地 `agent_runs`、`agent_steps` 和 `model_calls` 始终保留。

## 验证

1. 在 Langfuse 创建项目并取得 Public/Secret Key。
2. 配置环境变量并启动服务，日志应出现 `Langfuse tracing enabled`。
3. 创建一次脚本任务或发送一条通用对话。
4. 在 Langfuse Tracing 中检查 Agent 根 observation 及子 generation。
5. 确认模型名、Token、Run ID、Space ID 和错误状态正确。

如果没有 Trace，优先检查区域 Base URL、Key 是否属于同一项目，以及自托管版本是否支持 OpenTelemetry 摄取。
