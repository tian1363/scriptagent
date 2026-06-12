# ScriptAgent 技术设计确认文档

## 1. 项目目标

ScriptAgent 是一个用于生成 CreatiBI 分镜脚本的工作台型应用。用户通过前端上传参考视频和产品信息 Markdown 文件，系统调用多模态模型完成视频理解，生成 1 条复刻脚本和多条裂变脚本，并在用户确认后通过 CreatiBI CLI/API 创建符合分镜格式的脚本至 CreatiBI。

本项目第一版定位为内部生产工具，重点是稳定、可追踪、可复查，而不是开放式聊天产品。

## 2. 已确认条件

- 后端语言使用 Go。
- 前端需要界面，风格简洁，以输入框和结果查看为主。
- 前端必须支持历史记录。
- 产品信息由用户上传 Markdown 文件提供。
- 用户通过前端页面上传参考视频素材。
- 视频不需要先上传 CreatiBI 素材库再理解，后端可直接调模型理解视频。
- 模型由用户提供，第一版使用 `qwen3.6-plus`。
- 前端组件和视觉规范使用 `CreatiBI Design System (Remix)(2).zip`。
- 第一版采用单 Agent 工作流，不采用复杂 Multi-Agent 架构。
- 需求文档必须持续维护，见 `docs/requirements.md`。
- 每次代码变更必须使用 Git 管理，相关约束写入需求文档。

## 3. 非目标

第一版暂不做以下内容：

- 不做完全自由的多 Agent 协商系统。
- 不做投放策略、预算建议、广告平台自动投放。
- 不做复杂素材关系树分析。
- 不做团队权限、审批流、多人协同编辑。
- 不做大型脚本编辑器，只做生成结果查看和发布前确认。
- 不自动把生成结果写入 CreatiBI，默认由用户确认后手动发布。

## 4. Agent 类型与架构选择

当前采用“顺序脚本 Agent + ReAct 通用对话 Agent”并存：

```text
ScriptAgent
├── 读取产品 Markdown
├── 调用 qwen3.6-plus 分析视频
├── 输出详细分镜表
├── 提取复刻结构
├── 生成复刻脚本
├── 生成多条裂变脚本
├── 校验 CreatiBI 分镜格式
├── 保存历史记录
└── 用户确认后写入 CreatiBI
```

脚本任务仍保留顺序工作流的原因：

- 当前流程顺序固定，不需要多个 Agent 自主协商。
- 视频理解、结构提取、脚本生成可通过明确 Prompt 和 Schema 顺序执行。
- 单 Agent 更容易调试失败点。
- 延迟和成本更低。
- 更适合 Go 后端实现任务编排。

通用对话升级为 ReAct 编排：

```text
用户消息
  ↓
ReAct Runner
  ├── qwen3.6-plus 输出下一步 JSON 动作
  ├── Tool: list_products
  ├── Tool: retrieve_product_sections
  ├── Tool: read_product_markdown
  ├── Tool: call_skill
  └── qwen3.6-plus 输出 final answer
  ↓
保存助手回复 + 返回 Agent steps / citations
```

后续只有在游戏、电商、审核、批处理、投放策略等能力显著复杂化时，才考虑升级为主控编排型 Multi-Agent。

当前前端使用 React。通用对话后端使用 ReAct（Reason + Act）循环，但前端只展示可审计的决策摘要、工具名、工具输入和工具观察，不展示或要求模型隐式逐字思维链。对话前端采用乐观消息展示，用户消息发送后立即上屏，等待模型期间展示思考状态，模型返回后用打字机效果渲染助手回复，并展示本轮 Agent 工具步骤。

## 5. 总体系统架构

```text
用户
  ↓
React 前端
  ↓
Go API Server
  ↓
Job Runner
  ↓
ScriptAgent
  ├── qwen3.6-plus 视频理解
  ├── qwen3.6-plus 脚本生成
  ├── Go 规则校验
  └── CreatiBI Publisher
  ↓
SQLite 历史记录 + 本地文件存储
```

## 6. 推荐技术栈

### 6.1 后端

