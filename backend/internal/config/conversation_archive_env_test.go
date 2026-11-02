//go:build unit

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadConversationArchiveExcludedClientsFromEnv 守住一个容易踩空的 viper 细节：
// AutomaticEnv 只能覆盖 AllKeys() 里已有的 key，所以 conversation_archive.excluded_harness_user_ids
// 必须在 setEnvReachableDefaults 里注册空切片默认值，否则环境变量会被静默丢弃，
// 「本机不归档」看似配了却依然是全量归档。
//
// 与其它标量切片一致，多个取值用逗号分隔；逗号两侧的空格由服务层
// NormalizeConversationArchiveExcludedHarnessUserIDs 统一 trim，所以这里按原样切分即可。
func TestLoadConversationArchiveExcludedClientsFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("CONVERSATION_ARCHIVE_EXCLUDED_HARNESS_USER_IDS",
		"a11a320c-7a36-4aee-89ba-92f917953065,other-machine")

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, []string{
		"a11a320c-7a36-4aee-89ba-92f917953065",
		"other-machine",
	}, cfg.ConversationArchive.ExcludedHarnessUserIDs)
}

// 没配任何东西时应当是空名单（= 不排除任何客户端），而不是 nil 导致的意外行为。
func TestLoadConversationArchiveExcludedClientsDefaultEmpty(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("CONVERSATION_ARCHIVE_EXCLUDED_HARNESS_USER_IDS", "")

	cfg, err := Load()
	require.NoError(t, err)
	require.Empty(t, cfg.ConversationArchive.ExcludedHarnessUserIDs)
}
