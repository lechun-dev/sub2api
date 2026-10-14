package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type clientAccessTestRepo struct {
	SettingRepository
	mu     sync.Mutex
	raw    string
	exists bool
	err    error
}

func (r *clientAccessTestRepo) GetValue(context.Context, string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return "", r.err
	}
	if !r.exists {
		return "", ErrSettingNotFound
	}
	return r.raw, nil
}
func (r *clientAccessTestRepo) CompareAndSetClientAccess(_ context.Context, old, next string, _ []int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.raw != old {
		return ErrClientAccessConflict
	}
	r.raw, r.exists = next, true
	return nil
}

func TestClientAccessModesAndRecognition(t *testing.T) {
	for _, tc := range []struct {
		name, mode, ua, code string
		ids                  []string
		allow                bool
	}{
		{"default", "off", "unknown", "", nil, true},
		{"codex allowed", "allow", "codex_cli_rs/0.150.0", "", []string{"codex"}, true},
		{"Harness allowed", "allow", "deepseek-harness/1.2.3 (+https://example.test)", "", []string{"deepseek_harness"}, true},
		{"cross client", "allow", "deepseek-harness/1.2.3", "client_not_allowed", []string{"codex"}, false},
		{"unknown whitelist", "allow", "curl/8", "client_unrecognized", []string{"codex"}, false},
		{"missing UA", "allow", "", "client_unrecognized", []string{"codex"}, false},
		{"multi whitelist", "allow", "deepseek-harness/1", "", []string{"codex", "deepseek_harness"}, true},
		{"blacklist", "deny", "deepseek-harness/1", "client_forbidden", []string{"codex", "deepseek_harness"}, false},
		{"unknown blacklist", "deny", "curl/8", "", []string{"codex"}, true},
		{"no substring bypass", "allow", "browser deepseek-harness/1", "client_unrecognized", []string{"deepseek_harness"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultClientAccessConfig()
			cfg.Groups["38"] = GroupClientPolicy{Mode: tc.mode, ClientIDs: tc.ids}
			require.NoError(t, ValidateClientAccessConfig(&cfg))
			h := http.Header{}
			h.Set("User-Agent", tc.ua)
			decision := EvaluateClientAccess(&cfg, 38, h)
			require.Equal(t, tc.allow, decision.Allowed)
			require.Equal(t, tc.code, decision.Code)
			require.True(t, EvaluateClientAccess(&cfg, 99, h).Allowed)
		})
	}
}

func TestClientAccessCustomRulesAndAmbiguousMatches(t *testing.T) {
	cfg := DefaultClientAccessConfig()
	cfg.Clients = append(cfg.Clients, ClientDefinition{ID: "company", Name: "内部工具", Enabled: true, Detector: "headers", Match: "all", Rules: []ClientHeaderRule{{Header: "User-Agent", Operator: "contains", Value: "harness"}, {Header: "X-Company-Client", Operator: "exact", Value: "desktop"}}})
	require.NoError(t, ValidateClientAccessConfig(&cfg))
	h := http.Header{"User-Agent": {"deepseek-harness/1"}, "X-Company-Client": {"desktop"}}
	require.True(t, ClientDefinitionMatches(cfg.Clients[2], h))
	h.Del("X-Company-Client")
	require.False(t, ClientDefinitionMatches(cfg.Clients[2], h))
	cfg.Clients[2].Match = "any"
	require.True(t, ClientDefinitionMatches(cfg.Clients[2], h))
	cfg.Groups["7"] = GroupClientPolicy{Mode: "deny", ClientIDs: []string{"company"}}
	require.False(t, EvaluateClientAccess(&cfg, 7, h).Allowed, "a second matching definition must not bypass the blacklist")
	cfg.Groups["7"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"company"}}
	cfg.Clients[2].Name = "更名工具"
	require.Contains(t, EvaluateClientAccess(&cfg, 7, http.Header{}).Message, "更名工具")
}

func TestClientAccessValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ClientAccessConfig)
	}{
		{"empty selection", func(c *ClientAccessConfig) { c.Groups["1"] = GroupClientPolicy{Mode: "allow"} }},
		{"unknown id", func(c *ClientAccessConfig) {
			c.Groups["1"] = GroupClientPolicy{Mode: "deny", ClientIDs: []string{"missing"}}
		}},
		{"inactive reference", func(c *ClientAccessConfig) {
			c.Clients[0].Enabled = false
			c.Groups["1"] = GroupClientPolicy{Mode: "deny", ClientIDs: []string{"codex"}}
		}},
		{"duplicate id", func(c *ClientAccessConfig) { c.Clients = append(c.Clients, c.Clients[0]) }},
		{"duplicate selection", func(c *ClientAccessConfig) {
			c.Groups["1"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex", "codex"}}
		}},
		{"bad mode", func(c *ClientAccessConfig) { c.Groups["1"] = GroupClientPolicy{Mode: "typo"} }},
		{"invalid group", func(c *ClientAccessConfig) { c.Groups["01"] = GroupClientPolicy{Mode: "off"} }},
		{"no conditions", func(c *ClientAccessConfig) { c.Clients[1].Rules = nil }},
		{"blank condition", func(c *ClientAccessConfig) { c.Clients[1].Rules[0].Value = " " }},
		{"credentials", func(c *ClientAccessConfig) { c.Clients[1].Rules[0].Header = "X-Api-Key" }},
		{"cookie", func(c *ClientAccessConfig) { c.Clients[1].Rules[0].Header = "Cookie" }},
		{"unsupported operator", func(c *ClientAccessConfig) { c.Clients[1].Rules[0].Operator = "regex" }},
		{"builtin changed", func(c *ClientAccessConfig) {
			c.Clients[0].Detector = "headers"
			c.Clients[0].Rules = c.Clients[1].Rules
		}},
		{"off selection", func(c *ClientAccessConfig) {
			c.Groups["1"] = GroupClientPolicy{Mode: "off", ClientIDs: []string{"codex"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultClientAccessConfig()
			tc.change(&cfg)
			require.Error(t, ValidateClientAccessConfig(&cfg))
		})
	}
}

func TestClientAccessPersistenceCacheAndCAS(t *testing.T) {
	repo := &clientAccessTestRepo{}
	svc := NewSettingService(repo, nil)
	cfg, err := svc.GetClientAccessConfig(context.Background())
	require.NoError(t, err)
	require.Empty(t, cfg.Revision)
	stale := *cfg
	_, err = svc.RuntimeClientAccessConfig(context.Background())
	require.NoError(t, err)
	cfg.Groups["38"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex"}}
	require.NoError(t, svc.SaveClientAccessConfig(context.Background(), *cfg))
	require.ErrorIs(t, svc.SaveClientAccessConfig(context.Background(), stale), ErrClientAccessConflict)
	runtime, err := svc.RuntimeClientAccessConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "allow", runtime.Groups["38"].Mode)
	next, err := svc.GetClientAccessConfig(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, next.Revision)
	delete(next.Groups, "38")
	require.NoError(t, svc.SaveClientAccessConfig(context.Background(), *next))
	runtime, err = svc.RuntimeClientAccessConfig(context.Background())
	require.NoError(t, err)
	require.Empty(t, runtime.Groups)
	repo.err = errors.New("db unavailable")
	svc.clientAccessCache.Expires = time.Now().Add(-time.Second)
	_, err = svc.RuntimeClientAccessConfig(context.Background())
	require.Error(t, err)
}

func TestClientAccessCorruptConfigFailsClosed(t *testing.T) {
	for _, raw := range []string{"", "broken", "null", `{"clients":[],"groups":{"1":{"mode":"allow","client_ids":["missing"]}}}`} {
		svc := NewSettingService(&clientAccessTestRepo{raw: raw, exists: true}, nil)
		_, err := svc.RuntimeClientAccessConfig(context.Background())
		require.Error(t, err)
	}
	cfg := DefaultClientAccessConfig()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	svc := NewSettingService(&clientAccessTestRepo{raw: string(raw), exists: true}, nil)
	_, err = svc.RuntimeClientAccessConfig(context.Background())
	require.NoError(t, err)
}