- Go 1.22+
- HTTP 框架：Gin 或 chi，推荐 chi，轻量且适合 API 服务。
- 数据库：SQLite。
- 文件存储：本地 `uploads/`，后续可替换为 OSS/S3。
- 任务与上传文件必须本地持久化，程序重启后历史记录仍可查看。
- 任务执行：Go goroutine + 状态表。
- 任务进度：第一版使用轮询，后续可升级 SSE。
- 任务日志：保存可审计步骤摘要，不保存或展示模型隐式逐字思维链。
- CreatiBI 写入：优先封装为独立 `creatibi` 模块，内部可调用 CLI 或 API。

### 6.2 前端

- React + Vite。
- 使用 CreatiBI Design System 的 token、logo、组件规范。
- 前端由 Go 服务托管 Vite build 后的静态文件。
- 不做营销 landing page，首屏就是工作台。

### 6.3 模型

- 模型名称：`qwen3.6-plus`。
- 接入平台：DashScope/百炼。
- 主要用途：
  - 视频理解。
  - 分镜表生成。
  - 复刻结构提取。
  - 复刻脚本生成。
  - 裂变脚本生成。

## 7. 项目目录结构

```text
scriptagent/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── agent/
│   │   ├── script_agent.go
│   │   ├── prompts.go
│   │   ├── schemas.go
│   │   └── validator.go
│   ├── creatibi/
│   │   ├── publisher.go
│   │   └── types.go
│   ├── jobs/
│   │   ├── runner.go
│   │   ├── store.go
│   │   └── types.go
│   ├── model/
│   │   ├── dashscope_client.go
│   │   └── types.go
│   ├── storage/
│   │   └── files.go
│   └── web/
│       ├── handlers.go
│       └── routes.go
├── web/
│   ├── app/
│   │   ├── src/
│   │   ├── index.html
│   │   └── package.json
│   ├── design-system/
│   │   ├── colors_and_type.css
│   │   └── assets/
│   └── dist/
├── data/
│   └── scriptagent.db
├── uploads/
├── docs/
│   └── technical-design.md
├── go.mod
└── README.md
```

## 8. 前端设计要求

前端使用 CreatiBI 工作台风格，保持紧凑、白底、操作导向。

### 8.1 页面结构

```text
Top NavBar
左侧历史记录栏
右侧主工作区
```

默认主导航包含：

- 脚本任务。
- 产品库。
- 通用对话。
- 配置。

默认打开产品库。产品库作为首页使用全宽资产看板，不显示左侧产品列表栏；产品以大卡片展示视觉封面占位、产品名称、Markdown 文件名、已生成脚本数量、历史任务数量和最近生成时间。用户可以从产品卡片直接进入创建脚本任务，前端自动预选该产品。

模型调用属于开发者调试台，默认不与普通功能并列展示。用户需要在配置页的开发者选项中开启后，前端才显示“调试台”入口。

脚本任务工作区包含：

- 产品资产选择/新建，作为创建任务第一步。
- 参考视频上传。
- 行业选择：自动判断 / 游戏 / 电商。
- 裂变脚本数量输入。
- 补充要求输入框。
- 裂变方向前置选项：按裂变数量生成 N 行卡片按钮组，每行对应 1 条裂变脚本，且每行只能选择 1 个裂变元素。
- 生成按钮。
- 任务进度。
- 结果 Tabs：
  - 视频分析
  - 复刻脚本
  - 裂变脚本
  - CreatiBI 写入

创建任务区按产品资产、参考视频、生成设置、补充要求和裂变方向分组。裂变方向不使用裸下拉框，而以视听层、结构层、元素层按钮卡片展示，表单提交时通过隐藏字段按顺序提交 `fission_directions`。结果区按内容类型展示：运行日志渲染为步骤时间线，视频分析 Markdown 渲染标题和表格，JSON 结果展示脚本摘要卡片并保留原始 JSON。
产品资料库用于保存产品名称和产品 Markdown 文件，并支持在产品详情中读取和预览 Markdown 内容。通用对话空状态展示任务式快捷入口，引导用户从生成裂变方向、编写产品 Markdown、优化脚本等具体任务开始。模型配置页用于保存用户自己的 DashScope API Key、Endpoint 和模型名；用户配置优先于环境变量，并前置展示 API Key 本地保存和数据库文件安全提示。配置页开发者选项控制调试台是否显示。

