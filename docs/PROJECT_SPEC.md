# RelayHub 完整项目规格

## 1. 项目定位

RelayHub 是一个“本地优先、可桌面化、可服务化”的统一 AI 中转与控制台。

它不是单纯的 API 反向代理，也不是只服务某一个模型提供商的管理面，而是一个完整产品：

- 统一入口：兼容多种上游协议和多种客户端调用方式。
- 统一路由：基于规则、上下文、预算、健康度、会话、竞速策略进行动态选择。
- 统一治理：认证、限流、配额、审计、会话粘性、请求回放、故障切换。
- 统一控制台：Providers、Models、Rules、Sessions、Usage、MCP、Prompts、Settings。
- 统一交付：同一套前端 UI，既可作为 Web 控制台，也可被 Tauri 打包成桌面应用。

## 2. 设计目标

### 2.1 必须达成

- 支持本地中转，默认监听本机端口，供 Claude Code、Codex CLI、OpenAI SDK、Anthropic SDK 等调用。
- 支持多来源统一路由。
- 支持竞速、兜底、影子流量、粘性会话。
- 支持像 `octopus` 一样的用量追踪，但粒度更细。
- 支持桌面端完整控制面，包含配置、监控、调试、导入导出。
- 支持三端桌面打包：macOS、Windows、Linux。
- 支持纯服务部署，不依赖 Tauri 也能运行。

### 2.2 设计原则

- 本地优先：默认单机可用，零外部依赖即可完成核心中转。
- 统一抽象：把不同 Provider 的鉴权、模型名、协议差异收敛到统一适配层。
- 显式可观测：所有请求都可追踪到“为什么被路由到这里、消耗了多少、失败在哪里”。
- 不做空壳 UI：控制台的每个页面都必须对应真实后端能力。
- 配置可迁移：所有业务配置均可导入导出，桌面版和服务版共用格式。
- 兼容优先：对外优先兼容 OpenAI 风格与 Anthropic 风格入口。

## 3. 从五个参考项目吸收的能力

### 3.1 `octopus`

- 吸收：
  - 多上游 key / channel / model 的统一转发。
  - 明确的 relay 主链路。
  - 用量与成本统计。
  - 基础的负载均衡与故障切换。
- 不直接照搬：
  - 偏单体和偏后端中心的 UI 组织方式。
  - 相对朴素的状态缓存与任务组织方式。

### 3.2 `claude-code-router`

- 吸收：
  - 路由规则系统。
  - preset / transformer 思路。
  - 模型别名和 provider,model 映射。
  - 面向不同场景的策略化路由。
- 不直接照搬：
  - 过于依赖配置驱动的动态行为，而不提供足够强的运行期解释能力。

### 3.3 `claude-code-hub`

- 吸收：
  - 显式请求管线。
  - 会话管理与重绑定。
  - 限流、配额、审计、测试体系。
  - 管理控制台的治理视角。
- 不直接照搬：
  - 较重的企业管理面复杂度。
  - 不必要的组织级功能膨胀。

### 3.4 `aio-coding-hub`

- 吸收：
  - 本地桌面控制面。
  - GatewayManager / SessionManager 思路。
  - MCP、Skills、Providers 的可操作 UI。
  - 本地端口、连接模式、运行状态管理。
- 不直接照搬：
  - 桌面端专有的过重本地编排逻辑。

### 3.5 `cc-switch`

- 吸收：
  - 多工具链配置同步。
  - MCP 与 Prompt 资产管理。
  - 本地 Session 扫描与聚合。
  - 面向个人开发者的易用控制面。
- 不直接照搬：
  - 前后端超大文件中心化实现。

## 4. 产品范围

这份规格定义的是完整目标态，不按阶段、不按 P 级拆分。以下范围均属于 RelayHub 的正式能力。

### 4.1 对外代理入口

- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`
- `POST /v1/embeddings`
- `GET /v1/models`
- `POST /v1/audio/transcriptions`
- `POST /v1/audio/speech`

### 4.2 管理能力

- Provider 管理
- 模型目录与模型别名
- 路由规则
- 竞速策略
- API Keys / Projects / Quotas
- Sessions
- Requests / Replay / Audit
- Usage / Cost / Rate Limit Dashboard
- MCP Servers
- Prompt Assets
- App Settings
- 配置导入导出

### 4.3 桌面能力

- 系统托盘
- 开机自启
- 本地端口占用检测与自动切换
- 本地后端进程守护
- 桌面通知
- 文件系统导入导出
- 系统代理辅助配置
- 安全存储集成

## 5. 总体架构

## 5.1 组件视图

### Go 后端

- `gateway`
  - 对外暴露兼容 API。
  - 处理认证、标准化、路由、转发、竞速、流式聚合。
- `admin-api`
  - 提供 UI 所需管理接口。
- `router-engine`
  - 编译路由规则。
  - 计算候选 Provider、模型与策略。
- `provider-adapters`
  - 抽象不同上游协议与鉴权方式。
- `session-engine`
  - 粘性会话、会话绑定、失败重绑定。
- `usage-engine`
  - 记录逻辑用量、物理消耗、账单口径、成本口径。
- `policy-engine`
  - 限流、配额、请求过滤、敏感规则。
- `audit-engine`
  - 请求日志、事件流、回放与追责。
- `storage`
  - SQLite / PostgreSQL / Redis 抽象层。
- `runtime`
  - 健康检查、任务调度、统计聚合、事件总线。

### Next.js 前端

- `console-app`
  - 管理控制台。
- `shared-ui`
  - HeroUI 主题、图表、通用表单、日志查看器、代码编辑器封装。
- `typed-client`
  - 基于 OpenAPI 生成的前端 API Client。

### Tauri 桌面壳

- `desktop-shell`
  - 管理本地 Go 后端生命周期。
  - 暴露桌面专属能力。
  - 挂载系统托盘、更新、通知、开机自启、文件对话框。

## 5.2 运行形态

### 桌面模式

- Tauri 启动后拉起 Go Core。
- Go Core 监听本地回环地址，例如 `127.0.0.1:4317`。
- Next.js 控制台以静态构建产物方式嵌入桌面壳。
- 所有配置默认存于本地 SQLite。

### 服务模式

- 单独运行 Go Core。
- 前端构建为静态资源，由 Go Core 托管。
- 存储可切换为 PostgreSQL + Redis。

### 混合模式

- 用户在桌面端配置远端服务节点。
- 桌面端继续作为本地控制台，同时可控制远端中转实例。

## 6. 核心能力设计

## 6.1 Provider 抽象

Provider 类型统一抽象为：

- OpenAI 兼容
- Anthropic 原生
- Gemini 原生
- Azure OpenAI
- OpenRouter
- Ollama / Local OpenAI 兼容服务
- CLI Bridge Provider

每个 Provider 统一字段：

- `id`
- `type`
- `name`
- `base_url`
- `auth_type`
- `credential_ref`
- `enabled`
- `health_state`
- `priority`
- `tags`
- `capabilities`
- `cost_profile`
- `limits`

`capabilities` 必须至少描述：

- 是否支持流式响应
- 是否支持工具调用
- 是否支持长上下文
- 是否支持推理 token
- 是否支持 embeddings / audio / image
- 是否支持会话续接

## 6.2 模型目录

系统维护统一模型目录，不直接暴露上游模型名作为唯一入口。

模型目录包含：

- 逻辑模型 ID，例如 `smart-fast`、`smart-long`、`embedding-default`
- 展示名称
- 任务类型
- 默认 transformer
- 候选 Provider 映射
- 上下文长度
- 成本配置
- 标签

这样客户端只依赖逻辑模型名，路由层再决定最终落到哪个 Provider 模型。

## 6.3 请求标准化

所有外部请求先转换为内部统一结构 `NormalizedRequest`：

- `request_id`
- `entry_protocol`
- `project_id`
- `api_key_id`
- `session_key`
- `task_type`
- `logical_model`
- `messages`
- `tools`
- `attachments`
- `max_tokens`
- `temperature`
- `metadata`
- `user_context`

然后经过 transformer 链：

- 输入 transformer
- 协议修正 transformer
- provider 兼容 transformer
- 安全裁剪 transformer

## 6.4 路由规则系统

路由规则支持以下维度：

- 入口协议
- 项目 / API Key
- 逻辑模型
- 请求类型
- 是否流式
- 是否含工具调用
- 上下文 token 预估
- 用户标签
- provider 标签
- 预算阈值
- 时间窗口
- 桌面/服务运行模式

动作类型：

- 指定 Provider
- 指定 Provider 组
- 指定模型映射
- 设置竞速策略
- 设置超时
- 设置重试
- 设置 fallback 链
- 注入 transformer / preset
- 禁止某类上游

规则优先级采用：

- 显式优先级
- 条件匹配分值
- 更具体规则优先

## 6.5 竞速与故障切换

RelayHub 支持四种策略：

- `single`
  - 单一路由，不竞速。
- `failover`
  - 串行兜底，前者失败后切下一候选。
- `hedged`
  - 主候选先发，达到延迟阈值后补发第二候选。
- `race`
  - 并发多个候选，按策略选赢家。

赢家判定支持：

- `first_header`
- `first_token`
- `first_complete`
- `scored`

`scored` 评分由以下因素构成：

- 响应成功与否
- 首字节延迟
- 首 token 延迟
- 完整响应延迟
- 历史健康分
- 成本倍率
- 规则显式偏好

竞速时必须记录：

- 每个 attempt 的开始时间
- 取消时间
- 上游返回状态
- 消耗 token
- 实际成本
- 是否赢家

对外默认展示两套口径：

- `logical_usage`
  - 从客户端视角，一次请求应计的逻辑用量。
- `physical_cost`
  - 从系统运营视角，所有尝试真实产生的消耗。

这解决了竞速下“看似只回了一次，但后台可能打了两个上游”的统计歧义。

## 6.6 会话与粘性绑定

会话系统参考 `claude-code-hub` 与 `aio-coding-hub` 的思路，支持：

- `X-Relay-Session-ID` 显式会话头
- 自动派生 session key
- 项目维度粘性绑定
- provider 组内粘性
- TTL 过期
- 失败重绑定
- 手动终止与回收

会话绑定策略：

- 同一 session 默认优先复用上一次胜出的 Provider/Model。
- 若该 Provider 健康下降或达到配额上限，则在同一 provider 组中重选。
- 若规则允许，可跨组重绑定，并记录原因。

## 6.7 用量追踪

用量追踪是产品核心能力，必须达到以下粒度：

- 按请求
- 按 attempt
- 按 API Key
- 按 Project
- 按 Provider
- 按 Model
- 按 Session
- 按时间窗口

需记录指标：

- 输入 token
- 输出 token
- cache read / write token
- reasoning token
- audio seconds / characters
- image count
- 成本
- 延迟
- 错误率
- 取消率
- 竞速额外消耗

所有指标既要有明细表，也要有 rollup 聚合表。

## 6.8 限流与配额

支持四层治理：

- 全局实例级限流
- 项目级限流
- API Key 级限流
- Provider 级限流

限流维度：

- QPS / RPM
- 并发会话数
- token 每分钟
- token 每日
- 成本每日

配额超出后策略：

- 直接拒绝
- 降级到低成本路由
- 转入等待队列

## 6.9 请求回放与审计

必须支持：

- 原始请求体保存
- 标准化请求保存
- 路由决策解释保存
- 每个 attempt 的上游请求与响应元数据
- 脱敏展示
- 流式响应摘要保存
- 一键回放

回放模式：

- 仅重跑路由，不请求上游
- 重新请求上游
- 替换指定 Provider 回放

## 6.10 Preset 与 Transformer

参考 `claude-code-router`，但实现上更强调可解释性。

Preset 用于：

- 默认参数模板
- 协议转换模板
- 特定 Provider 兼容模板
- 特定客户端兼容模板

Transformer 用于：

- 输入消息重写
- 模型名映射
- 工具描述格式修正
- 响应字段映射
- 成本补全

每次请求都要能在日志里看到：

- 命中了哪个 preset
- 执行了哪些 transformer
- 修改了哪些字段

## 6.11 MCP 与 Prompt 资产

这是桌面版的重要增值能力，也在服务模式保留配置能力。

### MCP

- MCP Server 注册
- 本地命令型 MCP
- HTTP 型 MCP
- 环境变量模板
- 启停控制
- 健康检测
- 导入导出
- 与 Claude/Codex/Gemini/OpenCode 的配置同步模板

### Prompt Assets

- Prompt 模板库
- 变量占位符
- 标签与分组
- 版本历史
- 导入导出
- 运行时快速复制与注入

## 7. 前端信息架构

控制台页面不做空泛展示，必须全部可操作。

### 7.1 Dashboard

- 实时请求吞吐
- 延迟分布
- 错误率
- 竞速胜率
- 物理成本 vs 逻辑用量
- 热门模型
- 异常告警

### 7.2 Providers

- Provider 列表
- 健康状态
- 凭据配置
- 模型能力查看
- 成本参数配置
- 测试连通
- 优先级与标签管理

### 7.3 Models

- 逻辑模型列表
- 候选 Provider 映射
- 默认 transformer
- 成本预估参数
- 能力标签

### 7.4 Router Rules

- 规则列表
- 条件编辑器
- 动作编辑器
- 命中预览
- 冲突检测
- 路由仿真器

### 7.5 Race Policies

- 竞速策略列表
- 赢家判定方式
- hedged 延迟阈值
- 最大并发候选数
- loser cancel 策略

### 7.6 Sessions

- 当前活跃会话
- 绑定 Provider / Model
- TTL
- 最近请求
- 强制解绑
- 强制终止

### 7.7 Requests

- 请求列表
- 过滤器
- 单条详情
- 路由解释
- attempt 时间线
- 回放
- 导出

### 7.8 Usage

- 聚合图表
- 项目账单
- Provider 成本
- API Key 用量
- 竞速额外成本
- 时间窗口对比

### 7.9 MCP

- Server 列表
- 连接状态
- 启停
- 日志
- 各工具链导出模板

### 7.10 Prompts

- 模板库
- 标签过滤
- 版本对比
- 一键复制
- 导入导出

### 7.11 Settings

- 监听地址
- 本地端口
- 更新频道
- 安全设置
- 存储后端
- 导入导出
- 日志级别

## 8. 桌面端设计

Tauri 只负责桌面能力，不承担业务核心。

### 8.1 Tauri 责任

- 启动和监控 Go Core
- 提供托盘与窗口管理
- 处理系统级文件读写
- 处理开机自启
- 处理本地通知
- 处理安全存储桥接
- 处理自动更新

### 8.2 Go Core 与 Tauri 的关系

- Tauri 不复写业务逻辑。
- 所有路由、会话、统计、管理 API 都在 Go Core。
- Tauri 通过本地 HTTP / WebSocket 与 Go Core 通讯。
- 桌面专属动作通过 Tauri command 暴露给前端。

### 8.3 配置与密钥

- 普通业务配置：SQLite / JSON 导入导出。
- 敏感密钥：使用桌面系统安全存储保存主密钥。
- Go Core 存储密文字段，启动时由 Tauri 注入解密上下文。
- 服务模式下通过环境变量或外部 KMS 注入主密钥。

## 9. 后端详细设计

## 9.1 请求处理主链路

主链路严格固定为：

1. 接收请求
2. 分配 `request_id`
3. 鉴权与 API Key 查找
4. 协议识别
5. 请求标准化
6. 会话提取/生成
7. 配额与限流校验
8. 预估 token 与预算判断
9. 路由规则匹配
10. 生成候选列表
11. 执行单路由 / failover / hedged / race
12. 聚合流式或非流式响应
13. 统计用量
14. 更新会话绑定
15. 写入审计日志
16. 推送实时事件
17. 返回客户端

## 9.2 路由内核

路由内核由三层组成：

- `selector`
  - 根据规则筛选候选集合。
- `planner`
  - 生成执行策略：单发、竞速、串行兜底。
- `executor`
  - 真正发送请求并聚合结果。

每次路由都必须返回 `RouteDecision`：

- `matched_rules`
- `logical_model`
- `candidate_attempts`
- `policy`
- `reason`
- `estimated_cost`

## 9.3 Provider Adapter

每个 Provider Adapter 提供统一接口：

- `Validate`
- `ListModels`
- `NormalizeOutbound`
- `Send`
- `SendStream`
- `ParseUsage`
- `Cancel`
- `HealthCheck`

这样竞速与回放只依赖统一接口，不依赖具体 Provider 实现。

## 9.4 事件系统

系统维护内部事件总线，事件包括：

- `request.received`
- `request.routed`
- `attempt.started`
- `attempt.succeeded`
- `attempt.failed`
- `attempt.cancelled`
- `usage.recorded`
- `session.bound`
- `provider.health.changed`
- `quota.exceeded`

事件既用于前端实时展示，也用于聚合任务。

## 10. 数据模型

## 10.1 核心表

- `providers`
- `provider_credentials`
- `provider_models`
- `logical_models`
- `logical_model_targets`
- `route_rules`
- `race_policies`
- `projects`
- `api_keys`
- `quotas`
- `sessions`
- `session_bindings`
- `requests`
- `request_attempts`
- `request_usage`
- `usage_rollups_hourly`
- `usage_rollups_daily`
- `audit_events`
- `presets`
- `transformers`
- `mcp_servers`
- `prompt_assets`
- `app_settings`

## 10.2 关键字段

### `requests`

- `id`
- `request_id`
- `project_id`
- `api_key_id`
- `entry_protocol`
- `logical_model`
- `final_provider_id`
- `final_model_name`
- `session_id`
- `status`
- `route_policy`
- `started_at`
- `completed_at`
- `latency_ms`
- `logical_input_tokens`
- `logical_output_tokens`
- `logical_cost`
- `physical_cost`
- `error_code`

### `request_attempts`

- `id`
- `request_id`
- `provider_id`
- `provider_model`
- `attempt_index`
- `is_winner`
- `launch_mode`
- `status`
- `started_at`
- `first_byte_at`
- `first_token_at`
- `completed_at`
- `cancelled_at`
- `input_tokens`
- `output_tokens`
- `cost`
- `error_detail`

### `sessions`

- `id`
- `session_key`
- `project_id`
- `status`
- `ttl_at`
- `last_request_at`

### `session_bindings`

- `id`
- `session_id`
- `provider_id`
- `provider_model`
- `binding_reason`
- `bound_at`
- `released_at`

## 11. API 设计

## 11.1 公共代理 API

- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`
- `POST /v1/embeddings`
- `GET /v1/models`
- `POST /v1/audio/transcriptions`
- `POST /v1/audio/speech`

