package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientPolicyDatesAuthoritativeAndLegacy(t *testing.T) {
	svc := NewSettingService(&clientAccessTestRepo{}, nil)
	cfg, err := svc.GetClientAccessConfig(context.Background())
	require.NoError(t, err)
	fake := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg.Groups["1"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex"}, CreatedAt: &fake, UpdatedAt: &fake}
	require.NoError(t, svc.SaveClientAccessConfig(context.Background(), *cfg))
	saved, err := svc.GetClientAccessConfig(context.Background())
	require.NoError(t, err)
	first := saved.Groups["1"]
	require.NotNil(t, first.CreatedAt)
	require.True(t, first.CreatedAt.After(fake))
	require.Equal(t, first.CreatedAt, first.UpdatedAt)
	forged := saved.Groups["1"]
	forged.CreatedAt, forged.UpdatedAt = &fake, &fake
	saved.Groups["1"] = forged
	require.NoError(t, svc.SaveClientAccessConfig(context.Background(), *saved))
	again, err := svc.GetClientAccessConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, first, again.Groups["1"])
	now := first.UpdatedAt.Add(time.Hour)
	edited := again.Groups["1"]
	edited.Mode = "deny"
	next := DefaultClientAccessConfig()
	next.Groups["1"] = edited
	stampClientPolicyDates(&next, again, now)
	require.Equal(t, first.CreatedAt, next.Groups["1"].CreatedAt)
	require.Equal(t, now, *next.Groups["1"].UpdatedAt)
	legacy := DefaultClientAccessConfig()
	legacy.Groups["2"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex"}}
	raw, _ := json.Marshal(legacy)
	var copy ClientAccessConfig
	require.NoError(t, json.Unmarshal(raw, &copy))
	stampClientPolicyDates(&copy, &legacy, now)
	require.Nil(t, copy.Groups["2"].CreatedAt)
	require.Nil(t, copy.Groups["2"].UpdatedAt)
	p := copy.Groups["2"]
	p.Mode = "deny"
	copy.Groups["2"] = p
	stampClientPolicyDates(&copy, &legacy, now)
	require.Nil(t, copy.Groups["2"].CreatedAt)
	require.Equal(t, now, *copy.Groups["2"].UpdatedAt)
}
func TestClientPolicyDatesOnlyAffectedDefinitionsAndOrder(t *testing.T) {
	current := DefaultClientAccessConfig()
	old := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	now := old.Add(time.Hour)
	current.Groups["1"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex", "deepseek_harness"}, CreatedAt: &old, UpdatedAt: &old}
	current.Groups["2"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex"}, CreatedAt: &old, UpdatedAt: &old}
	raw, _ := json.Marshal(current)
	var next ClientAccessConfig
	require.NoError(t, json.Unmarshal(raw, &next))
	p := next.Groups["1"]
	p.ClientIDs = []string{"deepseek_harness", "codex"}
	next.Groups["1"] = p
	stampClientPolicyDates(&next, &current, now)
	require.Equal(t, old, *next.Groups["1"].UpdatedAt)
	next.Clients[1].Name = "新名称"
	stampClientPolicyDates(&next, &current, now)
	require.Equal(t, now, *next.Groups["1"].UpdatedAt)
	require.Equal(t, old, *next.Groups["2"].UpdatedAt)
}