历史记录包含：

- 任务标题。
- 创建时间。
- 状态。
- 视频文件名。
- 产品 Markdown 文件名。
- 裂变数量。
- 裂变方向选择，按脚本顺序展示。
- 是否已写入 CreatiBI。

### 8.2 CreatiBI 设计规范约束

- 主色使用 `#8B4EEF`。
- 字体使用 Inter + Noto Sans SC。
- 页面背景以白色和 `#F9F8F8` 为主。
- 按钮和输入框圆角 6px。
- 卡片/面板圆角 12px。
- 文案使用 sentence case，中文文案保持简洁直接。
- 不使用 emoji。
- 不使用通用 AI 风格的大面积装饰渐变、毛玻璃效果；产品资产封面可使用受控色块/渐变作为缩略图占位。
- Tabs 使用紧凑样式，激活态黑色下划线。
- 控件密度保持紧凑，避免营销页式大 Hero。

## 9. 核心流程

### 9.1 创建任务

1. 用户先选择已有产品，或上传产品信息 Markdown 新建产品。
2. 用户上传参考视频。
3. 用户填写补充要求。
4. 用户设置行业和裂变脚本数量。
5. 用户按脚本顺序为每条裂变脚本选择 1 个裂变元素。
6. 前端请求创建任务。
7. 后端保存文件并创建 `job` 记录。
8. Job Runner 异步执行 ScriptAgent。

### 9.2 视频理解

ScriptAgent 将视频和产品 Markdown 输入模型，使用固定视频理解 Prompt，输出：

- 行业判断。
- 命名统一表。
- Markdown 详细分镜表。
- 核心亮点总结。

视频理解必须遵守：

- 只填视频中能直接观察到的内容。
- 无法判断填 `-`。
- 产品卖点可基于画面合理推断。
- 分镜表必须用 Markdown 表格输出。
- 同一道具、角色、场景、UI 元素必须全表统一命名。
- 游戏类追加游戏素材类型、游戏 UI 描述。
- 电商类追加产品素材类型、产品展示描述。

### 9.3 复刻结构提取

从视频理解结果提取：

- 创意结构。
- 镜头功能。
- 钩子方式。
- 卖点呈现方式。
- 节奏分布。
- CTA 方式。
- 必须保留的复刻元素。
- 可裂变替换的元素。

### 9.4 复刻脚本生成

生成 1 条复刻脚本。

约束：

- 复刻结构，不复制原片独有台词。
- 保留镜头功能和节奏。
- 替换成产品 Markdown 中的产品信息。
- 不复用原视频品牌名、人物肖像、版权音乐等敏感元素。
- 输出符合 CreatiBI 分镜格式。

### 9.5 裂变脚本生成

生成 N 条裂变脚本。

裂变维度以前置可选项展示给用户。界面根据 `fission_count` 动态生成 N 行单选下拉，每行只允许选择 1 个裂变元素：

- 视听层：换BGM、换音效、换色调/滤镜、换字幕&花字、换画幅、换配音(语速/声线)。
- 结构层：换开头钩子、换CTA、时长压缩/拉伸、变速·节奏调整、换首帧/封面、同素材高光重剪。
- 元素层：换局部角色/群演、换局部场景贴片、换局部道具/UI、字幕语言本地化。

用户选择按行保存为 `fission_directions`，第 1 行对应第 1 条 `fission_scripts`，第 2 行对应第 2 条 `fission_scripts`，依此类推。同一裂变元素允许重复选择。

每条裂变脚本只能使用 1 个裂变元素。后端在解析模型返回后校验 `metadata.fission_dimension`，该字段必须精准匹配对应行选择的方向；包含多个方向的组合值、未知方向或顺序不匹配视为无效输出。

