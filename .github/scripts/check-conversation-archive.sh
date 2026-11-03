#!/usr/bin/env bash
# 会话存档排除功能（excluded_harness_user_ids / X-Sub2API-Archive）完整性检查。
#
# 这是本地自定义功能，上游代码里没有。从上游合并分支时，如果冲突解决
# 不当，这一块的代码可能被上游版本覆盖掉。合并完成后运行本脚本：
# 任何一项缺失都会以非零码退出，提示恢复相关代码。
#
# 详细说明见 docs/UPSTREAM-MERGE-PROTECTION.md。
set -euo pipefail

# 无论从哪个目录调用，都以仓库根（含 .git 的目录）为基准。
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
while [ "$REPO_ROOT" != "/" ] && [ ! -d "$REPO_ROOT/.git" ]; do
  REPO_ROOT="$(dirname "$REPO_ROOT")"
done
cd "$REPO_ROOT"

fail=0

check_file() {
  local f="$1"
  if [ -f "$f" ]; then
    echo "ok   file  $f"
  else
    echo "MISS file  $f"
    fail=1
  fi
}

check_sym() {
  local sym="$1" f="$2"
  if grep -qF -- "$sym" "$f" 2>/dev/null; then
    echo "ok   sym   [$sym] @ $f"
  else
    echo "MISS sym   [$sym] @ $f"
    fail=1
  fi
}

echo "== 会话存档排除功能完整性检查（sub2api）=="

# 核心实现
check_file backend/internal/service/conversation_archive_exclusion.go
check_sym 'ConversationArchiveHarnessUserIDHeader' backend/internal/service/conversation_archive_exclusion.go
check_sym 'func IsExcludedConversationArchiveClient' backend/internal/service/conversation_archive_exclusion.go
check_sym 'func ParseConversationArchiveExcludedHarnessUserIDs' backend/internal/service/conversation_archive_exclusion.go
check_sym 'func conversationArchiveSkipRequested' backend/internal/server/middleware/conversation_archive.go
check_sym 'archive.ExcludedClient(c.Request.Context(),' backend/internal/server/middleware/conversation_archive.go
check_sym 'WithConversationArchiveExcludedClients' backend/internal/service/conversation_archive.go
check_sym 'func (s *ConversationArchiveService) ExcludedClient' backend/internal/service/conversation_archive.go

# 配置链路
check_sym 'ExcludedHarnessUserIDs' backend/internal/config/config.go
check_sym 'viper.SetDefault("conversation_archive.excluded_harness_user_ids", []string{})' backend/internal/config/config.go
check_sym 'SettingKeyConversationArchiveExcludedHarnessUserIDs' backend/internal/service/domain_constants.go
check_sym 'cfg.ConversationArchive.ExcludedHarnessUserIDs' backend/internal/service/wire.go
check_sym 'ProvideConversationArchiveService(conversationArchiveRepository, settingService, configConfig)' backend/cmd/server/wire_gen.go

# 管理端设置链路
check_sym 'conversation_archive_excluded_harness_user_ids' backend/internal/handler/dto/settings.go
check_sym 'ConversationArchiveExcludedHarnessUserIDs' backend/internal/handler/admin/setting_handler_update.go

# 文档与示例配置
check_sym 'excluded_harness_user_ids' deploy/config.example.yaml
check_sym 'excluded_harness_user_ids' docs/conversation-archive.md

# 回归测试
check_file backend/internal/service/conversation_archive_exclusion_test.go
check_file backend/internal/config/conversation_archive_env_test.go
check_file backend/internal/server/middleware/conversation_archive_test.go

if [ "$fail" -ne 0 ]; then
  echo
  echo "!! 会话存档排除功能缺失。这是本地自定义功能，上游没有这段代码。"
  echo "!! 请对照 docs/UPSTREAM-MERGE-PROTECTION.md 恢复相关代码，"
  echo "!! 不要在合并冲突中直接采用上游版本。"
  exit 1
fi

echo "== 全部通过 =="