公共代理认证：

- `Authorization: Bearer <relay-api-key>`
- 或 `x-api-key`

可选请求头：

- `X-Relay-Project`
- `X-Relay-Session-ID`
- `X-Relay-Route-Policy`
- `X-Relay-Debug`

可选响应头：

- `X-Relay-Request-ID`
- `X-Relay-Session-ID`
- `X-Relay-Provider`
- `X-Relay-Model`
- `X-Relay-Policy`

## 11.2 管理 API

- `GET /api/admin/overview`
- `GET /api/admin/providers`
- `POST /api/admin/providers`
- `POST /api/admin/providers/:id/test`
- `GET /api/admin/models`
- `POST /api/admin/models`
- `GET /api/admin/router/rules`
- `POST /api/admin/router/rules`
- `POST /api/admin/router/simulate`
- `GET /api/admin/race-policies`
- `POST /api/admin/race-policies`
- `GET /api/admin/sessions`
- `POST /api/admin/sessions/:id/rebind`
- `POST /api/admin/sessions/:id/terminate`
- `GET /api/admin/requests`
- `GET /api/admin/requests/:id`
- `POST /api/admin/requests/:id/replay`
- `GET /api/admin/usage/summary`
- `GET /api/admin/usage/timeseries`
- `GET /api/admin/mcp/servers`
- `POST /api/admin/mcp/servers`
- `GET /api/admin/prompts`
- `POST /api/admin/prompts`
- `GET /api/admin/settings`
- `POST /api/admin/settings`
- `POST /api/admin/export`
- `POST /api/admin/import`

