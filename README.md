Agent 是一个**通用智能体（Agent）平台**，覆盖了 Agent 开发的主线能力：

- **ReAct 智能体**：Function Calling / 推理-行动循环 / 工具调用，含思考过程与决策链可视化
- **RAG 知识库**：文档上传 → ETL 解析 → 分块 → 向量化 → 混合检索（稠密 + 稀疏 + RRF 融合），带检索测试台
- **多智能体协作**：Host/Expert 模式、A2A 远程 Agent 委派
- **工作流引擎**：YAML DSL 编排、条件分支、HITL 人工审批中断/恢复
- **MCP 集成**：作为 MCP Client 接入外部工具生态，支持调试与健康检查
- **长期记忆**：对话自动抽取记忆，跨会话沉淀
- **生产基建**：JWT 双 Token 认证、密钥 AES-GCM 加密、OpenTelemetry 全链路追踪、Prometheus 指标、Redis Streams 异步任务

Agent 能力层提供两套实现：`agent/naive/` 是零第三方依赖的最小参考实现，便于理解协议与流程本身；主链路基于 [eino](https://github.com/cloudwego/eino) 框架构建完整运行时。

<p align="center">
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go"></a>
  <a href="https://react.dev"><img src="https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black" alt="React"></a>
  <a href="#"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License"></a>
</p>

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.26+ · Hertz（HTTP）· GORM · PostgreSQL 16 · Redis 7（Streams）· Milvus 2.5（向量）· MinIO（对象存储）· golang-migrate |
| AI/Agent | DeepSeek / Ollama（OpenAI 兼容协议）· eino 框架 · mcp-go · Docker 沙箱（代码执行）· SearXNG（联网搜索） |
| 前端 | React 18.3 · TypeScript（strict）· TailwindCSS 3.4 · GSAP 3.13 · Zustand 5 · React Router 6 · Vite 5 |
| 可观测 | OpenTelemetry（Jaeger）· Prometheus · Grafana · zap 结构化日志 |

## 快速开始

### 环境要求

- **Go ≥ 1.26**（`go version` 检查）
- **Node.js ≥ 18**（前端）
- **Docker & Docker Compose**（基础设施一键起）

### ① 启动基础设施（PostgreSQL / Redis / MinIO）

```bash
cd backend
make compose-core          # 只起核心三件套；RAG/联网搜索需 make compose-up（含 Milvus/SearXNG/Ollama）
```

### ② 数据库迁移 + 初始化管理员

```bash
make migrate               # 执行 migrations/*.sql，并种子默认管理员 admin / admin123
```

### ③ 启动后端 API 服务

```bash
make run                   # 监听 http://localhost:8080，同时启动 A2A 端点 :8082
```

> 如需异步任务（文档 ETL 解析入库），另开一个终端执行 `make run-worker`。

### ④ 启动前端控制台

```bash
cd ../frontend
npm install
npm run dev                # http://localhost:5173，/api 已代理到 localhost:8080
```

### ⑤ 配置模型，开始对话

1. 打开 http://localhost:5173，用 `admin / admin123` 登录
2. 进入 **管理后台 → 模型配置**，给预置的 DeepSeek 供应商填入你的 `API Key`（或切换到 Ollama 本地模型，`config.yaml` 中 `llm.local: true`）
3. 回到 **对话页**，发送消息，即可看到流式输出、思考过程、工具调用决策链

> 完整体验 RAG / 联网搜索：`make compose-up` 启动全部基础设施（Milvus / SearXNG / Ollama），重启后端即可。

## 项目结构

```
agent/
├── README.md                  # 本文件
├── backend/                   # Go 后端
│   ├── cmd/
│   │   ├── server/            # API 服务入口（依赖装配）
│   │   ├── worker/            # 异步任务 Worker（文档 ETL / 评估）
│   │   ├── cli/               # 终端对话 Agent（跳过前端直接体验）
│   │   └── migrate/           # 数据库迁移工具（make migrate）
│   ├── configs/config.yaml    # 默认配置（AGENT_ 环境变量可覆盖）
│   ├── deployments/           # docker-compose / Dockerfile / Prometheus / Grafana
│   ├── migrations/            # SQL 版本化迁移（golang-migrate）
│   └── internal/
│       ├── api/               # HTTP 路由与 Handler（REST + SSE）
│       ├── service/           # 业务编排层
│       ├── domain/            # 领域模型
│       ├── repo/              # 数据访问层（GORM）
│       ├── agent/             # ReAct 循环（naive 参考实现 + eino 主链路）
│       ├── rag/               # 检索增强引擎（分块/混合检索/RRF）
│       ├── workflow/          # 工作流引擎（DSL/HITL）
│       ├── memory/            # 长期记忆抽取
│       ├── tool/              # 工具系统（内置/HTTP/沙箱/db_query）
│       ├── llm/               # LLM 网关（多供应商/熔断/重试/流式）
│       ├── mcp/ a2a/          # MCP Client / A2A 远程协作
│       ├── infra/             # PG/Redis/Milvus/MinIO 连接初始化
│       ├── security/          # JWT / AES-GCM 加密
│       ├── observability/     # OTel 追踪 / Prometheus 指标 / 日志
│       └── worker/            # Redis Streams 消费者
└── frontend/                  # React 控制台
    └── src/
        ├── api/               # 类型化接口封装（与后端 Handler 一一对应）
        ├── pages/             # 登录/对话/agents/kbs/workflows/admin
        ├── stores/            # zustand：auth / chat（SSE 状态机）/ ui
        ├── lib/               # request（401 刷新重放）/ sse（POST SSE 解析）/ gsap
        ├── components/        # 布局 + UI 原语 + 决策链时间线/计划卡片
        └── types/             # 领域类型
```

## 常用端口速查

| 服务 | 地址 | 说明 |
|---|---|---|
| 前端控制台 | http://localhost:5173 | 登录账号 `admin / admin123` |
| 后端 API | http://localhost:8080 | REST + SSE |
| A2A 端点 | http://localhost:8082 | `/.well-known/agent.json` |
| MinIO 控制台 | http://localhost:9001 | `minioadmin / minioadmin` |
| Jaeger UI | http://localhost:16686 | 链路追踪（需 obs profile） |
| Grafana | http://localhost:3004 | 监控面板（`admin / admin`） |

## 常见问题

- **`make migrate` 连接数据库失败**：先确认 `make compose-core` 已启动且 PG 健康检查通过，再重试。
- **对话报模型错误**：未配置 API Key。到「管理后台 → 模型配置」填写，或使用 Ollama 本地模型。
- **端口冲突**：Redis 使用 **6380**（避开本机 6379），Grafana 使用 3004，均可在 `config.yaml` / compose 中调整。
- **想体验 RAG 但不想起 Milvus**：`config.yaml` 中 `milvus.mode: lite` 可用本地嵌入式模式（仅开发用）。
