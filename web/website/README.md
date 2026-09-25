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

## 公网官网

GitHub Pages 从 `main` 自动发布到 <https://tian1363.github.io/scriptagent/>。发布流程见 [pages.yml](../../.github/workflows/pages.yml)。官网的“申请内测”按钮打开公开 GitHub Issue 表单，不收集私人联系方式。

## 公开 Skill 页面

首页的“公开 Skill”区块链接到 `/scriptagent/skills/`。构建脚本 `scripts/generate-public-skills.mjs` 从 `internal/chat/service.go` 的内置定义生成四个可直接分享的静态页面，附完整工作流、输入示例和源码入口；只发布仓库里已有的内置 Skill，不读取用户产品资料或自定义 Skill。生成结果位于 `public/skills/`，同时生成 `sitemap.xml` 和 `robots.txt`。运行 `npm run test:public-skills` 检查页面与来源同步。

页面可供 Google 等搜索引擎发现，但是否收录由搜索引擎决定。发布后可在 Google Search Console 提交站点地图：<https://tian1363.github.io/scriptagent/sitemap.xml>。
