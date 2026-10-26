# 按邮箱校验用户（管理端内部接口）

供内部系统（如 lechun-tools）校验某个邮箱对应的 sub2api 用户是否存在、是否为管理员。

## 接口

```
GET /api/v1/admin/users/by-email?email=user@example.com
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `email` | 是 | 邮箱精确匹配；不存在返回 404 |

### 返回

```json
{
  "code": 0,
  "data": {
    "user": {
      "id": 7,
      "username": "张三",
      "email": "zhang@example.com",
      "role": "admin",
      "status": "active"
    },
    "is_admin": true
  }
}
```

`is_admin` 等价于 `user.role == "admin"`；调用方应以此判断是否放行管理员能力，不要依赖用户名或邮箱格式。

## 鉴权

与其他管理接口一致：

- 管理员 JWT：`Authorization: Bearer <jwt>`
- 全权管理员 API Key：`x-api-key: admin-...`
- 受限管理密钥：`x-api-key: adm_sc_...`，权限需包含 `users.read_by_email`（分类「用户」）

该权限只覆盖按邮箱校验，不授予用户列表、余额、密钥管理等其他能力。读取会写入审计日志（`admin.users.read_by_email`）。

## 使用注意

- 该接口返回的是 PII（邮箱、用户名），只应在受信任的内部系统间调用，不要直接暴露给终端用户或 AI 的公共工具。
- 调用方拿到 `is_admin=true` 后再调用会话存档等管理接口，仍需使用自己的管理员凭证；本接口不签发 token。
