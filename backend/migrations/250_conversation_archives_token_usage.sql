-- 会话存档补充 token 用量：从客户端收到的响应中解析，上游未返回时为 NULL（未知，不是 0）。
ALTER TABLE conversation_archives
    ADD COLUMN IF NOT EXISTS input_tokens BIGINT;
ALTER TABLE conversation_archives
    ADD COLUMN IF NOT EXISTS output_tokens BIGINT;
ALTER TABLE conversation_archives
    ADD COLUMN IF NOT EXISTS cached_input_tokens BIGINT;
