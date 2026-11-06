-- 预置数据：管理员账号、模型供应商、预置 Expert Agent、内置工具、系统 Prompt
-- 说明：种子只在 users 表为空时插入（幂等），由 server 启动时执行

-- 管理员（密码 admin123，bcrypt 哈希在代码层生成，此处占位由 seed 逻辑填充）
-- 模型供应商（api_key 留空，由管理员在控制台配置）
INSERT INTO model_providers (code, name, base_url) VALUES
  ('deepseek', 'DeepSeek', 'https://api.deepseek.com/v1'),
  ('qwen',     '通义千问', 'https://dashscope.aliyuncs.com/compatible-mode/v1'),
  ('zhipu',    '智谱AI',   'https://open.bigmodel.cn/api/paas/v4'),
  ('moonshot', 'Kimi',     'https://api.moonshot.cn/v1'),
  ('doubao',   '豆包Ark',  'https://ark.cn-beijing.volces.com/api/v3'),
  ('ollama',   'Ollama本地', 'http://127.0.0.1:11434/v1')
ON CONFLICT (code) DO NOTHING;

-- 内置工具（parameters 为 JSON Schema，description 给模型看）
INSERT INTO tools (code, name, type, description, parameters, handler, timeout_ms, is_dangerous) VALUES
('calculator', '计算器', 'builtin',
 '执行四则运算表达式求值，输入合法的数学表达式，例如 (1+2)*3',
 '{"type":"object","properties":{"expression":{"type":"string","description":"数学表达式"}},"required":["expression"]}',
 'calculator', 5000, FALSE),
('datetime', '时间日期', 'builtin',
 '获取当前日期时间、时区转换、日期推算',
 '{"type":"object","properties":{"operation":{"type":"string","enum":["now","add_days","diff"],"description":"操作类型"},"tz":{"type":"string","description":"时区，默认 Asia/Shanghai"},"days":{"type":"integer"}},"required":["operation"]}',
 'datetime', 3000, FALSE),
('web_search', '网络搜索', 'builtin',
 '联网搜索最新信息，返回标题、链接与摘要。回答时效性问题时使用',
 '{"type":"object","properties":{"query":{"type":"string","description":"搜索关键词"},"count":{"type":"integer","description":"结果条数，默认5"}},"required":["query"]}',
 'web_search', 15000, FALSE),
('http_client', 'HTTP请求', 'builtin',
 '向白名单域名发起 GET 请求并返回响应内容',
 '{"type":"object","properties":{"url":{"type":"string","description":"完整 URL"},"headers":{"type":"object"}},"required":["url"]}',
 'http_client', 15000, FALSE),
('code_exec', '代码执行', 'builtin',
 '在隔离沙箱中执行代码，支持 python/go/shell，返回 stdout/stderr 与退出码',
 '{"type":"object","properties":{"language":{"type":"string","enum":["python","go","shell"]},"code":{"type":"string","description":"完整代码"}},"required":["language","code"]}',
 'code_exec', 35000, TRUE),
('file_ops', '文件操作', 'builtin',
 '读取或保存会话附件文件，路径必须位于授权目录内',
 '{"type":"object","properties":{"operation":{"type":"string","enum":["read","save"]},"path":{"type":"string"},"content":{"type":"string"}},"required":["operation","path"]}',
 'file_ops', 10000, TRUE),
('db_query', '数据库查询', 'builtin',
 '对业务数据库执行只读 SELECT 查询，用于数据分析场景',
 '{"type":"object","properties":{"sql":{"type":"string","description":"只读 SELECT 语句"}},"required":["sql"]}',
 'db_query', 10000, TRUE),
('kb_search', '知识库检索', 'builtin',
 '在指定知识库中进行混合检索，返回最相关的文档片段',
 '{"type":"object","properties":{"kb_id":{"type":"integer","description":"知识库ID"},"query":{"type":"string"},"top_k":{"type":"integer"}},"required":["kb_id","query"]}',
 'kb_search', 10000, FALSE)
ON CONFLICT (code) DO NOTHING;

-- 预置 Expert Agent / Host Agent（model_config_id 在 seed 逻辑中按可用模型填充）
-- 预置系统 Prompt（configs/prompts/*.yaml 启动时入库）
