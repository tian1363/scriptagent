# 邀请测试部署指南

本文描述 ScriptAgent 第二阶段“小范围邀请测试”的最低部署基线。它不是大规模、多组织 SaaS 的完整方案。

## 安全默认值

- 首个账号用于初始化实例；后续账号必须使用一次性邀请码。
- 用户默认使用自己的模型 API Key（BYOK）。
- 平台托管额度默认关闭。
- 用户 API Key 使用 AES-256-GCM 加密后写入数据库。
- 登录会话仅以 SHA-256 摘要写入数据库，原始 Cookie 不落盘。
- 图片和视频上传会同时校验扩展名、文件特征和大小上限。
- 浏览器请求保持同源，生产 Cookie 必须启用 `Secure`。

## 初始化配置

```bash
cp .env.example .env
openssl rand -base64 32
```

把生成结果写入 `.env` 的 `SCRIPT_AGENT_ENCRYPTION_KEY`。该密钥不能提交到 Git、不能与数据库备份放在同一位置，也不能在已有数据后随意更换。

为受邀用户生成随机邀请码：

```bash
openssl rand -hex 16
```

多个邀请码通过 `SCRIPT_AGENT_INVITE_CODES` 以逗号分隔。每个邀请码在首次成功校验后失效；新增邀请码后重启服务即可载入。

## 启动

本地验证：

```bash
docker compose up --build
```

生产环境应由 Nginx、Caddy 或云负载均衡器终止 HTTPS，并设置：

```bash
SCRIPT_AGENT_SECURE_COOKIES=true
SCRIPT_AGENT_REGISTRATION_MODE=invite
SCRIPT_AGENT_ALLOW_MANAGED_MODE=false
```

Compose 默认只将服务绑定到 `127.0.0.1`，需要由反向代理对外提供 HTTPS，不能直接暴露 `8080` 端口。

## 备份与恢复

宿主机直接部署时运行：

```bash
DATA_DIR=./data UPLOAD_DIR=./uploads BACKUP_DIR=/secure/backups ./scripts/backup.sh
```

备份包括 SQLite 一致性快照和上传素材压缩包。加密主密钥必须单独备份。每次发布前备份一次，并至少每周执行一次恢复演练。

## 邀请测试运营规则

- 首轮只邀请可直接联系和验证身份的用户。
- 不开启平台托管模型额度；需要时逐用户人工评估。
- 为用户说明上传素材会发送给其所配置的模型供应商处理。
- 发现账号异常、费用异常或素材投诉时暂停新增邀请。
- SQLite 和本地素材目录仅支持单实例运行，禁止同时启动多个副本。

当前邀请模式面向可人工确认身份的小范围用户，邀请码代替公开注册的邮箱验证。准备开放公开注册前，必须再接入邮件验证、密码找回和持久化或共享限流。
