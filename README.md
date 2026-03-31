# RelayHub

RelayHub 是一个本地优先的统一 AI 中转与控制台，目标是把多来源 Provider、多协议入口、会话粘性、竞速路由、用量追踪和桌面控制面整合成一个可直接运行的完整项目。

当前仓库已经包含可运行的 Go 后端、可静态导出的 Next.js + HeroUI 控制台、Tauri 桌面壳、Docker 构建与 GitHub Actions。

## 已实现能力

- 统一代理入口：
  - `POST /v1/chat/completions`
  - `POST /v1/responses`
  - `POST /v1/messages`
  - `POST /v1/embeddings`
  - `GET /v1/models`
- 多来源统一路由：
  - 逻辑模型 `smart-fast` / `smart-budget` / `smart-precise`
  - 策略 `single` / `failover` / `race`
  - 会话粘性绑定与回放
- 用量追踪：
  - 请求级逻辑用量
  - attempt 级真实消耗
  - `logical_cost` 与 `physical_cost` 分离
- 管理 API：
  - Overview / Providers / Models / Rules / Requests / Sessions / Usage / Replay / Export / Import
- 控制台：
  - Dashboard
  - Providers / Models / Rules / Requests / Sessions 视图
- 工程交付：
  - Dockerfile
  - `docker-compose.yml`
  - GitHub Actions `ci.yml`
  - GitHub Actions `desktop-tauri.yml`

## 技术栈

- 前端：Next.js App Router + HeroUI
- 桌面：Tauri 2，目标平台为 macOS / Windows / Linux
- 后端：Go
- 存储：SQLite（本地优先）+ 可选 PostgreSQL（服务化部署）
- 缓存与限流：内存 + 可选 Redis

## 默认凭据

- 代理 API Key：`relayhub-local-key`
- 管理端 Token：`relayhub-admin`
- 默认监听地址：`:8080`

## 快速运行

### 控制台

```bash
export NPM_CONFIG_CACHE=/tmp/npm-cache
npm install
npm run build:console
```

### 后端 Docker 测试

```bash
npm run test:server:docker
npm run smoke:server:docker
```

### 本地 Smoke 请求

```bash
curl -H "Authorization: Bearer relayhub-local-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-fast","messages":[{"role":"user","content":"写一个 hello world"}]}' \
  http://127.0.0.1:8080/v1/chat/completions
```

## 目录

- Go 后端入口：[main.go](/root/coding/any/relayhub/server/cmd/relayhub/main.go)
- 路由执行器：[app.go](/root/coding/any/relayhub/server/internal/runtime/app.go)
- 代理处理器：[handlers.go](/root/coding/any/relayhub/server/internal/gateway/handlers.go)
- 管理 API：[handlers.go](/root/coding/any/relayhub/server/internal/admin/handlers.go)
- 控制台主页：[page.tsx](/root/coding/any/relayhub/apps/console/app/page.tsx)
- 控制台主界面：[console-dashboard.tsx](/root/coding/any/relayhub/apps/console/components/console-dashboard.tsx)
- 规格总文档：[docs/PROJECT_SPEC.md](/root/coding/any/relayhub/docs/PROJECT_SPEC.md)
