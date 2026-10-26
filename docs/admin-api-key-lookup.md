# 按邮箱查询用户 API Key（管理端）

供管理员或带权限的管理密钥（含 AI）按邮箱精确定位用户，并列出该用户全部 API Key。

## 接口

```
GET /api/v1/admin/api-keys/by-email?email=user@example.com
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `email` | 是 | 用户邮箱，精确匹配（大小写/归一化由 `users` 查询处理）；未找到返回 404 |
| `status` | 否 | 按 Key 状态过滤：`active` / `disabled` / 过期等；不传返回该用户全部未删除 Key |

### 返回

```json
{
  "code": 0,
  "data": {
    "user": { "id": 7, "username": "张三", "email": "zhang@example.com", "status": "active" },
    "api_keys": [
      {
        "id": 11,
        "name": "默认",
        "key": "sk-...",
        "status": "active",
        "group_id": 2,
        "group_name": "Claude 分组",
        "quota": 10,
        "quota_used": 1.5,
        "expires_at": null,
        "last_used_at": "2026-10-20T08:00:00Z",
        "created_at": "2026-09-01T00:00:00Z"
      }
    ],
    "total": 1
  }
}
```

**注意：`key` 是明文密钥。** 这是管理端能力，调用方拿到后即可使用该 Key，务必只授予可信人员/系统，并优先使用受限管理密钥（按需配置 IP 白名单）。

## 鉴权

与其他管理接口一致，需要管理员身份：

- `x-api-key: <管理员 API Key>` 或 `x-api-key: adm_sc_...`（受限管理密钥）
- `Authorization: Bearer <管理员 JWT>`

受限管理密钥必须包含权限 `api_keys.read_by_email`（分类「密钥」）。该权限**只覆盖按邮箱查询**，不会授予全量密钥库存（`GET /api/v1/admin/api-keys`）或签发/修改/删除能力。

每次读取都会写入审计日志（动作 `admin.api_keys.read_by_email`）。

## 示例

```bash
curl -sG https://your-host/api/v1/admin/api-keys/by-email \
  -H "x-api-key: $ADMIN_KEY" \
  --data-urlencode "email=zhang@example.com"
```

## 与用量/会话的衔接

- 拿到 `api_keys[].id` 后，可用 `GET /api/v1/admin/conversation-archives?api_key_id=<id>` 查该 Key 的会话存档。
- 或直接用 `GET /api/v1/admin/conversation-archives?username=<姓名>`（同一接口见 [conversation-archive.md](./conversation-archive.md)）。
