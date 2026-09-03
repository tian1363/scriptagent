# ScriptAgent 文档中心

> 最近维护：2026-08-28 · 事实基线：当前 `main` 分支

## 阅读顺序

1. [根目录 README](../README.md)：产品现状、启动方式和安全边界。
2. [产品地址清单](product-addresses.md)：前台、开发者模式、API、Langfuse 与仓库地址。
3. [需求文档](requirements.md)：当前产品范围、用户流程和验收约束。
4. [技术设计](technical-design.md)：架构、数据、模型和接口设计。
5. [Agent Runtime 与 Harness](agent-runtime-harness.md)：Workflow、Agent Loop、可恢复性和观测。

## 专题文档

| 文档 | 内容 | 当前状态 |
| --- | --- | --- |
| [模型能力](model-capabilities.md) | 能力路由、托管/BYOK、默认模型 | 已按当前设置页更新 |
| [Token 优化](token-optimization.md) | 摘要、上下文、工具调用与缓存策略 | 已实施策略 |
| [Langfuse](langfuse-observability.md) | OTLP 接入、隐私与排障 | 可选能力 |
| [UGC 视频生成](ugc-video-generation.md) | 产品参考图、异步生成、预览与下载 | 当前可用 |
| [品牌规范](../brand-spec.md) | 色彩、层级与交互语言 | 当前 UI 基线 |
| [设计 QA](../design-qa.md) | 已修复问题与验证记录 | 回归记录 |

## 状态标签

- **当前可用**：已存在于 `main`，且有对应前后端实现。
- **需配置**：代码已存在，但需要密钥、环境变量或外部服务。
- **未合并**：其他分支存在实现，当前产品不可使用。
- **规划中**：只有设计或需求，不应出现在用户可用能力清单中。

## 维护规则

- 页面、API、模型、数据结构或环境变量变化时，同一提交更新相应文档。
- `internal/web/routes.go` 是 API 地址事实来源；`cmd/server/main.go` 是运行配置事实来源。
- `web/app/src/App.jsx` 是当前页面和文案事实来源。
- 历史方案如果仍需保留，必须明确标注为“历史设计”，不能与当前能力混写。