## 11.3 实时事件

- `GET /api/admin/events/stream`
- WebSocket 事件频道：
  - `requests`
  - `sessions`
  - `providers`
  - `usage`

## 12. 存储策略

### 本地默认

- SQLite
- 轻量 WAL 模式
- 本地文件滚动日志

### 服务模式

- PostgreSQL 存业务主数据
- Redis 存限流、短期会话与热点统计

### 兼容要求

- 数据访问层必须支持 SQLite / PostgreSQL 双实现。
- Redis 不可用时，限流和会话必须退化到本地实现，但功能不失效。

## 13. 安全设计

### 13.1 鉴权

- 管理端与代理端分离鉴权。
- 管理端支持本地管理员密码或本地设备令牌。
- 服务模式下支持 JWT / OIDC 预留接口，但默认先提供本地管理员体系。

### 13.2 密钥保护

- 所有 Provider 密钥必须加密存储。
- 明文密钥不落日志。
- 请求回放默认脱敏。

### 13.3 审计与脱敏

- 默认记录请求元数据与部分 body。
- 对可识别敏感字段做掩码。
- 开启“高调试模式”时才允许保存完整 payload，并且需显式开关。

### 13.4 本地暴露面

- 默认只监听 `127.0.0.1`。
- 若用户开启局域网监听，必须出现风险提示并强制设置管理鉴权。