每条裂变脚本必须标注：

- 父脚本。
- 裂变方向。
- 保留元素。
- 替换元素。
- 适用测试目标。

### 9.6 校验

Go 规则校验第一版至少检查：

- 是否有标题。
- 是否有脚本类型。
- 是否有分镜数组。
- 每个分镜是否有时间段。
- 每个分镜是否有画面描述。
- 每个分镜是否有动作描述。
- 每个分镜是否有旁白/字幕字段。
- 每个分镜是否有镜头说明。
- 裂变脚本数量是否等于用户要求。

校验失败时，任务状态记为 `failed`，保存错误原因。后续可支持自动重试。

### 9.7 CreatiBI 写入

第一版采用用户确认后手动发布：

1. 用户查看生成结果。
2. 用户点击发布至 CreatiBI。
3. 后端调用 CreatiBI Publisher。
4. Publisher 调用 CreatiBI CLI/API。
5. 保存 CreatiBI 返回的脚本 ID、链接或错误信息。

## 10. API 设计

### 10.1 创建任务

```http
POST /api/jobs
```

表单字段：

- `video`: 视频文件。
- `product_id`: 可选，已有产品 ID。
- `product_md`: 可选，未选择 `product_id` 时必须上传产品 Markdown 文件。
- `product_title`: 可选，上传新产品 Markdown 时的产品名称。
- `requirement`: 用户补充要求。
- `industry`: `auto` / `game` / `ecommerce`。
- `fission_count`: 裂变数量。
- `fission_directions`: 裂变方向，允许重复，按表单控件顺序提交，第 N 个值对应第 N 条裂变脚本。

返回：

```json
{
  "job_id": "job_...",
  "status": "pending"
}
```

### 10.1A 产品资料库

```http
GET /api/products
POST /api/products
GET /api/products/{id}/markdown
```

`POST /api/products` 使用 multipart 表单：

- `title`: 产品名称，可选。
- `product_md`: 产品 Markdown 文件，必填。

`GET /api/products/{id}/markdown` 返回：

- `id`: 产品 ID。
- `title`: 产品名称。
- `md_name`: Markdown 原始文件名。
- `content`: Markdown 文件内容，用于前端预览。

约束：

- 预览接口只读取该产品记录关联的 Markdown 文件。
- 预览内容受 Markdown 文件大小上限保护。

### 10.1B 模型配置

```http
GET /api/settings/model
PUT /api/settings/model
```

`PUT /api/settings/model` JSON 字段：

- `api_key`: DashScope API Key，留空时保留原 Key。
- `endpoint`: DashScope Endpoint。
- `model`: 模型名，例如 `qwen3.6-plus`。

接口响应只返回脱敏 Key，不返回完整 API Key。

### 10.2 获取历史记录

```http
GET /api/jobs
```

### 10.3 获取任务详情

```http
GET /api/jobs/{id}
```

### 10.4 获取任务结果

```http
GET /api/jobs/{id}/result
```

### 10.5 发布至 CreatiBI

```http
POST /api/jobs/{id}/publish
```

### 10.6 重试任务

```http
POST /api/jobs/{id}/retry
```

约束：

- 只允许非运行中的任务重试。
- 重试复用原输入文件和参数。
- 重试清空旧生成结果与发布结果，并追加运行日志。

### 10.7 通用对话

```http
GET /api/chats
GET /api/chats/{id}
POST /api/chats/messages
POST /api/chats/{id}/messages
```

`POST /api/chats/messages` 和 `POST /api/chats/{id}/messages` 使用 JSON：

- `content`: 用户消息，必填。
- `product_id`: 可选，产品库产品 ID。传入后，ReAct Agent 可在需要产品资料时优先使用该产品调用检索工具。

约束：

