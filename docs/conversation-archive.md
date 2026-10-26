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
| `input_tokens` / `output_tokens` / `cached_input_tokens` | 从本次响应解析的 token 用量；上游未返回时为 NULL（未知，不是 0） |
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

## 查询接口（管理员 / AI 只读）

### 鉴权

三个管理端入口二选一，**其他身份一律 401/403**：

| 方式 | 请求头 | 说明 |
|---|---|---|
| 管理员 JWT | `Authorization: Bearer <jwt>` | 用户角色必须为 admin |
| 管理员 API Key | `x-api-key: <admin-api-key>` | 系统设置里的全权管理密钥 |
| 受限管理密钥 | `x-api-key: adm_sc_...` | 权限策略必须包含 `conversation_archive.read` |

权限是 deny-by-default：新路由未登记到 `adminaccess` 目录时受限密钥直接 403；本接口已登记为全局权限 `conversation_archive.read`（分类「会话」）。每次读取都会写入审计日志（动作 `admin.conversation_archives.read`）。

### 列表

```
GET /api/v1/admin/conversation-archives
```

| 参数 | 类型 | 说明 |
|---|---|---|
| `page` / `page_size` | int | 默认 1 / 20，`page_size` 上限 100 |
| `user_id` / `api_key_id` / `group_id` | int64 | 精确匹配 |
| `username`（别名 `name`） | string | 按用户姓名模糊查询：匹配 `users.username` 或 `users.email`（ILIKE，最多 100 字） |
| `request_id` | string | 与 usage_logs 关联 |
| `session_key` | string | 会话串联键 |
| `protocol` | enum | `anthropic_messages` / `openai_chat` / `openai_responses` / `gemini` |
| `model` | string | 精确匹配（区分大小写） |
| `endpoint` | string | 精确匹配，如 `/v1/messages` |
| `status_code` | int | 100–599 |
| `stream` | bool | 是否 SSE |
| `truncated` | bool | 是否被截断 |
| `start_time` / `end_time` | string | RFC3339（`2026-10-01T00:00:00Z`）或 `YYYY-MM-DD`（UTC 零点） |
| `keyword`（别名 `q`） | string | 在请求消息与回复文本中做 ILIKE 匹配，上限 200 字；建议配合时间范围 |
| `include_content` | bool | 默认 `false`，只返回元数据与长度；`true` 时带 `request_messages` / `response_text` |
| `order` | `asc`/`desc` | 按 `created_at`，默认 `desc` |

返回：`code=0`，`data.items[]` 含 `id/request_id/user_id/username/email/api_key_id/group_id/group_name/group_platform/protocol/endpoint/model/stream/status_code/session_key/message_count/response_chars/input_tokens/output_tokens/cached_input_tokens/truncated/created_at`；`include_content=true` 时额外返回 `request_messages` 与 `response_text`。

`group_name` / `group_platform` 来自查询时关联的 `groups` 表：分组改名后返回当前名称；分组已删除时两者为空，但 `group_id` 仍保留。

`username`/`email` 来自查询时关联的 `users` 表：用户改名后按姓名查询与返回值都反映当前名称；用户已删除时这两个字段为空，但按 `user_id` 仍可查到历史存档。

### 详情

```
GET /api/v1/admin/conversation-archives/:id
```

始终返回完整 `request_messages` 与 `response_text`；不存在返回 404。

### 调用示例

```bash
# 某用户最近 7 天的对话（不含正文）
curl -sG https://your-host/api/v1/admin/conversation-archives \
  -H "x-api-key: $ADMIN_KEY" \
  --data-urlencode "user_id=42" \
  --data-urlencode "start_time=2026-10-19T00:00:00Z" \
  --data-urlencode "page_size=50"

# 按关键词搜索并直接取正文
curl -sG https://your-host/api/v1/admin/conversation-archives \
  -H "x-api-key: $ADMIN_KEY" \
  --data-urlencode "keyword=退款" \
  --data-urlencode "protocol=openai_chat" \
  --data-urlencode "include_content=true" \
  --data-urlencode "page_size=20"

# 按会话串起多轮
curl -sG https://your-host/api/v1/admin/conversation-archives \
  -H "x-api-key: $ADMIN_KEY" \
  --data-urlencode "session_key=$SESSION" \
  --data-urlencode "include_content=true" \
  --data-urlencode "order=asc"
```

### AI 使用建议

1. 先不带 `include_content` 翻页，按元数据定位候选记录，减少上下文占用。
2. 对候选记录用详情接口（或小页 `include_content=true`）取原文。
3. `keyword` 是 `ILIKE '%...%'`，大表上请始终带 `start_time`/`end_time` 或用户维度，避免全表扫描。
4. `truncated=true` 表示内容被截断（请求超 1 MiB 会整条放弃，不会出现半条对话）。
5. token 用量来自客户端收到的响应（Anthropic / OpenAI / Gemini 的 usage 字段）；上游流式不返回 usage 时为空，不要当作 0 参与均值计算，口径以 `usage_logs` 为准。
