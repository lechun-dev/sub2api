package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConversationArchiveExcludedHarnessUserIDs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "空值", raw: "", want: []string{}},
		{name: "JSON 数组", raw: `["A11A320C-7A36-4AEE-89BA-92F917953065"]`, want: []string{"a11a320c-7a36-4aee-89ba-92f917953065"}},
		{name: "逗号分隔", raw: "id-1, ID-2 ,, id-1", want: []string{"id-1", "id-2"}},
		{name: "分号与换行", raw: "id-1;\nid-2\r\nid-3", want: []string{"id-1", "id-2", "id-3"}},
		{name: "非法 JSON", raw: `["id-1`, want: []string{}},
		{name: "空白项", raw: "   ,  ; ", want: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ParseConversationArchiveExcludedHarnessUserIDs(tc.raw))
		})
	}
}

func TestNormalizeConversationArchiveExcludedHarnessUserIDsDropsOverlongAndCaps(t *testing.T) {
	overlong := make([]byte, conversationArchiveExcludedClientMaxRunes+1)
	for i := range overlong {
		overlong[i] = 'a'
	}
	got := NormalizeConversationArchiveExcludedHarnessUserIDs([]string{" ok ", string(overlong), ""})
	require.Equal(t, []string{"ok"}, got)

	many := make([]string, conversationArchiveExcludedClientMaxEntries+20)
	for i := range many {
		many[i] = "id-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	require.Len(t, NormalizeConversationArchiveExcludedHarnessUserIDs(many), conversationArchiveExcludedClientMaxEntries)
}

func TestIsExcludedConversationArchiveClient(t *testing.T) {
	excluded := []string{"a11a320c-7a36-4aee-89ba-92f917953065"}
	require.True(t, IsExcludedConversationArchiveClient(excluded, "A11A320C-7A36-4AEE-89BA-92F917953065"))
	require.True(t, IsExcludedConversationArchiveClient(excluded, " a11a320c-7a36-4aee-89ba-92f917953065 "))
	require.False(t, IsExcludedConversationArchiveClient(excluded, "another-machine"))
	require.False(t, IsExcludedConversationArchiveClient(excluded, ""))
	require.False(t, IsExcludedConversationArchiveClient(nil, "a11a320c-7a36-4aee-89ba-92f917953065"))
}

type conversationArchiveMapSettingRepo struct {
	SettingRepository
	values map[string]string
}

func (r conversationArchiveMapSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func TestConversationArchiveServiceExcludedClient(t *testing.T) {
	const machineID = "a11a320c-7a36-4aee-89ba-92f917953065"
	settings := NewSettingService(conversationArchiveMapSettingRepo{values: map[string]string{
		SettingKeyConversationArchiveEnabled:                "true",
		SettingKeyConversationArchiveExcludedHarnessUserIDs: `["` + machineID + `"]`,
	}}, nil)
	svc := NewConversationArchiveService(&recordingConversationArchiveRepo{}, settings)

	require.True(t, svc.ExcludedClient(context.Background(), machineID))
	require.True(t, svc.ExcludedClient(context.Background(), " A11A320C-7A36-4AEE-89BA-92F917953065 "))
	require.False(t, svc.ExcludedClient(context.Background(), "other-machine"))
	require.False(t, svc.ExcludedClient(context.Background(), ""))
}

func TestConversationArchiveServiceExcludedClientWithoutSetting(t *testing.T) {
	settings := NewSettingService(conversationArchiveMapSettingRepo{values: map[string]string{
		SettingKeyConversationArchiveEnabled: "true",
	}}, nil)
	svc := NewConversationArchiveService(&recordingConversationArchiveRepo{}, settings)
	require.False(t, svc.ExcludedClient(context.Background(), "a11a320c-7a36-4aee-89ba-92f917953065"))
}

// 配置文件里的名单不依赖数据库设置，也不需要 settingService。
func TestConversationArchiveServiceExcludedClientFromConfig(t *testing.T) {
	const machineID = "a11a320c-7a36-4aee-89ba-92f917953065"
	svc := NewConversationArchiveService(&recordingConversationArchiveRepo{}, nil,
		WithConversationArchiveExcludedClients([]string{" " + machineID + " "}))

	require.True(t, svc.ExcludedClient(context.Background(), "A11A320C-7A36-4AEE-89BA-92F917953065"))
	require.False(t, svc.ExcludedClient(context.Background(), "other-machine"))
	require.False(t, svc.ExcludedClient(context.Background(), ""))
}

// 配置文件与库表名单是并集：任一处命中都排除。
func TestConversationArchiveServiceExcludedClientMergesConfigAndSetting(t *testing.T) {
	settings := NewSettingService(conversationArchiveMapSettingRepo{values: map[string]string{
		SettingKeyConversationArchiveEnabled:                "true",
		SettingKeyConversationArchiveExcludedHarnessUserIDs: `["from-db"]`,
	}}, nil)
	svc := NewConversationArchiveService(&recordingConversationArchiveRepo{}, settings,
		WithConversationArchiveExcludedClients([]string{"from-config"}))

	require.True(t, svc.ExcludedClient(context.Background(), "from-config"))
	require.True(t, svc.ExcludedClient(context.Background(), "from-db"))
	require.False(t, svc.ExcludedClient(context.Background(), "from-elsewhere"))
}

// 同一个请求可以上报多个标识（DSH 头 + 通用头），任一命中即排除。
func TestConversationArchiveServiceExcludedClientAcceptsMultipleIDs(t *testing.T) {
	settings := NewSettingService(conversationArchiveMapSettingRepo{values: map[string]string{
		SettingKeyConversationArchiveEnabled:                "true",
		SettingKeyConversationArchiveExcludedHarnessUserIDs: `["openclaw-lq-mbp"]`,
	}}, nil)
	svc := NewConversationArchiveService(&recordingConversationArchiveRepo{}, settings)

	require.True(t, svc.ExcludedClient(context.Background(), "", " OPENCLAW-LQ-MBP "))
	require.True(t, svc.ExcludedClient(context.Background(), "a11a320c-7a36-4aee-89ba-92f917953065", "openclaw-lq-mbp"))
	require.False(t, svc.ExcludedClient(context.Background(), "", "another-host"))
	require.False(t, svc.ExcludedClient(context.Background()))
}