- `product_id` 只影响本次消息的 ReAct 工具默认产品，不强制绑定整个会话。
- 未传 `product_id` 时仍执行 ReAct 通用对话；如果问题依赖产品资料，Agent 可先调用 `list_products`。
- 模型调用日志中记录的是脱敏后的可见请求体，不记录 API Key。
- ReAct Runner 要求模型每轮只输出一个严格 JSON 动作：`type=tool` 调用工具，或 `type=final` 输出最终答案。
- ReAct 动作中的 `reason` 字段只保存可见决策摘要，不保存隐式逐字思维链。
- Prompt 上下文按“系统角色 -> ReAct 协议 -> 长期会话摘要 -> 最近 12 条消息 -> 可用工具 -> 已执行步骤观察 -> 最后一条用户目标”的顺序组织。
- 当会话消息超过阈值时，后端调用模型生成长期会话摘要，并保存到 `chat_conversations.summary`。
- 摘要只压缩上次摘要之后、且不属于最近尾部窗口的旧消息；摘要失败不阻断本轮正常回复。
- 产品 Markdown 不再默认注入每轮 prompt；Agent 需要产品事实时调用 `retrieve_product_sections` 或 `read_product_markdown`。
- `retrieve_product_sections` 对长文档优先使用 embedding + Top-K 语义检索，只返回相关 chunk。
- 产品 Markdown 首次参与语义检索时，后端按 Markdown 标题和段落切块，调用 DashScope `text-embedding-v4` 建索引，并把向量保存到 `product_chunks`。
- 本轮用户问题会单独生成 query embedding，后端在当前产品 chunks 内计算 cosine similarity，默认取 Top-K 相关片段。
- embedding 调用失败、索引为空或检索无结果时，系统回退到本地关键词章节筛选。
- 通用对话响应包含 `citations`，用于前端展示“本轮引用”的产品章节。引用字段包括产品 ID、产品名、chunk ID、标题、摘要、相似度和来源。
- 通用对话响应包含 `agent_steps`，用于前端展示本轮 ReAct 步骤。步骤字段包括序号、类型、可见决策摘要、工具名、工具输入、工具观察和错误信息。
- 第一版 ReAct 工具为只读能力：`list_products`、`retrieve_product_sections`、`read_product_markdown`、`call_skill`。
- `call_skill` 调用的是内置工作流/提示词模板，不执行外部命令或系统工具。
- 内置 skill 包括：`fission_strategy`、`product_markdown_writer`、`script_review`、`creatibi_storyboard_mapping`、`dataeye_hot_material_analysis`、`seedance_video_prompt_writer`。
- 前端发送后先用临时消息本地展示用户输入；接口返回真实消息后再同步会话记录。
- 等待接口响应期间显示模型思考状态；助手回复返回后按打字机效果展示，展示完成后回落到真实会话消息。

### 10.8 脚本转 Seedance 视频提示词

```http
POST /api/jobs/{id}/video-prompts
```

请求 JSON：

- `source`: 可选，`all`、`replica` 或 `fission`，默认 `all`。

响应 JSON：

- `job_id`: 任务 ID。
- `source`: 转换来源。
- `content`: Seedance 视频生成提示词 Markdown。

实现约束：

- 第一版使用后端确定性转换器，不额外调用模型。
- 转换器读取 `replica_script_json` 和 `fission_scripts_json`，兼容 `replica_script.storyboards`、根级 `storyboards`、`fission_scripts[]` 等结构。
- 输出按脚本和分镜分组，每个分镜包含正向提示词、声音与字幕、负向提示词。
- 正向提示词从时间段、画面、动作、景别、镜头、道具场景、镜头目的等字段拼接。
- 声音与字幕从旁白、字幕、音效、BGM 等字段拼接；字段缺失时明确说明无明确声音信息。
- 负向提示词使用统一安全约束，避免乱码、水印、产品不一致、版权角色和夸大功效。
- 前端任务结果页新增“视频提示词”Tab，打开时调用该接口并以 Markdown 样式展示。

### 10.9 DataEye 爆款素材分析产品化

当前阶段先配置两层能力：

- Codex 本机 skill：`dataeye-video-download`，用于在开发/运营环境中基于 DataEye 登录态抓取素材元数据和视频文件。
- ScriptAgent 内置 ReAct skill：`dataeye_hot_material_analysis`，用于把已有 DataEye 数据和素材转成爆款特征、创意母题和下一批脚本 brief。

