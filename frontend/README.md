# Agent Web 控制台

React 18 + TypeScript + TailwindCSS + GSAP 生产级前端，与 `../backend`（Hertz）通过 REST + SSE 通信。

## 快速开始

```bash
npm install
npm run dev        # http://localhost:5173，/api 已代理到 http://localhost:8080
npm run build      # 类型检查 + 生产构建（dist/）
npm run typecheck  # 仅类型检查
```

后端启动见 `../backend`（`make compose-core && make migrate && make run`）。
后端地址可用环境变量覆盖：`AGENT_BACKEND_URL=http://other:8080 npm run dev`。

## 功能地图

| 路由 | 说明 |
|---|---|
| `/chat` | 流式对话：12 种 SSE 事件渲染（打字机/思考过程/计划卡片/决策链时间线/RAG 引用/用量），会话管理，Agent 选择 |
| `/agents` | Agent 配置：host/expert、提示词、参数、工具绑定、知识库绑定 |
| `/memories` | 长期记忆：列表/归档/删除/手动抽取 |
| `/kbs` `/kbs/:id` | 知识库：文档上传（multipart）、ETL 状态、分块查看、检索测试台 |
| `/workflows` `/workflows/runs` | 工作流：YAML DSL 编辑 + 校验 + 同步执行；执行记录 + HITL 审批 |
| `/admin/models` | 供应商 / 模型配置（admin） |
| `/admin/integrations` | MCP Server（同步/健康检查/工具调试）、A2A 远程 Agent（注册/委派）（admin） |

## 架构

```
src/
├── api/        # 接口封装（与 backend/internal/api 一一对应，类型化）
├── types/      # 领域类型（与 backend/internal/model 一一对应）
├── lib/        # request（401 静默刷新单飞）/ sse（POST SSE 解析器）/ gsap 动效 / format
├── stores/     # zustand：auth（双 Token）/ chat（SSE 状态机）/ ui（Toast）
├── components/ # layout（侧边栏/顶栏）+ ui 原语（Modal/Toast/Confirm…）
└── pages/      # 路由页面（router.tsx 含登录/admin 守卫）
```

关键设计（契约驱动，字段均可在后端找到出处）：

- **SSE 解析**：`lib/sse.ts` 用 fetch + ReadableStream 手动解析 `event:\ndata:` 帧（EventSource 不支持 POST），半帧缓冲、中断静默、异常兜底。
- **状态机**：`stores/chat.ts` 以 `reduce` 归约 12 种事件；流结束以服务端消息列表为唯一事实源重拉。
- **认证**：access 在内存、refresh 在 localStorage；401 单飞刷新并重放；启动 bootstrap 恢复会话。
- **设计系统**：四级黑底 + 荧光绿 `#e4f222`（tailwind.config.js），GSAP 统一动效曲线（lib/gsap.ts）。
