# 上游合并保护：会话存档排除功能（禁止被合并覆盖）

> **这是本地自定义功能，上游没有这段代码。**
> 从上游同步分支时，冲突解决或「checkout 上游版本」都会把这一块删掉。
> 合并完成后**必须**运行：
>
> ```bash
> bash .github/scripts/check-conversation-archive.sh
> ```

## 功能背景

会话存档（`conversation_archives`）的「本机客户端排除」：

- 请求头 `X-Sub2API-Archive: skip/off/no/none/local/false/0` → 该请求不入存档（读完即删，不转发上游）。
- 请求头 `x-deepseek-harness-user-id`（DSH 本机标识，`~/.dsh/.anonymous-user-id`）
  或通用头 `x-sub2api-archive-client-id`（OpenClaw 等可自定义头的客户端）命中排除名单 → 不 tee、不入队、不落库。
- 名单来源取并集：配置文件 `conversation_archive.excluded_harness_user_ids`（重启生效）
  ∪ 环境变量 `CONVERSATION_ARCHIVE_EXCLUDED_HARNESS_USER_IDS`
  ∪ 库表设置键 `conversation_archive_excluded_harness_user_ids`（2 秒进程内缓存生效，无页面入口）。
- 命中的请求完全不采集：不解析正文、不复制响应、不写 `conversation_archives`。

本仓库引入提交：`067e09eb9`（tag `v0.2.29`）、`909a9d188`（config 示例）。
生产实例（openclaw 机）已配置排除本机 DSH `a11a320c-7a36-4aee-89ba-92f917953065`。

## 合并上游时不能丢的文件与符号

| 文件 | 必须保留的内容 |
|---|---|
| `backend/internal/service/conversation_archive_exclusion.go` | 整个文件：三个头常量、`Normalize/Parse/IsExcludedConversationArchiveClient` |
| `backend/internal/server/middleware/conversation_archive.go` | `conversationArchiveSkipRequested`、skip 头读完即删、`archive.ExcludedClient(c.Request.Context(), harnessID, clientID)` 命中即放行 |
| `backend/internal/service/conversation_archive.go` | `configExcludedClients` 字段、`WithConversationArchiveExcludedClients`、`ExcludedClient(...)`（先判配置名单再判库表缓存） |
| `backend/internal/config/config.go` | `ConversationArchiveConfig.ExcludedHarnessUserIDs`（mapstructure/yaml `excluded_harness_user_ids`）；`viper.SetDefault("conversation_archive.excluded_harness_user_ids", []string{})`（否则 AutomaticEnv 下切片环境变量被静默丢弃） |
| `backend/internal/service/domain_constants.go` | `SettingKeyConversationArchiveExcludedHarnessUserIDs` |
| `backend/internal/service/setting_parse.go` / `setting_update.go` / `settings_view.go` | 该设置键的解析/更新/视图 |
| `backend/internal/service/wire.go` | `ProvideConversationArchiveService` 里 `cfg.ConversationArchive.ExcludedHarnessUserIDs` 注入 |
| `backend/cmd/server/wire_gen.go` | `ProvideConversationArchiveService(conversationArchiveRepository, settingService, configConfig)` |
| `backend/internal/handler/dto/settings.go`、`handler/admin/setting_handler*.go` | `conversation_archive_excluded_harness_user_ids` DTO 与指针式更新（省略=保持现值） |
| `deploy/config.example.yaml` | `conversation_archive.excluded_harness_user_ids` 示例 |
| `docs/conversation-archive.md` | 排除名单/DSH/OpenClaw 配置章节 |
| 测试 | `conversation_archive_exclusion_test.go`、`conversation_archive_env_test.go`、`middleware/conversation_archive_test.go`、`api_contract_test.go` 相关断言 |

## 冲突处理原则

1. `conversation_archive.go`（middleware 与 service 两处）凡是包含 `ExcludedClient` /
   `SkipHeader` / `excluded_harness` 字样的冲突块，**以本地版本为准**。
2. 如果上游重构了中间件链、`ConversationArchiveService` 或设置体系，需要把
   排除判定**移植**到新结构里，而不是删除；`ExcludedClient` 必须在 tee 之前调用。
3. 不确定时先看 `git log --follow backend/internal/service/conversation_archive_exclusion.go`，
   再对照本文档核对符号。

## 合并后验证

```bash
bash .github/scripts/check-conversation-archive.sh               # 符号完整性（CI 也会跑）
cd backend && make test-unit                                     # 相关单测
# 冒烟：带排除头发请求，确认 conversation_archives 不新增：
#   curl -H "x-deepseek-harness-user-id: <名单中的 id>" ... /v1/messages
```