当前 ReAct 对话仍保持只读工具边界，不直接执行 DataEye 下载脚本。后续产品化建议增加独立任务：

```text
选择产品
  ↓
配置素材来源（DataEye URL / 产品 ID / 产品名）
  ↓
设置筛选（近 30 天 / 媒体 / 国家 / 排序指标 / 样本数）
  ↓
后端白名单拉取任务
  ↓
保存素材批次与视频文件
  ↓
爆款分析 Agent
  ↓
爆款特征报告 + 创意方向
  ↓
一键进入裂变脚本任务
```

建议新增数据模型：

- `material_sources`：产品 ID、来源类型、DataEye URL/产品 ID/产品名、媒体、国家、排序指标、创建时间。
- `material_batches`：产品 ID、时间范围、样本数、拉取状态、输出目录、原始 JSON 路径、创建时间。
- `material_items`：批次 ID、素材 ID、标题、指标 JSON、首见时间、媒体、国家、视频路径、封面路径。
- `creative_insight_reports`：产品 ID、批次 ID、样本说明、爆款特征 Markdown、创意方向 JSON、创建时间。

安全约束：

- DataEye 抓取必须通过后端 allowlist job 执行，禁止前端传任意命令。
- 不在数据库保存浏览器 Cookie 或明文登录态；优先复用用户本机浏览器授权或部署环境显式配置。
- 报告中必须展示数据口径和缺失字段，禁止模型补造指标。
- 没有指标数据时，只能标记为“素材内容分析”，不能标记为“爆款表现分析”。

### 10.10 产品创意策略报告

产品库支持从产品详情直接生成创意策略报告，并把报告转入裂变脚本任务。

```http
GET /api/products/{id}/creative-reports
POST /api/products/{id}/creative-reports
```

`POST /api/products/{id}/creative-reports` 使用 JSON：

- `source_type`: 来源类型，第一版固定为 `dataeye`。
- `dataeye_url`: DataEye 产品或素材页面 URL，可为空。
- `dataeye_id`: DataEye 产品 ID，可为空。
- `product_name`: DataEye 中的产品名，默认产品库名称。
- `date_range`: 时间范围，默认近 30 天。
- `media`: 媒体过滤，例如 TikTok、Meta。
- `country`: 国家/地区过滤。
- `sort_metric`: 排序指标，例如热度、曝光、播放。
- `sample_count`: 样本数，默认 50。
- `requirement`: 用户补充分析要求。
- `material_note`: 用户手动补充的素材观察或 DataEye 导出摘要。

实现约束：

- 第一版生成报告时读取产品 Markdown 与 DataEye 来源配置，调用当前模型生成创意策略报告。
- 在真实 DataEye 白名单拉取任务接入前，报告必须标注为策略预案，不得编造素材指标。
- 模型调用 scope 为 `creative_report`，ref_id 为产品 ID，方便调试台追踪 token 与输入输出。
- 报告保存到 `creative_reports` 表，产品详情读取历史报告并默认选中最新报告。
- 前端“转裂变任务”只做工作流跳转：进入脚本任务页，自动选中产品，并把 `report_summary` 填入补充要求；创建任务仍需要用户上传参考视频。

## 11. 数据库设计

第一版使用 SQLite，多表保存任务、产品、模型配置、对话和模型调用记录。

