# 分组客户端访问策略

## 已确认的范围
- 客户端集中维护；内置 Codex 官方客户端和 DeepSeek Harness，可添加简单请求头识别规则。
- 分组按唯一 ID 配置 `off`、`allow`、`deny`，客户端多选；限制模式不能为空。
- 默认不限制。白名单未识别则拒绝，黑名单未识别则允许；多条客户端规则命中时，黑名单任一命中即拒绝。
- Key 鉴权之后、合成路由/账号调度之前检查，覆盖原生、兼容、根路径及 WebSocket 握手，不改变账号级限制。
- 返回动态中文提示和稳定错误码；维护 API 仅管理员可访问，沿用审计和合规守卫。

## 实现与边界
- 复用 settings 保存单个配置文档，包含客户端和 group ID 策略映射；不新增 SQL 表。
- 基于原始配置 CAS 保存，冲突返回 409；被引用的规则不可删除或停用。
- 热路径使用最多 2 秒的进程缓存；保存清除本实例缓存，其他实例最长 2 秒观察到变更。
- 配置损坏/数据库读取失败返回 503，不使用无限期陈旧配置放行。
- 分组复制同步复制策略，删除同步清理策略，均与分组变更同事务提交。
- Responses WebSocket 每轮复核最新策略，拒绝时发送动态错误帧并关闭连接；其他实时协议仅检查握手，不主动打断已开始的流。
- 客户端标识可伪造，不属于密码学身份认证；自定义头不得配置凭据或 Cookie。
- Harness 默认 User-Agent `deepseek-harness/<version> (+<url>)` 已由本地官方源码确认；白标分支需另建规则。
- 客户端/策略维护在“分组管理 → 客户端访问管理”，先建立分组，再按 ID 配置。
- 不发布、不部署；采用现有 Go/Gin/Ent、Vue/TypeScript 风格，不新增依赖。

## 验证
`cd backend && go test ./internal/service ./internal/server/middleware ./internal/server/routes ./internal/repository -run 'ClientAccess|ClientPolicy'`

`cd frontend && pnpm typecheck && pnpm test:run src/components/admin/__tests__/ClientAccessManager.spec.ts`

覆盖默认/允许/禁止、多选、未知、规则歧义、动态文案、失效配置、并发保存、引用保护和请求不进入下游。

## 使用入口
1. 管理后台“分组管理”右上角点击“客户端访问管理”；分组行及编辑窗口也提供指定分组入口。
2. 在客户端规则中使用预设，或新增唯一标识、名称和公开请求头匹配条件；可选择 AND/OR。
3. 按名称或 ID 搜索并选择分组，设置不限制/仅允许/禁止，勾选客户端，核对动态提示预览。
4. 点击“保存全部规则与策略”。遇到并发冲突时保留当前草稿，重新加载再调整；不自动覆盖他人的修改。
5. Codex 分组使用“仅允许 Codex”；DeepSeek 分组使用“仅允许 DeepSeek Harness”；仍须独立绑定正确上游资源。
6. 新分组先保存基本信息，再进入访问管理配置。访问策略独立保存，不随分组名称编辑自动提交。

## 数据核验（本项目为 PostgreSQL）
不涉及 MySQL DDL 或迁移。可只读检查 settings 中 `group_client_access_v1` 的存在及配置格式；不读取/输出 API Key、令牌或用户请求内容。

客户端识别只使用公开请求头，不读取提示词，不写模型用量。SQL 复用 Ent 参数化更新及既有分组锁函数；复制/删除会锁配置行，防止生命周期操作覆盖管理员保存。

## 验证记录（2026-10-13）
- 客户端识别、策略校验、动态提示、CAS 冲突、分组复制/删除、WS 复核及管理接口测试通过。
- service/middleware/repository 新功能测试通过 race 检查；网关 routes 全包回归通过。
- 前端客户端管理和分组相关 26 项测试通过，双语 key 完整性 3 项测试通过；类型检查及新增文件 lint 通过。
- 在真实浏览器使用本地模拟管理 API 验证新增客户端、多选禁止、保存回读、动态提示和 390px 手机宽度。未向线上写配置，也未用真实用户 Key 调用模型。
- 扩大后端 unit 测试时，既有 `TestInflightEstimate_AccountMappingNoDBAndBoundedMemory` 一次内存阈值断言失败（8,390,824 vs 8,388,608 bytes）；单独重跑通过，未修改计费/内存测试。故不宣称全部后端测试无异常。
- PostgreSQL 实际数据库集成及两款真实客户端调用需在测试环境继续验证；当前持久化验证包括参数化 SQL mock、事务路径代码审查和规则单元测试。
