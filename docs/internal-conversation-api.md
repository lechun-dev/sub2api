# 内部签名身份接口（会话存档）

供内部系统（lechun-tools）代表某个管理员查询会话存档。调用方不持有管理员 API Key，
而是用共享密钥对「邮箱 + 时间戳 + 方法 + 路径」签名；sub2api 验签后查用户、校验管理员，
再执行只读查询。

## 1. 信任模型

- 共享密钥只在 sub2api 与 lechun-tools 之间共享，不能调用 `/api/v1/admin/*` 等其他管理接口。
- 每次请求都按签名中的邮箱重新校验用户与管理员身份，不依赖调用方自己的判断。
- 用户身份仍由 lechun 登录体系保证；本接口只回答「这个邮箱对应的人是不是 sub2api 管理员」。

## 2. 签名规范

请求头：

| Header | 说明 |
|---|---|
| `X-Lechun-Email` | 声明的用户邮箱 |
| `X-Lechun-Timestamp` | Unix 秒时间戳，超出 `max_skew_seconds`（默认 300 秒）拒绝 |
| `X-Lechun-Signature` | 十六进制 HMAC-SHA256 |

签名串（严格按换行拼接）：

```
<email>\n<timestamp>\n<METHOD>\n<PATH>
```

示例（伪代码）：

```text
canonical = "admin@example.com\n1770000000\nGET\n/api/v1/internal/conversation-archives"
signature = hex(hmac_sha256(secret, canonical))
```

注意：`PATH` 不含查询串，因此筛选参数不在签名范围内；请求必须走 HTTPS，避免被中途篡改。

## 3. 接口

### 3.1 会话列表

```
GET /api/v1/internal/conversation-archives
```

参数与返回结构与管理端 [conversation-archive.md](./conversation-archive.md) 一致（page、page_size、user_id、username、model、protocol、start_time、end_time、keyword、session_key、request_id、order、status_code、stream、truncated、include_content）。

### 3.2 会话详情

```
GET /api/v1/internal/conversation-archives/:id
```

### 3.3 token 用量统计

```
GET /api/v1/internal/conversation-archives/stats
```

筛选参数与列表一致（`user_id`、`username`、`api_key_id`、`group_id`、`model`、`protocol`、`start_time`、`end_time`、`keyword` 等），额外支持：

| 参数 | 说明 |
|---|---|
| `group_by` | `none`（只返回总计）/ `user` / `date`（UTC YYYY-MM-DD）/ `model` / `protocol` / `group` / `api_key` |
| `limit` | 分组返回条数，默认 50，上限 200；按输入+输出 token 排序 |
| `order` | `asc` / `desc`（默认 `desc`） |

返回：

```json
{
  "group_by": "user",
  "order": "desc",
  "items": [
    {
      "key": "7",
      "user_id": 7,
      "username": "张三",
      "email": "zhang@example.com",
      "records": 3,
      "records_with_usage": 2,
      "input_tokens": 100,
      "output_tokens": 200,
      "cached_input_tokens": 50
    }
  ],
  "totals": {
    "records": 3,
    "records_with_usage": 2,
    "input_tokens": 100,
    "output_tokens": 200,
    "cached_input_tokens": 50
  }
}
```

`records_with_usage` 表示有 usage 的记录数；token 字段只累计非 NULL 值，上游未返回时不会按 0 计入。

## 3.4 计费口径 token 统计

```
GET /api/v1/internal/usage/stats
```

数据来自 `usage_logs`，口径与计费一致，不受“上游流式未返回 usage”影响。

| 参数 | 说明 |
|---|---|
| `user_id` / `api_key_id` / `group_id` | 精确筛选 |
| `username` | 按用户名/邮箱模糊筛选 |
| `model` | 模型精确筛选 |
| `start_time` / `end_time` | RFC3339 或 `YYYY-MM-DD`；不传默认最近 30 天，范围上限 366 天；`end_time` 为开区间，date-only 时按次日 00:00 UTC |
| `group_by` | `none` / `user` / `date`（UTC）/ `model` / `group` / `api_key` |
| `limit` | 分组条数，默认 50，上限 200，按输入+输出 token 排序 |
| `order` | `asc` / `desc`，默认 `desc` |

返回：

```json
{
  "group_by": "user",
  "order": "desc",
  "start_time": "2026-10-01T00:00:00Z",
  "end_time": "2026-11-01T00:00:00Z",
  "items": [
    {
      "key": "7",
      "user_id": 7,
      "username": "张三",
      "email": "zhang@example.com",
      "requests": 10,
      "input_tokens": 1000,
      "output_tokens": 2000,
      "cache_creation_tokens": 0,
      "cache_read_tokens": 500,
      "total_tokens": 3000,
      "total_cost": 0.12
    }
  ],
  "totals": {
    "requests": 10,
    "input_tokens": 1000,
    "output_tokens": 2000,
    "cache_creation_tokens": 0,
    "cache_read_tokens": 500,
    "total_tokens": 3000,
    "total_cost": 0.12
  }
}
```

## 4. 配置

```yaml
internal_identity:
  enabled: false
  secret: ""
  max_skew_seconds: 300
```

也可用环境变量 `INTERNAL_IDENTITY_ENABLED` / `INTERNAL_IDENTITY_SECRET` 覆盖。`enabled=false` 或密钥为空时，内部路由返回 404。

## 5. 审计与错误

- 读取会写入审计日志（动作 `internal.conversation_archives.read`），记录生效的管理员用户。
- 错误码：

| 状态 | 含义 |
|---|---|
| 404 | 内部接口未启用 |
| 401 `INVALID_INTERNAL_SIGNATURE` | 缺少头、签名错误 |
| 401 `INTERNAL_SIGNATURE_EXPIRED` | 时间戳超出窗口 |
| 401 `INVALID_INTERNAL_IDENTITY` | 邮箱对应用户不存在或已禁用 |
| 403 | 该用户不是 sub2api 管理员 |

## 6. 运维注意

- 共享密钥属于机器凭证，放在 lechun-tools 的 Nacos 与 sub2api 的服务端配置中，不写入 Git。
- 轮换时先更新 sub2api（旧密钥仍可验证新请求前会有短暂不一致），建议同时更新两侧后观察审计；如需零中断可扩展为双密钥并存，当前实现为单密钥。
- 该接口只读；不要在没有额外保护的情况下把 `/api/v1/internal/*` 暴露到公网入口。
