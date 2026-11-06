-- 回滚种子数据
DELETE FROM tools WHERE type = 'builtin';
DELETE FROM model_providers WHERE code IN ('deepseek','qwen','zhipu','moonshot','doubao','ollama');
