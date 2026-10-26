-- 会话存档：按请求轮次保存用户与 AI 的文本对话，供后续分析使用。
--
-- 设计约束：
--   * 默认关闭，由管理员在系统设置中开启（conversation_archive_enabled）。
--   * 只保存文本内容：图片、附件、工具调用、思考过程不写入。
--   * 分析型大表，不加外键（避免插入检查与级联删除拖慢写入），user_id/api_key_id 仅作关联列。
--   * 按 created_at 索引支撑保留期清理与时间范围分析。
--   * 索引与建表放在同一迁移中：上线时是空表，无锁风险。
CREATE TABLE IF NOT EXISTS conversation_archives (
    id                BIGSERIAL PRIMARY KEY,
    request_id        VARCHAR(128) NOT NULL DEFAULT '',
    user_id           BIGINT,
    api_key_id        BIGINT,
    group_id          BIGINT,
    protocol          VARCHAR(32) NOT NULL DEFAULT '',
    endpoint          VARCHAR(255) NOT NULL DEFAULT '',
    model             VARCHAR(128) NOT NULL DEFAULT '',
    stream            BOOLEAN NOT NULL DEFAULT FALSE,
    status_code       INT NOT NULL DEFAULT 0,
    session_key       VARCHAR(128),
    request_messages  JSONB NOT NULL DEFAULT '[]'::jsonb,
    response_text     TEXT NOT NULL DEFAULT '',
    request_bytes     INT NOT NULL DEFAULT 0,
    response_bytes    INT NOT NULL DEFAULT 0,
    truncated         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_archives_created_at
    ON conversation_archives (created_at);
CREATE INDEX IF NOT EXISTS idx_conversation_archives_user_created
    ON conversation_archives (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_archives_model_created
    ON conversation_archives (model, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_archives_session
    ON conversation_archives (session_key);
