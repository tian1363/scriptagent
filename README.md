# ScriptAgent

> 文档基线：2026-08-28 · 当前分支：`main`

ScriptAgent 是面向广告内容生产的 Agent 工作台。用户维护产品资料与图片/视频素材，在创作空间中定义广告目标和营销阶段，再通过对话让 Agent 完成素材分析、策略、脚本与分镜提示词等任务。

## 当前产品

- **开始**：直接向 Agent 描述任务，可选择产品资料。
- **历史**：统一检索对话与脚本执行记录。
- **创作空间**：管理长期目标、产品资料、广告目标和营销阶段；空间设置只影响后续对话。
- **产品资料**：编辑 Markdown，上传图片和视频；素材会参与多模态解析并以 CID 方式进入脚本与分镜上下文。
- **对话**：ReAct Agent 按需使用产品检索、Skill 和素材；只展示摘要、行动与结果。
- **Skill**：预置 Skill 只读；用户可用一句话生成自己的 Skill，之后可预览和编辑。
- **设置**：按文本、多模态、图片、视频和向量能力配置模型；支持托管与 BYOK。
- **开发者模式**：查看模型调用、Token、耗时、输入输出和 Agent 步骤。
- **Langfuse**：可选外部观测层；本地 SQLite 始终保留运行记录。

当前版本支持多账号注册、登录和 HttpOnly Session 保护。产品资料、创作空间、对话、任务、自定义 Skill、模型配置和调试记录按账号隔离；升级前的历史数据自动归属最早创建的账号。正式运营后台尚未上线。

## 快速启动

```bash
cd web/app
npm install
npm run build
cd ../..
go run ./cmd/server
```

打开：<http://127.0.0.1:8080/>

未登录时可选择登录或注册。创建成功后，产品资料、创作空间、对话和设置 API 都需要登录才能访问。

开发前端时可另开终端：

```bash
cd web/app
npm run dev
```

前端开发地址为 <http://127.0.0.1:5173/>；它仍调用 `8080` 上的 Go API。完整地址见 [产品地址清单](docs/product-addresses.md)。

## 模型配置

推荐在产品的“设置”中按能力配置模型。服务端托管模式至少需要：

```bash
export DASHSCOPE_API_KEY="sk-your-key"
export SCRIPT_AGENT_MODEL="qwen3.6-plus"
go run ./cmd/server
```

常用可选配置：

```bash
export SCRIPT_AGENT_MODE="mock"                    # 不调用真实模型
export SCRIPT_AGENT_VIDEO_FPS="2"
export SCRIPT_AGENT_MAX_DATA_URI_MB="20"
export SCRIPT_AGENT_EMBEDDING_MODEL="text-embedding-v4"
export SCRIPT_AGENT_EMBEDDING_DIMENSIONS="1024"
```

用户保存的能力配置优先于环境变量，并按账号隔离。API Key 只由后端保存，前端接口仅返回掩码。

## Langfuse

```bash
export LANGFUSE_PUBLIC_KEY="pk-lf-..."
export LANGFUSE_SECRET_KEY="sk-lf-..."
export LANGFUSE_BASE_URL="https://cloud.langfuse.com"
export LANGFUSE_ENVIRONMENT="development"
go run ./cmd/server
```

默认不上传 Prompt 和模型输出正文。只有经过数据安全确认后才开启：

```bash
export LANGFUSE_CAPTURE_CONTENT="true"
export LANGFUSE_RELEASE="local-dev"
```

## 数据与安全

- SQLite：`data/scriptagent.db`
- 上传文件：`uploads/`
- 前端构建产物：`web/app/dist/`
- 默认监听端口：`8080`
- 健康检查：`GET /api/health`

删除 `data/` 或 `uploads/` 会造成不可恢复的数据丢失。公网部署必须启用 HTTPS；面向多个组织开放前还需要补齐租户隔离、密钥加密、细粒度权限和备份策略。

## 验证

```bash
npm --prefix web/app run build
go test ./internal/jobs ./internal/chat ./internal/web ./internal/agent
```

## 文档导航

- [文档中心](docs/README.md)
- [产品地址清单](docs/product-addresses.md)
- [需求基线](docs/requirements.md)
- [技术设计](docs/technical-design.md)
- [Agent Runtime 与 Harness](docs/agent-runtime-harness.md)
- [模型能力与 BYOK](docs/model-capabilities.md)
- [Token 优化策略](docs/token-optimization.md)
- [Langfuse 可观测性](docs/langfuse-observability.md)
- [品牌规范](brand-spec.md)
- [设计验证记录](design-qa.md)

## 仓库

- GitHub：<https://github.com/tian1363/scriptagent>
- 当前主分支：`main`
