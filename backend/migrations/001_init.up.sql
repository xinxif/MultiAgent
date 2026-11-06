-- Agent 初始表结构
-- 规范：主键 BIGSERIAL 或 UUID；时间 TIMESTAMPTZ；灵活配置 JSONB

-- ===== 用户 =====
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(128) NOT NULL UNIQUE,
    password_hash VARCHAR(256) NOT NULL,
    role          VARCHAR(20)  NOT NULL DEFAULT 'user',
    avatar_url    VARCHAR(512) NOT NULL DEFAULT '',
    status        SMALLINT     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ===== 会话 =====
CREATE TABLE sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       BIGINT      NOT NULL REFERENCES users(id),
    agent_id      BIGINT      REFERENCES agents(id),
    title         VARCHAR(256) NOT NULL DEFAULT '新对话',
    status        VARCHAR(20) NOT NULL DEFAULT 'active',
    pinned        BOOLEAN     NOT NULL DEFAULT FALSE,
    message_count INT         NOT NULL DEFAULT 0,
    token_usage   BIGINT      NOT NULL DEFAULT 0,
    last_msg_at   TIMESTAMPTZ,
    metadata      JSONB       NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sessions_user ON sessions(user_id, last_msg_at DESC);

-- ===== 消息 =====
CREATE TABLE messages (
    id                BIGSERIAL PRIMARY KEY,
    session_id        UUID        NOT NULL REFERENCES sessions(id),
    role              VARCHAR(20) NOT NULL,
    content           TEXT        NOT NULL,
    reasoning_content TEXT,
    tool_calls        JSONB,
    tool_call_id      VARCHAR(128),
    refs              JSONB,
    model_name        VARCHAR(64),
    prompt_tokens     INT         NOT NULL DEFAULT 0,
    completion_tokens INT         NOT NULL DEFAULT 0,
    first_token_ms    INT         NOT NULL DEFAULT 0,
    latency_ms        INT         NOT NULL DEFAULT 0,
    status            SMALLINT    NOT NULL DEFAULT 1,
    trace_id          VARCHAR(64) NOT NULL DEFAULT '',
    metadata          JSONB       NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_messages_session ON messages(session_id, created_at);

-- ===== 模型供应商 =====
CREATE TABLE model_providers (
    id         BIGSERIAL PRIMARY KEY,
    code       VARCHAR(32)  NOT NULL UNIQUE,
    name       VARCHAR(64)  NOT NULL,
    base_url   VARCHAR(512) NOT NULL,
    api_key    TEXT         NOT NULL DEFAULT '',
    status     SMALLINT     NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ===== 模型配置 =====
CREATE TABLE model_configs (
    id             BIGSERIAL PRIMARY KEY,
    provider_id    BIGINT      NOT NULL REFERENCES model_providers(id),
    model_name     VARCHAR(64) NOT NULL,
    alias          VARCHAR(64) NOT NULL,
    type           VARCHAR(16) NOT NULL,
    context_window INT         NOT NULL DEFAULT 8192,
    max_output     INT         NOT NULL DEFAULT 4096,
    input_price    NUMERIC(12,6) NOT NULL DEFAULT 0,
    output_price   NUMERIC(12,6) NOT NULL DEFAULT 0,
    priority       INT         NOT NULL DEFAULT 100,
    capabilities   JSONB       NOT NULL DEFAULT '[]',
    is_default     BOOLEAN     NOT NULL DEFAULT FALSE,
    status         SMALLINT    NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_model_configs_type ON model_configs(type, priority);

-- ===== Agent 配置 =====
CREATE TABLE agents (
    id                BIGSERIAL PRIMARY KEY,
    name              VARCHAR(128) NOT NULL,
    description       TEXT,
    type              VARCHAR(20)  NOT NULL DEFAULT 'expert',
    system_prompt     TEXT,
    model_config_id   BIGINT      NOT NULL REFERENCES model_configs(id),
    fallback_model_config_id BIGINT REFERENCES model_configs(id),
    temperature       REAL        NOT NULL DEFAULT 0.7,
    top_p             REAL        NOT NULL DEFAULT 0.9,
    max_tokens        INT         NOT NULL DEFAULT 4096,
    max_iterations    INT         NOT NULL DEFAULT 10,
    memory_enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    memory_window     INT         NOT NULL DEFAULT 20,
    memory_long_enabled BOOLEAN   NOT NULL DEFAULT FALSE,
    kb_ids            JSONB       NOT NULL DEFAULT '[]',
    tools             JSONB       NOT NULL DEFAULT '[]',
    is_preset         BOOLEAN     NOT NULL DEFAULT FALSE,
    config            JSONB       NOT NULL DEFAULT '{}',
    status            SMALLINT    NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===== 工具 =====
CREATE TABLE tools (
    id            BIGSERIAL PRIMARY KEY,
    code          VARCHAR(64)  NOT NULL UNIQUE,
    name          VARCHAR(128) NOT NULL,
    type          VARCHAR(20)  NOT NULL,
    description   TEXT         NOT NULL,
    parameters    JSONB        NOT NULL,
    handler       VARCHAR(64),
    mcp_server_id BIGINT REFERENCES mcp_servers(id),
    endpoint      VARCHAR(512),
    auth_config   JSONB,
    default_config JSONB      NOT NULL DEFAULT '{}',
    timeout_ms    INT          NOT NULL DEFAULT 30000,
    is_dangerous  BOOLEAN      NOT NULL DEFAULT FALSE,
    status        SMALLINT     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ===== MCP 服务 =====
CREATE TABLE mcp_servers (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(64)  NOT NULL,
    transport      VARCHAR(20)  NOT NULL,
    url            VARCHAR(512) NOT NULL DEFAULT '',
    command        VARCHAR(512) NOT NULL DEFAULT '',
    args           JSONB        NOT NULL DEFAULT '[]',
    env            JSONB        NOT NULL DEFAULT '{}',
    headers        JSONB        NOT NULL DEFAULT '{}',
    status         VARCHAR(20)  NOT NULL DEFAULT 'disconnected',
    last_health_at TIMESTAMPTZ,
    tools_cache    JSONB        NOT NULL DEFAULT '[]',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ===== A2A 远程 Agent 注册 =====
CREATE TABLE a2a_agents (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(128) NOT NULL,
    base_url       VARCHAR(512) NOT NULL,
    card           JSONB        NOT NULL,
    status         VARCHAR(20)  NOT NULL DEFAULT 'unknown',
    last_health_at TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ===== 知识库 =====
CREATE TABLE knowledge_bases (
    id                 BIGSERIAL PRIMARY KEY,
    name               VARCHAR(128) NOT NULL,
    description        TEXT,
    user_id            BIGINT      NOT NULL REFERENCES users(id),
    embedding_config_id BIGINT     NOT NULL REFERENCES model_configs(id),
    chunk_size         INT         NOT NULL DEFAULT 512,
    chunk_overlap      INT         NOT NULL DEFAULT 64,
    parser_config      JSONB       NOT NULL DEFAULT '{}',
    doc_count          INT         NOT NULL DEFAULT 0,
    milvus_collection  VARCHAR(128) NOT NULL,
    status             VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===== 文档 =====
CREATE TABLE documents (
    id           BIGSERIAL PRIMARY KEY,
    kb_id        BIGINT       NOT NULL REFERENCES knowledge_bases(id),
    filename     VARCHAR(256) NOT NULL,
    file_type    VARCHAR(16)  NOT NULL,
    object_key   VARCHAR(512) NOT NULL,
    file_size    BIGINT       NOT NULL DEFAULT 0,
    source_url   VARCHAR(512) NOT NULL DEFAULT '',
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending',
    error        VARCHAR(512) NOT NULL DEFAULT '',
    chunk_count  INT          NOT NULL DEFAULT 0,
    token_count  INT          NOT NULL DEFAULT 0,
    enable_graph BOOLEAN      NOT NULL DEFAULT FALSE,
    metadata     JSONB        NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_documents_kb ON documents(kb_id, status);

-- ===== 文档分块 =====
CREATE TABLE document_chunks (
    id          BIGSERIAL PRIMARY KEY,
    kb_id       BIGINT      NOT NULL,
    document_id BIGINT      NOT NULL REFERENCES documents(id),
    seq         INT         NOT NULL DEFAULT 0,
    content     TEXT        NOT NULL,
    token_count INT         NOT NULL DEFAULT 0,
    meta        JSONB       NOT NULL DEFAULT '{}',
    vector_id   VARCHAR(64) NOT NULL,
    status      SMALLINT    NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_chunks_doc ON document_chunks(document_id, seq);

-- ===== 长期记忆 =====
CREATE TABLE memories (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id),
    agent_id    BIGINT      REFERENCES agents(id),
    content     VARCHAR(1024) NOT NULL,
    vector_id   VARCHAR(64) NOT NULL,
    scope       VARCHAR(16) NOT NULL DEFAULT 'user',
    importance  SMALLINT    NOT NULL DEFAULT 5,
    hit_count   INT         NOT NULL DEFAULT 0,
    last_hit_at TIMESTAMPTZ,
    status      SMALLINT    NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_memories_user ON memories(user_id, agent_id, status);

-- ===== Prompt 模板 =====
CREATE TABLE prompts (
    id         BIGSERIAL PRIMARY KEY,
    agent_id   BIGINT REFERENCES agents(id),
    name       VARCHAR(128) NOT NULL,
    content    TEXT         NOT NULL,
    variables  JSONB        NOT NULL DEFAULT '[]',
    version    INT          NOT NULL DEFAULT 1,
    is_active  BOOLEAN      NOT NULL DEFAULT FALSE,
    remark     VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ===== 工作流 =====
CREATE TABLE workflows (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(128) NOT NULL,
    description TEXT,
    dsl        TEXT         NOT NULL,
    version    INT          NOT NULL DEFAULT 1,
    is_active  BOOLEAN      NOT NULL DEFAULT FALSE,
    creator_id BIGINT       NOT NULL REFERENCES users(id),
    status     SMALLINT     NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ===== 工作流执行 =====
CREATE TABLE workflow_runs (
    id           BIGSERIAL PRIMARY KEY,
    workflow_id  BIGINT      NOT NULL REFERENCES workflows(id),
    trigger_type VARCHAR(20) NOT NULL DEFAULT 'manual',
    status       VARCHAR(24) NOT NULL DEFAULT 'running',
    input        JSONB       NOT NULL DEFAULT '{}',
    output       JSONB,
    error        VARCHAR(512) NOT NULL DEFAULT '',
    trace_id     VARCHAR(64) NOT NULL DEFAULT '',
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);

-- ===== 工作流步骤执行 =====
CREATE TABLE workflow_step_runs (
    id          BIGSERIAL PRIMARY KEY,
    run_id      BIGINT      NOT NULL REFERENCES workflow_runs(id),
    node_key    VARCHAR(64) NOT NULL,
    node_type   VARCHAR(24) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'running',
    input       JSONB,
    output      JSONB,
    error       VARCHAR(512) NOT NULL DEFAULT '',
    tokens      INT         NOT NULL DEFAULT 0,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE INDEX idx_step_runs_run ON workflow_step_runs(run_id);

-- ===== 评估集 =====
CREATE TABLE eval_datasets (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(128) NOT NULL,
    description TEXT,
    scene       VARCHAR(16) NOT NULL DEFAULT 'agent',
    case_count  INT         NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===== 评估用例 =====
CREATE TABLE eval_cases (
    id                 BIGSERIAL PRIMARY KEY,
    dataset_id         BIGINT NOT NULL REFERENCES eval_datasets(id),
    question           TEXT   NOT NULL,
    expected_answer    TEXT,
    expected_tools     JSONB  NOT NULL DEFAULT '[]',
    expected_chunk_ids JSONB  NOT NULL DEFAULT '[]',
    tags               VARCHAR(255) NOT NULL DEFAULT '',
    source             VARCHAR(16) NOT NULL DEFAULT 'manual',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===== 评估执行 =====
CREATE TABLE eval_runs (
    id          BIGSERIAL PRIMARY KEY,
    dataset_id  BIGINT      NOT NULL REFERENCES eval_datasets(id),
    agent_id    BIGINT      REFERENCES agents(id),
    config      JSONB       NOT NULL DEFAULT '{}',
    status      VARCHAR(16) NOT NULL DEFAULT 'pending',
    total       INT         NOT NULL DEFAULT 0,
    passed      INT         NOT NULL DEFAULT 0,
    metrics     JSONB,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===== 评估结果 =====
CREATE TABLE eval_results (
    id            BIGSERIAL PRIMARY KEY,
    run_id        BIGINT   NOT NULL REFERENCES eval_runs(id),
    case_id       BIGINT   NOT NULL REFERENCES eval_cases(id),
    actual_answer TEXT,
    tool_trace    JSONB,
    scores        JSONB   NOT NULL DEFAULT '{}',
    passed        BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms    INT     NOT NULL DEFAULT 0,
    tokens        INT     NOT NULL DEFAULT 0,
    error         VARCHAR(512) NOT NULL DEFAULT ''
);

-- ===== 反馈（坏例回流）=====
CREATE TABLE feedbacks (
    id            BIGSERIAL PRIMARY KEY,
    message_id    BIGINT      NOT NULL REFERENCES messages(id),
    session_id    UUID        NOT NULL,
    user_id       BIGINT      NOT NULL,
    rating        SMALLINT    NOT NULL,
    reason        VARCHAR(64) NOT NULL DEFAULT '',
    comment       VARCHAR(512) NOT NULL DEFAULT '',
    resolved      BOOLEAN     NOT NULL DEFAULT FALSE,
    eval_case_id  BIGINT      REFERENCES eval_cases(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===== 用量 =====
CREATE TABLE usage_logs (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL DEFAULT 0,
    agent_id          BIGINT NOT NULL DEFAULT 0,
    session_id        UUID,
    message_id        BIGINT NOT NULL DEFAULT 0,
    scene             VARCHAR(16) NOT NULL DEFAULT 'chat',
    model             VARCHAR(64) NOT NULL,
    prompt_tokens     INT    NOT NULL DEFAULT 0,
    completion_tokens INT    NOT NULL DEFAULT 0,
    cost_amount       NUMERIC(12,6) NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_usage_agent_time ON usage_logs(agent_id, created_at);
CREATE INDEX idx_usage_time ON usage_logs(created_at);

-- ===== 沙箱执行记录 =====
CREATE TABLE sandbox_executions (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL,
    tool_call_id VARCHAR(64) NOT NULL,
    language     VARCHAR(16) NOT NULL,
    code_hash    CHAR(64)    NOT NULL,
    exit_code    INT         NOT NULL DEFAULT -1,
    duration_ms  INT         NOT NULL DEFAULT 0,
    stdout_head  TEXT,
    stderr_head  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
