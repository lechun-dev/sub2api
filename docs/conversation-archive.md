# 会话存档

把用户与 AI 的文本对话按请求轮次归档到 `conversation_archives` 表，供后续 SQL / BI 分析使用。本期只落表，不提供管理端查询页面。

## 开关与保留期

| settings key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `conversation_archive_enabled` | bool | `false` | 关闭时请求完全不进入存档代码路径 |
| `conversation_archive_retention_days` | int | `90` | `<=0` 表示永久保留；清理任务每 24h 执行一次，分批删除 |

管理入口：管理后台「系统设置 → 安全 → 会话存档」，两个字段可同时保存。

## 覆盖范围

| 协议 | 端点 | 归档内容 |
|---|---|---|
| Anthropic Messages | `POST /v1/messages`、`/antigravity/v1/messages` | system + messages 文本 + 助手文本 |
| OpenAI Chat Completions | `POST /v1/chat/completions` | messages 文本 + 助手文本 |
| OpenAI Responses | `POST /v1/responses`、`/backend-api/codex/responses` | instructions + input 文本 + 助手文本 |
| Gemini 原生 | `POST /v1beta/models/*:generateContent`、`:streamGenerateContent` | systemInstruction + contents 文本 + 助手文本 |

**不归档**：`/v1/messages/count_tokens`、embeddings、images、audio、video、seedance 等非对话端点；Responses WebSocket（连接被 hijack，中间件拿不到帧，本期不覆盖）。

**只保存文本**：图片、附件、工具调用参数、tool_result、思考过程一律丢弃；需要分析工具行为时应另建方案。

## 表结构

| 列 | 说明 |
|---|---|
| `id` | 自增主键 |
| `request_id` | 与 usage_logs / ops 日志关联的请求 ID |
| `user_id` / `api_key_id` / `group_id` | 鉴权身份；不建外键，删除用户不会级联删存档 |
| `protocol` | `anthropic_messages` / `openai_chat` / `openai_responses` / `gemini` |
| `endpoint` | 客户端请求路径 |
| `model` | 请求模型（Gemini 取路径中的模型名） |
| `stream` | 客户端是否收到 SSE |
| `status_code` | 只归档 2xx，便于过滤 |
| `session_key` | 优先取 `session_id` / `x-session-id` / `conversation_id` 请求头，其次 Responses 的 `previous_response_id` / `conversation` |
| `request_messages` | `[{role, content}]` JSONB，仅文本 |
| `response_text` | 助手文本（SSE 已拼接） |
| `request_bytes` / `response_bytes` | 实际捕获字节数，用于发现异常大请求 |
| `truncated` | 内容被截断或响应超过捕获上限 |
| `created_at` | 归档时间（UTC） |

索引：`created_at`、`(user_id, created_at DESC)`、`(model, created_at DESC)`、`session_key`。

## 写入路径与取舍

- 网关中间件在 API Key 鉴权之后包装请求体（tee）与响应 Writer（tee），请求结束后解析并异步入队；不侵入 8+8 个流式/非流式转发函数。
- 请求体上限 1 MiB：超过直接放弃该请求（避免保存不完整对话）。
- 响应捕获上限 4 MiB：超过继续服务客户端，但存档标记 `truncated`。
- 单条消息 32K 字符、全部消息 128K 字符、助手回复 64K 字符，超出截断。
- 独立有界队列（8192）+ 批量写入（100 条 / 500ms），溢出丢弃并计数；存档是可丢失的派生数据，不阻塞也不重试。
- 进程关停时先排空队列再落盘。

## 分析示例

```sql
-- 某用户最近的对话
SELECT created_at, model, request_messages, response_text
FROM conversation_archives
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 50;

-- 各模型每日轮次
SELECT date_trunc('day', created_at) AS d, model, count(*)
FROM conversation_archives
GROUP BY 1, 2
ORDER BY 1 DESC;

-- 按会话串起多轮
SELECT session_key, created_at, request_messages, response_text
FROM conversation_archives
WHERE session_key = $1
ORDER BY created_at;
```

## 隐私与合规

- 会话内容包含用户输入与模型输出，可能含 PII，属敏感数据；只有管理员开关才能开启，默认关闭。
- 不记录凭据、请求头、Cookie、URL 参数与响应字节原文，只落解析后的文本与元数据。
- 保留期到期自动删除；多实例并发清理幂等。
- 该表是「尽力而为」的分析副本：队列溢出、请求超限、客户端断连都可能造成缺失，不能当作审计流水使用（审计请用 `audit_logs` / `prompt_audit_events`）。
