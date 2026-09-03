# ScriptAgent 产品地址清单

> 最近核对：2026-08-28 · 默认本地端口：`8080`

## 用户界面

当前前端是单页应用，没有独立的浏览器路径。以下模块都从同一个地址进入，再通过左侧菜单切换。

| 入口 | 地址 | 状态 | 说明 |
| --- | --- | --- | --- |
| 产品前台 | <http://127.0.0.1:8080/> | 当前可用 | 开始、历史、创作空间、产品资料、对话 |
| 设置 | <http://127.0.0.1:8080/> | 当前可用 | 左侧“设置”；模型能力、BYOK、开发者选项 |
| 开发者模式 | <http://127.0.0.1:8080/> | 当前可用 | 在设置中开启“显示开发者模式”后，左侧出现入口 |
| 运营后台 | <http://127.0.0.1:8080/> | 当前不可用 | 当前 `main` 没有注册 `/api/owner/*` 路由，界面代码不代表后端已上线 |
| 注册 / 登录 | <http://127.0.0.1:8080/> | 当前可用 | 未登录时自动显示；支持登录现有账号或注册新账号 |

## 开发与服务

| 入口 | 地址 | 状态 | 说明 |
| --- | --- | --- | --- |
| Go 应用与 API | <http://127.0.0.1:8080/> | 当前可用 | 同时托管生产前端静态文件和 API |
| 健康检查 | <http://127.0.0.1:8080/api/health> | 当前可用 | 服务存活检查 |
| 前端开发服务器 | <http://127.0.0.1:5173/> | 开发时可用 | 执行 `npm --prefix web/app run dev` 后开放 |
| API 根路径 | <http://127.0.0.1:8080/api> | 当前可用 | 无 API 首页；使用下表具体接口 |

## API 分组

| 分组 | 方法与路径 |
| --- | --- |
| 健康 | `GET /api/health` |
| 认证 | `GET /api/auth/status` · `POST /api/auth/register` · `POST /api/auth/login` · `POST /api/auth/logout` · `GET /api/auth/me` |
| Skill | `GET /api/skills` · `POST /api/skills/draft` · `POST /api/skills` · `PUT /api/skills/{id}` |
| 产品资料 | `GET/POST /api/products` · `PUT /api/products/{id}` · `GET /api/products/{id}/markdown` |
| 产品素材 | `GET/POST /api/products/{id}/assets` · `GET /api/assets/{id}/file` |
| 创意报告 | `GET/POST /api/products/{id}/creative-reports` |
| 创作空间 | `GET/POST /api/spaces` · `PUT/DELETE /api/spaces/{id}` · `GET /api/spaces/{id}/observability` |
| 执行任务 | `GET/POST /api/jobs` · `GET /api/jobs/{id}` · `POST /api/jobs/{id}/retry` · `POST /api/jobs/{id}/publish` · `POST /api/jobs/{id}/video-prompts` |
| 对话 | `GET/POST /api/chats` · `POST /api/chats/messages` · `GET /api/chats/{id}` · `POST /api/chats/{id}/messages` |
| 模型与观测 | `GET /api/model-calls` · `GET/PUT /api/settings/model` |

除健康检查和认证接口外，业务 API 均要求有效的 HttpOnly Session。产品资料、空间、对话、任务、模型配置和观测数据按账号隔离。

## 外部平台

| 平台 | 地址 | 状态 | 说明 |
| --- | --- | --- | --- |
| GitHub 仓库 | <https://github.com/tian1363/scriptagent> | 当前可用 | 代码与版本历史 |
| Langfuse Cloud | <https://cloud.langfuse.com/> | 需配置 | 登录后选择对应项目查看 Trace；项目详情 URL 由 Langfuse 账号和项目决定 |
| DashScope 控制台 | <https://bailian.console.aliyun.com/> | 需配置 | 创建模型服务 API Key、查看额度与调用情况 |
| CreatiBI | 无固定 Web 地址 | 需配置 | 当前发布通过本机 `cbi` CLI 与 `CREATIBI_PROJECT_ID` 完成 |

## 地址变更方式

- 修改应用端口：设置 `APP_PORT`。
- 修改前端开发端口：调整 `web/app/package.json` 中的 `dev` 脚本。
- 修改 Langfuse：设置 `LANGFUSE_BASE_URL`。
- 修改 DashScope 接口：在设置页按能力配置，或设置 `DASHSCOPE_ENDPOINT`。

部署到服务器时，将表中的 `127.0.0.1:8080` 替换为实际域名，并在公网开放前启用 HTTPS。面向多组织开放前还需补齐租户隔离、细粒度权限与密钥加密。
