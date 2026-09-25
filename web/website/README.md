# ScriptAgent 官网

ScriptAgent 的开源 Beta 官网，位于主仓库的 `web/website`。页面介绍面向跨境电商团队的 UGC 创意工作方式；视觉卡片是示意，产品界面使用经脱敏的截图。

## 本地运行

需要 Node.js 20.19+ 或 22.12+。

```bash
npm ci
npm run dev
```

运行 `npm run build` 生成站点产物，运行 `npm run test:sites` 检查静态资源与路由回退。静态站点配置位于 `.openai/hosting.json`。

本目录只包含官网，ScriptAgent 产品代码和运行说明见[主仓库 README](../../README.md)。