```sql
CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  video_path TEXT NOT NULL,
  product_md_path TEXT NOT NULL,
  requirement TEXT,
  industry TEXT NOT NULL,
  fission_count INTEGER NOT NULL,
  fission_directions TEXT,
  analysis_markdown TEXT,
  replica_script_json TEXT,
  fission_scripts_json TEXT,
  creatibi_result_json TEXT,
  error_message TEXT,
  run_log TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

```sql
CREATE TABLE products (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  md_path TEXT NOT NULL,
  md_name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE creative_reports (
  id TEXT PRIMARY KEY,
  product_id TEXT NOT NULL,
  product_title TEXT NOT NULL,
  source_config_json TEXT NOT NULL,
  report_markdown TEXT NOT NULL,
  report_summary TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE product_chunks (
  id TEXT PRIMARY KEY,
  product_id TEXT NOT NULL,
  chunk_index INTEGER NOT NULL,
  heading TEXT,
  content TEXT NOT NULL,
  embedding_json TEXT NOT NULL,
  embedding_model TEXT NOT NULL,
  embedding_dim INTEGER NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE chat_conversations (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  summary TEXT,
  summary_message_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE chat_messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE model_settings (
  id TEXT PRIMARY KEY,
  api_key TEXT,
  endpoint TEXT NOT NULL,
  model TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

任务状态：

```text
pending
running
analyzing_video
extracting_structure
generating_replica
generating_fission
validating
completed
publishing
published
failed
```

## 12. 配置项

```env
APP_ENV=development
APP_PORT=8080
DATA_DIR=./data
UPLOAD_DIR=./uploads

DASHSCOPE_API_KEY=
DASHSCOPE_ENDPOINT=https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation
SCRIPT_AGENT_MODEL=qwen3.6-plus
SCRIPT_AGENT_VIDEO_FPS=2
SCRIPT_AGENT_MODE=auto
SCRIPT_AGENT_MAX_DATA_URI_MB=20
SCRIPT_AGENT_EMBEDDING_MODEL=text-embedding-v4
SCRIPT_AGENT_EMBEDDING_DIMENSIONS=1024
DASHSCOPE_EMBEDDING_ENDPOINT=https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding

CREATIBI_PUBLISH_MODE=cli
CREATIBI_CLI_BIN=cbi
CREATIBI_PROJECT_ID=
CREATIBI_PUBLISH_TIMEOUT_SECONDS=120
```

## 13. 模型输入输出约束

### 13.1 视频输入

模型侧不应直接使用本地文件路径。后端需要提供模型可访问的视频 URL，或使用平台支持的文件上传方式。

当前实现路线：

- 本地视频转换为 Base64 data URL，传入 DashScope 多模态接口。
- 默认 `SCRIPT_AGENT_MAX_DATA_URI_MB=20`，按 DashScope data-uri 上限控制编码后体积。
- 如果本地视频编码后会超过 data-uri 上限，后端使用 `ffmpeg` 生成临时压缩 MP4，再转换为 Base64 data URL。
- 临时压缩文件只用于本次模型请求，读取后立即删除。

后续允许两种升级路线：

- 临时公开 URL：上传到 OSS/S3/临时文件服务后传入模型。
- 平台文件上传：如果 DashScope 支持当前环境直接上传文件，则使用平台 file API。

### 13.2 输出格式

视频分析结果使用 Markdown。

脚本生成结果使用 JSON，建议结构：

```json
{
  "title": "脚本标题",
  "script_type": "replica",
  "industry": "ecommerce",
  "duration_seconds": 25,
  "source_summary": "复刻来源说明",
  "storyboards": [
    {
      "scene_index": 1,
      "time_range": "00:00-00:03",
      "visual": "画面描述",
      "action": "动作描述",
      "voiceover": "旁白",
      "subtitle": "字幕",
      "shot_size": "近景",
      "camera_intent": "镜头动机",
      "props_scene": "道具场景",
      "audio": "音效/BGM",
      "purpose": "hook"
    }
  ],
  "metadata": {
    "parent_script_id": "",
    "fission_dimension": "",
    "kept_elements": [],
    "changed_elements": []
  }
}
```

## 14. CreatiBI 集成约束

- CreatiBI 写入逻辑必须封装在 `internal/creatibi` 中。
- 上层业务不直接拼 CLI 命令。
- 发布使用 `cbi project script-create` 创建复刻脚本和裂变子脚本；复刻脚本只传 `project-id/name`，裂变脚本通过 `parent-id` 挂载到复刻脚本下。
- 发布使用 `cbi project script-save --project-id ... --name ... --format 2 --script ...` 保存完整分镜 doc JSON，确保保存内容时不清空脚本名称。
- CreatiBI 分镜 doc 使用 `heading + CbiFrame + paragraph` 结构；一个 `CbiFrame` 包含多个 `CbiFrameItem`。
- 字段映射：`voiceover/subtitle -> Copy`，`visual/action/props_scene/shot_size -> Description`，`time_range/camera_intent/purpose/audio -> Note`，`action/props_scene/shot_size/audio -> property.Movement/Prop/ShotSize/text.SoundEffec`。
- 裂变提示词使用三层维度体系：视听层、结构层、元素层；生成结果的 `metadata.fission_dimension` 固定为“层级-维度”。
- `CREATIBI_PROJECT_ID` 可指定目标专案；未指定时使用 `cbi project list` 返回的第一个专案。
- CLI/API 返回结果必须保存到 `creatibi_result_json`。
- 写入失败不能删除本地生成结果。
- 写入失败后允许用户重试发布。
- 如果 CreatiBI 脚本字段标准与本文档 JSON 不一致，以 CreatiBI 实际接口为准，并更新 Schema。

## 15. 安全与文件约束

- 只允许上传视频文件和 Markdown 文件。
- 视频文件大小需要限制，默认建议 500MB 以内。
- Markdown 文件大小默认建议 2MB 以内。
- 上传文件保存时必须生成内部文件名，不能直接信任用户原始文件名。
- 历史记录中可显示原始文件名，但磁盘存储使用内部安全文件名。
- 任务失败时保留输入文件和错误信息，方便复查。
- API Key 不进入前端，不写入历史记录。

## 16. 开发里程碑

### M1 项目骨架

- Go API Server。
- React + Vite 前端。
- CreatiBI Design System token/assets 接入。
- SQLite 初始化。

### M2 任务与历史记录

- 创建任务。
- 文件上传。
- 历史列表。
- 任务详情页。
- 状态流转。

### M3 模型调用

- DashScope client。
- qwen3.6-plus 视频理解。
- 产品 Markdown 读取。
- 保存视频分析 Markdown。

### M4 脚本生成

- 复刻结构提取。
- 复刻脚本生成。
- 裂变脚本生成。
- JSON 格式校验。

### M5 CreatiBI 写入

- CreatiBI Publisher。
- 手动发布按钮。
- 发布结果保存。
- 失败重试。

### M6 体验完善

- 通用对话入口。
- 开发者调试台：默认隐藏，开启后查看模型调用输入、输出、token、耗时和错误。
- 历史记录搜索。
- 结果 Tabs。
- 加载状态。
- 错误提示。
- 基础端到端测试。

## 17. 待确认问题

开发前仍需确认：

- CreatiBI 创建脚本的最终 CLI/API 命令和字段格式。
- 视频临时 URL 的实现方式，OSS/S3、本地内网穿透，还是 DashScope 文件上传。
- `qwen3.6-plus` 在当前账号和地域下是否支持直接视频输入。
- 单次任务最大视频时长和文件大小。
- 裂变脚本默认数量。
- 是否需要支持中英文脚本。
- 是否需要把视频分析 Markdown 也写入 CreatiBI 备注或附件。

## 18. 开发前确认结论

本项目第一版按以下方案开发：

```text
Go 后端 + React 前端 + CreatiBI Design System
+ SQLite 历史记录
+ qwen3.6-plus 单模型
+ 单 Agent 工作流
+ 用户确认后写入 CreatiBI
```

该方案优先保证流程稳定、结果可查、生成内容可复核，为后续升级 Multi-Agent、批量处理和 CreatiBI 深度集成保留扩展点。

## 19. 文档与 Git 维护约束

- `docs/requirements.md` 是需求基线文档，功能范围、用户流程、字段、验收标准变化时必须同步更新。
- `docs/technical-design.md` 是技术设计基线文档，架构、技术栈、接口、数据结构、模型接入、CreatiBI 集成变化时必须同步更新。
- 每次代码变更前必须查看 Git 工作区状态。
- 每组相关代码变更完成后必须提交 Git commit。
- 不允许把无关改动混入同一次提交。
- 不允许回滚用户或其他开发者已有改动，除非用户明确要求。