## 14. 可观测性

### 14.1 指标

- 请求数
- 成功率
- 错误率
- 上游错误分布
- p50 / p95 / p99 延迟
- 首 token 延迟
- 竞速胜率
- provider 健康分
- 会话重绑定次数
- 逻辑成本
- 物理成本
- 竞速额外成本

### 14.2 日志

- 结构化日志
- `request_id` 全链路关联
- provider / project / session / policy 字段齐全

### 14.3 调试工具

- 路由模拟器
- 单 Provider 连通性测试
- 请求回放
- 会话绑定查看器
- 竞速轨迹查看器

## 15. 工程与仓库结构

采用 monorepo：

```text
relayhub/
  apps/
    console/              # Next.js + HeroUI
    desktop/              # Tauri 2 壳
  server/
    cmd/
      relayhub/
    internal/
      admin/
      gateway/
      router/
      providers/
      sessions/
      usage/
      policy/
      audit/
      storage/
      runtime/
    pkg/
      api/
      events/
      types/
  packages/
    ui/
    config-schema/
    api-client/
  docs/
    PROJECT_SPEC.md
```

### 15.1 前端约束

- Next.js App Router
- HeroUI 作为基础组件层
- 所有业务数据走 Go Admin API
- 尽量采用静态导出兼容桌面打包
- 仅桌面专属能力通过 Tauri bridge

### 15.2 后端约束

- Go 版本固定在现代稳定版本
- HTTP 层、路由层、provider 层、存储层严格解耦
- 所有 provider adapter 必须可单测

## 16. UI 风格方向

UI 不走“普通后台模板风”，而是偏操作台风格：

- 信息密度高，但分区明确。
- 首页重点突出实时状态与异常。
- 使用 HeroUI 做控件基础，但要自定义设计变量。
- 色彩以中性灰、蓝绿告警色、橙红异常色为主，不做默认紫色主题。
- 请求时间线、竞速轨迹、会话绑定图要有强视觉表达。

## 17. 测试与质量要求

不分阶段意味着质量要求也要一次性定义清楚。

### 17.1 后端测试

- provider adapter 单元测试
- router rule 匹配测试
- race engine 并发测试
- usage 聚合测试
- session rebinding 测试
- admin API 集成测试

### 17.2 前端测试

- 控制台页面组件测试
- 路由仿真器交互测试
- Requests / Sessions / Usage 页面关键流测试

### 17.3 桌面测试

- Go Core 生命周期测试
- 配置导入导出测试
- 本地端口切换测试
- 自动启动和托盘交互测试

### 17.4 端到端测试

- OpenAI 风格请求 -> 统一路由 -> 上游成功
- Anthropic 风格请求 -> 统一路由 -> 上游成功
- 竞速产生 winner / loser -> 统计正确
- 配额命中 -> 正确降级或拒绝
- 会话失败重绑定 -> 正确切换

## 18. 完整交付清单

以下交付物都属于项目正式组成部分：

- Go Core
- Web 控制台
- Tauri 桌面端
- OpenAPI 文档
- 配置 Schema
- 数据迁移脚本
- 默认示例配置
- 导入导出格式
- 指标与日志字段规范
- 测试基线
- 打包脚本
- 安装说明

## 19. 风险与约束

### 19.1 主要风险

- 竞速会增加真实成本，需要从统计口径、限额策略和 UI 呈现上一次解决。
- Next.js 在桌面壳里若过度依赖 SSR，会破坏打包与离线能力，因此前端必须保持“静态壳 + 客户端数据拉取”的原则。
- Provider 协议差异大，统一抽象时要防止为了兼容少数边缘特性而污染主模型。
- CLI Bridge Provider 很有价值，但必须限定在桌面模式或本地可信环境中启用。

### 19.2 约束决策

- 业务核心只放在 Go Core。
- 桌面壳不复制业务逻辑。
- 统计口径一开始就区分 logical 与 physical。
- 请求入口优先支持文本与 embeddings，再扩展 audio。

## 20. 结论

RelayHub 的产品目标不是“把五个项目拼在一起”，而是提炼出一个真正完整、可长期维护的统一 AI 中转产品：

- 用 `octopus` 的务实中转和用量统计做基础。
- 用 `claude-code-router` 的路由与 preset 思想做策略层。
- 用 `claude-code-hub` 的会话、限流、治理与测试意识做骨架。
- 用 `aio-coding-hub` 和 `cc-switch` 的桌面控制面能力做本地体验层。

如果按这份规格继续推进，下一步就不是再讨论“做不做这些能力”，而是直接开始建立仓库骨架、配置规范、OpenAPI 契约和核心模块代码。
